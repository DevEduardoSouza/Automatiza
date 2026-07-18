package monitoring

import (
	"context"
	"strings"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
)

// ---------------------------------------------------------------------------
// Mocks locais (CTA pool + cupom) — ver dispatch_text_test.go para os demais.
// ---------------------------------------------------------------------------

// mockCTARepo embute a interface para só implementar o que o pool usa; os
// métodos não sobrescritos causam panic se chamados (não deveriam neste caminho).
type mockCTARepo struct {
	repository.CTARepository
	GetUserCTAsFn  func(ctx context.Context, userID string, filters map[string]interface{}) ([]models.CTA, error)
	GetCTAsByIDsFn func(ctx context.Context, ctaIDs []string, userID string) ([]models.CTA, error)
}

func (m *mockCTARepo) GetUserCTAs(ctx context.Context, userID string, filters map[string]interface{}) ([]models.CTA, error) {
	return m.GetUserCTAsFn(ctx, userID, filters)
}

func (m *mockCTARepo) GetCTAsByIDs(ctx context.Context, ctaIDs []string, userID string) ([]models.CTA, error) {
	return m.GetCTAsByIDsFn(ctx, ctaIDs, userID)
}

type mockCouponApplier struct {
	called       bool
	lastPrice    float64
	lastStore    string
	lastOfficial bool
	fn           func(offerPrice float64, offerStore string, productIsOfficial bool) *CouponResult
}

func (m *mockCouponApplier) ApplyCoupon(_ context.Context, _ *models.Automation, offerPrice float64, offerStore string, productIsOfficial bool) (*CouponResult, error) {
	m.called = true
	m.lastPrice = offerPrice
	m.lastStore = offerStore
	m.lastOfficial = productIsOfficial
	if m.fn != nil {
		return m.fn(offerPrice, offerStore, productIsOfficial), nil
	}
	return &CouponResult{}, nil
}

// shopeeConvertAndEnrich devolve um mock Shopee que converte o link (com
// itemID/shopID) e enriquece o produto — o caminho feliz do espelho.
func shopeeConvertAndEnrich(name string, price, originalPrice float64, isMall bool) *mockShopeeService {
	return &mockShopeeService{
		GenerateAffiliateLinkFn: func(_ context.Context, _, _ string) (*models.ShopeeAffiliateLink, error) {
			return &models.ShopeeAffiliateLink{
				AffiliateLink: "https://s.shopee.com.br/aff123",
				ItemID:        123, ShopID: 456,
				ImageURL: "https://cf.shopee.com.br/img.jpg",
			}, nil
		},
		GetProductFn: func(_ context.Context, _, _ int64, _ string) (*models.ShopeeOffer, error) {
			return &models.ShopeeOffer{Name: name, Price: price, OriginalPrice: originalPrice, Discount: 50, IsMall: isMall}, nil
		},
	}
}

func templateAutomation() *models.Automation {
	zero := 0
	// O default global de monitoring_send_as_image virou TRUE (rollout 18/07).
	// Estas cascatas validam o card de preview, então mantêm send_as_image OFF
	// explicitamente — é o comportamento que testam.
	sendAsImageOff := false
	return &models.Automation{
		ID: "auto-1", UserID: "user-1", Name: "espelho",
		Stores:                []string{"shopee"},
		SendGroups:            []string{"dest@g.us"},
		Status:                "active",
		DelaySeconds:          &zero,
		DispatchTextID:        strPtr("dt-1"),
		MonitoringSendAsImage: &sendAsImageOff,
	}
}

func sourceMessage() *models.IncomingMessage {
	return &models.IncomingMessage{
		InstanceName:           "inst1",
		RemoteJID:              "source@g.us",
		Body:                   "Oferta original SOURCE_MARKER\nhttps://shopee.com.br/product-i.123.456",
		LinkPreviewTitle:       "TITULO DA ORIGEM",
		LinkPreviewDescription: "descricao da origem",
	}
}

var sourceLinks = []string{"https://shopee.com.br/product-i.123.456"}

