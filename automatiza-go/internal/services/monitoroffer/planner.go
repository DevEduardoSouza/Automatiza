// Package monitoroffer decide O QUE uma mensagem-fonte deve produzir: no
// máximo uma oferta, no máximo um aviso de campanha agregado, e no máximo um
// relay de conteúdo comum.
//
// O planner é PURO: recebe o corpo e um resolvedor injetado, devolve um plano
// imutável. Não conhece WAHA, chip, destino nem banco. É isso que torna a
// cardinalidade — a parte que o cliente enxerga quando algo sai errado —
// testável sem fake de provider.
//
// A regra que governa o arquivo inteiro: **ausência de dado nunca vira
// campanha**. Timeout, 429, HTML mudado e preview incompleto são `unknown`, e
// unknown retém o parágrafo até um retry resolver. O contrário — inferir
// "então é cupom" — é o que hoje manda aviso de cupom falso ao grupo do
// cliente.
package monitoroffer

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
)

// LinkSlot é uma URL comercial encontrada no corpo, com sua posição.
//
// Ordinal é a posição da PRIMEIRA ocorrência (o texto pode repetir a mesma
// URL); ParagraphOrdinal é o bloco em que ela vive, e é ele que decide se o
// parágrafo inteiro espera por um link pendente.
type LinkSlot struct {
	Ordinal          int16
	ParagraphOrdinal int16
	Raw              string
	URL              commerceurl.URL
	Local            commerceurl.Classification
	// LocalErr guarda o bloqueio de segurança quando o Parse recusou a URL.
	LocalErr *commerceurl.Error
}

// ResolveRequest é o que o planner pede ao resolvedor para um link que a
// classificação local não fechou, ou que precisa virar link do dono.
type ResolveRequest struct {
	URL    commerceurl.URL
	Local  commerceurl.Classification
	UserID string
	// FromMe autoriza a única exceção de passthrough (§5.4.7): URL ML validada
	// postada pelo PRÓPRIO dono. Post de terceiro sempre reatribui.
	FromMe bool
}

// ResolveResult é o veredito final de um link.
type ResolveResult struct {
	Class      string // models.URLClass*
	ErrorClass string // models.ErrorClass*
	Reason     string // models.Reason*
	// CanonicalKey identifica o produto ("shopee:<shop>:<item>", "ml:<MLB>").
	CanonicalKey string
	// ConvertedURL é o link DO TENANT. Vazio significa que nada pode sair —
	// nunca preencher com a URL de origem.
	ConvertedURL string
	Outcome      string // models.MonitorOutcome*
	// Offer carrega os dados do produto para o template. nil em campanha.
	Offer *OfferData
}

// OfferData é o mínimo que o renderer de oferta precisa. Deliberadamente sem
// URL de origem: o que sai para o grupo é sempre ConvertedURL.
type OfferData struct {
	Store         string
	Title         string
	Price         float64
	OriginalPrice float64
	ImageURL      string
	IsOfficial    bool
	CategoryIDs   []int64
}

// LinkResolver faz o I/O que o planner não faz. Uma implementação real fala
// com Shopee/ML; o teste passa um fake e prova a cardinalidade sem rede.
type LinkResolver interface {
	Resolve(ctx context.Context, in ResolveRequest) ResolveResult
}

// ItemResult é o desfecho de um link, pronto para virar linha no ledger.
type ItemResult struct {
	Slot         LinkSlot
	Class        string
	ErrorClass   string
	Reason       string
	Status       string
	Outcome      string
	CanonicalKey string
	ConvertedURL string
	OutputKey    string
	OutputKind   string
	// Offer só é preenchido quando Class == product.
	Offer *OfferData
}

// Output é uma saída planejada.
type Output struct {
	Kind string // models.OutputKind*
	Key  string
	Body string
	// LeaderOrdinal é o item que responde pela entrega desta saída. Só ele
	// pode ficar pending por falha de envio — sem isso, um aviso agregando
	// cinco links geraria cinco retries do MESMO aviso.
	LeaderOrdinal int16
	Offer         *OfferData
}

