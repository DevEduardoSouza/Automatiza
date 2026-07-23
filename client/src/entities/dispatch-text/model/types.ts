/**
 * Dispatch text entity for message templates with variable placeholders.
 * Used in automations to send customizable messages.
 */
export interface DispatchText {
  id: string;
  user_id: string;
  name: string;
  /** @deprecated Use copies instead. Returns copies[0] for backward compatibility. */
  body: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  copies: string[];
}

/**
 * Form state for creating/editing dispatch texts.
 */
export interface DispatchTextFormState {
  name: string;
  copies: string[];
  is_active: boolean;
}

/**
 * Variable keys available for template placeholders.
 */
export const VARIABLE_KEYS = [
  'productName',
  'price',
  'priceFormatted',
  'originalPrice',
  'originalPriceFormatted',
  'discount',
  'affiliateLink',
  'priceWarning',
  'cta',
  'couponBlock',
  'info',
] as const;

export type VariableKey = (typeof VARIABLE_KEYS)[number];

/**
 * Maps variable keys to their placeholder format in templates.
 */
export const VARIABLE_MAP: Record<VariableKey, string> = {
  productName: '{nome_produto}',
  price: '{preco}',
  priceFormatted: '{preco_formatado}',
  originalPrice: '{preco_original}',
  originalPriceFormatted: '{preco_original_formatado}',
  // O chip insere a versão FORMATADA ("25% OFF"), que é a que o usuário
  // escreve direto na mensagem. O número cru continua disponível como
  // {desconto} para quem monta o próprio sufixo (ex.: "{desconto}% OFF").
  // É o desconto do PRODUTO, nunca o do cupom; quando não houver, o backend
  // remove a linha inteira em vez de deixar decoração órfã.
  discount: '{desconto_formatado}',
  affiliateLink: '{link_afiliado}',
  priceWarning: '{aviso_preco}',
  cta: '{cta}',
  couponBlock: '{cupom_bloco}',
  // Texto livre extraído da tag info[...] no corpo da mensagem que
  // dispara o /offer-generate. Renderiza vazio quando a mensagem não
  // tem a tag, e o cleanup de newlines em RenderTemplate colapsa a
  // linha em branco resultante.
  info: '{info}',
};

/**
 * Renders a template string by replacing variables with values.
 * @param template - The template string with {variable} placeholders
 * @param vars - Key-value pairs for variable substitution
 * @returns The rendered string with variables replaced
 */
export function renderTemplate(template: string, vars: Record<string, string>): string {
  const replaced = String(template || '').replace(/\{\s*([a-zA-Z0-9_]+)\s*\}/g, (m, k) => {
    const key = String(k || '').trim();
    if (!key) return m;
    return vars[key] ?? '';
  });
  // Paridade com automatiza-go/internal/lib/text/dispatch_template.go:
  // RenderTemplate colapsa 3+ \n para \n\n. Sem esse passo o preview mostra
  // espaço extra quando {cupom_bloco} (que vem com \n\n prefixado pelo Go)
  // cai num template que já tem \n\n antes do placeholder.
  return replaced.replace(/\n{3,}/g, '\n\n');
}
