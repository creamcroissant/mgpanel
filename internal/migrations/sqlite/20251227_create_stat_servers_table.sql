-- +goose Up
CREATE TABLE IF NOT EXISTS stat_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    record_at INTEGER NOT NULL,
    record_type INTEGER NOT NULL DEFAULT 1,
    upload INTEGER NOT NULL DEFAULT 0,
    download INTEGER NOT NULL DEFAULT 0,
    cpu_avg REAL NOT NULL DEFAULT 0,
    mem_used INTEGER NOT NULL DEFAULT 0,
    mem_total INTEGER NOT NULL DEFAULT 0,
    disk_used INTEGER NOT NULL DEFAULT 0,
    disk_total INTEGER NOT NULL DEFAULT 0,
    online_users INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_stat_servers_unique ON stat_servers(server_id, record_type, record_at);
CREATE INDEX IF NOT EXISTS idx_stat_servers_server_time ON stat_servers(server_id, record_at);

-- +goose Down
DROP INDEX IF EXISTS idx_stat_servers_server_time;
DROP INDEX IF EXISTS idx_stat_servers_unique;
DROP TABLE IF EXISTS stat_servers;
