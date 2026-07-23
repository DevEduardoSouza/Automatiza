package failover

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/quota"
	"automatiza-go/internal/models"
)

// ProviderReader é a fatia SOMENTE-LEITURA do WAHA que o failover consome.
//
// Os três métodos abaixo são os únicos que este pacote pode chamar. Em
// particular NÃO existe aqui POST /api/{session}/groups/refresh: apesar do nome
// sugerir leitura, ele é mutável, e um "ensaio" que muda estado do provider não
// é ensaio. A cobertura sai de GET /groups e GET /groups/{id}/participants/v2.
type ProviderReader interface {
	GetSessionStatusFresh(ctx context.Context, sessionName string) (*whatsapp.FreshSessionStatus, error)
	GetGroups(ctx context.Context, sessionName string) ([]whatsapp.GroupInfo, error)
	GetGroupParticipantsForTracking(ctx context.Context, sessionName, groupID string) ([]whatsapp.TrackingParticipant, error)
}

// participantsGap é o intervalo mínimo entre chamadas de participantes. O
// dry-run é sequencial de propósito: uma conta com dezenas de grupos anunciados
// não pode virar uma rajada contra o shard WAHA que atende centenas de sessões.
const participantsGap = 200 * time.Millisecond

// CoverageResult é o que a checagem ao vivo apurou para UM chip alvo.
type CoverageResult struct {
	// SessionOK indica WORKING + identidade própria conhecida.
	SessionOK bool
	// Unavailable indica que NÃO foi possível saber (erro/timeout/payload
	// incompleto). Nunca é tratado como "falta" nem como "cobre".
	Unavailable bool
	Reasons     []string

	// MemberOf são os grupos dos quais o chip participa.
	MemberOf map[string]bool
	// AdminOf são os grupos onde o chip é admin/superadmin.
	AdminOf map[string]bool
	// Announce marca os grupos onde só admin escreve.
	Announce map[string]bool
}

// CoverageChecker faz as leituras ao vivo do WAHA.
type CoverageChecker struct {
	provider ProviderReader
	gap      time.Duration
}

// NewCoverageChecker cria o checador.
func NewCoverageChecker(provider ProviderReader) *CoverageChecker {
	return &CoverageChecker{provider: provider, gap: participantsGap}
}

// Inspect coleta status, grupos e (só onde necessário) participantes.
//
// requiredTargets são os grupos de DESTINO — só neles admin pode ser exigido.
// requiredAll é a união de destinos e origens: o chip precisa participar de
// todos, porque em modo espelho ele também precisa RECEBER do grupo-fonte.
func (c *CoverageChecker) Inspect(ctx context.Context, sessionName string, requiredAll, requiredTargets []string) *CoverageResult {
	res := &CoverageResult{
		MemberOf: map[string]bool{},
		AdminOf:  map[string]bool{},
		Announce: map[string]bool{},
	}

	status, err := c.provider.GetSessionStatusFresh(ctx, sessionName)
	if err != nil {
		res.Unavailable = true
		res.Reasons = []string{models.ReasonProviderUnavailable}
		return res
	}
	if !status.IsWorking() {
		// Inclui o caso "status que não reconhecemos": um estado desconhecido
		// não autoriza uma troca de chip.
		res.Reasons = []string{models.ReasonTargetNotWorking}
		return res
	}
	// Sem me.id não há como identificar o próprio participante na lista, e sem
	// isso não dá para afirmar admin. Bloqueia — nunca infere pelo telefone
	// gravado no banco, que pode estar velho ou pertencer a outra sessão.
	if strings.TrimSpace(status.MeID) == "" {
		res.Reasons = []string{models.ReasonReserveIdentity}
		return res
	}
	res.SessionOK = true

	groups, err := c.provider.GetGroups(ctx, sessionName)
	if err != nil {
		res.Unavailable = true
		res.Reasons = append(res.Reasons, models.ReasonProviderUnavailable)
		return res
	}

	groupByID := make(map[string]*whatsapp.GroupInfo, len(groups))
	for i := range groups {
		g := &groups[i]
		res.MemberOf[g.ID] = true
		res.Announce[g.ID] = g.Announce
		groupByID[g.ID] = g
	}

	// Admin só é buscado onde ele é EXIGIDO: destino que é announce-only.
	needAdmin := make([]string, 0, len(requiredTargets))
	for _, gid := range requiredTargets {
		g, ok := groupByID[gid]
		if !ok || !g.Announce {
			continue
		}
		needAdmin = append(needAdmin, gid)
	}
	sort.Strings(needAdmin)

	meVariants := jidVariants(status.MeID)
	for i, gid := range needAdmin {
		if i > 0 {
			select {
			case <-ctx.Done():
				res.Unavailable = true
				res.Reasons = append(res.Reasons, models.ReasonProviderUnavailable)
				return res
			case <-time.After(c.gap):
			}
		}

		participants, err := c.provider.GetGroupParticipantsForTracking(ctx, sessionName, gid)
		if err != nil {
			res.Unavailable = true
			res.Reasons = append(res.Reasons, models.ReasonProviderUnavailable)
			return res
		}

		matched := false
		for _, p := range participants {
			if !matchesSelf(p, meVariants) {
				continue
			}
			matched = true
			// Enum do OpenAPI: left|participant|admin|superadmin.
			if p.Role == "admin" || p.Role == "superadmin" {
				res.AdminOf[gid] = true
			}
			if p.Role == "left" {
				// A listagem disse que participa e a de membros diz que saiu:
				// a verdade mais recente é a de membros.
				delete(res.MemberOf, gid)
			}
			break
		}
		if !matched {
			// A sessão não se encontra na lista do próprio grupo: não dá para
			// afirmar nem participação nem admin.
			res.Unavailable = true
			res.Reasons = append(res.Reasons, models.ReasonReserveIdentity)
			return res
		}
	}

	res.Reasons = dedupeSorted(res.Reasons)
	return res
}