// captureCard instala mocks de envio que capturam o card e sinalizam se
// image/text foram usados.
func captureCard(s *monitoringService, card **whatsapp.SendLinkPreviewInput, imageCalled, textCalled *bool) {
	s.waProvider = &mockWAProvider{
		SendLinkCustomPreviewFn: func(_ context.Context, in whatsapp.SendLinkPreviewInput) (*whatsapp.MessageResult, error) {
			captured := in
			*card = &captured
			return &whatsapp.MessageResult{ID: "card-1"}, nil
		},
		SendImageFn: func(_ context.Context, _ whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
			*imageCalled = true
			return &whatsapp.MessageResult{ID: "img-1"}, nil
		},
		SendTextFn: func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
			*textCalled = true
			return &whatsapp.MessageResult{ID: "txt-1"}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// Cascata de envio: título do card (asserções Vary-safe — Title não passa por
// text.Vary, só o corpo passa).
// ---------------------------------------------------------------------------

// COM template + produto enriquecido: o título do card é o NOME DO PRODUTO,
// nunca o título da origem (que poderia carregar o CTA de outro afiliado).
func TestCascade_Template_CardTitleFromProduct(t *testing.T) {
	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: "TEMPLATE_MARKER {nome_produto}\n{link_afiliado}"}}
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = shopeeConvertAndEnrich("Produto XYZ", 50.0, 100.0, false)
		captureCard(s, &card, &imageCalled, &textCalled)
	})

	if err := svc.processAutomation(context.Background(), templateAutomation(), sourceMessage(), sourceLinks, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("esperava card de preview (send_as_image OFF)")
	}
	if card.Title != "Produto XYZ" {
		t.Errorf("card.Title = %q, want %q (nome do produto enriquecido)", card.Title, "Produto XYZ")
	}
	if card.Title == "TITULO DA ORIGEM" {
		t.Error("card.Title NÃO pode vir da origem quando há template")
	}
	if card.Description != "" {
		t.Errorf("card.Description = %q, want vazio (não vazar preview da origem)", card.Description)
	}
}

// COM template mas enriquecimento falha (sem nome de produto): mantém o card
// (decisão do produto), com título vazio — WhatsApp monta o card do link.
func TestCascade_Template_EnrichFails_CardTitleEmpty(t *testing.T) {
	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: "TEMPLATE_MARKER {link_afiliado}"}}
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = &mockShopeeService{
			GenerateAffiliateLinkFn: func(_ context.Context, _, _ string) (*models.ShopeeAffiliateLink, error) {
				return &models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com.br/aff123", ItemID: 123, ShopID: 456}, nil
			},
			GetProductFn: func(_ context.Context, _, _ int64, _ string) (*models.ShopeeOffer, error) {
				return nil, context.DeadlineExceeded
			},
		}
		captureCard(s, &card, &imageCalled, &textCalled)
	})

	if err := svc.processAutomation(context.Background(), templateAutomation(), sourceMessage(), sourceLinks, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("esperava card mesmo sem enrich (título vazio, não suprimir)")
	}
	if card.Title != "" {
		t.Errorf("card.Title = %q, want vazio (sem nome de produto → WhatsApp monta do link)", card.Title)
	}
}

// SEM template (repasse histórico): o título do card é o da origem — regressão
// do comportamento atual, que NÃO pode mudar.
func TestCascade_NoTemplate_CardTitleFromSource(t *testing.T) {
	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	svc := newTestService(t, func(s *monitoringService) {
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = shopeeConvertAndEnrich("Produto XYZ", 50.0, 100.0, false)
		captureCard(s, &card, &imageCalled, &textCalled)
	})

	auto := templateAutomation()
	auto.DispatchTextID = nil // sem texto de disparo → repasse

	if err := svc.processAutomation(context.Background(), auto, sourceMessage(), sourceLinks, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("esperava card de preview")
	}
	if card.Title != "TITULO DA ORIGEM" {
		t.Errorf("card.Title = %q, want título da origem (repasse não muda)", card.Title)
	}
}

// Gate OFF com dispatch_text_id setado: a feature é ignorada — cai no repasse
// (título do card = origem), provando o kill-switch por env.
func TestCascade_GateOff_IgnoresTemplate(t *testing.T) {
	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: "TEMPLATE_MARKER {nome_produto}"}}
		s.dispatchText = parseDispatchTextGate("off")
		s.shopeeService = shopeeConvertAndEnrich("Produto XYZ", 50.0, 100.0, false)
		captureCard(s, &card, &imageCalled, &textCalled)
	})

	if err := svc.processAutomation(context.Background(), templateAutomation(), sourceMessage(), sourceLinks, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("esperava card de preview")
	}
	if card.Title != "TITULO DA ORIGEM" {
		t.Errorf("gate off: card.Title = %q, want origem (template ignorado)", card.Title)
	}
}

