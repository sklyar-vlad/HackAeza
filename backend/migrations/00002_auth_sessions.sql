-- +goose Up
ALTER TABLE app_users ADD COLUMN session_version bigint NOT NULL DEFAULT 1 CHECK (session_version > 0);
ALTER TABLE auth_sessions ADD COLUMN session_data bytea;

-- +goose Down
ALTER TABLE auth_sessions DROP COLUMN session_data;
ALTER TABLE app_users DROP COLUMN session_version;
