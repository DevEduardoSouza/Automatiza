package monitoring

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/shopee"
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
	called          bool
	lastPrice       float64
	lastStore       string
	lastOfficial    bool
	lastCategoryIDs []int64
	fn              func(offerPrice float64, offerStore string, productIsOfficial bool) *CouponResult
}

func (m *mockCouponApplier) ApplyCoupon(_ context.Context, _ *models.Automation, offerPrice float64, offerStore string, productIsOfficial bool, productCategoryIDs []int64) (*CouponResult, error) {
	m.called = true
	m.lastPrice = offerPrice
	m.lastStore = offerStore
	m.lastOfficial = productIsOfficial
	m.lastCategoryIDs = productCategoryIDs
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
	return svc.renderMonitorDispatchText(context.Background(), auto, aff, "https://shopee.com.br/prod", "", "", "", false)
}

// O caminho Shopee do espelho repassa ao matcher os IDs oficiais de
// categoria que o GetProduct enriqueceu — é o que habilita regra de cupom
// por categoria no monitoramento.
func TestCoupon_ShopeePassesProductCategoryIDs(t *testing.T) {
	applier := &mockCouponApplier{}
	shopee := shopeeConvertAndEnrich("Produto XYZ", 50.0, 100.0, false)
	inner := shopee.GetProductFn
	shopee.GetProductFn = func(ctx context.Context, itemID, shopID int64, userID string) (*models.ShopeeOffer, error) {
		offer, err := inner(ctx, itemID, shopID, userID)
		if err != nil {
			return nil, err
		}
		offer.ProductCategoryIDs = []int64{100001, 100002}
		return offer, nil
	}
	renderShopeeTemplate(t, "{nome_produto} {link_afiliado}", shopee, applier, nil)

	if !applier.called {
		t.Fatal("couponHandler deveria ter sido chamado (preço > 0)")
	}
	if len(applier.lastCategoryIDs) != 2 ||
		applier.lastCategoryIDs[0] != 100001 || applier.lastCategoryIDs[1] != 100002 {
		t.Errorf("categorias repassadas = %v, want [100001 100002]", applier.lastCategoryIDs)
	}
}

// Produto Shopee sem categoria na API: nil chega ao matcher, que então só
// enxerga regras genéricas (mesmo comportamento de antes de 075).
func TestCoupon_ShopeeWithoutCategoriesPassesNil(t *testing.T) {
	applier := &mockCouponApplier{}
	renderShopeeTemplate(t, "{nome_produto} {link_afiliado}",
		shopeeConvertAndEnrich("Produto XYZ", 50.0, 100.0, false), applier, nil)

	if !applier.called {
		t.Fatal("couponHandler deveria ter sido chamado (preço > 0)")
	}
	if applier.lastCategoryIDs != nil {
		t.Errorf("categorias repassadas = %v, want nil", applier.lastCategoryIDs)
	}
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
	out := svc.renderMonitorDispatchText(context.Background(), auto, aff, "https://shopee.com.br/prod", "", "", "", false)
	if !strings.Contains(out.Body, "CTA_INLINE_FALLBACK") {
		t.Errorf("esperava fallback para CTA inline quando o pool falha; got:\n%s", out.Body)
	}
}

// ---------------------------------------------------------------------------
// Cupom/campanha Shopee no texto de disparo (link convertido SEM itemID/shopID).
// ---------------------------------------------------------------------------

const (
	couponURL      = "https://shopee.com.br/user/voucher-wallet"
	couponAffURL   = "https://s.shopee.com.br/AFFCUPOM"
	coupon2URL     = "https://shopee.com.br/m/promo-frete"
	coupon2AffURL  = "https://s.shopee.com.br/AFFPROMOFRETE"
	productURL     = "https://shopee.com.br/product-i.123.456"
	productAffURL  = "https://s.shopee.com.br/aff123"
	couponTemplate = "TEMPLATE_MARKER {nome_produto}\nPor {preco_formatado}\n{link_afiliado}"
)

// couponAutomation é a templateAutomation com humanização OFF: text.Vary é
// no-op, então as asserções sobre o CORPO enviado são exatas (as URLs já eram
// protegidas, o texto plano não).
func couponAutomation() *models.Automation {
	auto := templateAutomation()
	off := "off"
	auto.HumanizeMode = &off
	return auto
}

// shopeeByURL converte cada link de acordo com o mapa (afiliado, itemID,
// shopID): itemID/shopID zerados = cupom/campanha (o que a API devolve quando
// a URL não é de produto).
func shopeeByURL(m map[string]models.ShopeeAffiliateLink, product *models.ShopeeOffer) *mockShopeeService {
	return &mockShopeeService{
		GenerateAffiliateLinkFn: func(_ context.Context, url, _ string) (*models.ShopeeAffiliateLink, error) {
			aff, ok := m[url]
			if !ok {
				return nil, fmt.Errorf("mock: url inesperada %q", url)
			}
			return &aff, nil
		},
		GetProductFn: func(_ context.Context, _, _ int64, _ string) (*models.ShopeeOffer, error) {
			if product == nil {
				return nil, context.DeadlineExceeded
			}
			return product, nil
		},
	}
}

