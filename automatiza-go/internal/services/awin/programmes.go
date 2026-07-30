// Package awin converte links de lojas atendidas pela rede Awin para o
// deeplink de afiliado do usuário.
//
// A conversão em si não faz rede: um deeplink Awin é
// awin1.com/cread.php?awinmid=<anunciante>&awinaffid=<publisher>&ued=<destino>.
// A única chamada externa é FetchProgrammes, que descobre o awinmid de cada
// anunciante — e ela roda no cadastro da credencial, nunca num disparo.
package awin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidCredentials marca publisher ID / token recusados pela Awin (401 e
// 403). É determinístico: pede ação do usuário, não retry.
var ErrInvalidCredentials = fmt.Errorf("credenciais Awin invalidas (o token OAuth2 nao foi aceito)")

// Programme é um anunciante em que o publisher está aprovado.
type Programme struct {
	// ID é o awinmid — chega numérico no JSON e vira string aqui porque é
	// identificador, não quantidade.
	ID string `json:"id"`
	// Name é o rótulo que o usuário vê no seletor de marcas.
	Name string `json:"name"`
	// DisplayURL é o site do anunciante: a chave que casa uma URL de produto
	// com o programa certo.
	DisplayURL string `json:"displayUrl"`
}

// apiProgramme espelha o shape cru da Awin (id numérico, campos extras).
type apiProgramme struct {
	ID              json.Number `json:"id"`
	Name            string      `json:"name"`
	DisplayURL      string      `json:"displayUrl"`
	ClickThroughURL string      `json:"clickThroughUrl"`
}

// FetchProgrammes lista os programas em que o publisher tem relação ativa.
//
// baseURL vazio usa a API de produção (parametrizado para testes).
//
// Fail-open igual ao da Shopee (validate_credentials.go): só devolve
// ErrInvalidCredentials em 401/403, que são determinísticos. Timeout, 5xx e
// 429 devolvem erro genérico — o caller trata como indisponibilidade e não
// bloqueia o cadastro por instabilidade da Awin.
func FetchProgrammes(ctx context.Context, baseURL, publisherID, token string) ([]Programme, error) {
	if publisherID == "" || token == "" {
		return nil, ErrInvalidCredentials
	}
	if baseURL == "" {
		baseURL = "https://api.awin.com"
	}

	endpoint := fmt.Sprintf("%s/publishers/%s/programmes?relationship=joined",
		strings.TrimRight(baseURL, "/"), url.PathEscape(publisherID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("awin: request invalida: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("awin: falha de rede: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return nil, ErrInvalidCredentials
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("awin: HTTP %d", resp.StatusCode)
	}

	// 2 MiB cobre com folga uma conta com milhares de programas.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("awin: leitura da resposta: %w", err)
	}

	var raw []apiProgramme
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("awin: resposta ilegivel: %w", err)
	}

	out := make([]Programme, 0, len(raw))
	for _, p := range raw {
		id := strings.TrimSpace(p.ID.String())
		if id == "" || id == "0" {
			continue
		}
		display := hostOf(p.DisplayURL)
		if display == "" {
			display = hostOf(p.ClickThroughURL)
		}
		out = append(out, Programme{ID: id, Name: strings.TrimSpace(p.Name), DisplayURL: display})
	}
	return out, nil
}

// DomainMap reduz os programas ao mapa domínio→awinmid usado na conversão.
// Esse mapa é o que fica guardado na credencial do usuário.
func DomainMap(programmes []Programme) map[string]string {
	out := make(map[string]string, len(programmes))
	for _, p := range programmes {
		if p.DisplayURL == "" || p.ID == "" {
			continue
		}
		out[p.DisplayURL] = p.ID
	}
	return out
}

// hostOf extrai o host normalizado de uma URL de anunciante, tolerando as
// formas sem esquema que a Awin devolve ("www.loja.com.br", "loja.com.br/br").
func hostOf(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	// Host puramente numérico é lixo de parse, não domínio.
	if _, err := strconv.Atoi(strings.ReplaceAll(host, ".", "")); err == nil {
		return ""
	}
	return host
}
