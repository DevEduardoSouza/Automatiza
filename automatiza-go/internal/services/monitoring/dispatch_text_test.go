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

// ---------------------------------------------------------------------------
// Helpers de substituição/relay (ordem determinística + trecho de cupom).
// ---------------------------------------------------------------------------

// Uma URL curta que é PREFIXO de outra (mesma campanha com querystring de
// tracking) não pode ser trocada antes da longa: soldaria o link afiliado
// dentro da querystring da outra e corromperia as duas.
func TestReplaceAffiliateLinksLongestFirst(t *testing.T) {
	const (
		short = "https://shopee.com.br/m/cupom"
		long  = "https://shopee.com.br/m/cupom?mmp_pid=fonte"
	)
	replacements := []affiliateLinkReplacement{
		{Original: short, Affiliate: "https://s.shopee.com.br/AFFSHORT"},
		{Original: long, Affiliate: "https://s.shopee.com.br/AFFLONG"},
	}
	body := "Curta: " + short + "\nLonga: " + long

	got := replaceAffiliateLinksLongestFirst(body, replacements)

	want := "Curta: https://s.shopee.com.br/AFFSHORT\nLonga: https://s.shopee.com.br/AFFLONG"
	if got != want {
		t.Errorf("substituição fora de ordem.\n got: %q\nwant: %q", got, want)
	}
	if strings.Contains(got, "mmp_pid=fonte") {
		t.Errorf("querystring da fonte sobreviveu soldada ao link afiliado: %q", got)
	}
	if replacements[0].Original != short || replacements[1].Original != long {
		t.Error("o slice do caller não pode ser reordenado in-place")
	}

	t.Run("original vazio é ignorado e affiliate vazio remove", func(t *testing.T) {
		got := replaceAffiliateLinksLongestFirst("A https://amazon.com.br/x B", []affiliateLinkReplacement{
			{Original: "", Affiliate: "NUNCA"},
			{Original: "https://amazon.com.br/x", Affiliate: ""},
		})
		if got != "A  B" {
			t.Errorf("got %q, want %q", got, "A  B")
		}
	})
}

// O corpo canônico do repasse é o ponto ÚNICO do pipeline regras → links →
// lojas bloqueadas: relay puro, wrapper de cupom e trecho misto leem daqui, e
// não podem divergir.
func TestBuildMonitoringRelayBody(t *testing.T) {
	source := "PROMO DO CANAL X\nhttps://shopee.com.br/prod-i.1.2\nCompre tambem https://amazon.com.br/dp/A1\nRODAPE_FONTE"
	rules := &models.MonitoringTextRules{
		Substituicoes: []models.Substituicao{{De: "PROMO DO CANAL X", Para: "PROMO NOSSA"}},
		RemoverTermos: []string{"RODAPE_FONTE"},
	}
	replacements := []affiliateLinkReplacement{
		{Original: "https://shopee.com.br/prod-i.1.2", Affiliate: "https://s.shopee.com.br/AFF1"},
	}

	got := buildMonitoringRelayBody(source, rules, replacements, []string{"https://amazon.com.br/dp/A1"})

	if !strings.Contains(got, "PROMO NOSSA") || strings.Contains(got, "PROMO DO CANAL X") {
		t.Errorf("substituição do operador não aplicada; got:\n%s", got)
	}
	if strings.Contains(got, "RODAPE_FONTE") {
		t.Errorf("termo removido pelo operador sobreviveu; got:\n%s", got)
	}
	if !strings.Contains(got, "https://s.shopee.com.br/AFF1") || strings.Contains(got, "shopee.com.br/prod-i.1.2") {
		t.Errorf("link não foi trocado pelo afiliado; got:\n%s", got)
	}
	if strings.Contains(got, "amazon.com.br") {
		t.Errorf("loja bloqueada deveria ter sido removida; got:\n%s", got)
	}
	if strings.HasSuffix(got, "RODAPE") || got != strings.TrimRight(got, "") {
		t.Error("buildMonitoringRelayBody não pode normalizar — o caller normaliza uma vez só")
	}
	if source != "PROMO DO CANAL X\nhttps://shopee.com.br/prod-i.1.2\nCompre tambem https://amazon.com.br/dp/A1\nRODAPE_FONTE" {
		t.Error("a string da fonte não pode ser mutada")
	}
}

