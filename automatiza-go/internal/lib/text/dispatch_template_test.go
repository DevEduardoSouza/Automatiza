package text

import (
	"strings"
	"testing"
)

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
		"{aviso_preco}",
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
		"{aviso_preco}",
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