// Allowlist: usuário FORA da lista não ativa a feature.
func TestCascade_Allowlist_UserOutside(t *testing.T) {
	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: "TEMPLATE_MARKER {nome_produto}"}}
		s.dispatchText = parseDispatchTextGate("outro-user,mais-um")
		s.shopeeService = shopeeConvertAndEnrich("Produto XYZ", 50.0, 100.0, false)
		captureCard(s, &card, &imageCalled, &textCalled)
	})

	if err := svc.processAutomation(context.Background(), templateAutomation(), sourceMessage(), sourceLinks, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil || card.Title != "TITULO DA ORIGEM" {
		t.Errorf("usuário fora da allowlist não deveria ativar o template (card.Title inesperado)")
	}
}

// ---------------------------------------------------------------------------
// dispatchTextGate — parsing e enabledFor.
// ---------------------------------------------------------------------------

func TestDispatchTextGate(t *testing.T) {
	cases := []struct {
		raw    string
		user   string
		wantOn bool
	}{
		{"", "u1", false},
		{"off", "u1", false},
		{"OFF", "u1", false},
		{"all", "qualquer", true},
		{"ALL", "qualquer", true},
		{"u1,u2", "u1", true},
		{"u1,u2", "u3", false},
		{" u1 , u2 ", "u2", true},
	}
	for _, c := range cases {
		if got := parseDispatchTextGate(c.raw).enabledFor(c.user); got != c.wantOn {
			t.Errorf("parseDispatchTextGate(%q).enabledFor(%q) = %v, want %v", c.raw, c.user, got, c.wantOn)
		}
	}
}

// ---------------------------------------------------------------------------
// Cupom no monitor (nível render — sem Vary): preço>0, preço 0, não-aplicado.
// ---------------------------------------------------------------------------

func renderShopeeTemplate(t *testing.T, body string, shopee *mockShopeeService, coupon CouponApplier, cta repository.CTARepository) monitorDispatchRender {
	t.Helper()
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: body}}
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = shopee
		s.couponHandler = coupon
		s.ctaRepo = cta
	})
	auto := templateAutomation()
	auto.ApplyCouponRules = true
	aff := &models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com.br/aff123", ItemID: 123, ShopID: 456}
	return svc.renderMonitorDispatchText(context.Background(), auto, aff, "https://shopee.com.br/prod", "", "", "")
}

// Preço enriquecido > 0 e regra casa: injeta bloco de cupom (append, sem
// {cupom_bloco}) e sobrescreve o preço pelo descontado.
func TestCoupon_AppliedOverridesPriceAndAppendsBlock(t *testing.T) {
	applier := &mockCouponApplier{fn: func(_ float64, _ string, _ bool) *CouponResult {
		return &CouponResult{Applied: true, CouponCode: "C10", CouponText: "\n\n🎟️ CUPOM_BLOCO_X", DiscountedPrice: 45.0}
	}}
	out := renderShopeeTemplate(t, "{nome_produto}\nPor {preco_formatado}\n{link_afiliado}",
		shopeeConvertAndEnrich("Produto XYZ", 50.0, 100.0, false), applier, nil)

	if !applier.called {
		t.Fatal("couponHandler deveria ter sido chamado (preço > 0)")
	}
	if applier.lastPrice != 50.0 || applier.lastStore != "shopee" || applier.lastOfficial {
		t.Errorf("args do cupom = (%.2f, %q, %v), want (50.00, shopee, false)", applier.lastPrice, applier.lastStore, applier.lastOfficial)
	}
	if !strings.Contains(out.Body, "45,00") {
		t.Errorf("esperava preço descontado 45,00; got:\n%s", out.Body)
	}
	if strings.Contains(out.Body, "50,00") {
		t.Errorf("preço original 50,00 não deveria aparecer (foi descontado); got:\n%s", out.Body)
	}
	if !strings.Contains(out.Body, "CUPOM_BLOCO_X") {
		t.Errorf("bloco de cupom deveria ter sido anexado; got:\n%s", out.Body)
	}
}