// PlanInput é a entrada do planner.
type PlanInput struct {
	// Body é o corpo JÁ sem {story} (removido no webhook original).
	Body   string
	FromMe bool
	UserID string
	// CTA entra uma única vez no aviso agregado.
	CTA string
	// HasMedia habilita relay de conteúdo comum sem texto.
	HasMedia bool
	// MirrorCommon libera o relay de conteúdo comum (gate MONITOR_MIRROR_CONTENT).
	MirrorCommon bool
}

// Plan é o resultado imutável.
type Plan struct {
	Offer    *Output
	Campaign *Output
	Relay    *Output
	Items    []ItemResult
	// BlockedReason != "" significa que a mensagem INTEIRA foi recusada antes
	// de qualquer I/O (too_many_links). Nada sai, nada é chamado.
	BlockedReason string
}

// Outputs devolve as saídas na ordem de envio: oferta → campanha → relay.
func (p Plan) Outputs() []*Output {
	var out []*Output
	for _, o := range []*Output{p.Offer, p.Campaign, p.Relay} {
		if o != nil {
			out = append(out, o)
		}
	}
	return out
}

// maxConcurrentResolves é o teto de resoluções simultâneas (§5.2). Baixo de
// propósito: uma mensagem com 20 links não pode abrir 20 conexões e consumir a
// cota 10030 do usuário de uma vez.
const maxConcurrentResolves = 4

// ClassifyBody encontra os links comerciais do corpo e os classifica
// LOCALMENTE (sem I/O).
//
// Duas decisões estruturais:
//
//   - `info[...]`, `cta[...]` e `desc[...]` saem da superfície de
//     classificação antes da extração. O caso canônico é um cupom dentro de
//     `info[...]`: sem isso ele viraria um link comercial concorrendo com o
//     produto real da mensagem.
//   - links non_commerce NÃO contam para o teto de 20 e nunca provocam I/O.
//     Contá-los faria uma mensagem com muitos links de Instagram bloquear.
func ClassifyBody(body string) ([]LinkSlot, string) {
	surface := text.StripCTATag(text.StripInfoTag(body))
	surface = stripDescTag(surface)

	paragraphs := text.SplitParagraphs(surface)
	paragraphOf := func(raw string) int16 {
		for i, p := range paragraphs {
			if strings.Contains(p, raw) {
				return int16(i)
			}
		}
		return 0
	}

	var slots []LinkSlot
	var ordinal int16
	for _, raw := range text.ExtractLinks(surface) {
		u, err := commerceurl.Parse(raw)
		if err != nil {
			cErr, _ := commerceurl.AsError(err)
			// Bloqueio de segurança CONTA como comercial: ele precisa de uma
			// linha visível no ledger e a URL precisa sair do corpo público.
			slots = append(slots, LinkSlot{
				Ordinal:          ordinal,
				ParagraphOrdinal: paragraphOf(raw),
				Raw:              raw,
				LocalErr:         cErr,
			})
			ordinal++
			continue
		}
		if u.Store == "" {
			// non_commerce: relaya como está, sem I/O e sem ocupar cota.
			continue
		}
		slots = append(slots, LinkSlot{
			Ordinal:          ordinal,
			ParagraphOrdinal: paragraphOf(raw),
			Raw:              raw,
			URL:              u,
			Local:            commerceurl.ClassifyLocal(u),
		})
		ordinal++
	}

	if len(slots) > models.MaxCommercialLinks {
		// Mensagem inteira bloqueada: representar input ilimitado em linhas é
		// pior que recusar, e o job faz ZERO I/O e ZERO chamada WAHA.
		return nil, models.ReasonTooManyLinks
	}
	return slots, ""
}

// descTagRegex casa desc[...]. Lida como metadado e removida do corpo público,
// igual a info/cta.
var descTagRegex = regexp.MustCompile(`(?i)desc\[[^\]]*\]`)

func stripDescTag(body string) string {
	return descTagRegex.ReplaceAllString(body, "")
}

// PlanMessage produz o plano completo.
func PlanMessage(ctx context.Context, in PlanInput, slots []LinkSlot, r LinkResolver) Plan {
	results := resolveAll(ctx, in, slots, r)
	results = dedupe(results)
	results = pickSingleProduct(results)

	plan := Plan{Items: results}
	buildOutputs(&plan, in, results)
	return plan
}

