-- +goose Up
-- +goose StatementBegin

-- 出口节点集合：一组可用的出口 agent，按标签/地区/解锁能力分组
CREATE TABLE IF NOT EXISTS exit_node_sets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '',               -- 逗号分隔，如 "region:jp,unlock:netflix,unlock:disney"
    strategy TEXT NOT NULL DEFAULT 'round_robin', -- round_robin / least_ping / random / weighted_random
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- 出口节点集合成员
CREATE TABLE IF NOT EXISTS exit_node_set_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id INTEGER NOT NULL REFERENCES exit_node_sets(id) ON DELETE CASCADE,
    agent_host_id INTEGER NOT NULL REFERENCES agent_hosts(id) ON DELETE CASCADE,
    weight INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(set_id, agent_host_id)
);
CREATE INDEX IF NOT EXISTS idx_exit_node_set_members_set ON exit_node_set_members(set_id);

-- 路由策略：按 geosite/domain 匹配流量，路由到出口集合
CREATE TABLE IF NOT EXISTS routing_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    core_type TEXT NOT NULL DEFAULT 'sing-box',
    priority INTEGER NOT NULL DEFAULT 0,
    match_type TEXT NOT NULL DEFAULT 'geosite',   -- geosite / domain / ip_cidr
    match_value TEXT NOT NULL DEFAULT '',          -- 如 "netflix" 或 "*.example.com"
    action TEXT NOT NULL DEFAULT 'route_to_set',   -- route_to_set
    target_set_id INTEGER REFERENCES exit_node_sets(id) ON DELETE SET NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_routing_policies_core ON routing_policies(core_type);

-- inbound_specs 增加出口节点集合引用
ALTER TABLE inbound_specs ADD COLUMN exit_node_set_id INTEGER REFERENCES exit_node_sets(id) ON DELETE SET NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE inbound_specs DROP COLUMN exit_node_set_id;
DROP TABLE IF EXISTS routing_policies;
DROP TABLE IF EXISTS exit_node_set_members;
DROP TABLE IF EXISTS exit_node_sets;
-- +goose StatementEnd
