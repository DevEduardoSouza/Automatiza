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
  'affiliateLink',
  'priceWarning',
  'cta',
  'couponBlock',
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
  affiliateLink: '{link_afiliado}',
  priceWarning: '{aviso_preco}',
  cta: '{cta}',
  couponBlock: '{cupom_bloco}',
};

/**
 * Renders a template string by replacing variables with values.
 * @param template - The template string with {variable} placeholders
 * @param vars - Key-value pairs for variable substitution
 * @returns The rendered string with variables replaced
 */
export function renderTemplate(template: string, vars: Record<string, string>): string {
  return String(template || '').replace(/\{\s*([a-zA-Z0-9_]+)\s*\}/g, (m, k) => {
    const key = String(k || '').trim();
    if (!key) return m;
    return vars[key] ?? '';
  });
}
