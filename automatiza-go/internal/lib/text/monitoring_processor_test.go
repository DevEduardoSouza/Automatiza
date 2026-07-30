package text

import (
	"automatiza-go/internal/models"
	"testing"
)

func TestProcessarMensagem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		rules *models.MonitoringTextRules
		want  string
	}{
		{
			name:  "nil rules returns text unchanged",
			text:  "hello world",
			rules: nil,
			want:  "hello world",
		},
		{
			name: "substitutions applied",
			text: "compre agora no site",
			rules: &models.MonitoringTextRules{
				Substituicoes: []models.Substituicao{
					{De: "compre", Para: "confira"},
					{De: "site", Para: "link"},
				},
			},
			want: "confira agora no link",
		},
		{
			name: "remove terms plain string",
			text: "oferta especial: produto em promoção",
			rules: &models.MonitoringTextRules{
				RemoverTermos: []string{"oferta especial: "},
			},
			want: "produto em promoção",
		},
		{
			name: "remove terms regex pattern",
			text: "item ABC123 disponível DEF456 agora",
			rules: &models.MonitoringTextRules{
				RemoverTermos: []string{`[A-Z]{3}\d{3}\s*`},
			},
			want: "item disponível agora",
		},
		{
			name: "clean phones removes Brazilian numbers",
			text: "ligue para +5511912345678 ou 11987654321",
			rules: &models.MonitoringTextRules{
				LimparTelefones: true,
			},
			want: "ligue para ou",
		},
		{
			name: "clean mentions removes @mentions",
			text: "cc @5511912345678 e @1234567890",
			rules: &models.MonitoringTextRules{
				LimparArrobas: true,
			},
			want: "cc e",
		},
		{
			name:  "whitespace normalization",
			text:  "  hello    world  \n   foo   bar  ",
			rules: &models.MonitoringTextRules{},
			want:  "hello world\nfoo bar",
		},
		{
			name: "prefix added",
			text: "oferta imperdível",
			rules: &models.MonitoringTextRules{
				Prefixo: "🔥 PROMO",
			},
			want: "🔥 PROMO\noferta imperdível",
		},
		{
			name: "suffix added",
			text: "oferta imperdível",
			rules: &models.MonitoringTextRules{
				Sufixo: "⚠️ Preço sujeito a alteração",
			},
			want: "oferta imperdível\n⚠️ Preço sujeito a alteração",
		},
		{
			name: "all rules combined",
			text: "compre agora +5511912345678 @123456 palavra_secreta  em   promoção",
			rules: &models.MonitoringTextRules{
				Substituicoes: []models.Substituicao{
					{De: "compre", Para: "confira"},
				},
				RemoverTermos:   []string{"palavra_secreta"},
				LimparTelefones: true,
				LimparArrobas:   true,
				Prefixo:         "INICIO",
				Sufixo:          "FIM",
			},
			want: "INICIO\nconfira agora em promoção\nFIM",
		},
		{
			name:  "empty text with empty rules",
			text:  "",
			rules: &models.MonitoringTextRules{},
			want:  "",
		},
		{
			name: "remove terms invalid regex falls back to literal replace",
			text: "remove [this] from text",
			rules: &models.MonitoringTextRules{
				RemoverTermos: []string{"[this"},
			},
			want: "remove ] from text",
		},
		{
			// The lay-user case: the client pastes the footer exactly as it
			// appears on screen (no asterisks), but the inbound WhatsApp message
			// carries *bold* markers and hard line breaks. Tolerant matching
			// removes it and preserves the offer above.
			name: "remove terms tolerant removes footer with whatsapp bold and newlines",
			text: "🛒 Oferta top\nhttps://meli.la/2nxYtCP\n\n💖 *Nos siga no instagram* @canal.oficial e caso *esse número pare* entre por aqui https://site.com.br/",
			rules: &models.MonitoringTextRules{
				RemoverTermos: []string{"💖 Nos siga no instagram @canal.oficial e caso esse número pare entre por aqui https://site.com.br/"},
			},
			want: "🛒 Oferta top\nhttps://meli.la/2nxYtCP",
		},
		{
			// Same idea but the client used the Substituições field with an empty
			// "para" (a very common way people try to delete a footer). Exact
			// match fails on the bold markers; tolerant fallback removes it.
			name: "substitution tolerant fallback removes footer pasted as-seen",
			text: "Produto X\n*Nos siga* aqui @loja",
			rules: &models.MonitoringTextRules{
				Substituicoes: []models.Substituicao{
					{De: "Nos siga aqui @loja", Para: ""},
				},
			},
			want: "Produto X",
		},
		{
			// Rebrand case: the client's pasted term spans a bold boundary
			// ("@canal *antigo*"), so exact match fails and tolerant kicks in;
			// the orphaned "*" delimiters are absorbed, no stray markers leak.
			name: "substitution tolerant fallback rebrands across bold boundary",
			text: "confira @canal *antigo* hoje",
			rules: &models.MonitoringTextRules{
				Substituicoes: []models.Substituicao{
					{De: "@canal antigo", Para: "@novoperfil"},
				},
			},
			want: "confira @novoperfil hoje",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ProcessarMensagem(tt.text, tt.rules)
			if got != tt.want {
				t.Errorf("ProcessarMensagem(%q, rules) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestExtractLinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "no links returns empty slice",
			text: "no links here",
			want: nil,
		},
		{
			name: "http link",
			text: "visit http://example.com today",
			want: []string{"http://example.com"},
		},
		{
			name: "https link",
			text: "visit https://example.com/page today",
			want: []string{"https://example.com/page"},
		},
		{
			name: "multiple links deduplicated",
			text: "https://example.com and https://example.com again",
			want: []string{"https://example.com"},
		},
		{
			name: "trailing period removed",
			text: "visit https://example.com.",
			want: []string{"https://example.com"},
		},
		{
			name: "trailing comma removed",
			text: "visit https://example.com, ok?",
			want: []string{"https://example.com"},
		},
		{
			name: "trailing semicolon removed",
			text: "visit https://example.com;",
			want: []string{"https://example.com"},
		},
		{
			name: "trailing exclamation removed",
			text: "visit https://example.com!",
			want: []string{"https://example.com"},
		},
		{
			name: "trailing question mark removed",
			text: "visit https://example.com?",
			want: []string{"https://example.com"},
		},
		{
			name: "links in text with surrounding text",
			text: "check https://foo.com and also https://bar.com for details",
			want: []string{"https://foo.com", "https://bar.com"},
		},
		{
			name: "link with path and query",
			text: "https://example.com/path?q=1&r=2",
			want: []string{"https://example.com/path?q=1&r=2"},
		},
		{
			name: "shopee url with parentheses in slug preserves shop/item IDs",
			text: "https://shopee.com.br/M%C3%A1quina-de-Lavar-Electrolux-11kg-Branca-Essential-Care-com-Easy-Clean-e-Filtro-Fiapos-(LES11)-i.1526836548.23294017893?extraParams=%7B%22display_model_id%22%3A239422052096%7D desc",
			want: []string{"https://shopee.com.br/M%C3%A1quina-de-Lavar-Electrolux-11kg-Branca-Essential-Care-com-Easy-Clean-e-Filtro-Fiapos-(LES11)-i.1526836548.23294017893?extraParams=%7B%22display_model_id%22%3A239422052096%7D"},
		},
		{
			name: "markdown wrapped link strips trailing close paren",
			text: "veja aqui (https://example.com/foo) e depois",
			want: []string{"https://example.com/foo"},
		},
		{
			name: "markdown wrap around url with balanced parens preserves inner pair",
			text: "veja (https://en.wikipedia.org/wiki/Foo_(bar)) ok",
			want: []string{"https://en.wikipedia.org/wiki/Foo_(bar)"},
		},
		{
			name: "url with balanced parens and no wrap is preserved",
			text: "visit https://en.wikipedia.org/wiki/Go_(programming_language) today",
			want: []string{"https://en.wikipedia.org/wiki/Go_(programming_language)"},
		},
		{
			name: "trailing period after close paren is cleaned",
			text: "see (https://foo.com/bar).",
			want: []string{"https://foo.com/bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractLinks(tt.text)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("ExtractLinks(%q) returned %d links, want %d: got=%v", tt.text, len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractLinks(%q)[%d] = %q, want %q", tt.text, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractShopeeLinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "shopee.com.br link",
			text: "buy at https://shopee.com.br/product123",
			want: []string{"https://shopee.com.br/product123"},
		},
		{
			name: "s.shopee.com.br link",
			text: "buy at https://s.shopee.com.br/abc",
			want: []string{"https://s.shopee.com.br/abc"},
		},
		{
			name: "shp.ee link",
			text: "buy at https://shp.ee/abc123",
			want: []string{"https://shp.ee/abc123"},
		},
		{
			name: "filters out non-shopee links",
			text: "https://example.com https://shopee.com.br/item https://google.com",
			want: []string{"https://shopee.com.br/item"},
		},
		{
			name: "no shopee links",
			text: "https://example.com https://amazon.com.br",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractShopeeLinks(tt.text)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("ExtractShopeeLinks(%q) returned %d links, want %d: got=%v", tt.text, len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractShopeeLinks(%q)[%d] = %q, want %q", tt.text, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractMLLinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "mercadolivre.com.br link",
			text: "buy at https://mercadolivre.com.br/item123",
			want: []string{"https://mercadolivre.com.br/item123"},
		},
		{
			name: "mercadolibre.com link",
			text: "buy at https://mercadolibre.com/item",
			want: []string{"https://mercadolibre.com/item"},
		},
		{
			name: "ml.com.br link",
			text: "buy at https://ml.com.br/abc",
			want: []string{"https://ml.com.br/abc"},
		},
		{
			name: "filters out non-ML links",
			text: "https://example.com https://mercadolivre.com.br/item https://google.com",
			want: []string{"https://mercadolivre.com.br/item"},
		},
		{
			name: "no ML links",
			text: "https://example.com https://shopee.com.br",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractMLLinks(tt.text)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("ExtractMLLinks(%q) returned %d links, want %d: got=%v", tt.text, len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractMLLinks(%q)[%d] = %q, want %q", tt.text, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsShopeeLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "shopee.com.br",
			url:  "https://shopee.com.br/product",
			want: true,
		},
		{
			name: "s.shopee.com.br",
			url:  "https://s.shopee.com.br/abc",
			want: true,
		},
		{
			name: "shp.ee",
			url:  "https://shp.ee/abc",
			want: true,
		},
		{
			name: "shope.ee (encurtador global)",
			url:  "https://shope.ee/abc123",
			want: true,
		},
		{
			name: "case insensitive SHOPEE",
			url:  "https://SHOPEE.COM.BR/product",
			want: true,
		},
		{
			name: "case insensitive SHP",
			url:  "https://SHP.EE/abc",
			want: true,
		},
		{
			name: "not shopee",
			url:  "https://example.com",
			want: false,
		},
		{
			name: "amazon",
			url:  "https://amazon.com.br",
			want: false,
		},
		{
			name: "empty string",
			url:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsShopeeLink(tt.url)
			if got != tt.want {
				t.Errorf("IsShopeeLink(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestIsMLLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "mercadolivre.com.br",
			url:  "https://mercadolivre.com.br/item",
			want: true,
		},
		{
			name: "mercadolibre.com",
			url:  "https://mercadolibre.com/item",
			want: true,
		},
		{
			name: "ml.com.br",
			url:  "https://ml.com.br/abc",
			want: true,
		},
		{
			name: "meli.la short link",
			url:  "https://meli.la/28fhqrh",
			want: true,
		},
		{
			name: "case insensitive ML",
			url:  "https://MERCADOLIVRE.COM.BR/item",
			want: true,
		},
		{
			name: "case insensitive mercadolibre",
			url:  "https://MERCADOLIBRE.COM/item",
			want: true,
		},
		{
			name: "not ML link",
			url:  "https://example.com",
			want: false,
		},
		{
			name: "shopee link",
			url:  "https://shopee.com.br",
			want: false,
		},
		{
			name: "empty string",
			url:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsMLLink(tt.url)
			if got != tt.want {
				t.Errorf("IsMLLink(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestReplaceLinksInText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		oldToNew map[string]string
		want     string
	}{
		{
			name: "replaces single link",
			text: "visit https://old.com today",
			oldToNew: map[string]string{
				"https://old.com": "https://new.com",
			},
			want: "visit https://new.com today",
		},
		{
			name: "replaces multiple links",
			text: "https://a.com and https://b.com",
			oldToNew: map[string]string{
				"https://a.com": "https://x.com",
				"https://b.com": "https://y.com",
			},
			want: "https://x.com and https://y.com",
		},
		{
			name:     "empty map returns unchanged",
			text:     "https://example.com",
			oldToNew: map[string]string{},
			want:     "https://example.com",
		},
		{
			name:     "nil map returns unchanged",
			text:     "hello world",
			oldToNew: nil,
			want:     "hello world",
		},
		{
			name: "no match returns unchanged",
			text: "https://example.com",
			oldToNew: map[string]string{
				"https://other.com": "https://new.com",
			},
			want: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ReplaceLinksInText(tt.text, tt.oldToNew)
			if got != tt.want {
				t.Errorf("ReplaceLinksInText(%q, ...) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestContentFingerprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		links []string
		want  string
	}{
		{
			name:  "text only normalized lowercase trimmed",
			text:  "  Hello   World  ",
			links: nil,
			want:  "hello world",
		},
		{
			name:  "text with links",
			text:  "check this",
			links: []string{"https://a.com", "https://b.com"},
			want:  "check this|https://a.com,https://b.com",
		},
		{
			name:  "no links just text",
			text:  "simple text",
			links: []string{},
			want:  "simple text",
		},
		{
			name:  "empty text with links",
			text:  "",
			links: []string{"https://a.com"},
			want:  "|https://a.com",
		},
		{
			name:  "text with extra whitespace normalized",
			text:  "  multiple   spaces  \n  newlines  ",
			links: nil,
			want:  "multiple spaces newlines",
		},
		{
			name:  "case normalized to lowercase",
			text:  "HELLO WORLD",
			links: nil,
			want:  "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ContentFingerprint(tt.text, tt.links)
			if got != tt.want {
				t.Errorf("ContentFingerprint(%q, %v) = %q, want %q", tt.text, tt.links, got, tt.want)
			}
		})
	}
}

func TestExtractLinks_EmojiGluedToURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "emoji ‼️ colado (caso real de produção)",
			in:   "corre https://s.shopee.com.br/qem5OVT0i‼️",
			want: "https://s.shopee.com.br/qem5OVT0i",
		},
		{
			name: "emoji 🔥 colado",
			in:   "https://s.shopee.com.br/abc123🔥🔥",
			want: "https://s.shopee.com.br/abc123",
		},
		{
			name: "acento fora da URL não entra",
			in:   "veja https://shopee.com.br/product/1/2é",
			want: "https://shopee.com.br/product/1/2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := ExtractLinks(tt.in)
			if len(links) != 1 || links[0] != tt.want {
				t.Errorf("ExtractLinks(%q) = %v, want [%s]", tt.in, links, tt.want)
			}
		})
	}
}

func TestCutURLAtNonASCII(t *testing.T) {
	if got := CutURLAtNonASCII("https://x.com/a‼️b"); got != "https://x.com/a" {
		t.Errorf("got %q", got)
	}
	if got := CutURLAtNonASCII("https://x.com/ab"); got != "https://x.com/ab" {
		t.Errorf("ascii-only must pass unchanged, got %q", got)
	}
}

func TestIsOtherStoreLink(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"amazon.com.br", "https://www.amazon.com.br/dp/B0ABC123", true},
		{"amzn.to short", "https://amzn.to/3xYzAbC", true},
		{"magazineluiza", "https://www.magazineluiza.com.br/produto/p/123", true},
		{"magazinevoce (divulgador)", "https://www.magazinevoce.com.br/loja/produto/p/1", true},
		{"shein", "https://br.shein.com/item-p-123.html", true},
		{"aliexpress", "https://pt.aliexpress.com/item/100500.html", true},
		// Awin: anunciante e deeplink da rede. Ambos precisam ser stripados
		// por omissão — um cread.php da fonte carrega o awinaffid DELA.
		{"awin advertiser", "https://www.netshoes.com.br/produto/tenis-1", true},
		{"awin deeplink", "https://www.awin1.com/cread.php?awinmid=1&awinaffid=9&ued=x", true},
		{"shopee NOT other store", "https://shopee.com.br/produto", false},
		{"mercadolivre NOT other store", "https://meli.la/1PQkeS1", false},
		{"instagram passes", "https://instagram.com/perfil", false},
		{"site generico passa", "https://meusite.com.br/blog", false},
		{"convite de grupo passa", "https://chat.whatsapp.com/ABC", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOtherStoreLink(tt.url); got != tt.want {
				t.Errorf("IsOtherStoreLink(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
