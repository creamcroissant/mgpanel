-- +goose Up
CREATE TABLE IF NOT EXISTS plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER,
    name TEXT NOT NULL,
    prices TEXT,
    sell INTEGER NOT NULL DEFAULT 0,
    transfer_enable INTEGER NOT NULL DEFAULT 0,
    speed_limit INTEGER,
    device_limit INTEGER,
    show INTEGER NOT NULL DEFAULT 0,
    renew INTEGER NOT NULL DEFAULT 1,
    content TEXT,
    tags TEXT,
    reset_traffic_method INTEGER,
    capacity_limit INTEGER,
    sort INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE INDEX IF NOT EXISTS idx_plans_show_sell ON plans(show, sell);
CREATE INDEX IF NOT EXISTS idx_plans_sort ON plans(sort);

-- +goose Down
DROP TABLE IF EXISTS plans;