// jidVariants devolve as formas comparáveis de um JID: o valor cru, sem o
// sufixo de dispositivo (":12") e só a parte local.
func jidVariants(jid string) map[string]struct{} {
	out := map[string]struct{}{}
	jid = strings.TrimSpace(jid)
	if jid == "" {
		return out
	}
	out[jid] = struct{}{}

	local, domain := jid, ""
	if at := strings.Index(jid, "@"); at >= 0 {
		local, domain = jid[:at], jid[at+1:]
	}
	if colon := strings.Index(local, ":"); colon >= 0 {
		local = local[:colon]
	}
	out[local] = struct{}{}
	if domain != "" {
		out[local+"@"+domain] = struct{}{}
	}
	return out
}

// matchesSelf compara um participante com a identidade da sessão, por `id` e —
// quando o WAHA o fornece — por `pn`. Nome, pushname e telefone do banco NÃO
// participam: nenhum deles identifica uma sessão de forma confiável.
func matchesSelf(p whatsapp.TrackingParticipant, me map[string]struct{}) bool {
	for _, candidate := range []string{p.ID, p.PN} {
		if candidate == "" {
			continue
		}
		for v := range jidVariants(candidate) {
			if _, ok := me[v]; ok {
				return true
			}
		}
	}
	return false
}

// ItemCoverage é o veredito de cobertura de UMA automação.
type ItemCoverage struct {
	RequiredTargets []string
	RequiredSources []string
	AdminRequired   []string
	MissingGroups   []string
	MissingAdmin    []string
	Reasons         []string
	Result          string
}

// EvaluateItem cruza o que a automação precisa com o que o chip alvo tem.
func EvaluateItem(item *EligibleItem, cov *CoverageResult) ItemCoverage {
	out := ItemCoverage{
		RequiredTargets: append([]string(nil), item.After.TargetGroups()...),
		RequiredSources: append([]string(nil), item.After.SourceGroups()...),
	}

	if cov.Unavailable || !cov.SessionOK {
		out.Reasons = dedupeSorted(append(append([]string(nil), cov.Reasons...), item.Reasons...))
		out.Result = models.FailoverCoverageBlocked
		return out
	}

	var reasons []string
	reasons = append(reasons, item.Reasons...)

	// Participação é exigida na UNIÃO: no espelho, deixar de RECEBER do
	// grupo-fonte mata a automação tão certo quanto não conseguir enviar.
	for _, gid := range append(append([]string(nil), out.RequiredTargets...), out.RequiredSources...) {
		if !cov.MemberOf[gid] {
			out.MissingGroups = append(out.MissingGroups, gid)
		}
	}
	// Admin é exigido SÓ nos destinos announce-only.
	for _, gid := range out.RequiredTargets {
		if !cov.Announce[gid] {
			continue
		}
		out.AdminRequired = append(out.AdminRequired, gid)
		if !cov.AdminOf[gid] {
			out.MissingAdmin = append(out.MissingAdmin, gid)
		}
	}

	if len(out.MissingGroups) > 0 {
		reasons = append(reasons, models.ReasonGroupMissing)
	}
	if len(out.MissingAdmin) > 0 {
		reasons = append(reasons, models.ReasonAdminMissing)
	}

	// Capacidade: conservadoramente, TODOS os grupos que a reserva precisa
	// provar contam para ela. Destinos e origens contam separadamente, como a
	// validação de quota já faz por automação. O cap vale mesmo com
	// FEATURE_ENFORCE_GROUP_LIMITS desligada: a flag existe para não punir
	// configuração legada existente, não para autorizar criar uma nova.
	if err := quota.CheckAutomationGroupsPerInstance(map[int]int{1: len(out.RequiredTargets)}); err != nil {
		reasons = append(reasons, models.ReasonCapacityExceeded)
	}
	if err := quota.CheckAutomationGroupsPerInstance(map[int]int{1: len(out.RequiredSources)}); err != nil {
		reasons = append(reasons, models.ReasonCapacityExceeded)
	}

	out.MissingGroups = dedupeSorted(out.MissingGroups)
	out.MissingAdmin = dedupeSorted(out.MissingAdmin)
	out.AdminRequired = dedupeSorted(out.AdminRequired)
	out.Reasons = dedupeSorted(reasons)
	out.Result = models.FailoverCoverageCovered
	if len(out.Reasons) > 0 {
		out.Result = models.FailoverCoverageBlocked
	}
	return out
}

// CoverageHash resume o relatório de cobertura para entrar no dry_run_ref.
// Dois ensaios com a mesma cobertura produzem o mesmo hash; qualquer grupo que
// entre ou saia muda a ref e invalida a confirmação pendente.
func CoverageHash(items map[string]ItemCoverage) string {
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("v1\n")
	for _, k := range keys {
		it := items[k]
		b.WriteString(k)
		b.WriteString("|t:" + strings.Join(it.RequiredTargets, ","))
		b.WriteString("|s:" + strings.Join(it.RequiredSources, ","))
		b.WriteString("|a:" + strings.Join(it.AdminRequired, ","))
		b.WriteString("|mg:" + strings.Join(it.MissingGroups, ","))
		b.WriteString("|ma:" + strings.Join(it.MissingAdmin, ","))
		b.WriteString("|r:" + strings.Join(it.Reasons, ","))
		b.WriteString("|" + it.Result + "\n")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
