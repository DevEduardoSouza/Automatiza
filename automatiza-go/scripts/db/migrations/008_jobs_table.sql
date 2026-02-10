-- Migration: 008_jobs_table.sql
-- Descrição: Tabela de jobs agendados com payload flexível

-- Função update_updated_at (se não existir)
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    action_type VARCHAR(50) NOT NULL,
    scheduled_for TIMESTAMPTZ NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT chk_jobs_action_type CHECK (action_type IN (
        'send_message', 'send_poll', 'send_ml_offer',
        'update_description', 'close_groups', 'open_groups',
        'change_group_name', 'change_group_photo'
    )),
    CONSTRAINT chk_jobs_status CHECK (status IN (
        'pending', 'running', 'done', 'failed', 'cancelled'
    ))
);

-- Índices baseados em queries reais
CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status_scheduled ON jobs(status, scheduled_for)
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_jobs_user_status ON jobs(user_id, status);

-- Trigger para updated_at (safe: drop first)
DROP TRIGGER IF EXISTS update_jobs_updated_at ON jobs;
CREATE TRIGGER update_jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE jobs IS 'Jobs agendados para execução em grupos WhatsApp';
COMMENT ON COLUMN jobs.action_type IS 'Tipo de ação: send_message, send_poll, etc';
COMMENT ON COLUMN jobs.payload IS 'Dados específicos em JSONB';
COMMENT ON COLUMN jobs.status IS 'pending → running → done|failed';
