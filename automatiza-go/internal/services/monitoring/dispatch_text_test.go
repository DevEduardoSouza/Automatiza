package monitoring

import (
	"context"
	"strings"
	"testing"

	"automatiza-go/internal/models"
)

// mockSettingsRepo satisfaz repository.SettingsRepository; só GetActiveDispatchTextByID
// é exercitado (retorna dt). O resto é stub.
type mockSettingsRepo struct {
	dt *models.DispatchText
}

func (m *mockSettingsRepo) Get(context.Context, ...string) (models.SettingsMap, error) {
	return nil, nil
}
func (m *mockSettingsRepo) GetAll(context.Context) (models.SettingsMap, error) { return nil, nil }
func (m *mockSettingsRepo) Update(context.Context, string, interface{}) error  { return nil }
func (m *mockSettingsRepo) GetDispatchText(context.Context, string) (*models.DispatchText, error) {
	return m.dt, nil
}
func (m *mockSettingsRepo) GetDispatchTextsByUser(context.Context, string) ([]models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) GetActiveDispatchTextByID(context.Context, string, string) (*models.DispatchText, error) {
	return m.dt, nil
}
func (m *mockSettingsRepo) GetActiveDispatchTextsByUser(context.Context, string) ([]models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) CreateDispatchText(context.Context, string, *models.DispatchText) (*models.DispatchText, error) {
	return m.dt, nil
}
func (m *mockSettingsRepo) UpdateDispatchText(context.Context, string, map[string]interface{}) error {
	return nil
}
func (m *mockSettingsRepo) DeleteDispatchText(context.Context, string, string) error { return nil }

func strPtr(s string) *string { return &s }

// Caso do cliente: monitor Shopee com texto de disparo setado. O template deve
// renderizar COM preço — o monitor enriquece via GetProduct (itemId/shopId do
// passo do afiliado), não sai "Por: R$ " vazio.
func TestRenderMonitorDispatchText_ShopeeFillsPrice(t *testing.T) {
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{
			dt: &models.DispatchText{Body: "🛍️ {nome_produto}\n💸 Por {preco_formatado}\n🛒 Compre: {link_afiliado}"},
		}
		s.shopeeService = &mockShopeeService{
			GetProductFn: func(_ context.Context, _, _ int64, _ string) (*models.ShopeeOffer, error) {
				return &models.ShopeeOffer{
					Name: "Fone Bluetooth XYZ", Price: 99.90, OriginalPrice: 199.90, Discount: 50,
					Image: "https://img/product.jpg",
				}, nil
			},
		}
	})

	automation := &models.Automation{ID: "auto-1", UserID: "user-1", DispatchTextID: strPtr("dt-1")}
	aff := &models.ShopeeAffiliateLink{
		AffiliateLink: "https://s.shopee.com.br/aff123", ItemID: 123, ShopID: 456, ImageURL: "https://img/aff.jpg",
	}

	rendered := svc.renderMonitorDispatchText(context.Background(), automation, aff, "https://shopee.com.br/prod", "", "", "")
	out := rendered.Body

	if !strings.Contains(out, "Fone Bluetooth XYZ") {
		t.Fatalf("template deveria conter o nome do produto; got:\n%s", out)
	}
	if !strings.Contains(out, "99,90") {
		t.Fatalf("template deveria conter o preço (99,90); got:\n%s", out)
	}
	if !strings.Contains(out, "https://s.shopee.com.br/aff123") {
		t.Fatalf("template deveria conter o link de afiliado; got:\n%s", out)
	}
	if rendered.CardTitle != "Fone Bluetooth XYZ" {
		t.Fatalf("CardTitle deveria ser o nome do produto enriquecido; got: %q", rendered.CardTitle)
	}
}

// GetProduct falhando: best-effort — ainda renderiza o template com o link de
// afiliado (preço vazio), em vez de retornar "" e cair no texto da fonte.
func TestRenderMonitorDispatchText_ShopeeEnrichFails_StillRenders(t *testing.T) {
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{
			dt: &models.DispatchText{Body: "OFERTA 🔥 Compre: {link_afiliado}"},
		}
		s.shopeeService = &mockShopeeService{
			GetProductFn: func(_ context.Context, _, _ int64, _ string) (*models.ShopeeOffer, error) {
				return nil, context.DeadlineExceeded
			},
		}
	})

	automation := &models.Automation{ID: "auto-1", UserID: "user-1", DispatchTextID: strPtr("dt-1")}
	aff := &models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com.br/aff999", ItemID: 1, ShopID: 2}

	out := svc.renderMonitorDispatchText(context.Background(), automation, aff, "https://shopee.com.br/x", "", "", "").Body
	if !strings.Contains(out, "https://s.shopee.com.br/aff999") {
		t.Fatalf("mesmo sem preço, deveria renderizar com o link de afiliado; got:\n%s", out)
	}
	if !strings.Contains(out, "OFERTA") {
		t.Fatalf("deveria manter o corpo do template; got:\n%s", out)
	}
}