// O trecho de cupom que o template de produto descartaria: parágrafos inteiros
// do relay JÁ PROCESSADO que contêm um link Shopee não-produto convertido.
func TestExtractShopeeCouponParagraphs(t *testing.T) {
	coupons := []affiliateLinkReplacement{
		{Original: "https://shopee.com.br/m/cupom1", Affiliate: "https://s.shopee.com.br/AFFC1"},
		{Original: "https://shopee.com.br/m/cupom2", Affiliate: "https://s.shopee.com.br/AFFC2"},
	}

	tests := []struct {
		name  string
		body  string
		reps  []affiliateLinkReplacement
		want  string
		wantN int
	}{
		{
			name: "codigo e link em linhas diferentes do mesmo paragrafo",
			body: "OFERTA\nhttps://s.shopee.com.br/AFFPROD\n\nCUPOM SAVE20\nResgate: https://s.shopee.com.br/AFFC1",
			reps: coupons,
			want: "CUPOM SAVE20\nResgate: https://s.shopee.com.br/AFFC1",
		},
		{
			name: "dois cupons em paragrafos distintos mantem a ordem da fonte",
			body: "CUPOM A https://s.shopee.com.br/AFFC1\n\nMEIO SEM CUPOM\n\nCUPOM B https://s.shopee.com.br/AFFC2",
			reps: coupons,
			want: "CUPOM A https://s.shopee.com.br/AFFC1\n\nCUPOM B https://s.shopee.com.br/AFFC2",
		},
		{
			name: "dois links no mesmo paragrafo devolvem o paragrafo uma vez",
			body: "CUPONS:\nhttps://s.shopee.com.br/AFFC1\nhttps://s.shopee.com.br/AFFC2",
			reps: coupons,
			want: "CUPONS:\nhttps://s.shopee.com.br/AFFC1\nhttps://s.shopee.com.br/AFFC2",
		},
		{
			name: "paragrafo sem link de cupom e omitido",
			body: "SO PRODUTO\nhttps://s.shopee.com.br/AFFPROD",
			reps: coupons,
			want: "",
		},
		{
			name: "chaves literais intactas",
			body: "CUPOM {SAVE20} e {{literal}}\nhttps://s.shopee.com.br/AFFC1",
			reps: coupons,
			want: "CUPOM {SAVE20} e {{literal}}\nhttps://s.shopee.com.br/AFFC1",
		},
		{
			name: "CRLF e linha em branco com espacos separam paragrafos",
			body: "PRODUTO\r\nhttps://s.shopee.com.br/AFFPROD\r\n \r\nCUPOM\r\nhttps://s.shopee.com.br/AFFC1",
			reps: coupons,
			want: "CUPOM\nhttps://s.shopee.com.br/AFFC1",
		},
		{
			name: "cupom removido pelas regras nao e ressuscitado",
			body: "SO PRODUTO\nhttps://s.shopee.com.br/AFFPROD",
			reps: coupons,
			want: "",
		},
		{
			name: "produto e cupom no mesmo paragrafo anexam o paragrafo inteiro",
			body: "OFERTA https://s.shopee.com.br/AFFPROD\nCUPOM https://s.shopee.com.br/AFFC1",
			reps: coupons,
			want: "OFERTA https://s.shopee.com.br/AFFPROD\nCUPOM https://s.shopee.com.br/AFFC1",
		},
		{
			name: "corpo vazio",
			body: "   ",
			reps: coupons,
			want: "",
		},
		{
			name: "sem replacements de cupom",
			body: "CUPOM https://s.shopee.com.br/AFFC1",
			reps: nil,
			want: "",
		},
		{
			name: "replacement com affiliate vazio nao seleciona nada",
			body: "TEXTO QUALQUER",
			reps: []affiliateLinkReplacement{{Original: "https://amazon.com.br/x"}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractShopeeCouponParagraphs(tt.body, tt.reps); got != tt.want {
				t.Errorf("extractShopeeCouponParagraphs()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

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

	rendered := svc.renderMonitorDispatchText(context.Background(), automation, aff, "https://shopee.com.br/prod", "", "", "", false)
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

	out := svc.renderMonitorDispatchText(context.Background(), automation, aff, "https://shopee.com.br/x", "", "", "", false).Body
	if !strings.Contains(out, "https://s.shopee.com.br/aff999") {
		t.Fatalf("mesmo sem preço, deveria renderizar com o link de afiliado; got:\n%s", out)
	}
	if !strings.Contains(out, "OFERTA") {
		t.Fatalf("deveria manter o corpo do template; got:\n%s", out)
	}
}

// Fix (bug do cupom ML 20/07): quando o link ML é de cupom/campanha (preview sem
// produto — Title/Price vazios), o template de OFERTA saía com "Por: R$ " vazio.
// Agora devolve Body vazio → o caller (usingTemplate=false) cai no repasse
// histórico, que preserva o texto-fonte e troca o link pelo afiliado do dono.
func TestRenderMonitorDispatchText_MLNonProductPreview_FallsBackToSourceRelay(t *testing.T) {
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{
			dt: &models.DispatchText{Body: "🛍️ {nome_produto}\n💸 Por {preco_formatado}"},
		}
		s.mlService = &mockMLService{
			GetProductPreviewFn: func(_ context.Context, _, _ string) (*models.MLProductPreview, error) {
				// Página de cupom/campanha: preview vazio não-nil, sem produto.
				return &models.MLProductPreview{Permalink: "https://mercadolivre.com.br/sec/ABC", Currency: "BRL"}, nil
			},
		}
	})
	automation := &models.Automation{ID: "auto-ml", UserID: "user-1", DispatchTextID: strPtr("dt-1")}

	rendered := svc.renderMonitorDispatchText(context.Background(), automation, nil,
		"", "https://mercadolivre.com.br/sec/ABC", "https://mercadolivre.com.br/sec/ABC", "", false)

	if rendered.Body != "" {
		t.Fatalf("preview sem produto deveria devolver Body vazio (repasse histórico); got: %q", rendered.Body)
	}
}

// Produto ML real: o template continua renderizando com título e preço — o fix
// não pode degradar oferta legítima.
func TestRenderMonitorDispatchText_MLProductPreview_StillRenders(t *testing.T) {
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{
			dt: &models.DispatchText{Body: "🛍️ {nome_produto}\n💸 Por {preco_formatado}\n🛒 {link_afiliado}"},
		}
		s.mlService = &mockMLService{
			GetProductPreviewFn: func(_ context.Context, _, _ string) (*models.MLProductPreview, error) {
				return &models.MLProductPreview{
					Title: "Cafeteira ML", Price: 210.50,
					Permalink: "https://mercadolivre.com.br/p/MLB1", Image: "https://img/x.jpg",
				}, nil
			},
		}
	})
	automation := &models.Automation{ID: "auto-ml", UserID: "user-1", DispatchTextID: strPtr("dt-1")}

	rendered := svc.renderMonitorDispatchText(context.Background(), automation, nil,
		"", "https://mercadolivre.com.br/p/MLB1", "https://mercadolivre.com.br/sec/OWN", "", false)

	if !strings.Contains(rendered.Body, "Cafeteira ML") {
		t.Fatalf("preview de produto deveria renderizar o template com o título; got: %q", rendered.Body)
	}
	if !strings.Contains(rendered.Body, "210,50") {
		t.Fatalf("template ML deveria conter o preço; got: %q", rendered.Body)
	}
	if rendered.PreviewURL != "https://mercadolivre.com.br/sec/OWN" {
		t.Fatalf("PreviewURL deveria ser o afiliado ML; got: %q", rendered.PreviewURL)
	}
}

// ---------------------------------------------------------------------------
// Roteamento produto × cupom Shopee.
// ---------------------------------------------------------------------------

// ctaPoolRepo devolve um pool com um CTA por plataforma, para provar QUAL foi
// escolhido no roteamento.
func ctaPoolRepo() *mockCTARepo {
	return &mockCTARepo{
		GetUserCTAsFn: func(_ context.Context, _ string, _ map[string]interface{}) ([]models.CTA, error) {
			return []models.CTA{
				{Message: "CTA_POOL_SHOPEE", Platform: models.CTAPlatformShopee, IsActive: true},
				{Message: "CTA_POOL_ML", Platform: models.CTAPlatformMercadoLivre, IsActive: true},
			}, nil
		},
	}
}

// Cupom/campanha Shopee sem produto: não existe template possível (a API de
// afiliado não expõe lookup de cupom), então o renderer sinaliza relay em vez
// de renderizar um card com {nome_produto}/{preco} vazios.
func TestRenderMonitorDispatchText_ShopeeCouponOnlySignalsRelay(t *testing.T) {
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{
			dt: &models.DispatchText{Body: "TEMPLATE_MARKER {nome_produto}\nPor {preco_formatado}"},
		}
		s.ctaRepo = ctaPoolRepo()
	})
	automation := &models.Automation{ID: "auto-cupom", UserID: "user-1", DispatchTextID: strPtr("dt-1")}

	rendered := svc.renderMonitorDispatchText(context.Background(), automation, nil, "", "", "", "", true)

	if rendered.Body != "" {
		t.Errorf("sem produto não pode haver template; got: %q", rendered.Body)
	}
	if !rendered.ShopeeCouponRelay {
		t.Error("deveria sinalizar ShopeeCouponRelay para o caller montar o aviso")
	}
	if rendered.CTA != "CTA_POOL_SHOPEE" {
		t.Errorf("CTA = %q, want o CTA de plataforma Shopee", rendered.CTA)
	}
}