// Cupom sem produto: nada para o template descrever, então sai CTA + texto da
// fonte (com o link já convertido) + aviso de validade — nunca o template com
// "Por: R$ " vazio.
func TestCascade_ShopeeCouponOnlyUsesCouponNotice(t *testing.T) {
	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	sends := 0
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: couponTemplate}}
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = shopeeByURL(map[string]models.ShopeeAffiliateLink{
			couponURL: {AffiliateLink: couponAffURL},
		}, nil)
		s.ctaRepo = ctaPoolRepo()
		captureCard(s, &card, &imageCalled, &textCalled)
		inner := s.waProvider.(*mockWAProvider).SendLinkCustomPreviewFn
		s.waProvider.(*mockWAProvider).SendLinkCustomPreviewFn = func(ctx context.Context, in whatsapp.SendLinkPreviewInput) (*whatsapp.MessageResult, error) {
			sends++
			return inner(ctx, in)
		}
	})

	msg := sourceMessage()
	msg.Body = "CUPOM {SAVE20}\n\nResgate aqui:\n" + couponURL

	if err := svc.processAutomation(context.Background(), couponAutomation(), msg, []string{couponURL}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("esperava card de preview")
	}
	if sends != 1 {
		t.Errorf("envios = %d, want 1", sends)
	}

	body := card.Text
	if !strings.HasPrefix(body, "CTA_POOL_SHOPEE") {
		t.Errorf("corpo deveria começar pelo CTA Shopee; got:\n%s", body)
	}
	if !strings.Contains(body, "CUPOM {SAVE20}") {
		t.Errorf("código/texto do cupom deveria sobreviver (inclusive as chaves); got:\n%s", body)
	}
	if !strings.Contains(body, couponAffURL) {
		t.Errorf("link convertido deveria estar no corpo; got:\n%s", body)
	}
	if !strings.HasSuffix(body, text.ShopeeCouponLimitedNotice) {
		t.Errorf("corpo deveria terminar no aviso fixo; got:\n%s", body)
	}
	if strings.Contains(body, couponURL) {
		t.Errorf("URL original não pode sair (comissão da fonte); got:\n%s", body)
	}
	if strings.Contains(body, "TEMPLATE_MARKER") || strings.Contains(body, "Por ") {
		t.Errorf("template de produto não pode ser usado num cupom; got:\n%s", body)
	}
	if card.Title != "TITULO DA ORIGEM" {
		t.Errorf("card.Title = %q, want o título da origem (não há template)", card.Title)
	}
	if card.URL != couponAffURL {
		t.Errorf("card.URL = %q, want o link convertido", card.URL)
	}
}

// mixedSourceBody monta a mensagem com cupom ANTES do produto (a ordem que
// quebrava o template: o cupom, sem IDs, era eleito como "o produto").
func mixedSourceBody(first, second string) string {
	return "CUPOM {SAVE20}\n" + first + "\n\nOFERTA DA FONTE\n" + second
}

func runMixedCascade(t *testing.T, body string, links []string) *whatsapp.SendLinkPreviewInput {
	t.Helper()
	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: couponTemplate}}
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = shopeeByURL(map[string]models.ShopeeAffiliateLink{
			couponURL:  {AffiliateLink: couponAffURL},
			coupon2URL: {AffiliateLink: coupon2AffURL},
			productURL: {AffiliateLink: productAffURL, ItemID: 123, ShopID: 456},
		}, &models.ShopeeOffer{Name: "Produto XYZ", Price: 50.0, OriginalPrice: 100.0})
		captureCard(s, &card, &imageCalled, &textCalled)
	})

	msg := sourceMessage()
	msg.Body = body
	if err := svc.processAutomation(context.Background(), couponAutomation(), msg, links, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("esperava card de preview")
	}
	return card
}

