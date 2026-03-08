package mercadolivre

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// decodeHTMLEntities
// ---------------------------------------------------------------------------

func TestDecodeHTMLEntities(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "amp", input: "&amp;", want: "&"},
		{name: "quot", input: "&quot;", want: "\""},
		{name: "apos", input: "&#39;", want: "'"},
		{name: "lt", input: "&lt;", want: "<"},
		{name: "gt", input: "&gt;", want: ">"},
		{name: "multiple entities", input: "&lt;div class=&quot;foo&quot;&gt;", want: `<div class="foo">`},
		{name: "no entities passthrough", input: "hello world", want: "hello world"},
		{name: "empty string", input: "", want: ""},
		{name: "mixed text and entities", input: "A &amp; B &lt; C", want: "A & B < C"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeHTMLEntities(tt.input)
			if got != tt.want {
				t.Errorf("decodeHTMLEntities(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractMeta
// ---------------------------------------------------------------------------

func TestExtractMeta(t *testing.T) {
	tests := []struct {
		name string
		html string
		key  string
		want string
	}{
		{name: "empty html", html: "", key: "og:title", want: ""},
		{name: "empty key", html: `<meta property="og:title" content="Hello">`, key: "", want: ""},
		{
			name: "og:title extraction",
			html: `<meta property="og:title" content="Product Name">`,
			key:  "og:title",
			want: "Product Name",
		},
		{
			name: "twitter:title extraction",
			html: `<meta name="twitter:title" content="Tweet Product">`,
			key:  "twitter:title",
			want: "Tweet Product",
		},
		{
			name: "content before property order",
			html: `<meta content="Reversed" property="og:title">`,
			key:  "og:title",
			want: "Reversed",
		},
		{
			name: "special chars in key are escaped",
			html: `<meta property="product:price:amount" content="99.90">`,
			key:  "product:price:amount",
			want: "99.90",
		},
		{
			name: "not found returns empty",
			html: `<meta property="og:image" content="img.jpg">`,
			key:  "og:title",
			want: "",
		},
		{
			name: "HTML entities in content decoded",
			html: `<meta property="og:title" content="A &amp; B">`,
			key:  "og:title",
			want: "A & B",
		},
		{
			name: "single quotes",
			html: `<meta property='og:title' content='Single Quotes'>`,
			key:  "og:title",
			want: "Single Quotes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMeta(tt.html, tt.key)
			if got != tt.want {
				t.Errorf("extractMeta() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractTitle
// ---------------------------------------------------------------------------

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "og:title present",
			html: `<meta property="og:title" content="OG Title">`,
			want: "OG Title",
		},
		{
			name: "twitter:title fallback when no og:title",
			html: `<meta name="twitter:title" content="Twitter Title">`,
			want: "Twitter Title",
		},
		{
			name: "title tag fallback",
			html: `<html><head><title>Fallback Title</title></head></html>`,
			want: "Fallback Title",
		},
		{
			name: "title tag cleanup removes pipe suffix",
			html: `<html><head><title>Product | Loja Foo</title></head></html>`,
			want: "Product",
		},
		{
			name: "title tag cleanup removes Mercado Livre suffix",
			html: `<html><head><title>Product - Mercado Livre</title></head></html>`,
			want: "Product",
		},
		{
			name: "no title returns empty",
			html: `<html><body>No title</body></html>`,
			want: "",
		},
		{
			name: "og:title takes priority over title tag",
			html: `<html><head>
				<meta property="og:title" content="OG Priority">
				<title>Tag Title</title>
			</head></html>`,
			want: "OG Priority",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.html)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractImage
// ---------------------------------------------------------------------------

func TestExtractImage(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "og:image present",
			html: `<meta property="og:image" content="https://img.com/product.jpg">`,
			want: "https://img.com/product.jpg",
		},
		{
			name: "twitter:image fallback",
			html: `<meta name="twitter:image" content="https://img.com/twitter.jpg">`,
			want: "https://img.com/twitter.jpg",
		},
		{
			name: "not found returns empty",
			html: `<html><body>No images</body></html>`,
			want: "",
		},
		{
			name: "og:image takes priority over twitter:image",
			html: `<html>
				<meta property="og:image" content="https://og.com/img.jpg">
				<meta name="twitter:image" content="https://tw.com/img.jpg">
			</html>`,
			want: "https://og.com/img.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractImage(tt.html)
			if got != tt.want {
				t.Errorf("extractImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// upscaleMLImageURL
// ---------------------------------------------------------------------------

func TestUpscaleMLImageURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "replaces -O with -F for webp",
			url:  "https://http2.mlstatic.com/D_NQ_NP_852672-MLA99562209030_122025-O.webp",
			want: "https://http2.mlstatic.com/D_NQ_NP_852672-MLA99562209030_122025-F.webp",
		},
		{
			name: "replaces -O with -F for jpg",
			url:  "https://http2.mlstatic.com/D_NQ_NP_807092-MLA100966388363_122025-O.jpg",
			want: "https://http2.mlstatic.com/D_NQ_NP_807092-MLA100966388363_122025-F.jpg",
		},
		{
			name: "replaces -I with -F",
			url:  "https://http2.mlstatic.com/D_NQ_NP_123-MLA456-I.webp",
			want: "https://http2.mlstatic.com/D_NQ_NP_123-MLA456-F.webp",
		},
		{
			name: "keeps -F unchanged",
			url:  "https://http2.mlstatic.com/D_NQ_NP_123-MLA456-F.webp",
			want: "https://http2.mlstatic.com/D_NQ_NP_123-MLA456-F.webp",
		},
		{
			name: "non-mlstatic URL unchanged",
			url:  "https://img.com/product-O.jpg",
			want: "https://img.com/product-O.jpg",
		},
		{
			name: "empty string unchanged",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := upscaleMLImageURL(tt.url)
			if got != tt.want {
				t.Errorf("upscaleMLImageURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseMoneyAmount
// ---------------------------------------------------------------------------

func TestParseMoneyAmount(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  float64
	}{
		{name: "empty string", value: "", want: 0},
		{name: "R$ with Brazilian format", value: "R$ 1.234,56", want: 1234.56},
		{name: "plain decimal format", value: "1234.56", want: 1234.56},
		{name: "R$ without space", value: "R$99,90", want: 99.90},
		{name: "integer", value: "100", want: 100},
		{name: "spaces and R$ stripped", value: " R$ 50 ", want: 50},
		{name: "Brazilian thousands and cents", value: "R$ 10.500,00", want: 10500.00},
		{name: "just cents", value: "0,99", want: 0.99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMoneyAmount(tt.value)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("parseMoneyAmount(%q) = %f, want %f", tt.value, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractPrice
// ---------------------------------------------------------------------------

func TestExtractPrice(t *testing.T) {
	tests := []struct {
		name         string
		html         string
		wantPrice    float64
		wantCurrency string
	}{
		{
			name:         "meta tag product:price:amount",
			html:         `<meta property="product:price:amount" content="99.90"><meta property="product:price:currency" content="BRL">`,
			wantPrice:    99.90,
			wantCurrency: "BRL",
		},
		{
			name: "JSON-LD with offers.price float64",
			html: `<script type="application/ld+json">
				{"offers": {"price": 149.90, "priceCurrency": "BRL"}}
			</script>`,
			wantPrice:    149.90,
			wantCurrency: "BRL",
		},
		{
			name: "JSON-LD with offers.price string",
			html: `<script type="application/ld+json">
				{"offers": {"price": "199,90"}}
			</script>`,
			wantPrice:    199.90,
			wantCurrency: "BRL",
		},
		{
			name: "JSON-LD with lowPrice fallback",
			html: `<script type="application/ld+json">
				{"offers": {"lowPrice": 79.90}}
			</script>`,
			wantPrice:    79.90,
			wantCurrency: "BRL",
		},
		{
			name: "JSON-LD with priceCurrency",
			html: `<script type="application/ld+json">
				{"offers": {"price": 100, "priceCurrency": "usd"}}
			</script>`,
			wantPrice:    100,
			wantCurrency: "USD",
		},
		{
			name:         "regex R$ fallback picks smallest price",
			html:         `<span>R$ 199,90</span><span>R$ 99,90</span>`,
			wantPrice:    99.90,
			wantCurrency: "BRL",
		},
		{
			name:         "no price returns 0",
			html:         `<html><body>No price</body></html>`,
			wantPrice:    0,
			wantCurrency: "BRL",
		},
		{
			name:         "og:price:amount meta",
			html:         `<meta property="og:price:amount" content="59.90">`,
			wantPrice:    59.90,
			wantCurrency: "BRL",
		},
		{
			name: "offers array in JSON-LD",
			html: `<script type="application/ld+json">
				{"offers": [{"price": 50.00}, {"price": 30.00}]}
			</script>`,
			wantPrice:    50.00,
			wantCurrency: "BRL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, currency := extractPrice(tt.html)
			if math.Abs(price-tt.wantPrice) > 0.01 {
				t.Errorf("extractPrice() price = %f, want %f", price, tt.wantPrice)
			}
			if currency != tt.wantCurrency {
				t.Errorf("extractPrice() currency = %q, want %q", currency, tt.wantCurrency)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractJSONLDObjects
// ---------------------------------------------------------------------------

func TestExtractJSONLDObjects(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantCount int
	}{
		{
			name:      "single JSON-LD object",
			html:      `<script type="application/ld+json">{"@type": "Product", "name": "Test"}</script>`,
			wantCount: 1,
		},
		{
			name:      "JSON-LD array",
			html:      `<script type="application/ld+json">[{"@type": "Product"}, {"@type": "Offer"}]</script>`,
			wantCount: 2,
		},
		{
			name: "multiple script tags",
			html: `<script type="application/ld+json">{"@type": "Product"}</script>
			       <script type="application/ld+json">{"@type": "Organization"}</script>`,
			wantCount: 2,
		},
		{
			name:      "invalid JSON skipped",
			html:      `<script type="application/ld+json">{not valid json}</script>`,
			wantCount: 0,
		},
		{
			name:      "empty script skipped",
			html:      `<script type="application/ld+json">  </script>`,
			wantCount: 0,
		},
		{
			name:      "no LD+JSON scripts",
			html:      `<script type="text/javascript">var x = 1;</script>`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONLDObjects(tt.html)
			if len(got) != tt.wantCount {
				t.Errorf("extractJSONLDObjects() returned %d objects, want %d", len(got), tt.wantCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractNumberByKey
// ---------------------------------------------------------------------------

func TestExtractNumberByKey(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		key     string
		wantNil bool
		want    float64
	}{
		{
			name:    "found integer",
			html:    `{"sold_quantity": 150}`,
			key:     "sold_quantity",
			wantNil: false,
			want:    150,
		},
		{
			name:    "found float",
			html:    `{"rating_average": 4.5}`,
			key:     "rating_average",
			wantNil: false,
			want:    4.5,
		},
		{
			name:    "not found returns nil",
			html:    `{"other_key": 10}`,
			key:     "sold_quantity",
			wantNil: true,
		},
		{
			name:    "spaces around colon",
			html:    `{"sold_quantity" : 200}`,
			key:     "sold_quantity",
			wantNil: false,
			want:    200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNumberByKey(tt.html, tt.key)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %f", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if math.Abs(*got-tt.want) > 0.001 {
				t.Errorf("extractNumberByKey() = %f, want %f", *got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractCondition
// ---------------------------------------------------------------------------

func TestExtractCondition(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantNil bool
		want    string
	}{
		{
			name:    "JSON-LD NewCondition",
			html:    `<script type="application/ld+json">{"itemCondition": "https://schema.org/NewCondition"}</script>`,
			wantNil: false,
			want:    "Novo",
		},
		{
			name:    "JSON-LD UsedCondition",
			html:    `<script type="application/ld+json">{"itemCondition": "https://schema.org/UsedCondition"}</script>`,
			wantNil: false,
			want:    "Usado",
		},
		{
			name:    "JSON-LD unknown condition passes through",
			html:    `<script type="application/ld+json">{"itemCondition": "RefurbishedCondition"}</script>`,
			wantNil: false,
			want:    "RefurbishedCondition",
		},
		{
			name:    "meta tag og:condition",
			html:    `<meta property="og:condition" content="new">`,
			wantNil: false,
			want:    "new",
		},
		{
			name:    "meta tag product:condition",
			html:    `<meta property="product:condition" content="used">`,
			wantNil: false,
			want:    "used",
		},
		{
			name:    "not found returns nil",
			html:    `<html><body>No condition</body></html>`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCondition(tt.html)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %q", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if *got != tt.want {
				t.Errorf("extractCondition() = %q, want %q", *got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractSoldQuantity
// ---------------------------------------------------------------------------

func TestExtractSoldQuantity(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantNil bool
		want    int
	}{
		{
			name:    "JSON key sold_quantity",
			html:    `{"sold_quantity": 42}`,
			wantNil: false,
			want:    42,
		},
		{
			name:    "JSON key soldQuantity",
			html:    `{"soldQuantity": 100}`,
			wantNil: false,
			want:    100,
		},
		{
			name:    "text +50mil vendidos",
			html:    `<span>+50mil vendidos</span>`,
			wantNil: false,
			want:    50000,
		},
		{
			name:    "text 100 vendidos",
			html:    `<span>100 vendidos</span>`,
			wantNil: false,
			want:    100,
		},
		{
			name:    "not found returns nil",
			html:    `<html><body>Nothing</body></html>`,
			wantNil: true,
		},
		{
			name:    "text 5mil vendidos without plus",
			html:    `<span>5mil vendidos</span>`,
			wantNil: false,
			want:    5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSoldQuantity(tt.html)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %d", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if *got != tt.want {
				t.Errorf("extractSoldQuantity() = %d, want %d", *got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractRating
// ---------------------------------------------------------------------------

func TestExtractRating(t *testing.T) {
	tests := []struct {
		name          string
		html          string
		wantRatingNil bool
		wantRating    float64
		wantCountNil  bool
		wantCount     int
	}{
		{
			name: "JSON-LD aggregateRating float64",
			html: `<script type="application/ld+json">
				{"aggregateRating": {"ratingValue": 4.5, "reviewCount": 200}}
			</script>`,
			wantRating: 4.5,
			wantCount:  200,
		},
		{
			name: "JSON-LD aggregateRating string types",
			html: `<script type="application/ld+json">
				{"aggregateRating": {"ratingValue": "4.2", "reviewCount": "150"}}
			</script>`,
			wantRating: 4.2,
			wantCount:  150,
		},
		{
			name:       "JSON keys rating_average and total_reviews",
			html:       `{"rating_average": 3.8, "total_reviews": 50}`,
			wantRating: 3.8,
			wantCount:  50,
		},
		{
			name:          "not found returns nil for both",
			html:          `<html><body>No rating</body></html>`,
			wantRatingNil: true,
			wantCountNil:  true,
		},
		{
			name: "JSON-LD only ratingValue, no reviewCount",
			html: `<script type="application/ld+json">
				{"aggregateRating": {"ratingValue": 4.0}}
			</script>`,
			wantRating:   4.0,
			wantCountNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rating, count := extractRating(tt.html)

			if tt.wantRatingNil {
				if rating != nil {
					t.Errorf("expected nil rating, got %f", *rating)
				}
			} else {
				if rating == nil {
					t.Fatal("expected non-nil rating")
				}
				if math.Abs(*rating-tt.wantRating) > 0.01 {
					t.Errorf("rating = %f, want %f", *rating, tt.wantRating)
				}
			}

			if tt.wantCountNil {
				if count != nil {
					t.Errorf("expected nil count, got %d", *count)
				}
			} else {
				if count == nil {
					t.Fatal("expected non-nil count")
				}
				if *count != tt.wantCount {
					t.Errorf("count = %d, want %d", *count, tt.wantCount)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractOriginalPrice
// ---------------------------------------------------------------------------

func TestExtractOriginalPrice(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantNil bool
		want    float64
	}{
		{
			name: "JSON key original_price",
			html: `{"original_price": 299.90}`,
			want: 299.90,
		},
		{
			name: "JSON key originalPrice",
			html: `{"originalPrice": 199.00}`,
			want: 199.00,
		},
		{
			name:    "not found returns nil",
			html:    `<html>No price</html>`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOriginalPrice(tt.html)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %f", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if math.Abs(*got-tt.want) > 0.01 {
				t.Errorf("extractOriginalPrice() = %f, want %f", *got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractDiscountPercent
// ---------------------------------------------------------------------------

func TestExtractDiscountPercent(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantNil bool
		want    float64
	}{
		{
			name: "30% OFF",
			html: `<span>30% OFF</span>`,
			want: 30.0,
		},
		{
			name: "5% OFF",
			html: `<span>5% OFF</span>`,
			want: 5.0,
		},
		{
			name:    "not found returns nil",
			html:    `<html>No discount</html>`,
			wantNil: true,
		},
		{
			name: "10% OFF with extra text",
			html: `Aproveite 10% OFF no produto`,
			want: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDiscountPercent(tt.html)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %f", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if math.Abs(*got-tt.want) > 0.01 {
				t.Errorf("extractDiscountPercent() = %f, want %f", *got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractInstallments
// ---------------------------------------------------------------------------

func TestExtractInstallments(t *testing.T) {
	tests := []struct {
		name         string
		html         string
		wantNil      bool
		wantQuantity int
		wantAmount   float64
	}{
		{
			name:         "text pattern em 3x R$ 33,33",
			html:         `<span>em 3x R$ 33,33 sem juros</span>`,
			wantQuantity: 3,
			wantAmount:   33.33,
		},
		{
			name:         "JSON installments pattern",
			html:         `"installments":{"quantity":6,"amount":16.5}`,
			wantQuantity: 6,
			wantAmount:   16.5,
		},
		{
			name:    "not found returns nil",
			html:    `<html>No installments</html>`,
			wantNil: true,
		},
		{
			name:         "text pattern em 12x R$ 8,33",
			html:         `em 12x R$ 8,33`,
			wantQuantity: 12,
			wantAmount:   8.33,
		},
		{
			name:         "text pattern em 1x R$ 99,90",
			html:         `em 1x R$ 99,90`,
			wantQuantity: 1,
			wantAmount:   99.90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInstallments(tt.html)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if got.Quantity != tt.wantQuantity {
				t.Errorf("Quantity = %d, want %d", got.Quantity, tt.wantQuantity)
			}
			if math.Abs(got.Amount-tt.wantAmount) > 0.01 {
				t.Errorf("Amount = %f, want %f", got.Amount, tt.wantAmount)
			}
			if got.Currency != "BRL" {
				t.Errorf("Currency = %q, want 'BRL'", got.Currency)
			}
		})
	}
}
