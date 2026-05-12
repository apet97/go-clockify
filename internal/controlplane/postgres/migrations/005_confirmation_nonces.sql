-- 005_confirmation_nonces.sql — single-use confirmation-token nonce store.

CREATE TABLE IF NOT EXISTS confirmation_nonces (
    replay_key TEXT PRIMARY KEY,
    nonce      TEXT NOT NULL,
    tool       TEXT NOT NULL,
    args_hash  TEXT NOT NULL DEFAULT '',
    tenant_id  TEXT NOT NULL DEFAULT '',
    subject    TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_confirmation_nonces_expires_at
    ON confirmation_nonces (expires_at);
