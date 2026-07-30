package affiliatelink

import (
	"context"
	"errors"
	"strings"
	"testing"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/models"
)

type fakeConverter struct {
	out  string
	err  error
	hits int
}

func (f *fakeConverter) Convert(_ context.Context, _ commerceurl.URL, _ string) (string, error) {
	f.hits++
	return f.out, f.err
}

const amazonURL = "https://www.amazon.com.br/dp/B09B8V1LZ3"

// TestRegistryNilNaoResolveNada é o contrato que torna a feature reversível:
// com o gate desligado o registry é nil, e todo caller precisa se comportar
// como antes (ok=false, err=nil → "mantenha o que você fazia").
func TestRegistryNilNaoResolveNada(t *testing.T) {
	var r *Registry
	if r.Handles(models.MonitorStoreAmazon) {
		t.Error("registry nil não pode declarar que trata loja")
	}
	got, ok, err := r.Convert(context.Background(), amazonURL, "user-1")
	if got != "" || ok || err != nil {
		t.Errorf("(%q, %v, %v), queria (\"\", false, nil)", got, ok, err)
	}
}

// Loja sem conversor devolve o mesmo contrato do registry nil.
func TestLojaSemConversorNaoEhErro(t *testing.T) {
	r := New(map[string]Converter{models.MonitorStoreAwin: &fakeConverter{out: "x"}})
	got, ok, err := r.Convert(context.Background(), amazonURL, "user-1")
	if got != "" || ok || err != nil {
		t.Errorf("(%q, %v, %v), queria (\"\", false, nil)", got, ok, err)
	}
}

func TestConvertDelegaParaOConversorDaLoja(t *testing.T) {
	amz := &fakeConverter{out: "https://link.do/TENANT"}
	awn := &fakeConverter{out: "https://outro"}
	r := New(map[string]Converter{
		models.MonitorStoreAmazon: amz,
		models.MonitorStoreAwin:   awn,
	})

	got, ok, err := r.Convert(context.Background(), amazonURL, "user-1")
	if err != nil || !ok || got != "https://link.do/TENANT" {
		t.Fatalf("(%q, %v, %v)", got, ok, err)
	}
	if amz.hits != 1 || awn.hits != 0 {
		t.Errorf("roteamento errado: amazon=%d awin=%d", amz.hits, awn.hits)
	}
}

// Conversor que devolve vazio ou a própria origem seria vazamento de comissão
// disfarçado de sucesso: vira falha, e o caller remove o link.
func TestConvertRecusaSaidaVaziaOuIgualAOrigem(t *testing.T) {
	cases := map[string]string{"vazio": "", "igual à origem": amazonURL}
	for name, out := range cases {
		t.Run(name, func(t *testing.T) {
			r := New(map[string]Converter{models.MonitorStoreAmazon: &fakeConverter{out: out}})
			got, ok, err := r.Convert(context.Background(), amazonURL, "user-1")
			if ok || got != "" {
				t.Fatalf("(%q, %v) — saída inválida passou", got, ok)
			}
			if !errors.Is(err, ErrUnconvertible) {
				t.Errorf("erro = %v, queria ErrUnconvertible", err)
			}
		})
	}
}

// Erro do conversor precisa chegar ao caller para ele REMOVER o link. Devolver
// (false, nil) aqui faria o caller manter o link original no corpo.
func TestConvertPropagaErroDoConversor(t *testing.T) {
	r := New(map[string]Converter{models.MonitorStoreAmazon: &fakeConverter{err: ErrNoCredentials}})
	_, ok, err := r.Convert(context.Background(), amazonURL, "user-1")
	if ok {
		t.Fatal("erro não pode virar sucesso")
	}
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("erro = %v, queria ErrNoCredentials", err)
	}
}

// URL bloqueada por segurança não é "sem conversor": tem de ser removida.
func TestConvertURLForjadaEhRemovida(t *testing.T) {
	r := New(map[string]Converter{models.MonitorStoreAmazon: &fakeConverter{out: "x"}})
	_, ok, err := r.Convert(context.Background(), "https://amazon.com.br.evil.test/dp/B09B8V1LZ3", "user-1")
	if ok {
		t.Fatal("host forjado não pode converter")
	}
	if !errors.Is(err, ErrUnconvertible) {
		t.Errorf("erro = %v, queria ErrUnconvertible", err)
	}
}

// As mensagens de erro não podem casar com os markers que process_list.go e
// offer_generate_service.go usam para PAUSAR a automação Shopee do cliente.
func TestMensagensNaoAcionamPausaDeAutomacao(t *testing.T) {
	markers := []string{
		"credentials not configured",
		"credentials incomplete",
		"Invalid Signature",
		"invalid_token",
	}
	for _, err := range []error{ErrNoCredentials, ErrUnconvertible, ErrUnavailable, ErrRejectedCredential} {
		for _, m := range markers {
			if strings.Contains(err.Error(), m) {
				t.Errorf("%q contém o marker %q — pausaria a automação do cliente", err, m)
			}
		}
	}
}
