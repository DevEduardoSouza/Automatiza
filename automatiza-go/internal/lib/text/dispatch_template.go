package text

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
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
	// Desconto do PRODUTO (nunca o do cupom), preenchido igual nos dois
	// marketplaces a partir do mesmo percentual normalizado: VarDesconto é o
	// inteiro cru ("25", que mantém o template legado "{desconto}% OFF"
	// funcionando) e VarDescontoFormatado é o texto pronto ("25% OFF").
	// Ambos vazios quando o percentual é inválido — nunca "0% OFF".
	VarDesconto          = "desconto"
	VarDescontoFormatado = "desconto_formatado"
	// ML-specific
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

// StripEmptyDiscountLine remove a linha do desconto quando o percentual é
// inválido (as duas variáveis renderizam ""), no mesmo molde de
// StripEmptyOriginalPriceLine.
//
// A linha só sai quando, tirados os placeholders de desconto, NÃO sobra
// nenhuma letra ou dígito: "🔥 {desconto_formatado} 🔥" e "-{desconto}%" são
// decoração órfã e somem inteiras. Já "Por {preco_formatado} {desconto_formatado}"
// e o legado "{desconto}% OFF" carregam conteúdo próprio (o outro placeholder
// tem letras; "OFF" também) e ficam — o RenderTemplate apenas apaga a variável,
// que é a regra "linha mista perde só a variável".
//
// Só considera as variáveis em português. O alias legado {discount} fica de
// fora de propósito: ele tem vazio próprio (FormatDiscount) e um Discount
// fracionário como 0.4 ainda imprime "0% OFF" ali — incluí-lo apagaria linha
// que hoje é enviada.
func StripEmptyDiscountLine(template string, discountEmpty bool) string {
	if !discountEmpty {
		return template
	}

	lines := strings.Split(template, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		rest := line
		for _, k := range []string{VarDescontoFormatado, VarDesconto} {
			rest = strings.ReplaceAll(rest, "{{"+k+"}}", "")
			rest = strings.ReplaceAll(rest, "{"+k+"}", "")
		}
		if rest != line && !hasLetterOrDigit(rest) {
			continue // era só desconto + decoração — descarta a linha
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// hasLetterOrDigit diz se a string tem algum caractere de conteúdo. Emoji,
// pontuação e espaço não contam — é o que separa "decoração órfã" de
// "texto que o usuário quis manter".
func hasLetterOrDigit(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
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

	discountPercent := NormalizeDiscountPercent(offer.Discount)

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
		// Desconto do produto — mesmo par de saídas do Mercado Livre.
		VarDesconto:          formatDiscountPercent(discountPercent),
		VarDescontoFormatado: FormatDiscount(discountPercent),
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
	template = StripEmptyDiscountLine(template, discountPercent <= 0)
	return RenderTemplate(template, vars)
}

// RenderMLOfferTemplate renders a template for a Mercado Livre offer
// Maps both Portuguese and English variable names, including ML-specific fields
func RenderMLOfferTemplate(template string, offer MLOfferVars) string {
	// Strip embedded price from title (e.g., "Product Name - R$ 199,90" → "Product Name")
	// Matches Node.js parity: buildMlDispatchVars calls stripEmbeddedPriceFromTitle
	cleanTitle := StripEmbeddedPriceFromTitle(offer.Title)

	discountPercent := NormalizeDiscountPercent(offer.Discount)

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
		// Desconto do produto — mesmo par de saídas da Shopee.
		VarDesconto:          formatDiscountPercent(discountPercent),
		VarDescontoFormatado: FormatDiscount(discountPercent),
		// ML-specific
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
	template = StripEmptyDiscountLine(template, discountPercent <= 0)
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

// NormalizeDiscountPercent devolve o percentual pronto para exibição, ou 0
// (= vazio) quando o valor não descreve um desconto real.
//
// Arredonda para inteiro ANTES de validar a faixa, então 25.4→25, 99.6→100 e
// 0.4→0 (meio ponto percentual não é "desconto de 0%" no texto: é ruído de
// cálculo por preços). Fora de 1..100 — negativo, 150, NaN, ±Inf — vira 0.
//
// É a única normalização usada pelos dois renderers; quem apresenta são
// formatDiscountPercent (número cru) e FormatDiscount (texto completo).
// FormatDiscount continua recebendo o valor CRU no alias legado {discount},
// que por contrato não muda de saída.
func NormalizeDiscountPercent(discount float64) float64 {
	if math.IsNaN(discount) || math.IsInf(discount, 0) {
		return 0
	}
	rounded := math.Round(discount)
	if rounded < 1 || rounded > 100 {
		return 0
	}
	return rounded
}

// DiscountPercentFromPrices calcula o percentual a partir dos preços, para
// quando a integração não informa a taxa. Exige preço cheio positivo e maior
// que o atual; qualquer outra combinação é 0 (= vazio).
//
// Vive aqui, mas é chamada nos SERVICES de propósito: só lá os dois preços
// são garantidamente pré-cupom. No renderer, Price já pode ter sido reduzido
// por cupom/tag de desconto, e o percentual do produto engoliria o do cupom.
func DiscountPercentFromPrices(original, current float64) float64 {
	if !(original > 0) || !(current >= 0) || original <= current {
		return 0
	}
	return NormalizeDiscountPercent((original - current) / original * 100)
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

// ShopeeCouponLimitedNotice é o rodapé fixo do aviso de cupom/campanha Shopee.
const ShopeeCouponLimitedNotice = "⚠ Cupom por tempo limitado, sujeito a alteração."

// RenderShopeeCouponNotice monta o corpo de um link Shopee convertido que NÃO é
// produto (cupom, campanha, coleção, carteira de vouchers, busca ou perfil): o
// CTA do cliente, o texto da origem já processado (regras + links trocados pelo
// afiliado do dono) e o aviso fixo. Deliberadamente sem RenderTemplate: o texto
// da fonte é conteúdo, não template — o sweep de placeholders comeria {chaves}
// literais do cupom.
func RenderShopeeCouponNotice(originalBody, cta string) string {
	body := strings.TrimSpace(originalBody)
	if body == "" {
		return ""
	}
	cta = strings.TrimSpace(cta)
	if cta == "" {
		return body + "\n\n" + ShopeeCouponLimitedNotice
	}
	return cta + "\n\n" + body + "\n\n" + ShopeeCouponLimitedNotice
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

// TemplateUsesVariable diz se o template autoriza uma variável. Reusa
// ExtractTemplateVariables — o tokenizador que já existe aqui — em vez de um
// regex próprio. Comparação crua e case-sensitive: `{cta}` e `{{cta}}` valem;
// `{CTA}`, `{ cta }`, `{cta_extra}` e `{cta|fallback}` (spintax) não.
//
// É um teste de AUTORIZAÇÃO, não de contagem: quem chama decide UM valor e o
// renderer o aplica em todas as ocorrências exatas daquele token.
//
// ATENÇÃO — este tokenizador NÃO é o mecanismo de substituição: RenderTemplate
// troca por strings.ReplaceAll("{"+key+"}"). Os dois concordam em toda
// autoria normal, mas divergem quando o token exato fica ENGOLIDO por um token
// maior (`{{cta}`, `{a{cta}`, `{opcao|compre {cta}}`): aqui dá false, e o
// ReplaceAll ainda acharia o `{cta}` interno. A divergência é sempre na direção
// segura — sem autorização o valor é "" e nada vaza — e é a mesma decisão da
// linha acima sobre spintax: token não-exato não autoriza CTA.
func TemplateUsesVariable(template, variable string) bool {
	for _, candidate := range ExtractTemplateVariables(template) {
		if candidate == variable {
			return true
		}
	}
	return false
}

// LinkReplacement é um par original→convertido na ORDEM em que o link aparece
// na mensagem-fonte. Converted vazio significa REMOÇÃO (loja bloqueada, link
// exhausted/superseded).
type LinkReplacement struct {
	Original  string
	Converted string
}

// ReplaceLinksLongestFirst troca os links no corpo aplicando primeiro os
// originais MAIS LONGOS.
//
// Sem a ordenação, uma URL curta (.../m/cupom) substituída antes da longa
// (.../m/cupom?mmp_pid=fonte) soldaria o link convertido dentro da querystring
// da outra, corrompendo as duas. Não normaliza espaços: quem chama normaliza
// uma vez só, para que corpo público e marca de selfsend vejam o mesmo texto.
func ReplaceLinksLongestFirst(body string, replacements []LinkReplacement) string {
	ordered := make([]LinkReplacement, len(replacements))
	copy(ordered, replacements)
	sort.SliceStable(ordered, func(i, j int) bool {
		return len(ordered[i].Original) > len(ordered[j].Original)
	})
	for _, r := range ordered {
		if r.Original == "" {
			continue
		}
		body = strings.ReplaceAll(body, r.Original, r.Converted)
	}
	return body
}

// paragraphBreakRegex separa parágrafos por uma ou mais linhas em branco
// (tolerando espaços/tabs na linha vazia). É a definição de "parágrafo" da
// §6.7 do plano: o bloco é a unidade que espera por link pending.
var paragraphBreakRegex = regexp.MustCompile(`\r?\n[ \t]*\r?\n+`)

// SplitParagraphs devolve os parágrafos não vazios do corpo, na ordem.
func SplitParagraphs(body string) []string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	raw := paragraphBreakRegex.Split(normalized, -1)
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// SelectParagraphsContaining devolve, na ordem da fonte e cada um UMA vez, os
// parágrafos do corpo já processado que contêm alguma das agulhas.
//
// Opera sobre o corpo processado de propósito: se uma regra do operador removeu
// o parágrafo, ele não é ressuscitado aqui. Generaliza o antigo
// extractShopeeCouponParagraphs para qualquer loja — o seletor nunca soube de
// loja, só de "qual link convertido está neste bloco".
func SelectParagraphsContaining(body string, needles []string) string {
	if strings.TrimSpace(body) == "" || len(needles) == 0 {
		return ""
	}
	var selected []string
	for _, p := range SplitParagraphs(body) {
		for _, needle := range needles {
			if needle == "" {
				continue
			}
			if strings.Contains(p, needle) {
				selected = append(selected, p)
				break
			}
		}
	}
	return strings.Join(selected, "\n\n")
}

// RenderMixedCampaignNotice monta o aviso agregado quando a mesma mensagem
// trouxe campanhas de MAIS DE UMA loja.
//
// Por que não encadear RenderShopeeCouponNotice + RenderMLCouponNotice: cada um
// carimba o rótulo da SUA loja e o seu apêndice, então encadear produziria dois
// cabeçalhos, dois CTAs e dois rodapés no mesmo aviso. Aqui o corpo já vem
// agregado (parágrafos das duas lojas, cada um uma vez, na ordem original, com
// os links já trocados longest-first) e recebe CTA e rodapé UMA única vez.
// Deliberadamente sem rótulo de loja: inventar "SHOPEE + MERCADO LIVRE" seria
// afirmar a composição do aviso, que muda conforme o que ficou pending.
func RenderMixedCampaignNotice(aggregatedBody, cta string) string {
	body := strings.TrimSpace(aggregatedBody)
	if body == "" {
		return ""
	}
	cta = strings.TrimSpace(cta)
	if cta == "" {
		return body + "\n\n" + ShopeeCouponLimitedNotice
	}
	return cta + "\n\n" + body + "\n\n" + ShopeeCouponLimitedNotice
}