// assertMixed valida o desfecho comum: template do produto + parágrafo do cupom
// anexado, sem URL da fonte e com o card apontando para o PRODUTO.
func assertMixed(t *testing.T, card *whatsapp.SendLinkPreviewInput) {
	t.Helper()
	body := card.Text
	if !strings.Contains(body, "Produto XYZ") || !strings.Contains(body, "50,00") {
		t.Errorf("template do produto deveria estar preenchido; got:\n%s", body)
	}
	if card.URL != productAffURL {
		t.Errorf("card.URL = %q, want o afiliado do PRODUTO (não o do cupom)", card.URL)
	}
	if card.Title != "Produto XYZ" {
		t.Errorf("card.Title = %q, want o nome do produto", card.Title)
	}
	if !strings.Contains(body, "CUPOM {SAVE20}") {
		t.Errorf("parágrafo do cupom deveria ter sido anexado; got:\n%s", body)
	}
	if strings.Count(body, couponAffURL) != 1 {
		t.Errorf("link do cupom deveria aparecer exatamente 1x; got:\n%s", body)
	}
	if strings.Count(body, productAffURL) != 1 {
		t.Errorf("link do produto deveria aparecer exatamente 1x; got:\n%s", body)
	}
	if strings.Contains(body, couponURL) || strings.Contains(body, productURL) {
		t.Errorf("nenhuma URL original pode sair; got:\n%s", body)
	}
	if strings.Contains(body, "OFERTA DA FONTE") {
		t.Errorf("parágrafo sem cupom não deveria ser anexado; got:\n%s", body)
	}
}

// Cupom antes do produto: o produto (IDs válidos) ganha o template, e o
// parágrafo do cupom é anexado em vez de sumir.
func TestCascade_MixedShopeeCouponBeforeProductKeepsProductTemplate(t *testing.T) {
	card := runMixedCascade(t, mixedSourceBody(couponURL, productURL), []string{couponURL, productURL})
	assertMixed(t, card)
}

// Mesma mensagem com o produto primeiro: desfecho funcional idêntico (a ordem
// da fonte não pode mudar o que sai).
func TestCascade_MixedShopeeProductBeforeCouponSameResult(t *testing.T) {
	body := "OFERTA DA FONTE\n" + productURL + "\n\nCUPOM {SAVE20}\n" + couponURL
	card := runMixedCascade(t, body, []string{productURL, couponURL})
	assertMixed(t, card)
}

// Dois cupons em parágrafos distintos: ambos anexados, na ordem da fonte.
func TestCascade_MixedTwoShopeeCouponsPreservesParagraphOrder(t *testing.T) {
	body := "OFERTA DA FONTE\n" + productURL +
		"\n\nCUPOM A\n" + couponURL +
		"\n\nCUPOM B\n" + coupon2URL
	card := runMixedCascade(t, body, []string{productURL, couponURL, coupon2URL})

	out := card.Text
	if !strings.Contains(out, "Produto XYZ") {
		t.Errorf("template do produto deveria estar presente; got:\n%s", out)
	}
	if strings.Count(out, "Produto XYZ") != 1 {
		t.Errorf("template deveria ser renderizado 1x; got:\n%s", out)
	}
	idxA, idxB := strings.Index(out, "CUPOM A"), strings.Index(out, "CUPOM B")
	if idxA < 0 || idxB < 0 {
		t.Fatalf("ambos os parágrafos de cupom deveriam ser anexados; got:\n%s", out)
	}
	if idxA > idxB {
		t.Errorf("ordem dos cupons deveria seguir a fonte (A antes de B); got:\n%s", out)
	}
	for _, link := range []string{productAffURL, couponAffURL, coupon2AffURL} {
		if strings.Count(out, link) != 1 {
			t.Errorf("link %s deveria aparecer 1x; got:\n%s", link, out)
		}
	}
	for _, link := range []string{productURL, couponURL, coupon2URL} {
		if strings.Contains(out, link) {
			t.Errorf("URL original %s não pode sair; got:\n%s", link, out)
		}
	}
}

// Produto ML + cupom Shopee: template ML (produto real) com o parágrafo do
// cupom Shopee anexado — não o aviso de cupom-only.
func TestCascade_MLProductAndShopeeCouponUsesMLTemplate(t *testing.T) {
	const mlURL = "https://mercadolivre.com.br/p/MLB1"
	const mlAffURL = "https://mercadolivre.com.br/sec/OWN"

	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: couponTemplate}}
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = shopeeByURL(map[string]models.ShopeeAffiliateLink{
			couponURL: {AffiliateLink: couponAffURL},
		}, nil)
		s.mlService = &mockMLService{
			CreateAffiliateLinkFn: func(_ context.Context, _, _ string) (*models.MLAffiliateLink, error) {
				return &models.MLAffiliateLink{AffiliateLink: mlAffURL}, nil
			},
			GetProductPreviewFn: func(_ context.Context, _, _ string) (*models.MLProductPreview, error) {
				return &models.MLProductPreview{Title: "Cafeteira ML", Price: 210.50, Permalink: mlURL}, nil
			},
		}
		captureCard(s, &card, &imageCalled, &textCalled)
	})

	auto := couponAutomation()
	auto.Stores = []string{"shopee", "mercadolivre"}
	msg := sourceMessage()
	msg.Body = "CUPOM {SAVE20}\n" + couponURL + "\n\nOFERTA ML\n" + mlURL

	if err := svc.processAutomation(context.Background(), auto, msg, []string{couponURL, mlURL}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("esperava card de preview")
	}

	out := card.Text
	if !strings.Contains(out, "Cafeteira ML") || !strings.Contains(out, "210,50") {
		t.Errorf("esperava o template ML preenchido; got:\n%s", out)
	}
	if card.URL != mlAffURL {
		t.Errorf("card.URL = %q, want o afiliado do produto ML", card.URL)
	}
	if !strings.Contains(out, "CUPOM {SAVE20}") || !strings.Contains(out, couponAffURL) {
		t.Errorf("parágrafo do cupom Shopee deveria ter sido anexado; got:\n%s", out)
	}
	if !strings.Contains(out, mlAffURL) {
		t.Errorf("link ML convertido deveria estar no corpo; got:\n%s", out)
	}
	if strings.Contains(out, text.ShopeeCouponLimitedNotice) {
		t.Errorf("com template ML não se usa o aviso de cupom-only; got:\n%s", out)
	}
}

