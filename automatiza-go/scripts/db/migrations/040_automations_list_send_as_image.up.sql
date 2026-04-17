-- Migration: Add list_send_as_image toggle to automations
-- LIST mode: choose between native WhatsApp link preview (default) or image attachment + caption.

ALTER TABLE automations ADD COLUMN IF NOT EXISTS list_send_as_image BOOLEAN DEFAULT FALSE;

COMMENT ON COLUMN automations.list_send_as_image IS 'LIST mode: enviar link como imagem baixada (true) em vez de preview nativo (false)';
