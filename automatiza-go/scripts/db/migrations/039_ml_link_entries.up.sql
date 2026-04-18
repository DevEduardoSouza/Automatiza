CREATE TABLE IF NOT EXISTS ml_link_entries (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id     TEXT NOT NULL,
    original_url TEXT NOT NULL,
    short_url   TEXT NOT NULL DEFAULT '',
    product_id  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, original_url)
);

CREATE INDEX IF NOT EXISTS idx_ml_link_entries_user_id ON ml_link_entries(user_id);
CREATE INDEX IF NOT EXISTS idx_ml_link_entries_created_at ON ml_link_entries(user_id, created_at DESC);
