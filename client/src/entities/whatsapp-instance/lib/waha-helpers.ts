import type { WahaCreateResponse, WahaStatusResponse } from '../model/types';

/**
 * Placeholder values that indicate an instance name is not yet set.
 * These strings are used in cleanup queries and validation.
 */
export const INSTANCE_PLACEHOLDER_VALUES = [
  'Será gerado automaticamente',
  'Gerado automaticamente',
] as const;

/**
 * Checks if the instance name is a real user-assigned name.
 * Returns false for empty strings, null, undefined, or placeholder values.
 */
export function hasRealInstanceName(name: string | null | undefined): boolean {
  const trimmed = (name ?? '').trim();
  if (!trimmed) return false;

  const lowerName = trimmed.toLowerCase();
  return !INSTANCE_PLACEHOLDER_VALUES.some(
    (placeholder) => placeholder.toLowerCase() === lowerName
  );
}

/**
 * Extracts the WAHA state from various response formats.
 * The WAHA API can return state in multiple nested locations.
 */
export function extractWahaState(data: WahaStatusResponse | null | undefined): string | null {
  if (!data) return null;

  const value =
    data.status ||
    data.instance?.state ||
    data.rawData?.instance?.state ||
    data.rawData?.state ||
    data.state;

  return value ? String(value) : null;
}

/**
 * Checks if the state indicates a connected WhatsApp instance.
 */
export function isConnectedState(state: string | null | undefined): boolean {
  const s = String(state ?? '').toLowerCase();
  return s === 'open' || s === 'connected';
}

/**
 * Checks if the state indicates the instance is connecting.
 */
export function isConnectingState(state: string | null | undefined): boolean {
  const s = String(state ?? '').toLowerCase();
  return s.includes('connecting');
}

/**
 * Checks if the state indicates a disconnected instance.
 */
export function isDisconnectedState(state: string | null | undefined): boolean {
  const s = String(state ?? '').toLowerCase();
  return s === 'close' || s === 'disconnected';
}

/**
 * Extracts QR code from various response formats.
 * The WAHA API can return QR codes in multiple formats.
 */
export function extractQrCode(
  data: WahaStatusResponse | WahaCreateResponse | null | undefined
): string | null {
  if (!data) return null;

  if (typeof data.qrcode === 'string' && data.qrcode) {
    return data.qrcode;
  }
  if (typeof data.qrcodeBase64 === 'string' && data.qrcodeBase64) {
    return data.qrcodeBase64;
  }
  if (typeof data.qrcode === 'object' && data.qrcode?.base64) {
    return data.qrcode.base64;
  }

  return null;
}
