package text

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// TemplateVar represents a variable for template rendering
type TemplateVar struct {
	Key   string
	Value string
}

// Portuguese template variable names (matching Node.js parity)
const (
	VarNomeProduto            = "nome_produto"
	VarLoja                   = "loja"
	VarPreco                  = "preco"
	VarPrecoFormatado         = "preco_formatado"
	VarPrecoOriginal          = "preco_original"
	VarPrecoOriginalFormatado = "preco_original_formatado"
	VarLinkAfiliado           = "link_afiliado"
	VarLinkProduto            = "link_produto"
	VarCTA                    = "cta"
	VarCupom                  = "cupom"
	VarCupomBloco             = "cupom_bloco"
	VarAvisoPreco             = "aviso_preco"
	// VarInfo is the free-form line populated by the info[...] tag in the
	// trigger message (see ParseInfoTag). Renders to "" when the message
	// has no info tag, which collapses any blank line around it via the
	// multi-newline cleanup in RenderTemplate.
	VarInfo = "info"
	// ML-specific
	VarDesconto        = "desconto"
	VarCondicao        = "condicao"
	VarVendidos        = "vendidos"
	VarAvaliacao       = "avaliacao"
	VarAvaliacoesTotal = "avaliacoes_total"
	VarParcelas        = "parcelas"

	// Legacy English names (kept for backward compat with existing templates)
	VarTitle         = "title"
	VarPrice         = "price"
	VarOriginalPrice = "originalPrice"
	VarDiscount      = "discount"
	VarLink          = "link"
	VarAffiliateLink = "affiliateLink"
	VarImage         = "image"
	VarStore         = "store"
	VarCategory      = "category"
	VarCouponCode    = "couponCode"
	VarCouponText    = "couponText"
	VarSeller        = "seller"
)

// AvisoPrecoDefault is the standard price disclaimer
const AvisoPrecoDefault = "⚠️ O preço e disponibilidade do produto podem variar. As promoções são por tempo limitado"

// RenderTemplate renders a template string with variables
// Supports both {{variable}} and {variable} syntax
func RenderTemplate(template string, vars map[string]string) string {
	result := template

	// Replace {{variable}} syntax first, then {variable}
	for key, value := range vars {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
		result = strings.ReplaceAll(result, "{"+key+"}", value)
	}

	// Clean up any remaining unreplaced variables.
	// Exclude spintax alternations ({a|b|c}) from the sweep so authored spintax
	// survives rendering and is expanded later, per-recipient, by text.Vary.
	// Disambiguation: a placeholder is {single_token} (no '|'); spintax is
	// {a|b} (contains '|'). The [^{}|] classes stop the match at a '|'.
	unreplacedRegex := regexp.MustCompile(`\{\{[^{}|]+\}\}|\{[^{}|]+\}`)
	result = unreplacedRegex.ReplaceAllString(result, "")

	// Clean up multiple newlines that might result from empty variables
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result)
}

