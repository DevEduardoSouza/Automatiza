package groupmembers

import (
	"math"
	"strings"
	"testing"

	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Todos os identificadores destes testes são SINTÉTICOS e declaradamente NÃO
// reais: "5511000000000" e "14155550000" usam faixas reservadas/nulas e os LIDs
// são inventados. Nenhum telefone de cliente entra em fixture.
const (
	synthUserA = "11111111-1111-4111-8111-111111111111"
	synthUserB = "22222222-2222-4222-8222-222222222222"

	synthPhoneBR    = "5511000000000"
	synthPhoneNonBR = "14155550000"
	synthLID        = "999000111222"
)

var synthSecret = []byte(strings.Repeat("s", minSecretLen))

func TestBuildIdentity_CUsDerivesPhoneAndDDD(t *testing.T) {
	id, err := BuildIdentity(synthSecret, synthUserA, synthPhoneBR+"@c.us", "")
	require.NoError(t, err)

	assert.Equal(t, models.GroupMemberIDTypeCUs, id.IDType)
	assert.Len(t, id.ParticipantHash, 64)
	require.NotNil(t, id.PhoneHash, "@c.us é telefone: phone_hash deve existir")
	assert.Len(t, *id.PhoneHash, 64)
	require.NotNil(t, id.DDD)
	assert.Equal(t, "11", *id.DDD)
}

// O ponto central da identidade: o hash primário vem SEMPRE de participant.id.
// Sem isso, um @lid visto primeiro sem pn e depois reconciliado com pn geraria
// duas projeções do mesmo membro.
func TestBuildIdentity_LIDHashIsStableWithAndWithoutPN(t *testing.T) {
	withoutPN, err := BuildIdentity(synthSecret, synthUserA, synthLID+"@lid", "")
	require.NoError(t, err)
	withPN, err := BuildIdentity(synthSecret, synthUserA, synthLID+"@lid", synthPhoneBR+"@c.us")
	require.NoError(t, err)

	assert.Equal(t, withoutPN.ParticipantHash, withPN.ParticipantHash,
		"hash primário do LID deve ser idêntico com e sem pn")
	assert.Equal(t, models.GroupMemberIDTypeLID, withoutPN.IDType)

	// @lid sem pn não tem telefone: derivar do LID fabricaria phone_hash/DDD.
	assert.Nil(t, withoutPN.PhoneHash)
	assert.Nil(t, withoutPN.DDD)

	// Com pn, o telefone auxiliar aparece — sem mudar a identidade.
	require.NotNil(t, withPN.PhoneHash)
	require.NotNil(t, withPN.DDD)
	assert.Equal(t, "11", *withPN.DDD)
}

func TestBuildIdentity_NonBrazilianPhoneHasNoDDD(t *testing.T) {
	id, err := BuildIdentity(synthSecret, synthUserA, synthPhoneNonBR+"@c.us", "")
	require.NoError(t, err)

	require.NotNil(t, id.PhoneHash, "telefone existe, só não é brasileiro")
	assert.Nil(t, id.DDD, "sem prefixo 55/comprimento BR não há DDD")
}

func TestBuildIdentity_IsolatedPerTenant(t *testing.T) {
	a, err := BuildIdentity(synthSecret, synthUserA, synthPhoneBR+"@c.us", "")
	require.NoError(t, err)
	b, err := BuildIdentity(synthSecret, synthUserB, synthPhoneBR+"@c.us", "")
	require.NoError(t, err)

	assert.NotEqual(t, a.ParticipantHash, b.ParticipantHash,
		"o mesmo número em dois tenants não pode ser correlacionável")
	assert.NotEqual(t, *a.PhoneHash, *b.PhoneHash)
}

func TestBuildIdentity_Deterministic(t *testing.T) {
	first, err := BuildIdentity(synthSecret, synthUserA, synthPhoneBR+"@c.us", "")
	require.NoError(t, err)
	second, err := BuildIdentity(synthSecret, synthUserA, synthPhoneBR+"@c.us", "")
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestBuildIdentity_RejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"sem sufixo":            synthPhoneBR,
		"sufixo desconhecido":   synthPhoneBR + "@s.whatsapp.net",
		"grupo":                 "120000000000000000@g.us",
		"não dígitos":           "abc" + "@c.us",
		"vazio antes do sufixo": "@lid",
		"string vazia":          "",
	}

	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := BuildIdentity(synthSecret, synthUserA, raw, "")
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidParticipantID)
			// O bruto NUNCA pode vazar na mensagem de erro.
			if raw != "" {
				assert.NotContains(t, err.Error(), raw)
			}
		})
	}
}

// pn inválido não invalida o participante: a identidade primária é o id.
func TestBuildIdentity_InvalidPNFallsBackToID(t *testing.T) {
	id, err := BuildIdentity(synthSecret, synthUserA, synthPhoneBR+"@c.us", "lixo")
	require.NoError(t, err)
	require.NotNil(t, id.PhoneHash)

	direct, err := BuildIdentity(synthSecret, synthUserA, synthPhoneBR+"@c.us", "")
	require.NoError(t, err)
	assert.Equal(t, *direct.PhoneHash, *id.PhoneHash)
}

