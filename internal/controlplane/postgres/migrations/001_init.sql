-- 001_init.sql — initial control-plane schema for the Postgres backend.
--
-- Tables mirror the four maps on the file-backed DevFileStore's State
-- struct (internal/controlplane/store.go State). JSONB for the slice /
-- map fields keeps the SQL shape stable across back-compat record
-- additions.
--
-- Indexes match the hot paths: sessions expires_at for the session
-- reaper (transport_streamable_http.reapLoop), audit_events.at for the
-- retention reaper (B2 RetainAudit).

CREATE TABLE IF NOT EXISTS tenants (
    id                TEXT PRIMARY KEY,
    credential_ref_id TEXT NOT NULL,
    workspace_id      TEXT NOT NULL DEFAULT '',
    base_url          TEXT NOT NULL DEFAULT '',
    timezone          TEXT NOT NULL DEFAULT '',
    policy_mode       TEXT NOT NULL DEFAULT '',
    deny_tools        JSONB NOT NULL DEFAULT '[]'::jsonb,
    deny_groups       JSONB NOT NULL DEFAULT '[]'::jsonb,
    allow_groups      JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata          JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS credential_refs (
    id           TEXT PRIMARY KEY,
    backend      TEXT NOT NULL,
    reference    TEXT NOT NULL,
    workspace_id TEXT NOT NULL DEFAULT '',
    base_url     TEXT NOT NULL DEFAULT '',
    metadata     JSONB NOT NULL DEFAULT '{}'::jsonb,
    modified_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS sessions (
    id                  TEXT PRIMARY KEY,
    tenant_id           TEXT NOT NULL,
    subject             TEXT NOT NULL,
    transport           TEXT NOT NULL,
    protocol_version    TEXT NOT NULL DEFAULT '',
    client_name         TEXT NOT NULL DEFAULT '',
    client_version      TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL,
    expires_at          TIMESTAMPTZ NOT NULL,
    last_seen_at        TIMESTAMPTZ NOT NULL,
    workspace_id        TEXT NOT NULL DEFAULT '',
    clockify_base_url   TEXT NOT NULL DEFAULT '',
    session_affinity_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at
    ON sessions (expires_at);

CREATE TABLE IF NOT EXISTS audit_events (
    id           BIGSERIAL PRIMARY KEY,
    external_id  TEXT NOT NULL UNIQUE,
    at           TIMESTAMPTZ NOT NULL,
    tenant_id    TEXT NOT NULL DEFAULT '',
    subject      TEXT NOT NULL DEFAULT '',
    session_id   TEXT NOT NULL DEFAULT '',
    transport    TEXT NOT NULL DEFAULT '',
    tool         TEXT NOT NULL DEFAULT '',
    action       TEXT NOT NULL DEFAULT '',
    outcome      TEXT NOT NULL DEFAULT '',
    reason       TEXT NOT NULL DEFAULT '',
    resource_ids JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata     JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_audit_events_at
    ON audit_events (at);
