-- Migration: Add search_send_as_image and monitoring_send_as_image toggles to automations
-- Estende o toggle "Enviar como imagem" para os modos SEARCH e MONITORING
-- (antes existia só para LIST e OFFER_GENERATE).
--
-- SEARCH: DEFAULT TRUE — preserva o comportamento atual (a Busca sempre
--   enviou a oferta como imagem baixada + legenda).
-- MONITORING: DEFAULT FALSE — preserva o comportamento atual (o monitoramento
--   usa o preview nativo do WhatsApp primeiro; mais leve).

ALTER TABLE automations ADD COLUMN IF NOT EXISTS search_send_as_image BOOLEAN DEFAULT TRUE;
ALTER TABLE automations ADD COLUMN IF NOT EXISTS monitoring_send_as_image BOOLEAN DEFAULT FALSE;

COMMENT ON COLUMN automations.search_send_as_image IS 'SEARCH: enviar oferta como imagem baixada + legenda (true, default) em vez de preview nativo (false)';
COMMENT ON COLUMN automations.monitoring_send_as_image IS 'MONITORING: enviar oferta como imagem baixada + legenda (true) em vez de preview nativo (false, default)';
