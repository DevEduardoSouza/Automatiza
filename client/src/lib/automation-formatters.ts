import type { Automation } from '@/types/automation';
import type { TFunction } from 'i18next';

/**
 * Returns the i18n key suffix for automation mode.
 * Used with `t("pages.offerAutomations.tabs.X")` for display labels.
 */
export function getModeKey(mode: string): 'search' | 'list' | 'monitor' {
  switch (mode) {
    case 'search':
      return 'search';
    case 'list':
      return 'list';
    default:
      return 'monitor';
  }
}

/**
 * Returns the Badge variant for automation mode.
 */
export function getModeVariant(mode: string): 'default' | 'outline' | 'secondary' {
  switch (mode) {
    case 'search':
      return 'default';
    case 'list':
      return 'outline';
    default:
      return 'secondary';
  }
}

/**
 * Formats the delay/interval display using i18n for desktop table view.
 */
export function formatDelayWithI18n(auto: Automation, t: TFunction): string {
  if (auto.mode === 'monitor') {
    const s = Number(auto.delay_seconds || 10);
    if (s >= 60 && s % 60 === 0) {
      return t('pages.offerAutomations.table.delayMinutes', { value: s / 60 });
    }
    return t('pages.offerAutomations.table.delaySeconds', { value: s });
  }
  return t('pages.offerAutomations.table.intervalMinutes', {
    value: auto.interval_minutes,
  });
}

/**
 * Formats the delay/interval display for mobile card view (compact format).
 */
export function formatDelayCompact(auto: Automation): string {
  if (auto.mode === 'monitor') {
    const s = Number(auto.delay_seconds || 10);
    return s >= 60 && s % 60 === 0 ? `${s / 60}m` : `${s}s`;
  }
  return `${auto.interval_minutes} min`;
}

/**
 * Resumo da fila da cadência do espelho (Melhoria 7): "N na fila · próxima em
 * <data/hora local>". String vazia quando não há fila — que é o caso de todo
 * mundo enquanto a feature está desligada, então nada muda na listagem.
 *
 * O horário chega em UTC (RFC3339) e é formatado no fuso do navegador: quem lê
 * quer saber quando VAI SAIR no relógio dele.
 */
export function formatMonitorQueue(auto: Automation, t: TFunction): string {
  const pending = Number(auto.monitor_pending_count ?? 0);
  if (!pending) {
    return '';
  }
  const when = auto.monitor_next_scheduled_at
    ? new Date(auto.monitor_next_scheduled_at).toLocaleString()
    : '—';
  return t('pages.offerAutomations.form.monitorQueue', { count: pending, when });
}
