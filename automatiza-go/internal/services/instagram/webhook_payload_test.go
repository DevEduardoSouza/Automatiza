package instagram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

func signBody(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// A assinatura é a ÚNICA autenticação de um endpoint público que grava dados de
// cliente. Cada caso abaixo é uma chamada forjada.
func TestVerifyWebhookSignature(t *testing.T) {
	body := []byte(`{"object":"instagram","entry":[]}`)
	const secret = "segredo-de-teste-nao-e-real-0000"

	if err := VerifyWebhookSignature(body, signBody(body, secret), secret); err != nil {
		t.Fatalf("assinatura válida deveria passar: %v", err)
	}

	cases := map[string]struct {
		body   []byte
		header string
		secret string
	}{
		"segredo errado":     {body, signBody(body, "outro-segredo-de-teste-00000000"), secret},
		"corpo adulterado":   {[]byte(`{"object":"instagram","entry":[{}]}`), signBody(body, secret), secret},
		"sem prefixo sha256": {body, hex.EncodeToString([]byte("x")), secret},
		"header vazio":       {body, "", secret},
		"hex inválido":       {body, "sha256=nao-e-hex", secret},
		"sem app secret":     {body, signBody(body, secret), ""},
		"assinatura curta":   {body, "sha256=ab", secret},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := VerifyWebhookSignature(tc.body, tc.header, tc.secret); !errors.Is(err, ErrInvalidWebhookSignature) {
				t.Fatalf("deveria ser recusado, got %v", err)
			}
		})
	}
}

// Comentário: o caminho principal da Fase 3.
func TestParseWebhook_Comment(t *testing.T) {
	raw := []byte(`{"object":"instagram","entry":[{"id":"IG-DONO","time":1753000000,
		"changes":[{"field":"comments","value":{"id":"COMENT-1","text":"quanto custa?",
		"from":{"id":"VISITANTE-9","username":"fulano"},"media":{"id":"POST-7"}}}]}]}`)

	events, err := ParseWebhook(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("esperado 1 evento, got %d", len(events))
	}
	e := events[0]
	if e.EventType != "comment" || e.ProviderEventID != "COMENT-1" {
		t.Errorf("evento mal normalizado: %+v", e)
	}
	if e.Content != "quanto custa?" || e.SenderID != "VISITANTE-9" || e.ThreadID != "POST-7" {
		t.Errorf("campos errados: %+v", e)
	}
	if e.OccurredAt == nil || e.OccurredAt.Year() != 2025 && e.OccurredAt.Year() != 2026 {
		t.Errorf("entry.time vem em SEGUNDOS; got %v", e.OccurredAt)
	}
}

// Direct: o timestamp vem em MILISSEGUNDOS. Confundir a escala jogaria a data
// para 1970 e o gate de 24h da Fase 4 decidiria errado.
func TestParseWebhook_MessageUsesMillisecondTimestamp(t *testing.T) {
	ms := time.Now().UnixMilli()
	raw := []byte(`{"object":"instagram","entry":[{"id":"IG-DONO","time":1753000000,
		"messaging":[{"sender":{"id":"VISITANTE-9"},"recipient":{"id":"IG-DONO"},
		"timestamp":` + itoa(ms) + `,"message":{"mid":"MSG-1","text":"oi"}}]}]}`)

	events, err := ParseWebhook(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != "message" {
		t.Fatalf("esperado 1 message, got %+v", events)
	}
	got := events[0].OccurredAt
	if got == nil {
		t.Fatal("timestamp perdido")
	}
	if diff := time.Since(*got); diff > time.Minute || diff < -time.Minute {
		t.Fatalf("timestamp interpretado na escala errada: %v (diff %v)", got, diff)
	}
}

// Anti-loop: eco da própria conta NÃO pode virar evento. Sem isto, a IA
// responderia a própria resposta, para sempre.
func TestParseWebhook_DropsEchoesAndSelf(t *testing.T) {
	for _, flag := range []string{`"is_echo":true`, `"is_self":true`} {
		raw := []byte(`{"object":"instagram","entry":[{"id":"IG-DONO",
			"messaging":[{"sender":{"id":"IG-DONO"},"recipient":{"id":"VISITANTE-9"},
			"timestamp":1753000000000,"message":{"mid":"MSG-ECO","text":"resposta da IA",` + flag + `}}]}]}`)
		events, err := ParseWebhook(raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 0 {
			t.Fatalf("eco (%s) virou evento — a IA responderia a si mesma: %+v", flag, events)
		}
	}
}

// Comentário do próprio dono no próprio post também não é atendimento.
func TestParseWebhook_DropsOwnAccountAsSender(t *testing.T) {
	raw := []byte(`{"object":"instagram","entry":[{"id":"IG-DONO",
		"changes":[{"field":"comments","value":{"id":"C1","text":"obrigado!",
		"from":{"id":"IG-DONO"}}}]}]}`)
	events, err := ParseWebhook(raw, map[string]bool{"IG-DONO": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("comentário do próprio dono virou evento: %+v", events)
	}
}

// Sem identificador não há idempotência possível — e a Meta reenvia.
func TestParseWebhook_DropsEventsWithoutID(t *testing.T) {
	raw := []byte(`{"object":"instagram","entry":[{"id":"IG-DONO",
		"changes":[{"field":"comments","value":{"text":"sem id"}}],
		"messaging":[{"sender":{"id":"X"},"message":{"text":"sem mid"}}]}]}`)
	events, err := ParseWebhook(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("evento sem id foi aceito: %+v", events)
	}
}

// Forma desconhecida não pode virar 500: a Meta usa a taxa de erro do endpoint
// para medir nossa saúde e pode desativar a assinatura.
func TestParseWebhook_UnknownShapesAreIgnoredNotFatal(t *testing.T) {
	for name, raw := range map[string]string{
		"campo nao tratado": `{"object":"instagram","entry":[{"id":"A","changes":[{"field":"story_insights","value":{"id":"S1"}}]}]}`,
		"entry vazio":       `{"object":"instagram","entry":[]}`,
		"sem entry":         `{"object":"instagram"}`,
		"value estranho":    `{"object":"instagram","entry":[{"id":"A","changes":[{"field":"comments","value":[1,2,3]}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			events, err := ParseWebhook([]byte(raw), nil)
			if err != nil {
				t.Fatalf("forma desconhecida virou erro: %v", err)
			}
			if len(events) != 0 {
				t.Errorf("não deveria produzir evento: %+v", events)
			}
		})
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
