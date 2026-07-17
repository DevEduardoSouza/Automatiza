package kiwify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// APIClient fala com a API pública da Kiwify (https://public-api.kiwify.com/v1)
// para o painel Financeiro. Fonte da verdade da receita real do produto
// AUTOMATISA. Verificado 17/07: token OAuth (client_id+client_secret,
// form-urlencoded) => Bearer; demais chamadas exigem header x-kiwify-account-id.
// net_amount vem em CENTAVOS; status ∈ paid|refunded|refused|waiting_payment.
// Rate limit 100/min — por isso o handler cacheia os resumos no Redis.
const kiwifyAPIBase = "https://public-api.kiwify.com/v1"

type APIClient struct {
	clientID     string
	clientSecret string
	accountID    string
	productID    string
	http         *http.Client
	logger       *zap.Logger

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// NewAPIClient monta o client. Se qualquer credencial faltar, Enabled() é false
// e o painel cai na estimativa por plano (fail-soft, nunca quebra).
func NewAPIClient(clientID, clientSecret, accountID, productID string, logger *zap.Logger) *APIClient {
	return &APIClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		accountID:    accountID,
		productID:    productID,
		http:         &http.Client{Timeout: 15 * time.Second},
		logger:       logger,
	}
}

// Enabled diz se há credenciais completas para consultar a Kiwify.
func (c *APIClient) Enabled() bool {
	return c.clientID != "" && c.clientSecret != "" && c.accountID != "" && c.productID != ""
}

// PlanAgg agrega valor pago e nº de vendas de um plano.
type PlanAgg struct {
	Plan       string
	GrossCents int64
	Count      int
}

// SalesSummary é o resumo agregado das vendas do produto AUTOMATISA numa janela.
type SalesSummary struct {
	GrossPaidCents int64 // Σ net_amount das vendas status=paid
	PaidCount      int
	RefundedCount  int
	RefusedCount   int
	WaitingCount   int
	TotalCount     int
	ByPlan         []PlanAgg
}

// getToken devolve um Bearer válido, cacheado em memória até ~60s antes de
// expirar. Protegido por mutex: chamadas concorrentes reusam o mesmo token.
func (c *APIClient) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}

	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		kiwifyAPIBase+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("kiwify token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kiwify token http %d", resp.StatusCode)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   any    `json:"expires_in"` // pode vir string ou number
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("kiwify token decode: %w", err)
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("kiwify token vazio")
	}
	c.token = body.AccessToken
	// expires_in ~86400s; renova 60s antes por segurança.
	c.tokenExp = time.Now().Add(23 * time.Hour)
	return c.token, nil
}

// kiwifySale é o subconjunto de campos de /v1/sales que o resumo usa.
type kiwifySale struct {
	Status    string `json:"status"`
	NetAmount int64  `json:"net_amount"` // CENTAVOS
	Product   struct {
		PlanName string `json:"plan_name"`
	} `json:"product"`
}

type salesPage struct {
	Pagination struct {
		Count      int `json:"count"`
		PageNumber int `json:"page_number"`
		PageSize   int `json:"page_size"`
	} `json:"pagination"`
	Data []kiwifySale `json:"data"`
}

// GetSalesSummary pagina /v1/sales filtrado pelo product_id (AUTOMATISA) na
// janela [from, to] e agrega por status e por plano. Cap de páginas evita
// varredura infinita se a paginação vier inconsistente.
func (c *APIClient) GetSalesSummary(ctx context.Context, from, to time.Time) (*SalesSummary, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	const pageSize = 100
	const maxPages = 200 // 20k vendas — muito acima do volume do AUTOMATISA
	sum := &SalesSummary{}
	planIdx := map[string]int{}

	for page := 1; page <= maxPages; page++ {
		q := url.Values{}
		q.Set("product_id", c.productID)
		q.Set("start_date", from.Format("2006-01-02"))
		q.Set("end_date", to.Format("2006-01-02"))
		q.Set("page_size", fmt.Sprintf("%d", pageSize))
		q.Set("page_number", fmt.Sprintf("%d", page))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			kiwifyAPIBase+"/sales?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("x-kiwify-account-id", c.accountID)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("kiwify sales request: %w", err)
		}
		var pg salesPage
		decErr := json.NewDecoder(resp.Body).Decode(&pg)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("kiwify sales http %d", resp.StatusCode)
		}
		if decErr != nil {
			return nil, fmt.Errorf("kiwify sales decode: %w", decErr)
		}

		for _, s := range pg.Data {
			sum.TotalCount++
			switch s.Status {
			case "paid":
				sum.PaidCount++
				sum.GrossPaidCents += s.NetAmount
				plan := s.Product.PlanName
				if plan == "" {
					plan = "—"
				}
				if i, ok := planIdx[plan]; ok {
					sum.ByPlan[i].GrossCents += s.NetAmount
					sum.ByPlan[i].Count++
				} else {
					planIdx[plan] = len(sum.ByPlan)
					sum.ByPlan = append(sum.ByPlan, PlanAgg{Plan: plan, GrossCents: s.NetAmount, Count: 1})
				}
			case "refunded", "chargedback":
				sum.RefundedCount++
			case "refused":
				sum.RefusedCount++
			case "waiting_payment":
				sum.WaitingCount++
			}
		}

		if len(pg.Data) == 0 || page*pageSize >= pg.Pagination.Count {
			break
		}
	}
	return sum, nil
}
