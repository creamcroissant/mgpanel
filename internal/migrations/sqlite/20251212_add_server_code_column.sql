-- +goose Up
ALTER TABLE servers ADD COLUMN code TEXT;
CREATE INDEX IF NOT EXISTS idx_servers_code ON servers(code);

-- +goose Down
DROP INDEX IF EXISTS idx_servers_code;

CREATE TABLE IF NOT EXISTS servers_tmp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL DEFAULT 0,
    route_id INTEGER NOT NULL DEFAULT 0,
    parent_id INTEGER NOT NULL DEFAULT 0,
    tags TEXT,
    name TEXT NOT NULL,
    rate TEXT NOT NULL DEFAULT '1.0',
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    server_port INTEGER NOT NULL,
    cipher TEXT,
    obfs TEXT,
    obfs_settings TEXT,
    "show" INTEGER NOT NULL DEFAULT 1,
    sort INTEGER NOT NULL DEFAULT 0,
    status INTEGER NOT NULL DEFAULT 1,
    type TEXT NOT NULL DEFAULT 'shadowsocks',
    settings TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);
INSERT INTO servers_tmp
    (id, group_id, route_id, parent_id, tags, name, rate, host, port, server_port, cipher, obfs, obfs_settings, "show", sort, status, type, settings, created_at, updated_at)
    SELECT id, group_id, route_id, parent_id, tags, name, rate, host, port, server_port, cipher, obfs, obfs_settings, "show", sort, status, type, settings, created_at, updated_at FROM servers;
DROP TABLE servers;
ALTER TABLE servers_tmp RENAME TO servers;
CREATE INDEX IF NOT EXISTS idx_servers_group_id ON servers(group_id);
CREATE INDEX IF NOT EXISTS idx_servers_parent_id ON servers(parent_id);
CREATE INDEX IF NOT EXISTS idx_servers_type ON servers(type);
CREATE INDEX IF NOT EXISTS idx_servers_show ON servers("show");
CREATE INDEX IF NOT EXISTS idx_servers_sort ON servers(sort);
