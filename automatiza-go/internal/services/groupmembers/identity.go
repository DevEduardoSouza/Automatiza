// Package groupmembers implementa a fundação histórica de membros de grupo
// (Melhoria 2, Fase A): ingestão idempotente de fatos group.v2.participants,
// projeção reconciliável e agregado diário.
//
// Este arquivo concentra normalização, validação e pseudonimização. É o ÚNICO
// ponto do sistema onde um identificador bruto de participante (id/pn) existe
// em memória; nada além do HMAC atravessa a fronteira do repositório ou entra
// em log/erro/métrica.
package groupmembers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"time"

	"automatiza-go/internal/models"
)

// minSecretLen é o mínimo exigido de GROUP_MEMBER_HASH_SECRET. Abaixo disso o
// serviço nasce em modo no-op (ver NewService) em vez de derrubar o processo.
const minSecretLen = 32

// Domínios de separação dos HMACs. Prefixo + NUL entre campos: impede que a
// concatenação de user_id + tipo + id de dois participantes diferentes colida.
const (
	participantHashDomain = "group-member-id:v1\x00"
	phoneHashDomain       = "group-member-phone:v1\x00"
)

// providerEventIDPrefix é o prefixo exemplificado no contrato WAHA
// (waha-plus-openapi.json, WebhookGroupV2Participants.id: "evt_01aaa...").
// É opcional: validamos o ULID depois de removê-lo, mas persistimos o ID
// ORIGINAL para manter a rastreabilidade com o provedor.
const providerEventIDPrefix = "evt_"

// ulidLen e ulidAlphabet: ULID minúsculo = 26 chars de Crockford base32
// (sem i, l, o, u). https://github.com/ulid/spec
const (
	ulidLen      = 26
	ulidAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"
)

// msMagnitude separa segundos de milissegundos. O payload usa Unix em SEGUNDOS
// (GroupV2ParticipantsEvent.timestamp, exemplo 1666943582) e o envelope usa
// milissegundos (exemplo 1634567890123) — 1e12 fica entre as duas magnitudes
// por séculos, então serve de discriminador estrutural.
const msMagnitude = 1e12

// Erros de validação. Todos são "malformados": o worker não deve retentar, e
// nenhum deles carrega o identificador bruto que o originou.
var (
	ErrInvalidProviderEventID  = errors.New("group member tracking: malformed provider event id")
	ErrInvalidProviderTime     = errors.New("group member tracking: malformed provider envelope timestamp")
	ErrInvalidPayloadTime      = errors.New("group member tracking: malformed payload timestamp")
	ErrInvalidParticipantID    = errors.New("group member tracking: malformed participant id")
	ErrInvalidRole             = errors.New("group member tracking: malformed participant role")
	ErrInvalidEventType        = errors.New("group member tracking: malformed event type")
	ErrInvalidGroupID          = errors.New("group member tracking: malformed group id")
	ErrInvalidPayloadStructure = errors.New("group member tracking: malformed payload structure")
)

// validRoles é o enum de GroupParticipant.role.
var validRoles = map[string]struct{}{
	"left": {}, "participant": {}, "admin": {}, "superadmin": {},
}

// validEventTypes é o enum de GroupV2ParticipantsEvent.type.
var validEventTypes = map[string]struct{}{
	models.GroupMemberEventJoin:    {},
	models.GroupMemberEventLeave:   {},
	models.GroupMemberEventPromote: {},
	models.GroupMemberEventDemote:  {},
}

// isValidRole reporta se role pertence ao enum do contrato.
func isValidRole(role string) bool {
	_, ok := validRoles[role]
	return ok
}

// isValidEventType reporta se t pertence ao enum do contrato.
func isValidEventType(t string) bool {
	_, ok := validEventTypes[t]
	return ok
}

// ValidateProviderEventID valida o ID original do envelope e o devolve
// inalterado. O prefixo evt_ é removido APENAS para a checagem de formato.
//
// Um envelope sem ID (ou com ID que não é ULID) não pode gerar fato: o ID é a
// chave idempotente por participante, e um fallback interno colidiria dois
// fatos legítimos do mesmo tipo no mesmo segundo.
func ValidateProviderEventID(raw string) (string, error) {
	body := strings.TrimPrefix(raw, providerEventIDPrefix)
	if len(body) != ulidLen {
		return "", ErrInvalidProviderEventID
	}
	for i := 0; i < len(body); i++ {
		if !strings.ContainsRune(ulidAlphabet, rune(body[i])) {
			return "", ErrInvalidProviderEventID
		}
	}
	return raw, nil
}

// ParseEnvelopeTimestamp converte o timestamp (ms) do envelope. Exige
// magnitude de milissegundos: um valor em segundos aqui indicaria que o campo
// não é o que o contrato diz e viraria uma ordem total errada.
func ParseEnvelopeTimestamp(ms int64) (time.Time, error) {
	if ms < int64(msMagnitude) {
		return time.Time{}, ErrInvalidProviderTime
	}
	return time.UnixMilli(ms).UTC(), nil
}

