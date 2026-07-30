package mercadolivre

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// O formato do endpoint de tags não pôde ser observado antes de escrever o
// parser (só responde autenticado), então ele aceita várias formas. O que NÃO
// pode acontecer é entrar em pânico ou inventar uma tag — inventar credita
// comissão na etiqueta errada em silêncio.
func TestParseMLTags(t *testing.T) {
	for _, tc := range []struct {
		nome     string
		body     string
		esperado []string
	}{
		{"objeto com tags (formato observado no painel)",
			`{"tags":[{"tag":"minhaloja20230920","in_use":true}]}`, []string{"minhaloja20230920"}},
		{"múltiplas tags", `{"tags":[{"tag":"a"},{"tag":"b"}]}`, []string{"a", "b"}},
		{"array de objetos na raiz", `[{"tag":"raiz1"}]`, []string{"raiz1"}},
		{"array de strings na raiz", `["str1","str2"]`, []string{"str1", "str2"}},
		{"fallback para name", `{"tags":[{"name":"pelonome"}]}`, []string{"pelonome"}},
		{"envelope data", `{"data":[{"tag":"viadata"}]}`, []string{"viadata"}},
		{"deduplica", `{"tags":[{"tag":"x"},{"tag":"x"}]}`, []string{"x"}},
		{"formato desconhecido não inventa", `{"outra_coisa":123}`, nil},
		{"lista vazia", `{"tags":[]}`, nil},
		{"json inválido não entra em pânico", `<html>bloqueado</html>`, nil},
		{"tag vazia é ignorada", `{"tags":[{"tag":""}]}`, nil},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			got := parseMLTags([]byte(tc.body))
			if len(got) != len(tc.esperado) {
				t.Fatalf("esperava %v, veio %v", tc.esperado, got)
			}
			for i := range got {
				if got[i] != tc.esperado[i] {
					t.Fatalf("esperava %v, veio %v", tc.esperado, got)
				}
			}
		})
	}
}

// A classificação decide se o cliente recebe "reconecte sua conta". Um falso
// positivo aqui manda gente reconectar à toa e queima a confiança no aviso.
func TestValidateMLSession(t *testing.T) {
	for _, tc := range []struct {
		nome     string
		status   int
		body     string
		erro     error
		wantTags int
	}{
		{"200 com tags é sessão viva", http.StatusOK, `{"tags":[{"tag":"abc"}]}`, nil, 1},
		{"401 é sessão expirada", http.StatusUnauthorized, `{"message":"Unauthorized"}`, ErrMLSessionExpired, 0},
		{"403 é sessão expirada", http.StatusForbidden, `{"message":"Forbidden"}`, ErrMLSessionExpired, 0},
		{"429 é bloqueio, NÃO sessão morta", http.StatusTooManyRequests, `{}`, ErrMLBlocked, 0},
		{"5xx é indeterminado, NÃO sessão morta", http.StatusBadGateway, `{}`, ErrMLUnavailable, 0},
		{"200 com HTML é WAF, NÃO sessão morta", http.StatusOK, `<html>Access Denied</html>`, ErrMLBlocked, 0},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			// Redireciona o endpoint fixo para o servidor de teste.
			client := srv.Client()
			client.Transport = rewriteHost{base: srv.URL, rt: srv.Client().Transport}

			check, err := ValidateMLSession(context.Background(), client, "cookie=1")
			if tc.erro != nil {
				if !errors.Is(err, tc.erro) {
					t.Fatalf("esperava %v, veio %v", tc.erro, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("esperava sucesso, veio %v", err)
			}
			if len(check.Tags) != tc.wantTags {
				t.Fatalf("esperava %d tags, veio %v", tc.wantTags, check.Tags)
			}
		})
	}

	t.Run("cookie vazio nem tenta a rede", func(t *testing.T) {
		if _, err := ValidateMLSession(context.Background(), nil, ""); !errors.Is(err, ErrMLNoCredentials) {
			t.Fatalf("esperava ErrMLNoCredentials, veio %v", err)
		}
	})
}

// rewriteHost aponta o endpoint de produção (fixo em ml_session.go) para o
// httptest, sem precisar parametrizar a URL no código de produção.
type rewriteHost struct {
	base string
	rt   http.RoundTripper
}

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	base := strings.TrimPrefix(r.base, "http://")
	req.URL.Scheme = "http"
	req.URL.Host = base
	rt := r.rt
	if rt == nil {
		rt = http.DefaultTransport
	}
	return rt.RoundTrip(req)
}
