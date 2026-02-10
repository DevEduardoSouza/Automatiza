-- Migration: Add counting fields to action_history
-- Paridade com sistema antigo (Supabase)

-- Adicionar campos de contagem
ALTER TABLE action_history ADD COLUMN IF NOT EXISTS target_count INTEGER DEFAULT 0;
ALTER TABLE action_history ADD COLUMN IF NOT EXISTS success_count INTEGER DEFAULT 0;
ALTER TABLE action_history ADD COLUMN IF NOT EXISTS error_count INTEGER DEFAULT 0;

-- Adicionar constraint de status (paridade com antigo)
-- Primeiro, garantir que todos os valores existentes são válidos
UPDATE action_history SET status = 'pending' WHERE status NOT IN ('pending', 'running', 'completed', 'failed');

-- Criar constraint de status
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'action_history_status_check'
    ) THEN
        ALTER TABLE action_history ADD CONSTRAINT action_history_status_check
        CHECK (status IN ('pending', 'running', 'completed', 'failed'));
    END IF;
END $$;

-- Índice para queries de status
CREATE INDEX IF NOT EXISTS idx_action_history_status ON action_history(status);

-- Comentários para documentação
COMMENT ON COLUMN action_history.target_count IS 'Número de alvos (ex: grupos) para ações em lote';
COMMENT ON COLUMN action_history.success_count IS 'Número de operações bem-sucedidas';
COMMENT ON COLUMN action_history.error_count IS 'Número de operações com falha';
