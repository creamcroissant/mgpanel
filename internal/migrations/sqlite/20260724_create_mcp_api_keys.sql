-- +goose Up
CREATE TABLE IF NOT EXISTS mcp_api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL DEFAULT '',
    prefix TEXT NOT NULL,                       -- e.g. "mcp_a1b2c3"
    key_hash TEXT NOT NULL,                     -- bcrypt hash
    enabled INTEGER NOT NULL DEFAULT 1,
    last_used_at INTEGER DEFAULT 0,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE INDEX IF NOT EXISTS idx_mcp_api_keys_prefix ON mcp_api_keys(prefix);
CREATE INDEX IF NOT EXISTS idx_mcp_api_keys_enabled ON mcp_api_keys(enabled);

-- +goose Down
DROP TABLE IF EXISTS mcp_api_keys;