// resolveAll roda os resolvedores com concorrência limitada, preservando a
// ORDEM original — a ordem é a identidade do "primeiro produto".
func resolveAll(ctx context.Context, in PlanInput, slots []LinkSlot, r LinkResolver) []ItemResult {
	out := make([]ItemResult, len(slots))
	sem := make(chan struct{}, maxConcurrentResolves)
	var wg sync.WaitGroup

	for i := range slots {
		slot := slots[i]

		// Bloqueio de segurança: terminal, sem I/O.
		if slot.LocalErr != nil {
			out[i] = ItemResult{
				Slot: slot, Class: models.URLClassUnknown,
				ErrorClass: slot.LocalErr.Class, Reason: slot.LocalErr.Reason,
				Status: models.MonitorItemBlocked, Outcome: models.MonitorOutcomeRemoved,
			}
			continue
		}

		// Loja sem conversor: link some do corpo, entorno pode ser relayado.
		if slot.Local.Class == models.URLClassUnsupportedCommerce {
			out[i] = ItemResult{
				Slot: slot, Class: models.URLClassUnsupportedCommerce,
				ErrorClass: models.ErrorClassNone, Reason: models.ReasonUnsupportedStore,
				Status: models.MonitorItemBlocked, Outcome: models.MonitorOutcomeRemoved,
			}
			continue
		}

		// permanent_content local (/cart): a URL não volta.
		if slot.Local.ErrorClass == models.ErrorClassPermanentConte {
			out[i] = ItemResult{
				Slot: slot, Class: slot.Local.Class,
				ErrorClass: slot.Local.ErrorClass, Reason: slot.Local.Reason,
				Status: models.MonitorItemBlocked, Outcome: models.MonitorOutcomeRemoved,
			}
			continue
		}

		wg.Add(1)
		go func(idx int, s LinkSlot) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := r.Resolve(ctx, ResolveRequest{
				URL: s.URL, Local: s.Local, UserID: in.UserID, FromMe: in.FromMe,
			})
			out[idx] = itemFromResolve(s, res)
		}(i, slot)
	}
	wg.Wait()
	return out
}

// itemFromResolve traduz o veredito do resolvedor em estado de item.
func itemFromResolve(s LinkSlot, res ResolveResult) ItemResult {
	it := ItemResult{
		Slot: s, Class: res.Class, ErrorClass: res.ErrorClass, Reason: res.Reason,
		CanonicalKey: res.CanonicalKey, ConvertedURL: res.ConvertedURL,
		Outcome: res.Outcome,
	}

	switch res.ErrorClass {
	case models.ErrorClassTransient:
		// Retém o parágrafo e volta num retry. NUNCA vira campanha.
		it.Status = models.MonitorItemPending
		it.Outcome = models.MonitorOutcomeNone
	case models.ErrorClassPermanentConfig, models.ErrorClassPermanentConte, models.ErrorClassSecurityBlock:
		it.Status = models.MonitorItemBlocked
		it.Outcome = models.MonitorOutcomeRemoved
	default:
		if res.ConvertedURL == "" {
			// Sem link do tenant não há o que emitir. Emitir a origem aqui
			// seria repassar a comissão de terceiro.
			it.Status = models.MonitorItemPending
			it.ErrorClass = models.ErrorClassTransient
			if it.Reason == models.ReasonNone || it.Reason == "" {
				it.Reason = models.ReasonPreviewIncomplete
			}
			it.Outcome = models.MonitorOutcomeNone
			break
		}
		it.Status = models.MonitorItemSucceeded
		if it.Outcome == "" {
			it.Outcome = models.MonitorOutcomeConverted
		}
	}
	it.Offer = res.Offer
	return it
}

