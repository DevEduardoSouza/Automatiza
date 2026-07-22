package text

import (
	"testing"
)

func TestParseDiscountTag(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantNil   bool
		wantPct   float64
		wantFixed float64
		wantMax   float64
		wantCode  string
	}{
		{
			name:    "percent with max",
			body:    "https://shopee.com.br/abc desc[25%|10]",
			wantPct: 25, wantMax: 10,
		},
		{
			name:    "percent only",
			body:    "https://shopee.com.br/abc desc[25%]",
			wantPct: 25,
		},
		{
			name:      "fixed value",
			body:      "https://shopee.com.br/abc desc[10]",
			wantFixed: 10,
		},
		{
			name:    "no tag",
			body:    "https://shopee.com.br/abc",
			wantNil: true,
		},
		{
			name:    "empty tag",
			body:    "https://shopee.com.br/abc desc[]",
			wantNil: true,
		},
		{
			name:    "invalid content",
			body:    "desc[abc]",
			wantNil: true,
		},
		{
			name:    "tag at start",
			body:    "desc[15%] https://shopee.com.br/abc",
			wantPct: 15,
		},
		{
			name:      "decimal fixed value",
			body:      "desc[9.90]",
			wantFixed: 9.90,
		},
		{
			name:    "decimal percent with decimal max",
			body:    "desc[10.5%|5.50]",
			wantPct: 10.5, wantMax: 5.50,
		},
		{
			name:      "fixed with coupon code",
			body:      "https://shopee.com.br/abc desc[10:EXEMPLO10]",
			wantFixed: 10, wantCode: "EXEMPLO10",
		},
		{
			name:    "percent with coupon code",
			body:    "desc[25%:CUPOM25]",
			wantPct: 25, wantCode: "CUPOM25",
		},
		{
			name:    "percent with max and coupon code",
			body:    "desc[25%|10:MEGA]",
			wantPct: 25, wantMax: 10, wantCode: "MEGA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := ParseDiscountTag(tt.body)
			if tt.wantNil {
				if tag != nil {
					t.Fatalf("expected nil, got %+v", tag)
				}
				return
			}
			if tag == nil {
				t.Fatal("expected non-nil tag")
			}
			if tag.Percent != tt.wantPct {
				t.Errorf("Percent = %v, want %v", tag.Percent, tt.wantPct)
			}
			if tag.FixedValue != tt.wantFixed {
				t.Errorf("FixedValue = %v, want %v", tag.FixedValue, tt.wantFixed)
			}
			if tag.MaxValue != tt.wantMax {
				t.Errorf("MaxValue = %v, want %v", tag.MaxValue, tt.wantMax)
			}
			if tag.CouponCode != tt.wantCode {
				t.Errorf("CouponCode = %q, want %q", tag.CouponCode, tt.wantCode)
			}
		})
	}
}

