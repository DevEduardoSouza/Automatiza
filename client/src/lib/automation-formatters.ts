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