// dedupe funde ocorrências do mesmo produto e do mesmo link não resolvido.
//
// A primeira ocorrência (menor ordinal) vence; as demais viram
// superseded/duplicate_link SEM nova chamada externa e SEM novo envio.
func dedupe(items []ItemResult) []ItemResult {
	seenKey := make(map[string]int)
	seenHash := make(map[string]int)

	for i := range items {
		it := &items[i]
		if it.Status == models.MonitorItemSuperseded {
			continue
		}
		var bucket map[string]int
		var key string
		if it.CanonicalKey != "" {
			bucket, key = seenKey, it.CanonicalKey
		} else {
			// Unknown ainda não tem identidade de produto: deduplica pelo hash
			// da URL de origem, senão o mesmo short link repetido geraria N
			// retries idênticos.
			bucket, key = seenHash, commerceurl.SourceHash(it.Slot.Raw)
		}
		if _, dup := bucket[key]; dup {
			it.Status = models.MonitorItemSuperseded
			it.Reason = models.ReasonDuplicateLink
			it.Outcome = models.MonitorOutcomeRemoved
			continue
		}
		bucket[key] = i
	}
	return items
}

// pickSingleProduct aplica a cardinalidade: no máximo UMA oferta por mensagem.
//
// O escolhido é o PRIMEIRO produto confirmado na ordem do texto. Um unknown
// anterior NÃO bloqueia um produto posterior — era exatamente esse bloqueio que
// fazia um cupom Shopee (unknown até o generateShortLink) roubar a oferta de um
// produto ML que vinha depois.
func pickSingleProduct(items []ItemResult) []ItemResult {
	chosen := -1
	for i := range items {
		it := &items[i]
		if it.Class != models.URLClassProduct || it.Status != models.MonitorItemSucceeded {
			continue
		}
		if chosen < 0 {
			chosen = i
			continue
		}
		it.Status = models.MonitorItemSuperseded
		it.Reason = models.ReasonExcessProduct
		it.Outcome = models.MonitorOutcomeRemoved
	}
	return items
}

// buildOutputs monta oferta, aviso agregado e relay comum.
func buildOutputs(plan *Plan, in PlanInput, items []ItemResult) {
	// --- oferta ---
	for i := range items {
		it := &items[i]
		if it.Class == models.URLClassProduct && it.Status == models.MonitorItemSucceeded {
			key := "product:" + it.CanonicalKey
			it.OutputKey, it.OutputKind = key, models.OutputKindProductOffer
			plan.Offer = &Output{
				Kind: models.OutputKindProductOffer, Key: key,
				LeaderOrdinal: it.Slot.Ordinal, Offer: it.Offer,
			}
			break
		}
	}

	// --- aviso de campanha agregado ---
	var campaignIdx []int
	for i := range items {
		if items[i].Class == models.URLClassCampaignOrCoupon &&
			items[i].Status == models.MonitorItemSucceeded &&
			items[i].ConvertedURL != "" {
			campaignIdx = append(campaignIdx, i)
		}
	}
	if len(campaignIdx) > 0 {
		body := renderCampaignBody(in, items, campaignIdx)
		if strings.TrimSpace(body) != "" {
			keys := make([]string, 0, len(campaignIdx))
			for _, i := range campaignIdx {
				keys = append(keys, items[i].ConvertedURL)
			}
			sort.Strings(keys)
			key := "campaign:" + commerceurl.SourceHash(strings.Join(keys, "|"))

			leader := items[campaignIdx[0]].Slot.Ordinal
			for _, i := range campaignIdx {
				items[i].OutputKey = key
				items[i].OutputKind = models.OutputKindCampaignNotice
				if items[i].Slot.Ordinal < leader {
					leader = items[i].Slot.Ordinal
				}
				// Não-líderes já estão entregues pelo agregado.
				items[i].Outcome = models.MonitorOutcomeIncluded
			}
			plan.Campaign = &Output{
				Kind: models.OutputKindCampaignNotice, Key: key,
				Body: body, LeaderOrdinal: leader,
			}
		}
	}

	// --- relay comum ---
	// Só existe quando NÃO há oferta nem campanha: senão o corpo sairia duas
	// vezes no mesmo grupo.
	if plan.Offer == nil && plan.Campaign == nil && in.MirrorCommon {
		body := renderCommonRelayBody(in, items)
		if strings.TrimSpace(body) != "" || in.HasMedia {
			plan.Relay = &Output{
				Kind: models.OutputKindCommonRelay,
				Key:  "relay:" + commerceurl.SourceHash(in.UserID+"|"+in.Body),
				Body: body,
			}
		}
	}
}

