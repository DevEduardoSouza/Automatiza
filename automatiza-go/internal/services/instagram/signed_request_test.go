package instagram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Segredo obviamente sintético — nada aqui pode parecer credencial real.
const testAppSecret = "segredo-de-teste-nao-e-real-0000"

// makeSignedRequest monta um signed_request válido do mesmo jeito que a Meta.
func makeSignedRequest(t *testing.T, secret string, payload map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	encPayload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) + "." + encPayload
}

func validPayload(now time.Time) map[string]any {
	return map[string]any{
		"algorithm": "HMAC-SHA256",
		"user_id":   "APP-SCOPED-123",
		"issued_at": now.Unix(),
	}
}

func TestParseSignedRequest_AcceptsValid(t *testing.T) {
	now := time.Now()
	raw := makeSignedRequest(t, testAppSecret, validPayload(now))

	p, err := ParseSignedRequest(raw, testAppSecret, now)
	if err != nil {
		t.Fatalf("assinatura válida deveria passar: %v", err)
	}
	if p.UserID != "APP-SCOPED-123" {
		t.Errorf("user_id = %q", p.UserID)
	}
}

// Cada um destes casos é uma forma de chamada forjada. O endpoint é público e
// apaga dados de cliente: aceitar qualquer um deles é perda de conta alheia.
func TestParseSignedRequest_RejectsForgeries(t *testing.T) {
	now := time.Now()

	cases := map[string]string{
		"vazio":               "",
		"sem ponto":           "apenasumsegmento",
		"assinatura vazia":    "." + base64.RawURLEncoding.EncodeToString([]byte(`{"algorithm":"HMAC-SHA256","user_id":"x"}`)),
		"payload vazio":       "abc.",
		"pontos demais":       makeSignedRequest(t, testAppSecret, validPayload(now)) + ".extra",
		"base64 inválido":     "!!!.###",
		"segredo errado":      makeSignedRequest(t, "outro-segredo-de-teste-000000000", validPayload(now)),
		"algoritmo diferente": makeSignedRequest(t, testAppSecret, map[string]any{"algorithm": "none", "user_id": "x"}),
		"sem user_id":         makeSignedRequest(t, testAppSecret, map[string]any{"algorithm": "HMAC-SHA256", "user_id": ""}),
		"payload não-JSON": base64.RawURLEncoding.EncodeToString([]byte("sig")) + "." +
			base64.RawURLEncoding.EncodeToString([]byte("nao-e-json")),
		"issued_at muito no futuro": makeSignedRequest(t, testAppSecret, map[string]any{
			"algorithm": "HMAC-SHA256", "user_id": "x", "issued_at": now.Add(time.Hour).Unix(),
		}),
		"expirado": makeSignedRequest(t, testAppSecret, map[string]any{
			"algorithm": "HMAC-SHA256", "user_id": "x", "issued_at": now.Add(-2 * time.Hour).Unix(),
			"expires": now.Add(-time.Hour).Unix(),
		}),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSignedRequest(raw, testAppSecret, now); err == nil {
				t.Fatal("deveria ter sido recusado")
			}
		})
	}
}

// Sem app secret configurado não há como verificar nada — recusar é a única
// resposta segura.
func TestParseSignedRequest_RequiresAppSecret(t *testing.T) {
	now := time.Now()
	raw := makeSignedRequest(t, testAppSecret, validPayload(now))
	if _, err := ParseSignedRequest(raw, "", now); err == nil {
		t.Fatal("sem app secret deveria recusar")
	}
}

// Uma pequena folga para o futuro é aceita (relógios desalinhados), mas não uma
// data arbitrária.
func TestParseSignedRequest_ToleratesSmallClockSkew(t *testing.T) {
	now := time.Now()
	raw := makeSignedRequest(t, testAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "x", "issued_at": now.Add(2 * time.Minute).Unix(),
	})
	if _, err := ParseSignedRequest(raw, testAppSecret, now); err != nil {
		t.Fatalf("2min de desvio deveria ser tolerado: %v", err)
	}
}

// A causa exata da rejeição não pode vazar: seria um oráculo para quem sonda.
func TestParseSignedRequest_ErrorIsUniform(t *testing.T) {
	now := time.Now()
	_, errBadSig := ParseSignedRequest(makeSignedRequest(t, "outro-segredo-de-teste-000000000", validPayload(now)), testAppSecret, now)
	_, errBadFormat := ParseSignedRequest("lixo", testAppSecret, now)

	if errBadSig == nil || errBadFormat == nil {
		t.Fatal("ambos deveriam falhar")
	}
	if errBadSig.Error() != errBadFormat.Error() {
		t.Errorf("mensagens distintas viram oráculo: %q vs %q", errBadSig, errBadFormat)
	}
	if strings.Contains(errBadSig.Error(), testAppSecret) {
		t.Error("erro não pode conter o segredo")
	}
}
