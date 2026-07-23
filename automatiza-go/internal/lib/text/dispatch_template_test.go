package text

import (
	"math"
	"strings"
	"testing"
)

func TestRenderMLCouponNotice(t *testing.T) {
	t.Parallel()

	body := "R$50 OFF acima de R$300\nhttps://mercadolivre.com.br/sec/ABC"
	got := RenderMLCouponNotice(body)

	if !strings.HasPrefix(got, "🎟️ *CUPOM MERCADO LIVRE* 🎟️\n\n") {
		t.Errorf("faltou o cabeçalho de cupom no início.\n got: %q", got)
	}
	if !strings.Contains(got, body) {
		t.Errorf("o corpo original deveria seguir verbatim.\n got: %q", got)
	}
	if RenderMLCouponNotice("") != "" {
		t.Errorf("corpo vazio deveria render \"\", got: %q", RenderMLCouponNotice(""))
	}
	if RenderMLCouponNotice("   ") != "" {
		t.Errorf("corpo só-espaços deveria render \"\", got: %q", RenderMLCouponNotice("   "))
	}
}

func TestRenderShopeeCouponNotice(t *testing.T) {
	t.Parallel()

	const notice = ShopeeCouponLimitedNotice

	tests := []struct {
		name string
		body string
		cta  string
		want string
	}{
		{
			name: "corpo vazio nao rende nada",
			body: "",
			cta:  "CTA QUALQUER",
			want: "",
		},
		{
			name: "corpo so espacos nao rende nada",
			body: "   ",
			cta:  "CTA QUALQUER",
			want: "",
		},
		{
			name: "sem cta e corpo mais aviso",
			body: "CUPOM {SAVE20}",
			cta:  "",
			want: "CUPOM {SAVE20}\n\n" + notice,
		},
		{
			name: "cta trimado antes do corpo",
			body: "CUPOM {SAVE20}",
			cta:  "  CTA CLIENTE  ",
			want: "CTA CLIENTE\n\nCUPOM {SAVE20}\n\n" + notice,
		},
		{
			name: "chaves literais sobrevivem",
			body: "Use {SAVE20} ou {{literal}}",
			cta:  "",
			want: "Use {SAVE20} ou {{literal}}\n\n" + notice,
		},
		{
			name: "multiline preserva ordem e conteudo",
			body: "Resgate aqui:\nhttps://s.shopee.com.br/abc",
			cta:  "CTA",
			want: "CTA\n\nResgate aqui:\nhttps://s.shopee.com.br/abc\n\n" + notice,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := RenderShopeeCouponNotice(tt.body, tt.cta); got != tt.want {
				t.Errorf("RenderShopeeCouponNotice()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{
			name:     "double braces syntax replaced",
			template: "Hello {{name}}, welcome to {{place}}!",
			vars:     map[string]string{"name": "Bruno", "place": "Automatiza"},
			want:     "Hello Bruno, welcome to Automatiza!",
		},
		{
			name:     "single braces syntax replaced",
			template: "Hello {name}, welcome to {place}!",
			vars:     map[string]string{"name": "Bruno", "place": "Automatiza"},
			want:     "Hello Bruno, welcome to Automatiza!",
		},
		{
			name:     "mixed braces syntax",
			template: "{{name}} likes {product}",
			vars:     map[string]string{"name": "Bruno", "product": "Coffee"},
			want:     "Bruno likes Coffee",
		},
		{
			name:     "unreplaced variables removed",
			template: "Hello {name}, {unknown} is here",
			vars:     map[string]string{"name": "Bruno"},
			want:     "Hello Bruno,  is here",
		},
		{
			name:     "unreplaced double braces removed",
			template: "Hello {{name}}, {{unknown}} is here",
			vars:     map[string]string{"name": "Bruno"},
			want:     "Hello Bruno,  is here",
		},
		{
			name:     "multiple newlines collapsed",
			template: "line1\n\n\n\nline2",
			vars:     map[string]string{},
			want:     "line1\n\nline2",
		},
		{
			name:     "trimmed",
			template: "  hello world  ",
			vars:     map[string]string{},
			want:     "hello world",
		},
		{
			name:     "empty vars map removes all variables",
			template: "{a} and {b} and {{c}}",
			vars:     map[string]string{},
			want:     "and  and",
		},
		{
			name:     "nil vars map removes all variables",
			template: "{a} and {b}",
			vars:     nil,
			want:     "and",
		},
		{
			name:     "empty template",
			template: "",
			vars:     map[string]string{"a": "b"},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RenderTemplate(tt.template, tt.vars)
			if got != tt.want {
				t.Errorf("RenderTemplate(%q, ...) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}
}

func TestRenderShopeeOfferTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		offer    ShopeeOfferVars
		check    func(t *testing.T, result string)
	}{
		{
			name:     "full offer all variables replaced",
			template: "{nome_produto} - {preco_formatado} at {loja} - {link_afiliado}",
			offer: ShopeeOfferVars{
				Title:         "Product A",
				Price:         199.90,
				Store:         "My Store",
				AffiliateLink: "https://shp.ee/abc",
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if !strings.Contains(result, "Product A") {
					t.Error("expected title in result")
				}
				if !strings.Contains(result, "R$ 199,90") {
					t.Error("expected formatted price in result")
				}
				if !strings.Contains(result, "My Store") {
					t.Error("expected store name in result")
				}
				if !strings.Contains(result, "https://shp.ee/abc") {
					t.Error("expected affiliate link in result")
				}
			},
		},
		{
			name:     "empty store defaults to Shopee",
			template: "{loja}",
			offer: ShopeeOfferVars{
				Title: "Product",
				Store: "",
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if result != "Shopee" {
					t.Errorf("expected 'Shopee', got %q", result)
				}
			},
		},
		{
			name:     "with coupon code adds coupon vars",
			template: "{couponCode} - {couponText}",
			offer: ShopeeOfferVars{
				Title:      "Product",
				CouponCode: "SAVE10",
				CouponText: "Use code SAVE10",
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if !strings.Contains(result, "SAVE10") {
					t.Error("expected coupon code in result")
				}
				if !strings.Contains(result, "Use code SAVE10") {
					t.Error("expected coupon text in result")
				}
			},
		},
		{
			name:     "without coupon coupon vars not in output",
			template: "product: {nome_produto} coupon: {couponCode}",
			offer: ShopeeOfferVars{
				Title:      "Product",
				CouponCode: "",
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if strings.Contains(result, "couponCode") {
					t.Error("expected couponCode variable to be removed")
				}
				if !strings.Contains(result, "Product") {
					t.Error("expected product title in result")
				}
			},
		},
		{
			name:     "zero price renders empty for price",
			template: "price: {preco_formatado}",
			offer: ShopeeOfferVars{
				Title: "Product",
				Price: 0,
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if strings.Contains(result, "R$") {
					t.Error("expected no R$ for zero price")
				}
			},
		},
		{
			name:     "english backward compat vars",
			template: "{title} at {store} for {price}",
			offer: ShopeeOfferVars{
				Title: "Widget",
				Price: 50.00,
				Store: "Test Shop",
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if !strings.Contains(result, "Widget") {
					t.Error("expected title via English var")
				}
				if !strings.Contains(result, "Test Shop") {
					t.Error("expected store via English var")
				}
				if !strings.Contains(result, "R$ 50,00") {
					t.Error("expected price via English var")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := RenderShopeeOfferTemplate(tt.template, tt.offer)
			tt.check(t, result)
		})
	}
}

func TestRenderMLOfferTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		offer    MLOfferVars
		check    func(t *testing.T, result string)
	}{
		{
			name:     "full offer all variables replaced",
			template: "{nome_produto} - {preco_formatado} at {loja} - {link_afiliado} {vendidos}",
			offer: MLOfferVars{
				Title:         "Product B",
				Price:         299.99,
				AffiliateLink: "https://ml.com.br/item",
				SoldCount:     1500,
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if !strings.Contains(result, "Product B") {
					t.Error("expected title in result")
				}
				if !strings.Contains(result, "R$ 299,99") {
					t.Error("expected formatted price in result")
				}
				if !strings.Contains(result, "Mercado Livre") {
					t.Error("expected 'Mercado Livre' as store")
				}
				if !strings.Contains(result, "https://ml.com.br/item") {
					t.Error("expected affiliate link in result")
				}
				if !strings.Contains(result, "mil vendidos") {
					t.Error("expected sold count in result")
				}
			},
		},
		{
			name:     "title with embedded price stripped",
			template: "{nome_produto}",
			offer: MLOfferVars{
				Title: "Product Name - R$ 199,90",
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if result != "Product Name" {
					t.Errorf("expected 'Product Name', got %q", result)
				}
			},
		},
		{
			name:     "store always Mercado Livre",
			template: "{loja} and {store}",
			offer: MLOfferVars{
				Title: "Product",
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if !strings.Contains(result, "Mercado Livre") {
					t.Error("expected 'Mercado Livre' in result")
				}
				// Both {loja} and {store} should be "Mercado Livre"
				count := strings.Count(result, "Mercado Livre")
				if count != 2 {
					t.Errorf("expected 'Mercado Livre' twice, found %d times in %q", count, result)
				}
			},
		},
		{
			name:     "discount and rating rendered",
			template: "{desconto}%% OFF - {avaliacao} stars",
			offer: MLOfferVars{
				Title:    "Product",
				Discount: 25,
				Rating:   4.5,
			},
			check: func(t *testing.T, result string) {
				t.Helper()
				if !strings.Contains(result, "25") {
					t.Error("expected discount in result")
				}
				if !strings.Contains(result, "4.5") {
					t.Error("expected rating in result")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := RenderMLOfferTemplate(tt.template, tt.offer)
			tt.check(t, result)
		})
	}
}

func TestFormatBRL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		price float64
		want  string
	}{
		{
			name:  "standard price",
			price: 199.90,
			want:  "R$199,90",
		},
		{
			name:  "price with thousands",
			price: 1234.56,
			want:  "R$1.234,56",
		},
		{
			name:  "zero returns empty",
			price: 0,
			want:  "",
		},
		{
			name:  "negative returns empty",
			price: -10.50,
			want:  "",
		},
		{
			name:  "small price",
			price: 0.99,
			want:  "R$0,99",
		},
		{
			name:  "whole number",
			price: 100.00,
			want:  "R$100,00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatBRL(tt.price)
			if got != tt.want {
				t.Errorf("FormatBRL(%v) = %q, want %q", tt.price, got, tt.want)
			}
		})
	}
}

func TestFormatBRLSpaced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		price float64
		want  string
	}{
		{
			name:  "standard price",
			price: 199.90,
			want:  "R$ 199,90",
		},
		{
			name:  "price with thousands",
			price: 1234.56,
			want:  "R$ 1.234,56",
		},
		{
			name:  "zero returns empty",
			price: 0,
			want:  "",
		},
		{
			name:  "negative returns empty",
			price: -5.00,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatBRLSpaced(tt.price)
			if got != tt.want {
				t.Errorf("FormatBRLSpaced(%v) = %q, want %q", tt.price, got, tt.want)
			}
		})
	}
}

func TestFormatBRNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value float64
		want  string
	}{
		{
			name:  "standard number",
			value: 199.90,
			want:  "199,90",
		},
		{
			name:  "number with thousands",
			value: 1234.56,
			want:  "1.234,56",
		},
		{
			name:  "zero returns empty",
			value: 0,
			want:  "",
		},
		{
			name:  "negative returns empty",
			value: -10,
			want:  "",
		},
		{
			name:  "small number",
			value: 9.99,
			want:  "9,99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatBRNumber(tt.value)
			if got != tt.want {
				t.Errorf("FormatBRNumber(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestFormatPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		price float64
		want  string
	}{
		{
			name:  "alias for FormatBRLSpaced",
			price: 199.90,
			want:  "R$ 199,90",
		},
		{
			name:  "zero returns empty",
			price: 0,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatPrice(tt.price)
			if got != tt.want {
				t.Errorf("FormatPrice(%v) = %q, want %q", tt.price, got, tt.want)
			}
		})
	}
}

func TestFormatDiscount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		discount float64
		want     string
	}{
		{
			name:     "50 percent",
			discount: 50.0,
			want:     "50% OFF",
		},
		{
			name:     "25 percent",
			discount: 25.0,
			want:     "25% OFF",
		},
		{
			name:     "zero returns empty",
			discount: 0,
			want:     "",
		},
		{
			name:     "negative returns empty",
			discount: -10,
			want:     "",
		},
		{
			name:     "100 percent",
			discount: 100.0,
			want:     "100% OFF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatDiscount(tt.discount)
			if got != tt.want {
				t.Errorf("FormatDiscount(%v) = %q, want %q", tt.discount, got, tt.want)
			}
		})
	}
}

func TestFormatBrazilianNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value float64
		want  string
	}{
		{
			name:  "number with thousands",
			value: 1234.56,
			want:  "1.234,56",
		},
		{
			name:  "number under 1000",
			value: 999.99,
			want:  "999,99",
		},
		{
			name:  "one million",
			value: 1000000.00,
			want:  "1.000.000,00",
		},
		{
			name:  "small number",
			value: 5.50,
			want:  "5,50",
		},
		{
			name:  "zero",
			value: 0,
			want:  "0,00",
		},
		{
			name:  "exactly 1000",
			value: 1000.00,
			want:  "1.000,00",
		},
		{
			name:  "large number",
			value: 99999.99,
			want:  "99.999,99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatBrazilianNumber(tt.value)
			if got != tt.want {
				t.Errorf("formatBrazilianNumber(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestStripEmbeddedPriceFromTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "title with spaced price",
			title: "Product - R$ 199,90",
			want:  "Product",
		},
		{
			name:  "title with unspaced price",
			title: "Product - R$199.90",
			want:  "Product",
		},
		{
			name:  "title without price",
			title: "No price here",
			want:  "No price here",
		},
		{
			name:  "title with complex price",
			title: "Great Product - R$ 1.234,56",
			want:  "Great Product",
		},
		{
			name:  "empty title",
			title: "",
			want:  "",
		},
		{
			name:  "only price",
			title: "- R$ 100,00",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := StripEmbeddedPriceFromTitle(tt.title)
			if got != tt.want {
				t.Errorf("StripEmbeddedPriceFromTitle(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestFormatDiscountPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		discount float64
		want     string
	}{
		{
			name:     "50 percent",
			discount: 50,
			want:     "50",
		},
		{
			name:     "zero returns empty",
			discount: 0,
			want:     "",
		},
		{
			name:     "negative returns empty",
			discount: -10,
			want:     "",
		},
		{
			name:     "fractional rounded",
			discount: 33.7,
			want:     "34",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatDiscountPercent(tt.discount)
			if got != tt.want {
				t.Errorf("formatDiscountPercent(%v) = %q, want %q", tt.discount, got, tt.want)
			}
		})
	}
}

func TestFormatSoldCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
		want  string
	}{
		{
			name:  "zero returns empty",
			count: 0,
			want:  "",
		},
		{
			name:  "negative returns empty",
			count: -5,
			want:  "",
		},
		{
			name:  "500 vendidos",
			count: 500,
			want:  "+500 vendidos",
		},
		{
			name:  "1000 becomes 1 mil",
			count: 1000,
			want:  "+1 mil vendidos",
		},
		{
			name:  "1500 becomes 1,5 mil",
			count: 1500,
			want:  "+1,5 mil vendidos",
		},
		{
			name:  "10000 becomes 10 mil",
			count: 10000,
			want:  "+10 mil vendidos",
		},
		{
			name:  "2000 becomes 2 mil",
			count: 2000,
			want:  "+2 mil vendidos",
		},
		{
			name:  "small number under 1000",
			count: 50,
			want:  "+50 vendidos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatSoldCount(tt.count)
			if got != tt.want {
				t.Errorf("formatSoldCount(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestFormatRating(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rating float64
		want   string
	}{
		{
			name:   "4.5 rating",
			rating: 4.5,
			want:   "4.5",
		},
		{
			name:   "5.0 rating",
			rating: 5.0,
			want:   "5.0",
		},
		{
			name:   "zero returns empty",
			rating: 0,
			want:   "",
		},
		{
			name:   "negative returns empty",
			rating: -1,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatRating(tt.rating)
			if got != tt.want {
				t.Errorf("formatRating(%v) = %q, want %q", tt.rating, got, tt.want)
			}
		})
	}
}

func TestFormatReviewCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
		want  string
	}{
		{
			name:  "100 reviews",
			count: 100,
			want:  "100",
		},
		{
			name:  "zero returns empty",
			count: 0,
			want:  "",
		},
		{
			name:  "negative returns empty",
			count: -5,
			want:  "",
		},
		{
			name:  "large count",
			count: 5000,
			want:  "5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatReviewCount(tt.count)
			if got != tt.want {
				t.Errorf("formatReviewCount(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestDefaultShopeeTemplate(t *testing.T) {
	t.Parallel()

	tmpl := DefaultShopeeTemplate()
	if tmpl == "" {
		t.Error("DefaultShopeeTemplate() returned empty string")
	}

	requiredVars := []string{
		"{nome_produto}",
		"{preco_formatado}",
		"{preco_original_formatado}",
		"{link_afiliado}",
	}
	for _, v := range requiredVars {
		if !strings.Contains(tmpl, v) {
			t.Errorf("DefaultShopeeTemplate() missing variable %q", v)
		}
	}
}

func TestDefaultMLTemplate(t *testing.T) {
	t.Parallel()

	tmpl := DefaultMLTemplate()
	if tmpl == "" {
		t.Error("DefaultMLTemplate() returned empty string")
	}

	requiredVars := []string{
		"{nome_produto}",
		"{preco_formatado}",
		"{preco_original_formatado}",
		"{link_afiliado}",
	}
	for _, v := range requiredVars {
		if !strings.Contains(tmpl, v) {
			t.Errorf("DefaultMLTemplate() missing variable %q", v)
		}
	}
}

func TestExtractTemplateVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{
			name:     "double brace variables",
			template: "{{name}} and {{price}}",
			want:     []string{"name", "price"},
		},
		{
			name:     "single brace variables",
			template: "{name} and {price}",
			want:     []string{"name", "price"},
		},
		{
			name:     "mixed brace variables",
			template: "{{name}} and {price}",
			want:     []string{"name", "price"},
		},
		{
			name:     "deduplicates same variable",
			template: "{name} and {name} and {{name}}",
			want:     []string{"name"},
		},
		{
			name:     "no variables",
			template: "plain text",
			want:     nil,
		},
		{
			name:     "empty template",
			template: "",
			want:     nil,
		},
		{
			name:     "complex template",
			template: "{nome_produto} - {preco_formatado} ({desconto}) link: {link_afiliado}",
			want:     []string{"nome_produto", "preco_formatado", "desconto", "link_afiliado"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractTemplateVariables(tt.template)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("ExtractTemplateVariables(%q) returned %d vars, want %d: got=%v", tt.template, len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractTemplateVariables(%q)[%d] = %q, want %q", tt.template, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TemplateUsesVariable é o gate de opt-in do CTA: ele é a única coisa que
// separa "o operador pediu CTA neste texto" de "existe CTA cadastrada na
// conta". Reusa ExtractTemplateVariables de propósito — um regex paralelo
// divergiria do renderer e o gate autorizaria o que o render não preenche.
func TestTemplateUsesVariable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		variable string
		want     bool
	}{
		{"chave simples", "Oferta {cta} fim", VarCTA, true},
		{"chave dupla", "Oferta {{cta}} fim", VarCTA, true},
		{"template inteiro é a variável", "{cta}", VarCTA, true},
		{"duas ocorrências", "{cta}\n{nome_produto}\n{cta}", VarCTA, true},
		{"ocorrências em sintaxes diferentes", "{cta} e {{cta}}", VarCTA, true},
		// "{cta}}" é um token {cta} reconhecido pelo parser mais um '}' literal.
		// O renderer trata exatamente assim, então o gate acompanha em vez de
		// criar um segundo validador de sintaxe que divergiria dele.
		{"chave extra à direita", "{cta}}", VarCTA, true},
		{"literal sem chaves", "compre agora cta", VarCTA, false},
		{"caixa diferente", "{CTA}", VarCTA, false},
		{"com espaços dentro das chaves", "{ cta }", VarCTA, false},
		{"prefixo de outra variável", "{cta_extra}", VarCTA, false},
		{"spintax simples não é placeholder", "{cta|aproveite}", VarCTA, false},
		{"spintax dupla não é placeholder", "{{cta|aproveite}}", VarCTA, false},
		{"abertura sem fechamento", "{cta", VarCTA, false},
		{"chave dupla mal fechada", "{{cta}", VarCTA, false},
		{"template vazio", "", VarCTA, false},
		{"variável vazia nunca casa", "{cta}", "", false},
		{"só outras variáveis", "{nome_produto} {preco_formatado}", VarCTA, false},
		{"a variável procurada é outra", "{nome_produto}", VarNomeProduto, true},
		// Os defaults reais do produto NÃO trazem CTA — nenhum disparo que caia
		// neles pode disparar consulta ao repositório de CTA.
		{"default Shopee atual", DefaultShopeeTemplate(), VarCTA, false},
		{"default ML atual", DefaultMLTemplate(), VarCTA, false},
		// Caso sintético: se um default um dia ganhar a variável, o gate passa a
		// autorizar sobre ele — sem precisar mexer no default de produção.
		{"default sintético com cta", DefaultShopeeTemplate() + "\n{cta}", VarCTA, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := TemplateUsesVariable(tt.template, tt.variable); got != tt.want {
				t.Errorf("TemplateUsesVariable(%q, %q) = %v, want %v", tt.template, tt.variable, got, tt.want)
			}
		})
	}
}

// Cardinalidade da CTA no render: UM valor é escolhido por mensagem e o
// renderer o aplica em cada ocorrência autorada. O que não pode existir é a
// ocorrência N+1 vinda de append pós-render (removido na melhoria 6).
func TestRenderTemplate_CTAOccurrences(t *testing.T) {
	t.Parallel()

	const cta = "CTA_MARKER"

	t.Run("um placeholder rende uma ocorrência", func(t *testing.T) {
		t.Parallel()
		got := RenderTemplate("Oferta\n\n{cta}", map[string]string{VarCTA: cta})
		if n := strings.Count(got, cta); n != 1 {
			t.Errorf("ocorrências de CTA = %d, want 1.\n got: %q", n, got)
		}
	})

	t.Run("dois placeholders rendem o mesmo valor duas vezes", func(t *testing.T) {
		t.Parallel()
		got := RenderTemplate("{cta}\nOferta\n{{cta}}", map[string]string{VarCTA: cta})
		if n := strings.Count(got, cta); n != 2 {
			t.Errorf("ocorrências de CTA = %d, want 2 (uma por placeholder autorado).\n got: %q", n, got)
		}
	})

	t.Run("spintax não recebe o valor da CTA", func(t *testing.T) {
		t.Parallel()
		got := RenderTemplate("{cta} {cta|aproveite}", map[string]string{VarCTA: cta})
		if !strings.Contains(got, "{cta|aproveite}") {
			t.Errorf("spintax deveria sobreviver ao render para o text.Vary expandir depois.\n got: %q", got)
		}
		if n := strings.Count(got, cta); n != 1 {
			t.Errorf("ocorrências de CTA = %d, want 1 (spintax não conta).\n got: %q", n, got)
		}
	})

	t.Run("CTA vazia não deixa placeholder nem buraco", func(t *testing.T) {
		t.Parallel()
		got := RenderTemplate("Oferta\n\n{cta}\n\nRodapé", map[string]string{VarCTA: ""})
		if strings.Contains(got, "{cta}") {
			t.Errorf("placeholder não substituído vazou para o disparo.\n got: %q", got)
		}
		if strings.Contains(got, "\n\n\n") {
			t.Errorf("linha vazia da CTA deveria ter colapsado.\n got: %q", got)
		}
	})

	t.Run("tags mistas: só as variáveis exatas recebem valor", func(t *testing.T) {
		t.Parallel()
		got := RenderTemplate("{cta} {info} {cta_extra} {cta|x}", map[string]string{
			VarCTA:  cta,
			VarInfo: "INFO_MARKER",
		})
		if !strings.Contains(got, cta) || !strings.Contains(got, "INFO_MARKER") {
			t.Errorf("{cta} e {info} deveriam ter sido preenchidos.\n got: %q", got)
		}
		if strings.Contains(got, "{cta_extra}") {
			t.Errorf("{cta_extra} é placeholder desconhecido — deveria ser varrido.\n got: %q", got)
		}
		if !strings.Contains(got, "{cta|x}") {
			t.Errorf("spintax deveria sobreviver.\n got: %q", got)
		}
	})
}

// TestRenderMLOfferTemplate_EmptyOriginalPrice cobre o bug do "De ~~"
// pendurado: oferta ML sem preço cheio (OriginalPrice == 0) não pode deixar
// a linha do preço original riscado vazia no disparo.
func TestRenderMLOfferTemplate_EmptyOriginalPrice(t *testing.T) {
	t.Parallel()

	const tmpl = "🛍️ {nome_produto}\n\nDe ~{preco_original_formatado}~\n💸 Por {preco_formatado}\n\n🛒Compre aqui: {link_afiliado}"

	t.Run("sem preco original remove a linha De", func(t *testing.T) {
		t.Parallel()
		got := RenderMLOfferTemplate(tmpl, MLOfferVars{
			Title:         "Sérum Kérastase",
			Price:         382.0,
			OriginalPrice: 0,
			AffiliateLink: "https://meli.la/2mAwoEq",
		})
		if strings.Contains(got, "~~") {
			t.Errorf("riscado vazio '~~' presente:\n%s", got)
		}
		if strings.Contains(got, "De ~") {
			t.Errorf("linha 'De' pendurada nao foi removida:\n%s", got)
		}
		if !strings.Contains(got, "💸 Por R$ 382,00") {
			t.Errorf("preco atual sumiu:\n%s", got)
		}
		if !strings.Contains(got, "https://meli.la/2mAwoEq") {
			t.Errorf("link sumiu:\n%s", got)
		}
	})

	t.Run("com preco original mantem a linha De", func(t *testing.T) {
		t.Parallel()
		got := RenderMLOfferTemplate(tmpl, MLOfferVars{
			Title:         "Sérum Kérastase",
			Price:         382.0,
			OriginalPrice: 500.0,
			AffiliateLink: "https://meli.la/2mAwoEq",
		})
		if !strings.Contains(got, "De ~R$ 500,00~") {
			t.Errorf("linha 'De' com preco original deveria permanecer:\n%s", got)
		}
		if !strings.Contains(got, "💸 Por R$ 382,00") {
			t.Errorf("preco atual sumiu:\n%s", got)
		}
	})
}

// TestRenderShopeeOfferTemplate_NoDiscount cobre o caso "De R$ X por R$ X":
// produto Shopee sem desconto (OriginalPrice <= Price). Os serviços (list e
// offer_generate) passam OriginalPrice=0 nesse caso, e o template precisa
// remover a linha "De" em vez de mostrar o mesmo valor duas vezes.
func TestRenderShopeeOfferTemplate_NoDiscount(t *testing.T) {
	t.Parallel()

	const tmpl = "🛍️ {nome_produto}\n\nDe {preco_original_formatado}\n💸 Por {preco_formatado}\n\n🛒Compre aqui: {link_afiliado}"

	t.Run("sem desconto (OriginalPrice 0) remove a linha De", func(t *testing.T) {
		t.Parallel()
		got := RenderShopeeOfferTemplate(tmpl, ShopeeOfferVars{
			Title:         "Hidratante Facial Epidrat Calm 40mL",
			Price:         99.90,
			OriginalPrice: 0,
			AffiliateLink: "https://s.shopee.com.br/1BKFyf6qll",
		})
		if strings.Contains(got, "De R$ 99,90") {
			t.Errorf("linha 'De' com preco igual ao atual nao foi removida:\n%s", got)
		}
		if !strings.Contains(got, "💸 Por R$ 99,90") {
			t.Errorf("preco atual sumiu:\n%s", got)
		}
	})

	t.Run("com desconto real mantem a linha De", func(t *testing.T) {
		t.Parallel()
		got := RenderShopeeOfferTemplate(tmpl, ShopeeOfferVars{
			Title:         "Hidratante Facial Epidrat Calm 40mL",
			Price:         79.90,
			OriginalPrice: 99.90,
			AffiliateLink: "https://s.shopee.com.br/1BKFyf6qll",
		})
		if !strings.Contains(got, "De R$ 99,90") {
			t.Errorf("linha 'De' com desconto real deveria permanecer:\n%s", got)
		}
		if !strings.Contains(got, "💸 Por R$ 79,90") {
			t.Errorf("preco atual sumiu:\n%s", got)
		}
	})
}

func TestStripEmptyOriginalPriceLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		template      string
		originalEmpty bool
		wantContains  []string
		wantMissing   []string
	}{
		{
			name:          "placeholder atual riscado removido quando vazio",
			template:      "🛍️ {nome_produto}\nDe ~{preco_original_formatado}~\n💸 Por {preco_formatado}",
			originalEmpty: true,
			wantMissing:   []string{"De ~", "preco_original_formatado"},
			wantContains:  []string{"{preco_formatado}", "{nome_produto}"},
		},
		{
			name:          "placeholder legado removido quando vazio",
			template:      "De {{PRODUTO_PRECO_ORIGINAL}}\n💸 Por {{PRODUTO_PRECO}}",
			originalEmpty: true,
			wantMissing:   []string{"PRODUTO_PRECO_ORIGINAL", "De "},
			wantContains:  []string{"{{PRODUTO_PRECO}}"},
		},
		{
			name:          "linha combinada De X por Y preservada",
			template:      "De ~{preco_original_formatado}~ por {preco_formatado}",
			originalEmpty: true,
			wantContains:  []string{"preco_original_formatado", "preco_formatado"},
		},
		{
			name:          "nada muda quando ha preco original",
			template:      "De ~{preco_original_formatado}~\n💸 Por {preco_formatado}",
			originalEmpty: false,
			wantContains:  []string{"preco_original_formatado", "preco_formatado"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := StripEmptyOriginalPriceLine(tt.template, tt.originalEmpty)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("esperava conter %q em:\n%s", want, got)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(got, missing) {
					t.Errorf("nao esperava %q em:\n%s", missing, got)
				}
			}
		})
	}
}

// --- Melhoria 10: {desconto} e {desconto_formatado} ---

func TestNormalizeDiscountPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		discount float64
		want     float64
	}{
		{"inteiro valido", 25, 25},
		{"fracionario arredonda pra baixo", 25.4, 25},
		{"fracionario arredonda pra cima", 99.6, 100},
		{"meio ponto percentual vira vazio", 0.4, 0},
		{"zero", 0, 0},
		{"negativo", -10, 0},
		{"acima de 100", 150, 0},
		{"limite inferior", 1, 1},
		{"limite superior", 100, 100},
		{"acima de 100 mas arredonda pra 100", 100.4, 100},
		{"NaN", math.NaN(), 0},
		{"mais infinito", math.Inf(1), 0},
		{"menos infinito", math.Inf(-1), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeDiscountPercent(tt.discount); got != tt.want {
				t.Errorf("NormalizeDiscountPercent(%v) = %v, want %v", tt.discount, got, tt.want)
			}
		})
	}
}

