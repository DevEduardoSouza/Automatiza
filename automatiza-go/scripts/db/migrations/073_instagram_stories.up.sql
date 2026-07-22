-- Instagram Stories via tag {story} no grupo monitor (20/07).
-- Aditiva: duas tabelas NOVAS, nenhum ALTER destrutivo. A feature nasce
-- desligada (sem INSTAGRAM_APP_ID no ambiente o backend nem escreve aqui).
--
-- instagram_credentials: 1 linha por usuário (clone de ml_credentials). O token
-- de longa duração (~60d) vive cifrado em encrypted_data (EncryptedBlobV1, AES);
-- ig_user_id/username/token_expires_at ficam em claro para o cron de refresh
-- listar expiráveis sem decriptar.
CREATE TABLE IF NOT EXISTS instagram_credentials (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL UNIQUE,
    encrypted_data   TEXT NOT NULL,
    ig_user_id       TEXT NOT NULL DEFAULT '',
    username         TEXT NOT NULL DEFAULT '',
    token_expires_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_instagram_credentials_expiry
    ON instagram_credentials (token_expires_at);

-- instagram_story_posts: ledger dos stories publicados (alimenta a página do
-- usuário e as provas M3). media_source distingue imagem espelhada de imagem
-- gerada a partir do texto.
CREATE TABLE IF NOT EXISTS instagram_story_posts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    source_group TEXT NOT NULL DEFAULT '',
    message_id   TEXT NOT NULL DEFAULT '',
    media_source TEXT NOT NULL CHECK (media_source IN ('image','rendered_text')),
    status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','published','failed')),
    ig_media_id  TEXT NOT NULL DEFAULT '',
    error        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_instagram_story_posts_user
    ON instagram_story_posts (user_id, created_at DESC);