// Enriquecimento falha → preço 0 → cupom NÃO é consultado (guard offerPrice>0).
func TestCoupon_PriceZeroSkipsCoupon(t *testing.T) {
	applier := &mockCouponApplier{fn: func(_ float64, _ string, _ bool) *CouponResult {
		return &CouponResult{Applied: true, CouponText: "\n\nNAO_DEVE_APARECER"}
	}}
	shopee := &mockShopeeService{
		GenerateAffiliateLinkFn: func(_ context.Context, _, _ string) (*models.ShopeeAffiliateLink, error) {
			return &models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com.br/aff123", ItemID: 123, ShopID: 456}, nil
		},
		GetProductFn: func(_ context.Context, _, _ int64, _ string) (*models.ShopeeOffer, error) {
			return nil, context.DeadlineExceeded
		},
	}
	out := renderShopeeTemplate(t, "OFERTA {link_afiliado}", shopee, applier, nil)

	if applier.called {
		t.Error("couponHandler NÃO deveria ser chamado com preço 0")
	}
	if strings.Contains(out.Body, "NAO_DEVE_APARECER") {
		t.Errorf("cupom não deveria ser aplicado sem preço; got:\n%s", out.Body)
	}
	if !strings.Contains(out.Body, "https://s.shopee.com.br/aff123") {
		t.Errorf("template deveria renderizar com o link mesmo sem preço; got:\n%s", out.Body)
	}
}

// Regra não casou (Applied=false): nenhum bloco de cupom e preço intacto.
func TestCoupon_NotApplied(t *testing.T) {
	applier := &mockCouponApplier{fn: func(_ float64, _ string, _ bool) *CouponResult {
		return &CouponResult{Applied: false}
	}}
	out := renderShopeeTemplate(t, "{nome_produto} Por {preco_formatado}", shopeeConvertAndEnrich("Produto XYZ", 50.0, 0, false), applier, nil)

	if !strings.Contains(out.Body, "50,00") {
		t.Errorf("sem cupom, preço deveria ser o original 50,00; got:\n%s", out.Body)
	}
}

// Sem couponHandler injetado (wiring de cupom ausente): fail-open, render OK.
func TestCoupon_NilHandler(t *testing.T) {
	out := renderShopeeTemplate(t, "{nome_produto} {link_afiliado}", shopeeConvertAndEnrich("Produto XYZ", 50.0, 0, false), nil, nil)
	if !strings.Contains(out.Body, "Produto XYZ") {
		t.Errorf("render deveria funcionar sem couponHandler; got:\n%s", out.Body)
	}
}

// ---------------------------------------------------------------------------
// CTA pool no monitor (nível render).
// ---------------------------------------------------------------------------

// Pool ativo com CTA Shopee: auto-append (template sem {cta}).
func TestCTA_PoolAppended(t *testing.T) {
	cta := &mockCTARepo{
		GetUserCTAsFn: func(_ context.Context, _ string, _ map[string]interface{}) ([]models.CTA, error) {
			return []models.CTA{
				{Message: "CTA_POOL_SHOPEE", Platform: models.CTAPlatformShopee, IsActive: true},
				{Message: "CTA_POOL_ML", Platform: models.CTAPlatformMercadoLivre, IsActive: true},
			}, nil
		},
	}
	out := renderShopeeTemplate(t, "OFERTA {link_afiliado}", shopeeConvertAndEnrich("Produto XYZ", 50.0, 0, false), nil, cta)
	if !strings.Contains(out.Body, "CTA_POOL_SHOPEE") {
		t.Errorf("esperava CTA do pool (plataforma Shopee) anexado; got:\n%s", out.Body)
	}
	if strings.Contains(out.Body, "CTA_POOL_ML") {
		t.Errorf("CTA de ML não deveria aparecer num link Shopee; got:\n%s", out.Body)
	}
}

// Pool falha (repo erro) → fallback para CTAs inline da automation.
func TestCTA_PoolErrorFallsBackToInline(t *testing.T) {
	cta := &mockCTARepo{
		GetUserCTAsFn: func(_ context.Context, _ string, _ map[string]interface{}) ([]models.CTA, error) {
			return nil, context.DeadlineExceeded
		},
	}
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: "OFERTA {link_afiliado}"}}
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = shopeeConvertAndEnrich("Produto XYZ", 50.0, 0, false)
		s.ctaRepo = cta
	})
	auto := templateAutomation()
	auto.CTAs = []string{"CTA_INLINE_FALLBACK"}
	aff := &models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com.br/aff123", ItemID: 123, ShopID: 456}
	out := svc.renderMonitorDispatchText(context.Background(), auto, aff, "https://shopee.com.br/prod", "", "", "")
	if !strings.Contains(out.Body, "CTA_INLINE_FALLBACK") {
		t.Errorf("esperava fallback para CTA inline quando o pool falha; got:\n%s", out.Body)
	}
}
