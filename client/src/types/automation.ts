/**
 * Types and interfaces for offer automations
 */

export type { DispatchText } from '@/entities/dispatch-text';

export type StoreKey = 'shopee' | 'amazon' | 'magalu' | 'ml' | 'shein' | 'aliexpress' | 'awin';

export type AutomationMode = 'search' | 'monitor' | 'list' | 'offer_generate';
export type PriorityType = 'discount' | 'price' | 'commission';
export type FilterType = 'light' | 'heavy';
export type AutomationStatus = 'active' | 'paused';

export interface MonitoringTextRules {
  substituicoes: Array<{ de: string; para: string }>;
  remover_termos: string[];
  prefixo: string;
  sufixo: string;
  limpar_telefones: boolean;
  limpar_arrobas: boolean;
}

export const DEFAULT_MONITOR_RULES: MonitoringTextRules = {
  substituicoes: [],
  remover_termos: [],
  prefixo: '',
  sufixo: '',
  limpar_telefones: false,
  limpar_arrobas: false,
};

export interface Automation {
  id: string;
  name: string;
  mode: AutomationMode;
  type?: 'monitoring' | 'search' | 'list' | 'offer_generate';
  dispatch_text_id?: string | null;
  monitoring_text_rules?: MonitoringTextRules | null;
  start_time: string;
  end_time: string;
  interval_minutes: number;
  send_groups: string[];
  monitor_groups?: string[];
  monitor_group_ids?: string[] | null;
  stores: StoreKey[];
  categories: string[];
  priority: PriorityType;
  filter_type: FilterType;
  min_discount?: number | null;
  min_price?: number | null;
  max_price?: number | null;
  texts: string[];
  ctas: string[];
  cta_ids: string[];
  last_run_at?: string;
  next_run_at?: string;
  status: AutomationStatus;
  last_error?: string;
  delay_seconds?: number;
  coupon_id?: string | null;
  ml_link_list_id?: string | null;
  list_items_per_run?: number;
  list_include_image?: boolean;
  list_send_as_image?: boolean;
}

export interface AutomationGroup {
  id: string;
  name: string;
  wa_group_id: string;
  is_favorite: boolean;
  // Database id of the WhatsApp instance that owns this group. Used by the
  // form to enforce the per-chip 20-groups cap before submit.
  instance_id?: string;
  // Every chip whose sync has seen this group (string form). The form's
  // chip filter uses this so groups shared between chips show under each
  // participating chip — not just the owner.
  participating_instance_ids?: string[];
}

export interface PreviewOffer {
  title?: string;
  originalPrice?: number;
  finalPrice?: number;
  discountPercent?: number;
  imageUrl?: string;
  productUrl?: string;
  storeName?: string;
  commissionRate?: number;
}

export interface AutomationCoupon {
  id: string;
  code: string;
  title: string;
  store: string;
}

export interface AutomationPayload extends Omit<Partial<Automation>, 'monitor_groups'> {
  user_id?: string;
  type?: 'monitoring' | 'search' | 'list' | 'offer_generate';
  monitor_group_ids?: string[] | null;
}

export interface MlLinkList {
  id: string;
  name: string;
}

export interface DiagStep {
  step: string;
  ok: boolean;
  detail: string;
  message?: string;
}

export interface DiagResult {
  data?: {
    code?: string;
    message?: string;
    steps?: DiagStep[];
  };
  code?: string;
  message?: string;
  steps?: DiagStep[];
  automation_id?: string;
  offers_found?: number;
  sent_to?: string[];
  error?: string;
}

/**
 * Safely get monitoring text rules with fallback to defaults
 */
export function getMonitorRules(
  rules: MonitoringTextRules | null | undefined
): MonitoringTextRules {
  return rules ?? DEFAULT_MONITOR_RULES;
}