func TestDiscountPercentFromPrices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		original float64
		current  float64
		want     float64
	}{
		{"desconto simples", 100, 75, 25},
		{"arredonda", 37.90, 11.99, 68},
		{"original igual ao atual", 100, 100, 0},
		{"original menor que o atual", 75, 100, 0},
		{"original zero", 0, 50, 0},
		{"original negativo", -10, 5, 0},
		{"diferenca irrisoria vira vazio", 100, 99.7, 0},
		{"atual negativo", 100, -5, 0},
		{"original NaN", math.NaN(), 50, 0},
		{"atual NaN", 100, math.NaN(), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := DiscountPercentFromPrices(tt.original, tt.current); got != tt.want {
				t.Errorf("DiscountPercentFromPrices(%v, %v) = %v, want %v", tt.original, tt.current, got, tt.want)
			}
		})
	}
}

// TestDiscountVarsParity é a prova central do contrato: o MESMO template
// produz a MESMA saída de desconto nos dois marketplaces, para todo valor de
// entrada — inclusive os inválidos, que somem nos dois.
func TestDiscountVarsParity(t *testing.T) {
	t.Parallel()

	const template = "cru=[{desconto}] fmt=[{desconto_formatado}]"

	tests := []struct {
		name     string
		discount float64
		want     string
	}{
		{"25 inteiro", 25, "cru=[25] fmt=[25% OFF]"},
		{"25.4 arredonda", 25.4, "cru=[25] fmt=[25% OFF]"},
		{"99.6 arredonda pra 100", 99.6, "cru=[100] fmt=[100% OFF]"},
		{"0.4 vazio nos dois", 0.4, "cru=[] fmt=[]"},
		{"zero vazio nos dois", 0, "cru=[] fmt=[]"},
		{"negativo vazio nos dois", -10, "cru=[] fmt=[]"},
		{"150 vazio nos dois", 150, "cru=[] fmt=[]"},
		{"NaN vazio nos dois", math.NaN(), "cru=[] fmt=[]"},
		{"infinito vazio nos dois", math.Inf(1), "cru=[] fmt=[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shopee := RenderShopeeOfferTemplate(template, ShopeeOfferVars{Title: "P", Discount: tt.discount})
			ml := RenderMLOfferTemplate(template, MLOfferVars{Title: "P", Discount: tt.discount})
			if shopee != tt.want {
				t.Errorf("Shopee = %q, want %q", shopee, tt.want)
			}
			if ml != tt.want {
				t.Errorf("ML = %q, want %q", ml, tt.want)
			}
			if shopee != ml {
				t.Errorf("paridade quebrada: Shopee=%q ML=%q", shopee, ml)
			}
		})
	}
}

