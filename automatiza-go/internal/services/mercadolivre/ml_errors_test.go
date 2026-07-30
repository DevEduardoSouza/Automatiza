package mercadolivre

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"automatiza-go/internal/models"

	"go.uber.org/zap/zaptest"
)

// countingTransport conta tentativas de rede e nunca deixa nenhuma sair. Serve
// para provar o que mais importa nos guards: que NENHUMA request foi enviada.
type countingTransport struct{ calls int }

func (c *countingTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	c.calls++
	return nil, fmt.Errorf("rede bloqueada no teste")
}

// Este é o teste do bug que motivou os erros tipados: uma falha de LEITURA da
// credencial (timeout de Postgres, ctx cancelado no shutdown) NÃO pode ser
// confundida com "o usuário não conectou a conta". O espelho pausava a
// automação inteira do cliente — inclusive a parte Shopee dela — por causa
// dessa confusão.
func TestLoadCredentials_IOErrorNaoEhSemCredencial(t *testing.T) {
	svc := NewMLService(nil, &mockMLCredsRepo{
		GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
			return nil, fmt.Errorf("context deadline exceeded")
		},
	}, &mockMLLinkRepo{}, testEncryptionKey, zaptest.NewLogger(t))

	_, err := svc.LoadCredentials(context.Background(), "user1")
	if !errors.Is(err, ErrMLCredentialsUnreadable) {
		t.Fatalf("erro de I/O deveria ser ErrMLCredentialsUnreadable, foi: %v", err)
	}
	if errors.Is(err, ErrMLNoCredentials) {
		t.Fatal("REGRESSÃO: erro de I/O virou ErrMLNoCredentials — isto pausava a automação do cliente")
	}
}

// O contraponto: ausência real de credencial precisa continuar sendo
// ErrMLNoCredentials, senão o cliente nunca é avisado de que precisa conectar.
func TestLoadCredentials_SemLinhaEhSemCredencial(t *testing.T) {
	svc := NewMLService(nil, &mockMLCredsRepo{
		GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
			return nil, nil
		},
	}, &mockMLLinkRepo{}, testEncryptionKey, zaptest.NewLogger(t))

	_, err := svc.LoadCredentials(context.Background(), "user1")
	if !errors.Is(err, ErrMLNoCredentials) {
		t.Fatalf("ausência de credencial deveria ser ErrMLNoCredentials, foi: %v", err)
	}
}

// Tag vazia geraria um link SEM atribuição — comissão perdida em silêncio, que
// é pior do que falhar. O guard tem que barrar ANTES de qualquer request.
func TestCreateShortLink_TagVaziaNaoChamaML(t *testing.T) {
	svc := NewMLService(nil, &mockMLCredsRepo{}, &mockMLLinkRepo{}, testEncryptionKey, zaptest.NewLogger(t)).(*mlService)
	tr := &countingTransport{}
	svc.httpClient = &http.Client{Transport: tr, Timeout: time.Second}

	for _, tc := range []struct {
		nome  string
		creds *models.MLCredentials
	}{
		{"tag vazia", &models.MLCredentials{Cookie: "c=1", CSRF: "t", Tag: ""}},
		{"tag só com espaço", &models.MLCredentials{Cookie: "c=1", CSRF: "t", Tag: "   "}},
		{"cookie vazio", &models.MLCredentials{Cookie: "", CSRF: "t", Tag: "minhatag"}},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			_, err := svc.createShortLink(context.Background(), "https://produto.mercadolivre.com.br/MLB-1-x", tc.creds, "user1")
			if !errors.Is(err, ErrMLNoCredentials) {
				t.Fatalf("esperava ErrMLNoCredentials, veio: %v", err)
			}
		})
	}

	if tr.calls != 0 {
		t.Fatalf("REGRESSÃO: %d request(s) enviadas ao ML com credencial incompleta — deveria ser 0", tr.calls)
	}
}

// Breaker aberto tem que responder instantâneo, sem tocar a rede: é isso que
// impede um ML fora do ar de queimar o orçamento de contexto do scheduler
// serial e matar de fome as outras automações.
func TestCreateShortLink_BreakerAbertoNaoChamaML(t *testing.T) {
	svc := NewMLService(nil, &mockMLCredsRepo{}, &mockMLLinkRepo{}, testEncryptionKey, zaptest.NewLogger(t)).(*mlService)
	tr := &countingTransport{}
	svc.httpClient = &http.Client{Transport: tr, Timeout: time.Second}

	for i := 0; i < 5; i++ {
		svc.breaker.recordFailure()
	}

	creds := &models.MLCredentials{Cookie: "c=1", CSRF: "t", Tag: "minhatag"}
	_, err := svc.createShortLink(context.Background(), "https://produto.mercadolivre.com.br/MLB-1-x", creds, "user1")
	if !errors.Is(err, ErrMLUnavailable) {
		t.Fatalf("esperava ErrMLUnavailable com breaker aberto, veio: %v", err)
	}
	if tr.calls != 0 {
		t.Fatalf("REGRESSÃO: breaker aberto ainda enviou %d request(s)", tr.calls)
	}
}

// classifyCreateLinkFailure é o que permite ao caller distinguir "sessão
// morreu" de "ML fora do ar" de "estamos sendo bloqueados". Se ela colapsar
// esses casos, o espelho volta a tratar tudo igual.
func TestClassifyCreateLinkFailure(t *testing.T) {
	svc := NewMLService(nil, &mockMLCredsRepo{}, &mockMLLinkRepo{}, testEncryptionKey, zaptest.NewLogger(t)).(*mlService)

	for _, tc := range []struct {
		nome     string
		status   int
		body     string
		esperado error
	}{
		{"401 é sessão expirada", http.StatusUnauthorized, `{"message":"Unauthorized"}`, ErrMLSessionExpired},
		{"403 é sessão expirada", http.StatusForbidden, `{"message":"Forbidden"}`, ErrMLSessionExpired},
		{"429 é bloqueio", http.StatusTooManyRequests, `{"message":"slow down"}`, ErrMLBlocked},
		{"HTML de WAF é bloqueio", http.StatusOK, `<html>Access Denied</html>`, ErrMLBlocked},
		{"400 é URL inconvertível", http.StatusBadRequest, `{"message":"invalid url"}`, ErrMLUnconvertibleURL},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			err := svc.classifyCreateLinkFailure(context.Background(), "user1", tc.status, []byte(tc.body))
			if !errors.Is(err, tc.esperado) {
				t.Fatalf("status %d: esperava %v, veio %v", tc.status, tc.esperado, err)
			}
		})
	}

	// 5xx é falha de infra: erro genérico (retentável), e conta para o breaker
	// de infra — não pode virar ErrMLUnconvertibleURL, que é permanente.
	err := svc.classifyCreateLinkFailure(context.Background(), "user1", http.StatusBadGateway, []byte(`{"m":"bad gateway"}`))
	if errors.Is(err, ErrMLUnconvertibleURL) || errors.Is(err, ErrMLSessionExpired) {
		t.Fatalf("5xx não pode virar erro permanente, veio: %v", err)
	}
}
