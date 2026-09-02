-- +goose Up
-- 套餐体系整体移除：个人面板直接由管理员对用户进行流量/分组管理。
DROP INDEX IF EXISTS idx_plan_server_groups_plan_id;
DROP INDEX IF EXISTS idx_plan_server_groups_group_id;
DROP TABLE IF EXISTS plan_server_groups;

DROP INDEX IF EXISTS idx_plans_group_id;
DROP INDEX IF EXISTS idx_plans_show_sell;
DROP INDEX IF EXISTS idx_plans_sort;
DROP TABLE IF EXISTS plans;

-- SQLite 3.35+ 支持直接 DROP COLUMN
ALTER TABLE users DROP COLUMN plan_id;

-- +goose Down
-- 套餐数据不可恢复，回滚仅保证结构可重建（供降级使用）。
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
    renew INTEGER NOT NULL DEFAULT 0,
    content TEXT,
    tags TEXT,
    reset_traffic_method INTEGER,
    capacity_limit INTEGER,
    sort INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_plans_group_id ON plans(group_id);

CREATE TABLE IF NOT EXISTS plan_server_groups (
    plan_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    PRIMARY KEY (plan_id, group_id)
);
CREATE INDEX IF NOT EXISTS idx_plan_server_groups_plan_id ON plan_server_groups(plan_id);
CREATE INDEX IF NOT EXISTS idx_plan_server_groups_group_id ON plan_server_groups(group_id);

ALTER TABLE users ADD COLUMN plan_id INTEGER NOT NULL DEFAULT 0;
