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
  // Preferred sender chip (database id of an instance, string form). Set
  // by the form's chip selector and persisted; the worker uses it as the
  // sender for every dispatch from this automation. Empty / null = no
  // preference (legacy default).
  //
  // Deprecated since 049: kept for rollback compat. The form now writes
  // instance_ids (multi-chip fan-out for offer_generate); the backend
  // syncs instance_id with instance_ids[0].
  instance_id?: string | null;
  // Multi-chip preference (049). Non-empty list activates fan-out for
  // offer_generate — the worker dispatches the same offer through every
  // selected chip on whichever destination groups they participate in.
  // For non-fan-out modes the dispatcher still uses just instance_ids[0].
  instance_ids?: string[];
  // Manual chip routing per destination group (050). Maps wa_group_id →
  // instance_id (as string, matching instance_ids). Groups present here
  // pin to the named chip; groups absent fall through to the sticky
  // hash distribution across instance_ids. Only consulted when 2+ chips
  // are selected — single-chip automations ignore the field.
  chip_overrides?: Record<string, string>;
  // Per-automation opt-out for the coupon rule engine. true (default)
  // means the worker matches global CouponRules on every dispatch.
  // false silences cupom injection for this automation entirely.
  apply_coupon_rules?: boolean;
  // LIST mode: quando true (default), o backend reseta automaticamente
  // os items consumidos da lista quando todos foram enviados e a
  // automation continua rodando em loop. Quando false, a automation
  // para após esgotar.
  repeat_when_finished?: boolean;
  // Data calendário (YYYY-MM-DD) a partir da qual o scheduler ativa.
  // Empty/unset = sem agendamento, roda imediatamente.
  start_date?: string;
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