// TestDiscountVarsLegacyCompat trava as saídas que NÃO podem mudar.
func TestDiscountVarsLegacyCompat(t *testing.T) {
	t.Parallel()

	t.Run("template legado {desconto}% OFF segue produzindo 25% OFF", func(t *testing.T) {
		t.Parallel()
		const template = "{desconto}% OFF"
		if got := RenderMLOfferTemplate(template, MLOfferVars{Title: "P", Discount: 25}); got != "25% OFF" {
			t.Errorf("ML = %q, want %q", got, "25% OFF")
		}
		if got := RenderShopeeOfferTemplate(template, ShopeeOfferVars{Title: "P", Discount: 25}); got != "25% OFF" {
			t.Errorf("Shopee = %q, want %q", got, "25% OFF")
		}
	})

	// Sem percentual, o legado NÃO pode virar "% OFF" pendurado — a linha some
	// inteira, e a linha do preço acima dela continua de pé.
	t.Run("template legado {desconto}% OFF sem desconto nao deixa sobra", func(t *testing.T) {
		t.Parallel()
		const template = "💸 Por {preco_formatado}\n{desconto}% OFF\n🛒 {link_afiliado}"
		want := "💸 Por R$ 11,99\n🛒 https://exemplo"
		for _, d := range []float64{0, 0.4, 150, -3} {
			got := RenderShopeeOfferTemplate(template, ShopeeOfferVars{
				Title: "P", Price: 11.99, Discount: d, AffiliateLink: "https://exemplo",
			})
			if got != want {
				t.Errorf("Shopee com Discount=%v =\n%q\nwant\n%q", d, got, want)
			}
			gotML := RenderMLOfferTemplate(template, MLOfferVars{
				Title: "P", Price: 11.99, Discount: d, AffiliateLink: "https://exemplo",
			})
			if gotML != want {
				t.Errorf("ML com Discount=%v =\n%q\nwant\n%q", d, gotML, want)
			}
		}
	})

	// Invariante do contrato §4: o percentual descreve o PRODUTO. O renderer
	// nunca o deriva de preços, então reduzir o Price (cupom, tag de desconto)
	// não pode mexer no desconto exibido.
	t.Run("percentual nao muda quando o preco cai por cupom", func(t *testing.T) {
		t.Parallel()
		const template = "{desconto_formatado}"
		semCupom := RenderShopeeOfferTemplate(template, ShopeeOfferVars{
			Title: "P", Price: 100, OriginalPrice: 125, Discount: 20,
		})
		comCupom := RenderShopeeOfferTemplate(template, ShopeeOfferVars{
			Title: "P", Price: 70, OriginalPrice: 125, Discount: 20, CouponCode: "CUPOM30",
		})
		if semCupom != "20% OFF" || comCupom != "20% OFF" {
			t.Errorf("cupom contaminou o percentual: sem=%q com=%q, want ambos %q", semCupom, comCupom, "20% OFF")
		}
	})

	t.Run("alias legado {discount} nao e normalizado", func(t *testing.T) {
		t.Parallel()
		// 150 e 0.4 são inválidos para as variáveis novas, mas o alias inglês
		// mantém a saída crua de FormatDiscount — contrato "não alterar
		// aliases ingleses neste item".
		cases := []struct {
			discount float64
			want     string
		}{
			{150, "150% OFF"},
			{0.4, "0% OFF"},
			{25.4, "25% OFF"},
			{0, ""},
		}
		for _, c := range cases {
			if got := RenderShopeeOfferTemplate("{discount}", ShopeeOfferVars{Title: "P", Discount: c.discount}); got != c.want {
				t.Errorf("Shopee {discount} com %v = %q, want %q", c.discount, got, c.want)
			}
			if got := RenderMLOfferTemplate("{discount}", MLOfferVars{Title: "P", Discount: c.discount}); got != c.want {
				t.Errorf("ML {discount} com %v = %q, want %q", c.discount, got, c.want)
			}
		}
	})

	t.Run("template sem a variavel nao ganha desconto", func(t *testing.T) {
		t.Parallel()
		const template = "🛍️ {nome_produto}\n💸 Por {preco_formatado}\n🛒 {link_afiliado}"
		want := "🛍️ Produto\n💸 Por R$ 10,00\n🛒 https://exemplo"
		offer := ShopeeOfferVars{Title: "Produto", Price: 10, Discount: 25, AffiliateLink: "https://exemplo"}
		if got := RenderShopeeOfferTemplate(template, offer); got != want {
			t.Errorf("Shopee = %q, want %q", got, want)
		}
		ml := MLOfferVars{Title: "Produto", Price: 10, Discount: 25, AffiliateLink: "https://exemplo"}
		if got := RenderMLOfferTemplate(template, ml); got != want {
			t.Errorf("ML = %q, want %q", got, want)
		}
	})
}

func TestStripEmptyDiscountLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		template      string
		discountEmpty bool
		want          string
	}{
		{
			name:          "linha so com a variavel some",
			template:      "Por {preco_formatado}\n{desconto_formatado}\n{link_afiliado}",
			discountEmpty: true,
			want:          "Por {preco_formatado}\n{link_afiliado}",
		},
		{
			name:          "linha so com variavel e decoracao some",
			template:      "Por {preco_formatado}\n🔥 {desconto_formatado} 🔥\n{link_afiliado}",
			discountEmpty: true,
			want:          "Por {preco_formatado}\n{link_afiliado}",
		},
		{
			name:          "decoracao de pontuacao tambem some",
			template:      "Por {preco_formatado}\n-{desconto}%\n{link_afiliado}",
			discountEmpty: true,
			want:          "Por {preco_formatado}\n{link_afiliado}",
		},
		{
			name:          "sintaxe de chave dupla tambem some",
			template:      "Por {preco_formatado}\n{{desconto_formatado}}\n{link_afiliado}",
			discountEmpty: true,
			want:          "Por {preco_formatado}\n{link_afiliado}",
		},
		{
			name:          "linha mista com preco fica",
			template:      "Por {preco_formatado} {desconto_formatado}",
			discountEmpty: true,
			want:          "Por {preco_formatado} {desconto_formatado}",
		},
		{
			name:          "linha legada {desconto}% OFF some inteira (o sufixo e da variavel)",
			template:      "Por {preco_formatado}\n{desconto}% OFF\n{link_afiliado}",
			discountEmpty: true,
			want:          "Por {preco_formatado}\n{link_afiliado}",
		},
		{
			name:          "sufixo % off em minuscula tambem conta",
			template:      "🔥 {desconto}% off 🔥",
			discountEmpty: true,
			want:          "",
		},
		{
			name:          "OFF sem % nao e sufixo: e texto, linha fica",
			template:      "{desconto_formatado} off",
			discountEmpty: true,
			want:          "{desconto_formatado} off",
		},
		{
			name:          "linha com texto proprio alem do sufixo fica",
			template:      "{desconto}% de desconto na categoria",
			discountEmpty: true,
			want:          "{desconto}% de desconto na categoria",
		},
		{
			name:          "alias legado {discount} nunca remove linha",
			template:      "{discount}",
			discountEmpty: true,
			want:          "{discount}",
		},
		{
			name:          "linha com spintax fica",
			template:      "{corre|voa} {desconto}",
			discountEmpty: true,
			want:          "{corre|voa} {desconto}",
		},
		{
			name:          "nada muda quando ha desconto",
			template:      "Por {preco_formatado}\n🔥 {desconto_formatado} 🔥",
			discountEmpty: false,
			want:          "Por {preco_formatado}\n🔥 {desconto_formatado} 🔥",
		},
		{
			name:          "template sem a variavel fica intacto",
			template:      "Por {preco_formatado}\n{link_afiliado}",
			discountEmpty: true,
			want:          "Por {preco_formatado}\n{link_afiliado}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StripEmptyDiscountLine(tt.template, tt.discountEmpty); got != tt.want {
				t.Errorf("StripEmptyDiscountLine() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// TestRenderOfferTemplate_EmptyDiscountLine prova end-to-end que desconto
// inválido não deixa linha em branco, decoração órfã nem placeholder — e que
// a linha mista com preço sobrevive.
func TestRenderOfferTemplate_EmptyDiscountLine(t *testing.T) {
	t.Parallel()

	const template = "🛍️ {nome_produto}\n💸 Por {preco_formatado}\n🔥 {desconto_formatado}\n\n🛒 {link_afiliado}"

	t.Run("sem desconto a linha some inteira", func(t *testing.T) {
		t.Parallel()
		want := "🛍️ Produto\n💸 Por R$ 11,99\n\n🛒 https://exemplo"
		shopee := RenderShopeeOfferTemplate(template, ShopeeOfferVars{
			Title: "Produto", Price: 11.99, Discount: 0, AffiliateLink: "https://exemplo",
		})
		if shopee != want {
			t.Errorf("Shopee =\n%q\nwant\n%q", shopee, want)
		}
		ml := RenderMLOfferTemplate(template, MLOfferVars{
			Title: "Produto", Price: 11.99, Discount: 0, AffiliateLink: "https://exemplo",
		})
		if ml != want {
			t.Errorf("ML =\n%q\nwant\n%q", ml, want)
		}
	})

	t.Run("com desconto a linha aparece", func(t *testing.T) {
		t.Parallel()
		want := "🛍️ Produto\n💸 Por R$ 11,99\n🔥 68% OFF\n\n🛒 https://exemplo"
		shopee := RenderShopeeOfferTemplate(template, ShopeeOfferVars{
			Title: "Produto", Price: 11.99, Discount: 68, AffiliateLink: "https://exemplo",
		})
		if shopee != want {
			t.Errorf("Shopee =\n%q\nwant\n%q", shopee, want)
		}
	})

	t.Run("linha mista com preco sobrevive sem desconto", func(t *testing.T) {
		t.Parallel()
		got := RenderShopeeOfferTemplate("Por {preco_formatado} {desconto_formatado}", ShopeeOfferVars{
			Title: "Produto", Price: 11.99, Discount: 0,
		})
		if got != "Por R$ 11,99" {
			t.Errorf("got %q, want %q", got, "Por R$ 11,99")
		}
	})
}
