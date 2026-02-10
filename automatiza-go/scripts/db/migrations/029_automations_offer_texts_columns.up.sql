-- Migration: Add offer texts and list mode columns to automations
-- Required by offer-automations feature (texts, ctas) and list mode (ml_link_list_id, etc.)

-- Offer automation texts (multiple keywords / headlines)
ALTER TABLE automations ADD COLUMN IF NOT EXISTS texts TEXT[] DEFAULT '{}';
ALTER TABLE automations ADD COLUMN IF NOT EXISTS ctas TEXT[] DEFAULT '{}';

-- Error tracking
ALTER TABLE automations ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE automations ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ;

-- LIST mode fields
ALTER TABLE automations ADD COLUMN IF NOT EXISTS ml_link_list_id UUID;
ALTER TABLE automations ADD COLUMN IF NOT EXISTS list_items_per_run INTEGER;
ALTER TABLE automations ADD COLUMN IF NOT EXISTS list_include_image BOOLEAN DEFAULT FALSE;

-- Priority type (discount | price | commission)
ALTER TABLE automations ADD COLUMN IF NOT EXISTS priority_type TEXT;

-- Comments
COMMENT ON COLUMN automations.texts IS 'Array de textos/keywords para busca de ofertas';
COMMENT ON COLUMN automations.ctas IS 'Array de call-to-action texts para mensagens';
COMMENT ON COLUMN automations.last_error IS 'Último erro de execução da automação';
COMMENT ON COLUMN automations.last_error_at IS 'Timestamp do último erro';
COMMENT ON COLUMN automations.ml_link_list_id IS 'ID da lista de links ML para modo LIST';
COMMENT ON COLUMN automations.list_items_per_run IS 'Quantidade de itens por execução no modo LIST';
COMMENT ON COLUMN automations.list_include_image IS 'Incluir imagem no modo LIST';
COMMENT ON COLUMN automations.priority_type IS 'Tipo de prioridade: discount, price ou commission';