// StripEmptyOriginalPriceLine remove a linha do "preço original" quando a
// oferta não tem esse valor (FormatBRLSpaced devolve "" para preço <= 0).
//
// Sem isso, um template como `De ~{preco_original_formatado}~` renderiza um
// "De ~~" pendurado (riscado vazio), e o default `De {preco_original_formatado}`
// vira um "De " solto. É comum no Mercado Livre, que frequentemente não traz o
// preço cheio. A linha só é removida quando NÃO contém também o preço atual,
// pra preservar layouts que juntem "De X por Y" na mesma linha.
//
// Reconhece tanto os placeholders atuais ({preco_original_formatado} etc.)
// quanto o legado {{PRODUTO_PRECO_ORIGINAL}}. Os callers que pré-substituem o
// placeholder legado por "" (ex.: automation_service) devem chamar esta função
// ANTES dessa substituição — depois a linha já virou "De " e não há mais o que
// casar. Os RenderXOfferTemplate já a aplicam internamente para os atuais.
func StripEmptyOriginalPriceLine(template string, originalPriceEmpty bool) string {
	if !originalPriceEmpty {
		return template
	}

	originalVars := []string{VarPrecoOriginalFormatado, VarPrecoOriginal, VarOriginalPrice, "PRODUTO_PRECO_ORIGINAL"}
	currentVars := []string{VarPrecoFormatado, VarPreco, VarPrice, "PRODUTO_PRECO"}

	lineHasVar := func(line string, keys []string) bool {
		for _, k := range keys {
			if strings.Contains(line, "{"+k+"}") || strings.Contains(line, "{{"+k+"}}") {
				return true
			}
		}
		return false
	}

	lines := strings.Split(template, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if lineHasVar(line, originalVars) && !lineHasVar(line, currentVars) {
			continue // linha "De ~~" pendurada — descarta
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// ShopeeOfferVars holds variables for a Shopee offer template
type ShopeeOfferVars struct {
	Title         string
	Price         float64
	OriginalPrice float64
	Discount      float64
	Link          string
	AffiliateLink string
	Image         string
	Category      string
	Store         string
	CTA           string
	CouponCode    string
	CouponText    string
	CouponBlock   string
	Info          string
}

// MLOfferVars holds variables for a Mercado Livre offer template
type MLOfferVars struct {
	Title         string
	Price         float64
	OriginalPrice float64
	Discount      float64
	Link          string
	AffiliateLink string
	Image         string
	Seller        string
	Condition     string
	SoldCount     int
	Rating        float64
	ReviewCount   int
	Installments  string
	CTA           string
	CouponCode    string
	CouponText    string
	CouponBlock   string
	Info          string
}

// RenderShopeeOfferTemplate renders a template for a Shopee offer
// Maps both Portuguese (nome_produto, preco) and English (title, price) variable names
func RenderShopeeOfferTemplate(template string, offer ShopeeOfferVars) string {
	store := offer.Store
	if store == "" {
		store = "Shopee"
	}

	vars := map[string]string{
		// Portuguese names (primary — used by dispatch texts)
		VarNomeProduto:            offer.Title,
		VarLoja:                   store,
		VarPreco:                  FormatBRNumber(offer.Price),
		VarPrecoFormatado:         FormatBRLSpaced(offer.Price),
		VarPrecoOriginal:          FormatBRNumber(offer.OriginalPrice),
		VarPrecoOriginalFormatado: FormatBRLSpaced(offer.OriginalPrice),
		VarLinkAfiliado:           offer.AffiliateLink,
		VarLinkProduto:            offer.Link,
		VarCTA:                    offer.CTA,
		VarCupom:                  offer.CouponCode,
		VarCupomBloco:             offer.CouponBlock,
		VarAvisoPreco:             AvisoPrecoDefault,
		VarInfo:                   offer.Info,
		// English names (backward compat with older templates)
		VarTitle:         offer.Title,
		VarPrice:         FormatBRLSpaced(offer.Price),
		VarOriginalPrice: FormatBRLSpaced(offer.OriginalPrice),
		VarDiscount:      FormatDiscount(offer.Discount),
		VarLink:          offer.Link,
		VarAffiliateLink: offer.AffiliateLink,
		VarImage:         offer.Image,
		VarStore:         store,
		VarCategory:      offer.Category,
	}

	if offer.CouponCode != "" {
		vars[VarCouponCode] = offer.CouponCode
		vars[VarCouponText] = offer.CouponText
	}

	template = StripEmptyOriginalPriceLine(template, offer.OriginalPrice <= 0)
	return RenderTemplate(template, vars)
}

// RenderMLOfferTemplate renders a template for a Mercado Livre offer
// Maps both Portuguese and English variable names, including ML-specific fields
func RenderMLOfferTemplate(template string, offer MLOfferVars) string {
	// Strip embedded price from title (e.g., "Product Name - R$ 199,90" → "Product Name")
	// Matches Node.js parity: buildMlDispatchVars calls stripEmbeddedPriceFromTitle
	cleanTitle := StripEmbeddedPriceFromTitle(offer.Title)

	vars := map[string]string{
		// Portuguese names (primary)
		VarNomeProduto:            cleanTitle,
		VarLoja:                   "Mercado Livre",
		VarPreco:                  FormatBRNumber(offer.Price),
		VarPrecoFormatado:         FormatBRLSpaced(offer.Price),
		VarPrecoOriginal:          FormatBRNumber(offer.OriginalPrice),
		VarPrecoOriginalFormatado: FormatBRLSpaced(offer.OriginalPrice),
		VarLinkAfiliado:           offer.AffiliateLink,
		VarLinkProduto:            offer.Link,
		VarCTA:                    offer.CTA,
		VarCupom:                  offer.CouponCode,
		VarCupomBloco:             offer.CouponBlock,
		VarAvisoPreco:             AvisoPrecoDefault,
		VarInfo:                   offer.Info,
		// ML-specific
		VarDesconto:        formatDiscountPercent(offer.Discount),
		VarCondicao:        offer.Condition,
		VarVendidos:        formatSoldCount(offer.SoldCount),
		VarAvaliacao:       formatRating(offer.Rating),
		VarAvaliacoesTotal: formatReviewCount(offer.ReviewCount),
		VarParcelas:        offer.Installments,
		// English names (backward compat)
		VarTitle:         cleanTitle,
		VarPrice:         FormatBRLSpaced(offer.Price),
		VarOriginalPrice: FormatBRLSpaced(offer.OriginalPrice),
		VarDiscount:      FormatDiscount(offer.Discount),
		VarLink:          offer.Link,
		VarAffiliateLink: offer.AffiliateLink,
		VarImage:         offer.Image,
		VarStore:         "Mercado Livre",
		VarSeller:        offer.Seller,
	}

	if offer.CouponCode != "" {
		vars[VarCouponCode] = offer.CouponCode
		vars[VarCouponText] = offer.CouponText
	}

	template = StripEmptyOriginalPriceLine(template, offer.OriginalPrice <= 0)
	return RenderTemplate(template, vars)
}

// FormatBRL formats a price as "R$206,99" (no space, comma separator)
// This matches the Node.js Intl.NumberFormat("pt-BR", {style: "currency", currency: "BRL"}).format()
func FormatBRL(price float64) string {
	if price <= 0 {
		return ""
	}
	return "R$" + formatBrazilianNumber(price)
}

// FormatBRLSpaced formats a price as "R$ 206,99" (with space, comma separator)
// This matches the Node.js formatBRL(price).replace("R$", "R$ ")
func FormatBRLSpaced(price float64) string {
	if price <= 0 {
		return ""
	}
	return "R$ " + formatBrazilianNumber(price)
}

// FormatBRNumber formats a number as "206,99" (Brazilian format, no currency symbol)
// This matches the Node.js Intl.NumberFormat("pt-BR", {minimumFractionDigits: 2}).format()
func FormatBRNumber(value float64) string {
	if value <= 0 {
		return ""
	}
	return formatBrazilianNumber(value)
}

// FormatPrice formats a price value for display (kept for backward compatibility)
// Uses the spaced format "R$ 206,99"
func FormatPrice(price float64) string {
	return FormatBRLSpaced(price)
}

// FormatDiscount formats a discount percentage
func FormatDiscount(discount float64) string {
	if discount <= 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%% OFF", discount)
}

// formatBrazilianNumber formats a float as "1.234,56" (Brazilian number format)
func formatBrazilianNumber(value float64) string {
	// Round to 2 decimal places
	rounded := math.Round(value*100) / 100

	// Format with 2 decimal places
	str := fmt.Sprintf("%.2f", rounded)

	// Split into integer and decimal parts
	parts := strings.SplitN(str, ".", 2)
	intPart := parts[0]
	decPart := parts[1]

	// Add thousands separator (period in Brazilian format)
	if len(intPart) > 3 {
		var result strings.Builder
		for i, ch := range intPart {
			if i > 0 && (len(intPart)-i)%3 == 0 {
				result.WriteByte('.')
			}
			result.WriteRune(ch)
		}
		intPart = result.String()
	}

	// Use comma as decimal separator
	return intPart + "," + decPart
}

// embeddedPriceRegex matches embedded price suffixes like " - R$ 123,45" or " - R$123.45"
var embeddedPriceRegex = regexp.MustCompile(`\s*-\s*R\$\s*[\d.,]+`)

// StripEmbeddedPriceFromTitle removes embedded price from ML product titles
// (matches Node.js parity: title.replace(/\s*-\s*R\$\s*[\d\.\,]+/i, "").trim())
func StripEmbeddedPriceFromTitle(title string) string {
	return strings.TrimSpace(embeddedPriceRegex.ReplaceAllString(title, ""))
}

// formatDiscountPercent formats discount as a simple percentage string
func formatDiscountPercent(discount float64) string {
	if discount <= 0 {
		return ""
	}
	return fmt.Sprintf("%.0f", discount)
}

// formatSoldCount formats sold count like "+1,2 mil vendidos" (matching Node.js parity)
func formatSoldCount(count int) string {
	if count <= 0 {
		return ""
	}
	if count >= 1000 {
		mil := math.Round(float64(count)/1000*10) / 10
		s := fmt.Sprintf("%.1f", mil)
		s = strings.Replace(s, ".", ",", 1)
		// Remove trailing ,0
		s = strings.TrimSuffix(s, ",0")
		return fmt.Sprintf("+%s mil vendidos", s)
	}
	return fmt.Sprintf("+%d vendidos", count)
}

// formatRating formats a rating value as a string
func formatRating(rating float64) string {
	if rating <= 0 {
		return ""
	}
	return fmt.Sprintf("%.1f", rating)
}

// formatReviewCount formats review count as string
func formatReviewCount(count int) string {
	if count <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", count)
}

// DefaultShopeeTemplate returns the default template for Shopee offers
func DefaultShopeeTemplate() string {
	return `🛍️ {nome_produto}

De {preco_original_formatado}
💸 Por {preco_formatado}

🛒Compre aqui: {link_afiliado}

⚠ Promoção sujeita a alteração a qualquer momento.`
}

// DefaultMLTemplate returns the default template for Mercado Livre offers
func DefaultMLTemplate() string {
	return `🛍️ {nome_produto}

De {preco_original_formatado}
💸 Por {preco_formatado}

🛒Compre aqui: {link_afiliado}

⚠ Promoção sujeita a alteração a qualquer momento.`
}

// RenderMLCouponNotice formats a mirrored Mercado Livre coupon/campaign notice:
// a fixed header plus the original message body untouched (the caller has
// already swapped any ML links for the owner's affiliate links). Hardcoded on
// purpose — a coupon notice is not an offer, so it needs no template entity,
// no config, and no product preview. The header sits on its own line so the
// send-time SeparateURLEmojiGlue normalization never diverges from the
// selfsend loop-guard mark (both see the same body).
func RenderMLCouponNotice(originalBody string) string {
	body := strings.TrimSpace(originalBody)
	if body == "" {
		return ""
	}
	return "🎟️ *CUPOM MERCADO LIVRE* 🎟️\n\n" + body
}

// ExtractTemplateVariables extracts variable names from a template
func ExtractTemplateVariables(template string) []string {
	varRegex := regexp.MustCompile(`\{\{([^}]+)\}\}|\{([^}]+)\}`)
	matches := varRegex.FindAllStringSubmatch(template, -1)

	vars := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		varName := match[1]
		if varName == "" {
			varName = match[2]
		}
		if varName != "" && !seen[varName] {
			seen[varName] = true
			vars = append(vars, varName)
		}
	}

	return vars
}
