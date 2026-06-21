-- Migration: Add offer_generate_send_as_image toggle to automations
-- OFFER_GENERATE mode: choose between image attachment + caption (default,
-- comportamento atual) or native WhatsApp link preview.
--
-- Default TRUE para preservar o comportamento existente: o offer_generate
-- sempre enviou a oferta como imagem quando havia imagem. Linhas existentes
-- recebem TRUE automaticamente via ADD COLUMN ... DEFAULT TRUE.

ALTER TABLE automations ADD COLUMN IF NOT EXISTS offer_generate_send_as_image BOOLEAN DEFAULT TRUE;

COMMENT ON COLUMN automations.offer_generate_send_as_image IS 'OFFER_GENERATE: enviar oferta como imagem baixada + legenda (true, default) em vez de preview nativo (false)';
