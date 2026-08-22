-- +goose Up
CREATE TABLE stat_users_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    server_rate REAL NOT NULL DEFAULT 1,
    record_at INTEGER NOT NULL,
    record_type INTEGER NOT NULL DEFAULT 1, -- 0: hourly, 1: daily, 2: monthly
    u INTEGER NOT NULL DEFAULT 0,
    d INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

INSERT INTO stat_users_new (id, user_id, server_rate, record_at, record_type, u, d, created_at, updated_at)
SELECT id, user_id, server_rate, record_at, CASE WHEN record_type = 'd' THEN 1 ELSE 0 END, u, d, created_at, updated_at
FROM stat_users;

DROP INDEX IF EXISTS idx_stat_users_unique;
DROP TABLE stat_users;
ALTER TABLE stat_users_new RENAME TO stat_users;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stat_users_unique ON stat_users(user_id, record_type, record_at);

-- +goose Down
CREATE TABLE stat_users_old (
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

INSERT INTO stat_users_old (id, user_id, server_rate, record_at, record_type, u, d, created_at, updated_at)
SELECT id, user_id, server_rate, record_at, CASE WHEN record_type = 1 THEN 'd' ELSE 'h' END, u, d, created_at, updated_at
FROM stat_users;

DROP INDEX IF EXISTS idx_stat_users_unique;
DROP TABLE stat_users;
ALTER TABLE stat_users_old RENAME TO stat_users;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stat_users_unique ON stat_users(user_id, record_type, record_at);
