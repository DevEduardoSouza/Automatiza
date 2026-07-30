package monitoroffer

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/mercadolivre"
	"automatiza-go/internal/services/shopee"
)

// TestErroDaShopeeNuncaViraCampanha é o oráculo central deste arquivo: seja
// qual for o erro, o desfecho JAMAIS pode ser campaign_or_coupon e JAMAIS pode
// trazer ConvertedURL preenchida (que é o que sairia para o grupo).
func TestErroDaShopeeNuncaViraCampanhaNemEmiteURL(t *testing.T) {
	erros := []error{
		shopee.ErrShopeeNoCredentials,
		shopee.ErrInvalidCredentials,
		shopee.ErrShopeeNoAPIAccess,
		shopee.ErrShopeeUnconvertibleURL,
		shopee.ErrShopeeUnavailable,
		errors.New("error code: 10030"),
		errors.New("error code: 10020"),
		errors.New("error code: 11001"),
		errors.New("Invalid Signature"),
		errors.New("failed to load shopee credentials: timeout"),
		errors.New("algo totalmente inesperado"),
		fmt.Errorf("embrulhado: %w", shopee.ErrShopeeNoCredentials),
	}
	for _, err := range erros {
		got := classifyShopeeError(err)
		if got.Class == models.URLClassCampaignOrCoupon {
			t.Errorf("%v virou campanha", err)
		}
		if got.ConvertedURL != "" {
			t.Errorf("%v devolveu URL para emitir: %q", err, got.ConvertedURL)
		}
		if !models.IsValidReasonCode(got.Reason) {
			t.Errorf("%v devolveu reason fora do enum: %q", err, got.Reason)
		}
	}
}

// TestRateLimitNaoPausaAutomacao é a distinção que evita repetir o incidente de
// pausa em massa: 10030 e circuit breaker voltam sozinhos.
func TestRateLimitNaoPausaMasCredencialPausa(t *testing.T) {
	transitorios := map[string]error{
		"10030":            errors.New("error code: 10030"),
		"circuit breaker":  shopee.ErrShopeeUnavailable,
		"leitura no banco": errors.New("failed to load shopee credentials: timeout"),
	}
	for nome, err := range transitorios {
		got := classifyShopeeError(err)
		if got.ErrorClass != models.ErrorClassTransient {
			t.Errorf("%s: error_class = %q, queria transient", nome, got.ErrorClass)
		}
		if PausesAutomation(got.ErrorClass) {
			t.Errorf("%s NÃO pode pausar a automação do cliente", nome)
		}
	}

	permanentes := map[string]error{
		"sem credencial": shopee.ErrShopeeNoCredentials,
		"assinatura":     shopee.ErrInvalidCredentials,
		"sem Open API":   shopee.ErrShopeeNoAPIAccess,
	}
	for nome, err := range permanentes {
		got := classifyShopeeError(err)
		if got.ErrorClass != models.ErrorClassPermanentConfig {
			t.Errorf("%s: error_class = %q, queria permanent_config", nome, got.ErrorClass)
		}
		if !PausesAutomation(got.ErrorClass) {
			t.Errorf("%s deveria pausar a automação (só o cliente resolve)", nome)
		}
	}
}

func TestCarrinhoEhPermanenteDeConteudo(t *testing.T) {
	got := classifyShopeeError(shopee.ErrShopeeUnconvertibleURL)
	if got.ErrorClass != models.ErrorClassPermanentConte || got.Reason != models.ReasonCart {
		t.Errorf("carrinho = %+v, queria permanent_content/cart", got)
	}
	if PausesAutomation(got.ErrorClass) {
		t.Error("carrinho não pode pausar a automação")
	}
}

