-- +goose Up
DROP TABLE IF EXISTS servers;
DROP TABLE IF EXISTS server_groups;
DROP TABLE IF EXISTS server_routes;

CREATE TABLE IF NOT EXISTS server_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'shadowsocks',
    sort INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE TABLE IF NOT EXISTS server_routes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    remarks TEXT,
    match TEXT,
    action TEXT,
    action_value TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE TABLE IF NOT EXISTS servers (
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

CREATE INDEX IF NOT EXISTS idx_servers_group_id ON servers(group_id);
CREATE INDEX IF NOT EXISTS idx_servers_parent_id ON servers(parent_id);
CREATE INDEX IF NOT EXISTS idx_servers_type ON servers(type);
CREATE INDEX IF NOT EXISTS idx_servers_show ON servers("show");
CREATE INDEX IF NOT EXISTS idx_servers_sort ON servers(sort);

-- +goose Down
DROP TABLE IF EXISTS servers;
DROP TABLE IF EXISTS server_routes;
DROP TABLE IF EXISTS server_groups;

CREATE TABLE IF NOT EXISTS server_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sort INTEGER DEFAULT 0,
    policy TEXT DEFAULT 'manual',
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE TABLE IF NOT EXISTS servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    protocol TEXT NOT NULL,
    config TEXT,
    sort INTEGER DEFAULT 0,
    status INTEGER DEFAULT 1,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE INDEX IF NOT EXISTS idx_servers_group_id ON servers(group_id);
