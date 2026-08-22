-- +goose Up
CREATE TABLE IF NOT EXISTS tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token TEXT NOT NULL,
    refresh_token TEXT NOT NULL UNIQUE,
    expires_at INTEGER NOT NULL,
    refresh_expires_at INTEGER NOT NULL,
    ip TEXT,
    user_agent TEXT,
    revoked INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE INDEX IF NOT EXISTS idx_tokens_user_id ON tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at);

-- +goose Down
DROP TABLE IF EXISTS tokens;
