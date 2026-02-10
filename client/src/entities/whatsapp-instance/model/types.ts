/**
 * WAHA API status response structure
 * Handles various response formats from the WAHA API
 */
export interface WahaStatusResponse {
  success?: boolean;
  status?: string;
  instance?: { state?: string };
  rawData?: { instance?: { state?: string }; state?: string };
  state?: string;
  qrcode?: string | { base64?: string };
  qrcodeBase64?: string;
}

/**
 * WAHA API create instance response
 */
export interface WahaCreateResponse {
  success: boolean;
  instance?: unknown;
  instanceName?: string;
  friendlyName?: string;
  error?: string;
  qrcode?: string | { base64?: string };
  qrcodeBase64?: string;
}

/**
 * Single WhatsApp group from WAHA API
 */
export interface WahaGroup {
  id: string;
  subject?: string;
  desc?: string;
  size?: number;
  restrict?: boolean;
  announce?: boolean;
}

/**
 * WAHA API fetch groups response
 */
export interface WahaGroupsResponse {
  success: boolean;
  groups?: WahaGroup[];
}

/**
 * Database instance row from PostgreSQL
 */
export interface InstanceRow {
  id: string;
  instance_id: string;
  friendly_name?: string;
  status: string;
}

/**
 * Possible instance connection states
 */
export type InstanceStatus = 'pending' | 'connected' | 'disconnected';