// Guard do renderer Shopee: sem os dois IDs não há produto para descrever.
// Renderizar mesmo assim era a causa do "Por: R$ " vazio nos grupos.
func TestRenderShopeeDispatchTemplate_InvalidAffiliateNeverRenders(t *testing.T) {
	cases := []struct {
		name string
		aff  *models.ShopeeAffiliateLink
	}{
		{"nil", nil},
		{"link vazio", &models.ShopeeAffiliateLink{ItemID: 1, ShopID: 2}},
		{"itemID zero", &models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com.br/aff", ShopID: 2}},
		{"shopID zero", &models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com.br/aff", ItemID: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getProductCalled := false
			applier := &mockCouponApplier{}
			svc := newTestService(t, func(s *monitoringService) {
				s.shopeeService = &mockShopeeService{
					GetProductFn: func(_ context.Context, _, _ int64, _ string) (*models.ShopeeOffer, error) {
						getProductCalled = true
						return &models.ShopeeOffer{Name: "NAO_DEVE_APARECER", Price: 10}, nil
					},
				}
				s.couponHandler = applier
			})
			automation := &models.Automation{ID: "auto-1", UserID: "user-1", ApplyCouponRules: true}

			got := svc.renderShopeeDispatchTemplate(context.Background(), automation,
				"{nome_produto} {link_afiliado}", "CTA", tc.aff, "https://shopee.com.br/x")

			if got != (monitorDispatchRender{}) {
				t.Errorf("esperava struct vazio; got: %+v", got)
			}
			if getProductCalled {
				t.Error("GetProduct não pode ser chamado sem itemID/shopID válidos")
			}
			if applier.called {
				t.Error("cupom não pode ser avaliado sem produto")
			}
		})
	}
}

