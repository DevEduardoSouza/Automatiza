-- Endurecimento do canal de Instagram Stories (Fase 0). Aditiva: nenhuma coluna
-- existente muda de tipo/semântica e nenhuma linha é reescrita — todo ADD COLUMN
-- tem DEFAULT, então a 073 continua válida para dados já gravados.
--
-- Por que cada peça existe:
--
-- app_scoped_user_id: a troca do authorization code devolve o id APP-SCOPED da
-- conta, que NÃO é o id profissional usado para publicar (esse vem do /me).
-- Guardar os dois separados é o que impede publicar no objeto errado; é também
-- a chave pela qual os callbacks de desautorização/exclusão da Meta encontram a
-- conta (o signed_request só carrega o app-scoped id).
--
-- Os dois índices únicos parciais impedem que duas contas locais reivindiquem a
-- mesma conta do Instagram — sem eles, dois usuários publicariam no mesmo perfil
-- e a exclusão de dados apagaria a credencial errada. Parciais em <> '' porque
-- linhas antigas da 073 nascem com string vazia e não devem colidir entre si.
ALTER TABLE instagram_credentials
    ADD COLUMN IF NOT EXISTS app_scoped_user_id TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_instagram_credentials_app_scoped_user_id
    ON instagram_credentials (app_scoped_user_id)
    WHERE app_scoped_user_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_instagram_credentials_ig_user_id
    ON instagram_credentials (ig_user_id)
    WHERE ig_user_id <> '';

-- container_id: o container de mídia da Meta precisa sobreviver ao processo. Se
-- o job estourar o timeout DEPOIS de a Meta aceitar o /media_publish, o retry
-- sem esse id criaria um segundo container e publicaria o Story duas vezes.
-- processing_started_at é o lease (não um estado público): quem reivindica a
-- linha carimba aqui, e um lease velho (>2min) pode ser tomado por outro worker.
-- error_code é o código seguro enumerado — o texto livre de `error` nunca recebe
-- err.Error(), que vazaria URL/ids/detalhe do provedor.
ALTER TABLE instagram_story_posts
    ADD COLUMN IF NOT EXISTS container_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS processing_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS error_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS published_reconciled BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE instagram_story_posts
    ADD CONSTRAINT instagram_story_posts_attempt_count_nonnegative
    CHECK (attempt_count >= 0);

-- Idempotência DURÁVEL do evento de origem. Antes o dedupe vivia só no Redis com
-- TTL de 1h e fail-open: redelivery tardio, corrida entre chips ou Redis fora do
-- ar publicavam o mesmo Story mais de uma vez. Parcial em message_id <> '' para
-- não travar linhas antigas sem id de mensagem.
CREATE UNIQUE INDEX IF NOT EXISTS uq_instagram_story_posts_source_event
    ON instagram_story_posts (user_id, source_group, message_id)
    WHERE message_id <> '';

CREATE INDEX IF NOT EXISTS idx_instagram_story_posts_retention
    ON instagram_story_posts (created_at, id);

-- Comprovante de exclusão exigido pelo callback de Data Deletion da Meta: ela
-- espera uma URL de status e um código de confirmação consultável. subject_hash
-- é HMAC do app-scoped id (pseudônimo) — o id bruto não é armazenado. O código
-- de confirmação é aleatório, NÃO derivado do hash, para não ser adivinhável a
-- partir do sujeito.
CREATE TABLE IF NOT EXISTS instagram_data_deletion_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_hash CHAR(64) NOT NULL UNIQUE,
    confirmation_code CHAR(64) NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'completed'
        CHECK (status IN ('completed')),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_instagram_data_deletion_requests_expiry
    ON instagram_data_deletion_requests (expires_at, id);
