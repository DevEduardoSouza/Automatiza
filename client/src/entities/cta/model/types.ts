/**
 * CTA (Call-to-Action) entity for affiliate marketing messages.
 * Used in automations to send customizable CTAs to WhatsApp groups.
 */
export interface CTA {
  id: string;
  user_id: string;
  message: string;
  platform: CTAPlatform;
  cta_type: CTATypeValue;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export type CTAPlatform = 'all' | 'shopee' | 'ml';
export type CTATypeValue = 'all' | 'normal' | 'discount';

/**
 * Form state for creating/editing CTAs.
 */
export interface CTAFormState {
  message: string;
  platform: CTAPlatform;
  cta_type: CTATypeValue;
  is_active: boolean;
}

export const CTA_PLATFORMS: { value: CTAPlatform; labelKey: string }[] = [
  { value: 'all', labelKey: 'pages.ctas.platforms.all' },
  { value: 'shopee', labelKey: 'pages.ctas.platforms.shopee' },
  { value: 'ml', labelKey: 'pages.ctas.platforms.ml' },
];

export const CTA_TYPES: { value: CTATypeValue; labelKey: string }[] = [
  { value: 'all', labelKey: 'pages.ctas.types.all' },
  { value: 'normal', labelKey: 'pages.ctas.types.normal' },
  { value: 'discount', labelKey: 'pages.ctas.types.discount' },
];
