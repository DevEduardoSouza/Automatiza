package text

import (
	"testing"
)

func TestParseDiscountTag(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantNil    bool
		wantPct    float64
		wantFixed  float64
		wantMax    float64
		wantCode   string
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

func TestApplyDiscountTag(t *testing.T) {
	tests := []struct {
		name  string
		price float64
		tag   *DiscountTag
		want  float64
	}{
		{
			name: "nil tag",
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
