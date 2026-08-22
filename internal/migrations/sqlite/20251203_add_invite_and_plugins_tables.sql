-- +goose Up
CREATE TABLE IF NOT EXISTS v2_invite_code (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    code TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 0,
    pv INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_v2_invite_code_code ON v2_invite_code(code);
CREATE INDEX IF NOT EXISTS idx_v2_invite_code_user_id ON v2_invite_code(user_id);

CREATE TABLE IF NOT EXISTS v2_plugins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'feature',
    version TEXT NOT NULL,
    is_enabled INTEGER NOT NULL DEFAULT 0,
    config TEXT,
    installed_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_v2_plugins_code ON v2_plugins(code);
CREATE INDEX IF NOT EXISTS idx_v2_plugins_type_enabled ON v2_plugins(type, is_enabled);

-- +goose Down
DROP TABLE IF EXISTS v2_plugins;
DROP TABLE IF EXISTS v2_invite_code;