// ParsePayloadTimestamp converte o timestamp do payload, aceitando SEGUNDOS ou
// MILISSEGUNDOS.
//
// O contrato descreve "Unix timestamp" com exemplo em segundos
// (waha-plus-openapi.json, GroupV2ParticipantsEvent.timestamp: 1666943582), mas
// o engine GOWS emite MILISSEGUNDOS em produção — capturado da fila em
// 23/07/2026: payload 1784785154370 contra envelope 1784785154371, 1ms de
// diferença. Rejeitar a magnitude de ms (como o exemplo da doc sugeria)
// descartava 100% dos eventos reais.
//
// 1e12 separa as duas magnitudes sem ambiguidade: como segundos seria o ano
// 33658, como ms é 2001 — nenhum evento de WhatsApp (2009+) cai na zona errada.
func ParsePayloadTimestamp(value float64) (time.Time, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return time.Time{}, ErrInvalidPayloadTime
	}

	// Normaliza para milissegundos com aritmética INTEIRA: dividir o valor em ms
	// por 1000 em float64 e remontar via nanossegundos perdia 1ms por
	// arredondamento (1784785154370 virava ...369), o que envenenaria o
	// desempate da ordem total.
	var ms int64
	if value >= msMagnitude {
		ms = int64(math.Round(value))
	} else {
		ms = int64(math.Round(value * 1000))
	}

	// Absurdo mesmo lido como ms (ano 33658+): não é timestamp.
	if ms >= int64(msMagnitude)*1000 {
		return time.Time{}, ErrInvalidPayloadTime
	}

	return time.UnixMilli(ms).UTC(), nil
}

// splitParticipantID separa "<dígitos>@c.us" ou "<dígitos>@lid" em (dígitos,
// tipo). Exige ao menos um dígito e SÓ dígitos antes do sufixo.
func splitParticipantID(raw string) (bare, idType string, err error) {
	switch {
	case strings.HasSuffix(raw, "@c.us"):
		bare, idType = strings.TrimSuffix(raw, "@c.us"), models.GroupMemberIDTypeCUs
	case strings.HasSuffix(raw, "@lid"):
		bare, idType = strings.TrimSuffix(raw, "@lid"), models.GroupMemberIDTypeLID
	default:
		return "", "", ErrInvalidParticipantID
	}
	if !isDigits(bare) {
		return "", "", ErrInvalidParticipantID
	}
	return bare, idType, nil
}

// isDigits reporta se s tem ao menos um caractere e só dígitos ASCII.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// extractDDD devolve o DDD quando o telefone é ESTRUTURALMENTE brasileiro:
// só dígitos, prefixo 55, comprimento 12 ou 13, e DDD != "00". Não afirma que
// o número está alocado ou ativo — é validação de forma, não de existência.
func extractDDD(digits string) *string {
	if !isDigits(digits) {
		return nil
	}
	if len(digits) != 12 && len(digits) != 13 {
		return nil
	}
	if !strings.HasPrefix(digits, "55") {
		return nil
	}
	ddd := digits[2:4]
	if ddd == "00" {
		return nil
	}
	return &ddd
}

// hashWithDomain devolve o HMAC-SHA256 hex dos campos, separados por NUL.
func hashWithDomain(secret []byte, domain string, parts ...string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(domain))
	for i, p := range parts {
		if i > 0 {
			mac.Write([]byte{0})
		}
		mac.Write([]byte(p))
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// BuildIdentity pseudonimiza um participante bruto.
//
// rawID é o participant.id obrigatório; rawPN é o pn opcional (pode ser "").
// O telefone auxiliar é, nessa ordem: pn válido; o próprio id quando é @c.us;
// ausente para @lid sem pn. O identificador BRUTO nunca sai desta função — nem
// no retorno, nem no erro.
func BuildIdentity(secret []byte, userID, rawID, rawPN string) (models.GroupMemberIdentity, error) {
	bare, idType, err := splitParticipantID(rawID)
	if err != nil {
		return models.GroupMemberIdentity{}, err
	}

	identity := models.GroupMemberIdentity{
		ParticipantHash: hashWithDomain(secret, participantHashDomain, userID, idType, bare),
		IDType:          idType,
	}

	// Telefone auxiliar: pn tem precedência; @c.us usa o próprio id; @lid sem
	// pn fica sem telefone (aplicar ExtractPhone a um LID fabricaria DDD).
	phoneDigits := ""
	if rawPN != "" {
		pnBare, pnType, pnErr := splitParticipantID(rawPN)
		if pnErr == nil && pnType == models.GroupMemberIDTypeCUs {
			phoneDigits = pnBare
		}
	}
	if phoneDigits == "" && idType == models.GroupMemberIDTypeCUs {
		phoneDigits = bare
	}
	if phoneDigits != "" {
		ph := hashWithDomain(secret, phoneHashDomain, userID, phoneDigits)
		identity.PhoneHash = &ph
		identity.DDD = extractDDD(phoneDigits)
	}

	return identity, nil
}
