-- Initial staff access schema, aligned with the proposed PostgreSQL diagram.
-- +goose Up
CREATE TABLE IF NOT EXISTS app_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    disabled_at TIMESTAMPTZ,
    oidc_issuer TEXT,
    oidc_subject TEXT,
    session_version BIGINT NOT NULL DEFAULT 1 CHECK (session_version > 0),
    UNIQUE (oidc_issuer, oidc_subject)
);

CREATE TABLE IF NOT EXISTS auth_sessions (
    token_hash BYTEA PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    auth_provider TEXT NOT NULL CHECK (auth_provider IN ('password', 'oidc')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    -- SCS owns serialization, token generation and expiry. No raw token is stored.
    session_data BYTEA NOT NULL
);
CREATE INDEX IF NOT EXISTS auth_sessions_expiry_idx ON auth_sessions (expires_at);
CREATE INDEX IF NOT EXISTS auth_sessions_user_idx ON auth_sessions (user_id);

-- Adopt databases initialized by the previous auth implementation without replacing user data.
DROP TABLE IF EXISTS auth_schema_migrations;

-- +goose Down
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS app_users;
