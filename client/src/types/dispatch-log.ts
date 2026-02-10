/**
 * Types and interfaces for dispatch logs (automation logs)
 */
import type { StoreKey } from './automation';

export type { StoreKey };

export type DispatchStatus = 'queued' | 'sent' | 'skipped' | 'error';

export interface DispatchLog {
  id: string;
  automation_id: string;
  automation_name: string;
  store: StoreKey;
  group_id: string;
  group_name?: string;
  product_category?: string | null;
  product_url: string;
  product_page_url?: string | null;
  affiliate_url?: string;
  status: DispatchStatus;
  error?: string;
  coupon_id?: string | null;
  coupon_code?: string | null;
  coupon_applied?: boolean | null;
  coupon_skip_reason?: string | null;
  created_at: string;
}

export interface DispatchLogAutomation {
  id: string;
  name: string;
}

export interface DispatchLogGroup {
  id: string;
  name: string;
  wa_group_id: string;
}

export interface QueueStatus {
  queue: {
    waiting: number;
    delayed: number;
    active: number;
    queued: number;
  };
  settings: {
    limitPerMinute: number | null;
    workerConcurrency: number | null;
  };
  estimatedMinutes: number | null;
}

export interface QueueStatusResponse {
  success: boolean;
  queue?: QueueStatus['queue'];
  settings?: QueueStatus['settings'];
  estimatedMinutes?: number | null;
}

export interface StoreOption {
  value: string;
  label: string;
}

export const STORES: StoreOption[] = [
  { value: 'shopee', label: 'Shopee' },
  { value: 'amazon', label: 'Amazon' },
  { value: 'magalu', label: 'Magazine Luiza' },
  { value: 'ml', label: 'Mercado Livre' },
  { value: 'shein', label: 'Shein' },
  { value: 'aliexpress', label: 'AliExpress' },
  { value: 'awin', label: 'Awin' },
];

/**
 * Checks if an error message indicates a missing WhatsApp instance.
 * Handles both Portuguese (pt-BR) and English error messages.
 */
export function isInstanceMissingError(msg?: string): boolean {
  const text = String(msg || '').toLowerCase();
  // Portuguese patterns
  const ptBR = text.includes('instância') && text.includes('não encontrada');
  // English patterns
  const en =
    text.includes('instance not found') ||
    text.includes('instancename') ||
    text.includes('not connected');
  return ptBR || en;
}
