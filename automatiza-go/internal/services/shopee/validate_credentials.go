package shopee

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrInvalidCredentials marks App ID/secret rejected by Shopee's API (10020).
var ErrInvalidCredentials = fmt.Errorf("credenciais Shopee invalidas (Invalid Signature)")

// ValidateAffiliateCredentials faz UMA chamada assinada de teste à API de
// afiliados da Shopee para verificar App ID + Senha ANTES de salvá-los.
//
// Motivação (incidente 11/07): um secret errado salvo no cadastro só aparece
// DEPOIS, como rajada de [10020] Invalid Signature queimando itens de lista e
// pausando automações. Rejeitar no salvamento corta o problema na origem.
//
// Fail-open deliberado: só retorna ErrInvalidCredentials quando a Shopee
// responde 10020/Invalid Signature/Invalid Credential (determinístico).
// Qualquer outra coisa — rede fora, 10030 rate limit, 5xx — retorna nil,
// porque bloquear o cadastro por instabilidade da Shopee seria pior.
// baseURL vazio usa a API de produção (parametrizado para testes).
func ValidateAffiliateCredentials(ctx context.Context, baseURL, appID, appSecret string) error {
	if appID == "" || appSecret == "" {
		return nil // formato é validado em outra camada; aqui só a assinatura
	}
	if baseURL == "" {
		baseURL = "https://open-api.affiliate.shopee.com.br"
	}

	payload := `{"query":"{productOfferV2(limit: 1){nodes{itemId}}}"}`
	ts := fmt.Sprintf("%d", time.Now().Unix())
	sig := shopeeSign(appID, ts, payload, appSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/graphql", strings.NewReader(payload))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization",
		fmt.Sprintf("SHA256 Credential=%s, Timestamp=%s, Signature=%s", appID, ts, sig))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil
	}
	text := string(body)
	if strings.Contains(text, "[10020]") ||
		strings.Contains(text, "Invalid Signature") ||
		strings.Contains(text, "Invalid Credential") {
		return ErrInvalidCredentials
	}
	return nil
}