func TestStripDiscountTag(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"https://shopee.com.br/abc desc[25%|10]", "https://shopee.com.br/abc"},
		{"desc[10] https://shopee.com.br/abc", "https://shopee.com.br/abc"},
		{"https://shopee.com.br/abc", "https://shopee.com.br/abc"},
		{"desc[25%] text after", "text after"},
	}

	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			got := StripDiscountTag(tt.body)
			if got != tt.want {
				t.Errorf("StripDiscountTag(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestParseInfoTag(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "plain text",
			body: "https://shopee.com.br/abc info[Frete grátis hoje]",
			want: "Frete grátis hoje",
		},
		{
			name: "embedded url",
			body: "https://shopee.com.br/abc\ninfo[Use o cupom https://s.shopee.com.br/cupom-link]",
			want: "Use o cupom https://s.shopee.com.br/cupom-link",
		},
		{
			name: "tag at start",
			body: "info[Linha extra] https://shopee.com.br/abc",
			want: "Linha extra",
		},
		{
			name: "empty tag",
			body: "https://shopee.com.br/abc info[]",
			want: "",
		},
		{
			name: "no tag",
			body: "https://shopee.com.br/abc",
			want: "",
		},
		{
			name: "whitespace trimmed",
			body: "info[   spaced content   ]",
			want: "spaced content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseInfoTag(tt.body)
			if got != tt.want {
				t.Errorf("ParseInfoTag(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestStripInfoTag(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"https://shopee.com.br/abc info[Cupom 25%]", "https://shopee.com.br/abc"},
		{"info[X] https://shopee.com.br/abc", "https://shopee.com.br/abc"},
		// URL inside info[] must be stripped so link extraction does not
		// treat the coupon link as a product link.
		{
			"https://shopee.com.br/produto info[Use https://s.shopee.com.br/cupom]",
			"https://shopee.com.br/produto",
		},
		{"https://shopee.com.br/abc", "https://shopee.com.br/abc"},
		{"info[] https://shopee.com.br/abc", "https://shopee.com.br/abc"},
	}

	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			got := StripInfoTag(tt.body)
			if got != tt.want {
				t.Errorf("StripInfoTag(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestParseCTATag(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "plain text",
			body: "https://shopee.com.br/abc cta[Corre que acaba logo 👉]",
			want: "Corre que acaba logo 👉",
		},
		{
			name: "embedded url",
			body: "https://shopee.com.br/abc\ncta[Garanta em https://s.shopee.com.br/link]",
			want: "Garanta em https://s.shopee.com.br/link",
		},
		{
			name: "tag at start",
			body: "cta[Clique agora] https://shopee.com.br/abc",
			want: "Clique agora",
		},
		{
			name: "empty tag",
			body: "https://shopee.com.br/abc cta[]",
			want: "",
		},
		{
			name: "no tag",
			body: "https://shopee.com.br/abc",
			want: "",
		},
		{
			name: "whitespace trimmed",
			body: "cta[   spaced cta   ]",
			want: "spaced cta",
		},
		{
			name: "info and cta coexist",
			body: "https://shopee.com.br/abc info[Frete grátis] cta[Compre já]",
			want: "Compre já",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCTATag(tt.body)
			if got != tt.want {
				t.Errorf("ParseCTATag(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestStripCTATag(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"https://shopee.com.br/abc cta[Compre já]", "https://shopee.com.br/abc"},
		{"cta[X] https://shopee.com.br/abc", "https://shopee.com.br/abc"},
		// URL inside cta[] must be stripped so link extraction does not
		// treat it as a product link.
		{
			"https://shopee.com.br/produto cta[Garanta em https://s.shopee.com.br/x]",
			"https://shopee.com.br/produto",
		},
		{"https://shopee.com.br/abc", "https://shopee.com.br/abc"},
		{"cta[] https://shopee.com.br/abc", "https://shopee.com.br/abc"},
	}

	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			got := StripCTATag(tt.body)
			if got != tt.want {
				t.Errorf("StripCTATag(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestApplyDiscountTag(t *testing.T) {
	tests := []struct {
		name  string
		price float64
		tag   *DiscountTag
		want  float64
	}{
		{
			name:  "nil tag",
			price: 100, tag: nil, want: 100,
		},
		{
			name:  "25% off, no max",
			price: 100, tag: &DiscountTag{Percent: 25}, want: 75,
		},
		{
			name:  "25% off, max 10",
			price: 100, tag: &DiscountTag{Percent: 25, MaxValue: 10}, want: 90,
		},
		{
			name:  "25% off, max 50 (no cap needed)",
			price: 100, tag: &DiscountTag{Percent: 25, MaxValue: 50}, want: 75,
		},
		{
			name:  "fixed R$10 off",
			price: 100, tag: &DiscountTag{FixedValue: 10}, want: 90,
		},
		{
			name:  "fixed discount exceeds price",
			price: 5, tag: &DiscountTag{FixedValue: 10}, want: 0,
		},
		{
			name:  "zero price",
			price: 0, tag: &DiscountTag{Percent: 25}, want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyDiscountTag(tt.price, tt.tag)
			if got != tt.want {
				t.Errorf("ApplyDiscountTag(%v, %+v) = %v, want %v", tt.price, tt.tag, got, tt.want)
			}
		})
	}
}

func TestHasStoryTag(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"início simples", "{story} promo do dia", true},
		{"caixa mista", "{Story} promo", true},
		{"maiúsculas", "{STORY} promo", true},
		{"espaço antes", "  {story} promo", true},
		{"só a tag", "{story}", true},
		{"tag no meio não casa", "veja {story} aqui", false},
		{"sem tag", "promoção normal", false},
		{"corpo vazio", "", false},
		{"palavra story sem chaves", "story do dia", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasStoryTag(tt.body); got != tt.want {
				t.Errorf("HasStoryTag(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

func TestStripStoryTag(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"início simples", "{story} promo do dia", "promo do dia"},
		{"caixa mista", "{Story}   promo", "promo"},
		{"espaço antes", "  {story} legenda", "legenda"},
		{"só a tag", "{story}", ""},
		{"tag no meio preservada", "veja {story} aqui", "veja {story} aqui"},
		{"sem tag intocado", "promoção normal", "promoção normal"},
		{"multilinha", "{story}\nlinha 1\nlinha 2", "linha 1\nlinha 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripStoryTag(tt.body); got != tt.want {
				t.Errorf("StripStoryTag(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}
