/**
 * Centralized formatting utilities.
 */

/**
 * Formats a number as BRL currency.
 * @param value - The numeric value to format
 * @returns Formatted string like "R$ 1.234,56"
 */
export function formatBRL(value: number): string {
  return new Intl.NumberFormat('pt-BR', {
    style: 'currency',
    currency: 'BRL',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
    .format(Number(value || 0))
    .replace('R$', 'R$ ');
}

/**
 * Formats an ISO date string to Brazilian date format.
 * @param iso - ISO date string or null/undefined
 * @returns Formatted string like "28/01/2026" or "-" if invalid
 */
export function formatDateBR(iso?: string | null): string {
  if (!iso) return '-';
  try {
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return '-';
    return new Intl.DateTimeFormat('pt-BR', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
    }).format(date);
  } catch {
    return '-';
  }
}