// renderCampaignBody monta o corpo do aviso.
//
// Regra dos parágrafos (§6.7): um bloco que contém ao menos um link PENDING
// espera inteiro — se ele saísse agora, o link pendente teria de ser removido
// e o parágrafo reapareceria mutilado quando o retry resolvesse.
func renderCampaignBody(in PlanInput, items []ItemResult, campaignIdx []int) string {
	pendingParagraphs := make(map[int16]bool)
	for i := range items {
		if items[i].Status == models.MonitorItemPending {
			pendingParagraphs[items[i].Slot.ParagraphOrdinal] = true
		}
	}

	var needles []string
	stores := make(map[string]struct{})
	for _, i := range campaignIdx {
		if pendingParagraphs[items[i].Slot.ParagraphOrdinal] {
			continue
		}
		needles = append(needles, items[i].ConvertedURL)
		stores[items[i].Slot.URL.Store] = struct{}{}
	}
	if len(needles) == 0 {
		return ""
	}

	body := publicBody(in.Body, items)
	aggregated := text.SelectParagraphsContaining(body, needles)
	if strings.TrimSpace(aggregated) == "" {
		return ""
	}

	// Uma loja só → renderer existente (preserva rótulo e rodapé de sempre).
	// Duas lojas → renderer misto, que aplica CTA e rodapé UMA vez.
	if len(stores) == 1 {
		for s := range stores {
			if s == models.MonitorStoreMercadoLivr {
				return text.RenderMLCouponNotice(aggregated)
			}
			return text.RenderShopeeCouponNotice(aggregated, in.CTA)
		}
	}
	return text.RenderMixedCampaignNotice(aggregated, in.CTA)
}

// renderCommonRelayBody monta o corpo do relay de conteúdo comum.
func renderCommonRelayBody(in PlanInput, items []ItemResult) string {
	body := publicBody(in.Body, items)
	// Bloco que ainda espera link pendente não sai nesta passagem.
	var keep []string
	for pi, p := range text.SplitParagraphs(body) {
		blocked := false
		for i := range items {
			if items[i].Status == models.MonitorItemPending && int(items[i].Slot.ParagraphOrdinal) == pi {
				blocked = true
				break
			}
		}
		if !blocked && strings.TrimSpace(p) != "" {
			keep = append(keep, p)
		}
	}
	return strings.Join(keep, "\n\n")
}

// publicBody constrói o corpo que pode ir ao grupo.
//
// Três coisas somem aqui e não voltam: as tags de metadado (info/cta/desc), as
// URLs bloqueadas/superseded (substituídas por vazio, não por "[link
// removido]" — texto técnico no grupo do cliente é ruído) e o tracking da
// origem (trocado pelo link do tenant).
func publicBody(raw string, items []ItemResult) string {
	body := stripDescTag(text.StripCTATag(text.StripInfoTag(raw)))

	repl := make([]text.LinkReplacement, 0, len(items))
	for i := range items {
		it := &items[i]
		switch {
		case it.Status == models.MonitorItemSucceeded && it.ConvertedURL != "":
			repl = append(repl, text.LinkReplacement{Original: it.Slot.Raw, Converted: it.ConvertedURL})
		case it.Status == models.MonitorItemPending:
			// Fica como está: o parágrafo inteiro é retido a montante.
		default:
			// blocked / superseded / exhausted → some.
			repl = append(repl, text.LinkReplacement{Original: it.Slot.Raw, Converted: ""})
		}
	}
	body = text.ReplaceLinksLongestFirst(body, repl)
	return collapseBlankRuns(body)
}

// collapseBlankRuns junta as linhas em branco que sobraram depois de remover
// URLs, para o grupo não receber buracos.
func collapseBlankRuns(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var out []string
	blank := 0
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			blank++
			if blank > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blank = 0
		out = append(out, strings.TrimRight(ln, " \t"))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
