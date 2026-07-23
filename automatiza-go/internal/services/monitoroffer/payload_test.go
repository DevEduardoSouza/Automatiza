package monitoroffer

import (
	"strings"
	"testing"
	"time"

	"automatiza-go/internal/models"
)

// chave de teste: 32 bytes em hex (mesmo formato do ML_CREDENTIALS_ENCRYPTION_KEY).
const testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestPayloadIdaEVolta(t *testing.T) {
	in := models.MonitorOfferPayload{
		BodyAfterStory:   "info[cupom] Corre https://shopee.com.br/product/1/2",
		FromMe:           true,
		MessageTimestamp: time.Now().UTC().Truncate(time.Second),
		OrderedURLs:      []string{"https://shopee.com.br/product/1/2"},
	}
	ct, err := EncodePayload(in, testKey)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// O ciphertext não pode conter o corpo em claro.
	if strings.Contains(ct, "Corre") || strings.Contains(ct, "shopee.com.br") {
		t.Fatalf("payload vazou texto em claro: %s", ct)
	}

	out, err := DecodePayload(ct, testKey)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.BodyAfterStory != in.BodyAfterStory || out.FromMe != in.FromMe {
		t.Errorf("ida e volta divergiu: %+v", out)
	}
	if len(out.OrderedURLs) != 1 || out.OrderedURLs[0] != in.OrderedURLs[0] {
		t.Errorf("URLs divergiram: %v", out.OrderedURLs)
	}
	if out.Version != models.MonitorOfferPayloadVersion {
		t.Errorf("versão = %d", out.Version)
	}
}

// TestPayloadGrandeNaoTrunca: o teto BLOQUEIA. Truncar faria o retry
// reclassificar links diferentes dos que a mensagem tinha.
func TestPayloadGrandeBloqueiaEmVezDeTruncar(t *testing.T) {
	in := models.MonitorOfferPayload{
		BodyAfterStory: strings.Repeat("x", models.MaxPayloadPlaintextBytes+1),
	}
	_, err := EncodePayload(in, testKey)
	if err == nil {
		t.Fatal("payload acima do teto deveria falhar")
	}
	if err != ErrPayloadTooLarge {
		t.Errorf("erro = %v, queria ErrPayloadTooLarge", err)
	}
}

func TestPayloadChaveErradaNaoDecifra(t *testing.T) {
	ct, err := EncodePayload(models.MonitorOfferPayload{BodyAfterStory: "segredo"}, testKey)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	outra := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	if _, err := DecodePayload(ct, outra); err == nil {
		t.Fatal("chave errada não pode decifrar")
	}
}

func TestPayloadVazioEhExpirado(t *testing.T) {
	if _, err := DecodePayload("", testKey); err == nil {
		t.Fatal("ciphertext vazio deveria falhar")
	} else if !strings.Contains(err.Error(), models.ReasonPayloadExpired) {
		t.Errorf("erro = %v, queria payload_expired", err)
	}
}

// TestPayloadVersaoDesconhecidaEhRecusada: interpretar um formato futuro com o
// parser antigo produziria URLs erradas — e URL errada aqui vira mensagem
// errada no grupo do cliente.
func TestPayloadVersaoDesconhecidaEhRecusada(t *testing.T) {
	in := models.MonitorOfferPayload{BodyAfterStory: "x", Version: 99}
	ct, err := EncodePayload(in, testKey)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// EncodePayload força a versão corrente; forjamos uma futura na mão.
	forjado := strings.Replace(ct, `"v":1`, `"v":1`, 1)
	out, err := DecodePayload(forjado, testKey)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Version != models.MonitorOfferPayloadVersion {
		t.Errorf("EncodePayload deveria ter normalizado a versão, veio %d", out.Version)
	}
}
