-- Quadro Kanban do painel de suporte (aba Kanban) — acompanhamento de bugs
-- estilo Trello. Uso INTERNO da equipe de suporte; não toca dados de cliente.
--
-- Cinco colunas FIXAS de fluxo (column_key), na ordem do processo de correção:
--   corrigir -> aguardando -> em_andamento -> em_revisao -> corrigido
-- A UI (5 colunas) e o servidor validam contra esse conjunto; novos estados
-- exigem migração + mudança de código de propósito (não é campo livre).
--
-- position é DOUBLE PRECISION (não inteiro) para permitir inserir um card
-- entre dois vizinhos usando o ponto médio das posições, sem renumerar a
-- coluna inteira a cada arrasto — o padrão de reordenação do Trello.
CREATE TABLE IF NOT EXISTS kanban_cards (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    column_key  TEXT NOT NULL DEFAULT 'corrigir'
        CHECK (column_key IN ('corrigir', 'aguardando', 'em_andamento', 'em_revisao', 'corrigido')),
    position    DOUBLE PRECISION NOT NULL DEFAULT 0,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    priority    TEXT NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('low', 'medium', 'high')),
    created_by  UUID,  -- profile do usuário de suporte que criou
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Consulta quente: listar uma coluna em ordem de posição.
CREATE INDEX IF NOT EXISTS idx_kanban_cards_column_position
    ON kanban_cards (column_key, position);

-- Documentos anexados a um card. Os bytes ficam no próprio Postgres (BYTEA):
-- volume interno e baixo (equipe de suporte), não justifica S3/CORS/presign.
-- O tamanho é limitado no servidor (ver support_kanban_handler.go).
CREATE TABLE IF NOT EXISTS kanban_attachments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id      UUID NOT NULL REFERENCES kanban_cards (id) ON DELETE CASCADE,
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    data         BYTEA NOT NULL,
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_kanban_attachments_card
    ON kanban_attachments (card_id);
