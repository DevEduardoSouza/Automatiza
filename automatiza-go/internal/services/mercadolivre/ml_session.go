package mercadolivre

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// tagsEndpoint é o endpoint do próprio painel de afiliados que lista as
// etiquetas (tags) da conta logada. Verificado ao vivo em 30/07/2026: responde
// 401 com JSON quando não há sessão, e uma rota inventada no mesmo prefixo
// responde 404 — ou seja, o 401 prova que a rota existe e está autenticada.
const tagsEndpoint = "https://www.mercadolivre.com.br/affiliate-program/api/v2/stripe/user/tags"

// MLSessionCheck é o resultado de uma verificação de sessão.
type MLSessionCheck struct {
	// Tags são as etiquetas de afiliado da conta. Vazio não significa sessão
	// morta — significa que não conseguimos parsear (formato mudou).
	Tags []string
}

// ValidateMLSession faz UMA chamada barata ao painel de afiliados para saber se
// o cookie ainda vale e, de quebra, descobrir a tag do afiliado.
//
// Motivação: as 7 credenciais que existiam em produção morreram entre março e
// abril de 2026 e ficaram 111-144 dias paradas sem que ninguém — nem o cliente,
// nem nós — soubesse. Não havia nenhuma verificação de sessão no sistema.
//
// Descobrir a tag aqui elimina um passo manual e um modo de falha inteiro: o
// cliente digitava a tag à mão e, se errasse, os links saíam com atribuição
// para outra etiqueta (ou nenhuma) sem nenhum sintoma visível.
//
// Fail-open deliberado, no mesmo espírito de shopee.ValidateAffiliateCredentials:
// só devolve ErrMLSessionExpired quando o ML responde de forma DETERMINÍSTICA
// (401/403). Rede fora, 5xx ou corpo estranho devolvem ErrMLUnavailable, que o
// caller trata como "não sei" — nunca como "sessão morta". Marcar uma sessão
// viva como morta manda o cliente reconectar à toa e mata a confiança no aviso.
func ValidateMLSession(ctx context.Context, client *http.Client, cookie string) (*MLSessionCheck, error) {
	if cookie == "" {
		return nil, ErrMLNoCredentials
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsEndpoint, nil)
	if err != nil {
		return nil, ErrMLUnavailable
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Referer", "https://www.mercadolivre.com.br/afiliados/linkbuilder")
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrMLUnavailable
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, ErrMLUnavailable
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrMLSessionExpired
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrMLBlocked
	case resp.StatusCode != http.StatusOK:
		return nil, ErrMLUnavailable
	case !json.Valid(body):
		// 200 com corpo não-JSON num endpoint que só fala JSON: página de
		// desafio do WAF. NÃO é sessão morta — não mandar o cliente reconectar.
		return nil, ErrMLBlocked
	}

	return &MLSessionCheck{Tags: parseMLTags(body)}, nil
}

// parseMLTags extrai as etiquetas da resposta do painel.
//
// Defensivo de propósito: o endpoint só responde autenticado, então não foi
// possível observar o formato exato antes de escrever isto. Aceita array na
// raiz ou objeto com "tags"/"data", e dentro de cada item tenta "tag", "name" e
// "id". Se nada casar, devolve vazio — e o caller cai no input manual de tag
// que já existe hoje, sem erro. Adivinhar aqui seria pior que não descobrir.
func parseMLTags(body []byte) []string {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil
	}

	items, ok := root.([]any)
	if !ok {
		obj, isObj := root.(map[string]any)
		if !isObj {
			return nil
		}
		for _, key := range []string{"tags", "data", "results"} {
			if v, found := obj[key]; found {
				if arr, isArr := v.([]any); isArr {
					items = arr
					break
				}
			}
		}
	}

	var tags []string
	seen := map[string]bool{}
	for _, it := range items {
		var tag string
		switch v := it.(type) {
		case string:
			tag = v
		case map[string]any:
			for _, key := range []string{"tag", "name", "id"} {
				if s, isStr := v[key].(string); isStr && s != "" {
					tag = s
					break
				}
			}
		}
		if tag != "" && !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}