// Produto ML + cupom Shopee: o template ML descreve o produto de verdade e tem
// prioridade sobre o aviso de cupom.
func TestRenderMonitorDispatchText_MLProductPreferredOverShopeeCoupon(t *testing.T) {
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{
			dt: &models.DispatchText{Body: "🛍️ {nome_produto}\n💸 Por {preco_formatado}\n🛒 {link_afiliado}"},
		}
		s.mlService = &mockMLService{
			GetProductPreviewFn: func(_ context.Context, _, _ string) (*models.MLProductPreview, error) {
				return &models.MLProductPreview{
					Title: "Cafeteira ML", Price: 210.50, Permalink: "https://mercadolivre.com.br/p/MLB1",
				}, nil
			},
		}
		s.ctaRepo = ctaPoolRepo()
	})
	automation := &models.Automation{ID: "auto-mix", UserID: "user-1", DispatchTextID: strPtr("dt-1")}

	rendered := svc.renderMonitorDispatchText(context.Background(), automation, nil,
		"", "https://mercadolivre.com.br/p/MLB1", "https://mercadolivre.com.br/sec/OWN", "", true)

	if !strings.Contains(rendered.Body, "Cafeteira ML") {
		t.Fatalf("esperava o template ML renderizado; got: %q", rendered.Body)
	}
	if rendered.ShopeeCouponRelay {
		t.Error("com produto ML o caller usa o template, não o aviso de cupom-only")
	}
	if rendered.PreviewURL != "https://mercadolivre.com.br/sec/OWN" {
		t.Errorf("PreviewURL = %q, want o afiliado ML", rendered.PreviewURL)
	}
	if !strings.Contains(rendered.Body, "CTA_POOL_ML") {
		t.Errorf("esperava o CTA de plataforma ML no template; got: %q", rendered.Body)
	}
}

// Link ML que também é cupom (preview sem produto) + cupom Shopee: nenhum
// template é possível, então cai no aviso de cupom Shopee com o CTA Shopee.
func TestRenderMonitorDispatchText_MLNonProductFallsThroughToShopeeCoupon(t *testing.T) {
	svc := newTestService(t, func(s *monitoringService) {
		s.settingsRepo = &mockSettingsRepo{dt: &models.DispatchText{Body: "TEMPLATE_MARKER {nome_produto}"}}
		s.mlService = &mockMLService{
			GetProductPreviewFn: func(_ context.Context, _, _ string) (*models.MLProductPreview, error) {
				return &models.MLProductPreview{Permalink: "https://mercadolivre.com.br/sec/ABC", Currency: "BRL"}, nil
			},
		}
		s.ctaRepo = ctaPoolRepo()
	})
	automation := &models.Automation{ID: "auto-mix2", UserID: "user-1", DispatchTextID: strPtr("dt-1")}

	rendered := svc.renderMonitorDispatchText(context.Background(), automation, nil,
		"", "https://mercadolivre.com.br/sec/ABC", "https://mercadolivre.com.br/sec/OWN", "", true)

	if rendered.Body != "" {
		t.Errorf("ML sem produto não pode renderizar template; got: %q", rendered.Body)
	}
	if !rendered.ShopeeCouponRelay {
		t.Error("deveria cair no aviso de cupom Shopee")
	}
	if rendered.CTA != "CTA_POOL_SHOPEE" {
		t.Errorf("CTA = %q, want o CTA Shopee", rendered.CTA)
	}
}
