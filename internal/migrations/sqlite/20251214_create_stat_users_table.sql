-- +goose Up
CREATE TABLE IF NOT EXISTS stat_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    server_rate REAL NOT NULL DEFAULT 1,
    record_at INTEGER NOT NULL,
    record_type TEXT NOT NULL DEFAULT 'd',
    u INTEGER NOT NULL DEFAULT 0,
    d INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_stat_users_unique ON stat_users(user_id, record_type, record_at);

-- +goose Down
DROP TABLE IF EXISTS stat_users;