func TestBuildIdentity_LIDWithInvalidPNStaysWithoutPhone(t *testing.T) {
	id, err := BuildIdentity(synthSecret, synthUserA, synthLID+"@lid", synthLID+"@lid")
	require.NoError(t, err)
	assert.Nil(t, id.PhoneHash, "pn que não é @c.us não é telefone")
	assert.Nil(t, id.DDD)
}

func TestExtractDDD_StructuralRules(t *testing.T) {
	cases := []struct {
		digits string
		want   string // "" = nil
	}{
		{"5511000000000", "11"}, // 13 dígitos, celular
		{"551100000000", "11"},  // 12 dígitos, fixo
		{"5500000000000", ""},   // DDD 00 é inválido
		{"14155550000", ""},     // sem prefixo 55
		{"55110000000000", ""},  // comprimento fora de 12/13
		{"551100000", ""},       // curto demais
		{"55a100000000", ""},    // não dígitos
		{"", ""},
	}

	for _, c := range cases {
		got := extractDDD(c.digits)
		if c.want == "" {
			assert.Nil(t, got, "digits=%s", c.digits)
			continue
		}
		require.NotNil(t, got, "digits=%s", c.digits)
		assert.Equal(t, c.want, *got, "digits=%s", c.digits)
	}
}

func TestValidateProviderEventID(t *testing.T) {
	valid := []string{
		"01aaaaaaaaaaaaaaaaaaaaaaaa",
		"evt_01aaaaaaaaaaaaaaaaaaaaaaaa",
		"01hz9m0q1r2s3t4v5w6x7y8z9a",
	}
	for _, raw := range valid {
		got, err := ValidateProviderEventID(raw)
		require.NoError(t, err, "raw=%s", raw)
		// O ID ORIGINAL é preservado: o prefixo só sai para a checagem.
		assert.Equal(t, raw, got)
	}

	invalid := []string{
		"",
		"evt_",
		"01aaaaaaaaaaaaaaaaaaaaaaa",   // 25 chars
		"01aaaaaaaaaaaaaaaaaaaaaaaaa", // 27 chars
		"01AAAAAAAAAAAAAAAAAAAAAAAA",  // maiúsculo
		"01iaaaaaaaaaaaaaaaaaaaaaaa",  // 'i' fora do alfabeto ULID
		"01uaaaaaaaaaaaaaaaaaaaaaaa",  // 'u' fora do alfabeto ULID
		"01-aaaaaaaaaaaaaaaaaaaaaaa",
		"a1b2:session:group.v2.participants:20260723T000000", // fallback interno
	}
	for _, raw := range invalid {
		_, err := ValidateProviderEventID(raw)
		assert.ErrorIs(t, err, ErrInvalidProviderEventID, "raw=%q", raw)
	}
}

func TestParseEnvelopeTimestamp(t *testing.T) {
	// Exemplo do contrato: 1634567890123 (ms).
	got, err := ParseEnvelopeTimestamp(1634567890123)
	require.NoError(t, err)
	assert.Equal(t, int64(1634567890123), got.UnixMilli())

	// Segundos no envelope = campo errado: recusado.
	_, err = ParseEnvelopeTimestamp(1666943582)
	assert.ErrorIs(t, err, ErrInvalidProviderTime)

	_, err = ParseEnvelopeTimestamp(0)
	assert.ErrorIs(t, err, ErrInvalidProviderTime)
}

func TestParsePayloadTimestamp(t *testing.T) {
	// Exemplo do contrato: 1666943582 (segundos).
	got, err := ParsePayloadTimestamp(1666943582)
	require.NoError(t, err)
	assert.Equal(t, int64(1666943582), got.Unix())

	// Fração de segundo é aceita.
	frac, err := ParsePayloadTimestamp(1666943582.5)
	require.NoError(t, err)
	assert.Equal(t, int64(1666943582), frac.Unix())
	assert.Equal(t, 500_000_000, frac.Nanosecond())

	// MILISSEGUNDOS no payload: é o que o engine GOWS realmente emite.
	// Valor real capturado da fila de produção em 23/07/2026 (o envelope do
	// mesmo evento trazia 1784785154371, 1ms depois).
	gows, err := ParsePayloadTimestamp(1784785154370)
	require.NoError(t, err, "GOWS manda ms no payload; rejeitar descartava 100% dos eventos")
	assert.Equal(t, int64(1784785154370), gows.UnixMilli())

	// A mesma instante em segundos precisa dar o mesmo segundo.
	secs, err := ParsePayloadTimestamp(1784785154)
	require.NoError(t, err)
	assert.Equal(t, gows.Unix(), secs.Unix(), "as duas magnitudes convergem no mesmo instante")

	for _, bad := range []float64{
		0, -1, math.NaN(), math.Inf(1), math.Inf(-1),
		1e15, // absurdo mesmo lido como ms (ano 33658+)
	} {
		_, err := ParsePayloadTimestamp(bad)
		assert.ErrorIs(t, err, ErrInvalidPayloadTime, "valor=%v", bad)
	}
}