// Regra do operador removeu o parágrafo do cupom: o helper NÃO o ressuscita —
// nenhum fallback sintético recoloca um link que o cliente mandou tirar.
func TestCascade_CouponRemovedByMonitoringRulesIsNotResurrected(t *testing.T) {
	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: couponTemplate}}
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = shopeeByURL(map[string]models.ShopeeAffiliateLink{
			couponURL:  {AffiliateLink: couponAffURL},
			productURL: {AffiliateLink: productAffURL, ItemID: 123, ShopID: 456},
		}, &models.ShopeeOffer{Name: "Produto XYZ", Price: 50.0})
		captureCard(s, &card, &imageCalled, &textCalled)
	})

	auto := couponAutomation()
	auto.MonitoringTextRules = &models.MonitoringTextRules{
		RemoverTermos: []string{"CUPOM {SAVE20}", couponURL},
	}
	msg := sourceMessage()
	msg.Body = mixedSourceBody(couponURL, productURL)

	if err := svc.processAutomation(context.Background(), auto, msg, []string{couponURL, productURL}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("esperava card de preview")
	}

	out := card.Text
	if !strings.Contains(out, "Produto XYZ") {
		t.Errorf("template do produto deveria ter sido enviado; got:\n%s", out)
	}
	if strings.Contains(out, "SAVE20") || strings.Contains(out, couponAffURL) || strings.Contains(out, couponURL) {
		t.Errorf("cupom removido pelas regras não pode voltar; got:\n%s", out)
	}
	if strings.Contains(out, text.ShopeeCouponLimitedNotice) {
		t.Errorf("sem cupom no corpo não há aviso a anexar; got:\n%s", out)
	}
}

// Conta sem Open API (10035): a conversão nem acontece, então não há sinal
// estruturado para classificar cupom — segue o relay simples de sempre.
func TestCascade_NoAPIAccessKeepsPlainRelayWithoutCouponNotice(t *testing.T) {
	var card *whatsapp.SendLinkPreviewInput
	var imageCalled, textCalled bool
	sends := 0
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: couponTemplate}}
		s.dispatchText = parseDispatchTextGate("all")
		s.shopeeService = &mockShopeeService{
			GenerateAffiliateLinkFn: func(_ context.Context, _, _ string) (*models.ShopeeAffiliateLink, error) {
				return nil, shopee.ErrShopeeNoAPIAccess
			},
		}
		s.ctaRepo = ctaPoolRepo()
		captureCard(s, &card, &imageCalled, &textCalled)
		s.waProvider.(*mockWAProvider).SendTextFn = func(_ context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
			sends++
			textCalled = true
			if !strings.Contains(in.Text, "CUPOM {SAVE20}") || !strings.Contains(in.Text, couponURL) {
				t.Errorf("relay simples deveria repassar o corpo original; got:\n%s", in.Text)
			}
			if strings.Contains(in.Text, text.ShopeeCouponLimitedNotice) {
				t.Errorf("sem conversão não há classificação de cupom; got:\n%s", in.Text)
			}
			if strings.Contains(in.Text, "CTA_POOL_SHOPEE") || strings.Contains(in.Text, "TEMPLATE_MARKER") {
				t.Errorf("relay histórico não aplica CTA/template; got:\n%s", in.Text)
			}
			return &whatsapp.MessageResult{ID: "txt-1"}, nil
		}
	})

	msg := sourceMessage()
	msg.Body = "CUPOM {SAVE20}\n\nResgate aqui:\n" + couponURL

	if err := svc.processAutomation(context.Background(), couponAutomation(), msg, []string{couponURL}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card != nil {
		t.Fatalf("sem link afiliado não há card de preview; got URL %q", card.URL)
	}
	if sends != 1 {
		t.Errorf("envios de texto = %d, want 1", sends)
	}
}