// TestErroDoMLUsaOTipadoQuandoExiste: o RemoteError já decidiu classe e reason
// na fronteira HTTP — reinterpretar a string aqui divergiria dele.
func TestErroDoMLUsaOTipadoQuandoExiste(t *testing.T) {
	casos := []struct {
		remote *mercadolivre.RemoteError
		class  string
	}{
		{&mercadolivre.RemoteError{Class: models.ErrorClassTransient, Reason: models.ReasonHTTP429}, models.ErrorClassTransient},
		{&mercadolivre.RemoteError{Class: models.ErrorClassPermanentConte, Reason: models.ReasonNotFound}, models.ErrorClassPermanentConte},
		{&mercadolivre.RemoteError{Class: models.ErrorClassPermanentConfig, Reason: models.ReasonInvalidToken}, models.ErrorClassPermanentConfig},
		{&mercadolivre.RemoteError{Class: models.ErrorClassSecurityBlock, Reason: models.ReasonUnsafeHost}, models.ErrorClassSecurityBlock},
	}
	for _, c := range casos {
		got := classifyMLError(fmt.Errorf("contexto: %w", c.remote))
		if got.ErrorClass != c.class {
			t.Errorf("%v: error_class = %q, queria %q", c.remote.Reason, got.ErrorClass, c.class)
		}
		if got.Reason != c.remote.Reason {
			t.Errorf("reason = %q, queria %q", got.Reason, c.remote.Reason)
		}
		if got.Class == models.URLClassCampaignOrCoupon {
			t.Errorf("%v virou campanha", c.remote.Reason)
		}
		if got.ConvertedURL != "" {
			t.Errorf("%v devolveu URL para emitir", c.remote.Reason)
		}
	}
}

// TestPreviewIncompletoDoMLEhTransient: HTTP 200 sem dados pode ser HTML
// mudado, bloqueio ou produto esgotado. Nunca é prova de cupom.
func TestPreviewIncompletoNaoViraCampanha(t *testing.T) {
	got := classifyMLError(&mercadolivre.RemoteError{
		Class: models.ErrorClassTransient, Reason: models.ReasonPreviewIncomplete,
	})
	if got.Class == models.URLClassCampaignOrCoupon {
		t.Fatal("preview incompleto virou campanha — é o cupom falso")
	}
	if got.ErrorClass != models.ErrorClassTransient {
		t.Errorf("error_class = %q, queria transient (retry)", got.ErrorClass)
	}
}

// TestServicoAusenteNaoEmiteOriginal: sem serviço configurado, o link fica
// pendente — nunca é repassado cru.
func TestServicoAusenteNaoEmiteOriginal(t *testing.T) {
	r := NewStoreResolver(nil, nil, nil, nil)
	for _, store := range []string{models.MonitorStoreShopee, models.MonitorStoreMercadoLivr} {
		u := mustURL(t, "https://shopee.com.br/product/1/2")
		u.Store = store
		got := r.Resolve(context.Background(), ResolveRequest{URL: u})
		if got.ConvertedURL != "" {
			t.Errorf("%s devolveu URL sem serviço: %q", store, got.ConvertedURL)
		}
		if got.ErrorClass != models.ErrorClassTransient {
			t.Errorf("%s: error_class = %q, queria transient", store, got.ErrorClass)
		}
	}
}

func TestLojaSemConversorNaoChamaNinguem(t *testing.T) {
	r := NewStoreResolver(nil, nil, nil, nil)
	u := mustURL(t, "https://amazon.com.br/dp/B01")
	got := r.Resolve(context.Background(), ResolveRequest{URL: u})
	if got.Class != models.URLClassUnsupportedCommerce || got.Reason != models.ReasonUnsupportedStore {
		t.Errorf("got = %+v", got)
	}
	if got.ConvertedURL != "" {
		t.Error("loja sem conversor não pode emitir URL")
	}
}

// TestTodosOsReasonsDoResolverEstaoNoEnum impede que uma tradução nova escape
// do CHECK do banco e só falhe no INSERT em produção.
func TestTodosOsReasonsDoResolverEstaoNoEnum(t *testing.T) {
	for _, got := range []ResolveResult{
		transientResult(models.ReasonHTTP429),
		blockedResult(models.ErrorClassPermanentConfig, models.ReasonMissingCredentials),
		classifyMLError(errors.New("credentials not configured")),
		classifyMLError(errors.New("invalid mercadolivre URL")),
		classifyMLError(errors.New("qualquer coisa")),
	} {
		if !models.IsValidReasonCode(got.Reason) {
			t.Errorf("reason %q fora do enum", got.Reason)
		}
	}
}

// mustURL parseia uma URL de teste.
func mustURL(t *testing.T, raw string) commerceurl.URL {
	t.Helper()
	u, err := commerceurl.Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q): %v", raw, err)
	}
	return u
}
