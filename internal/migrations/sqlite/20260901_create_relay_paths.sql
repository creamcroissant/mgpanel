-- +goose Up
-- +goose StatementBegin

-- 服务器中继链路：受控服务器之间的多跳流量走向（如 s1→s2→s3），与入口协议配置正交。
CREATE TABLE IF NOT EXISTS relay_paths (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    core_type TEXT NOT NULL DEFAULT 'sing-box',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- 中继链路节点：sequence 0 = 入口，N-1 = 出口；每跳通过 mesh socks 隧道转发到下一跳。
CREATE TABLE IF NOT EXISTS relay_path_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path_id INTEGER NOT NULL REFERENCES relay_paths(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL,
    agent_host_id INTEGER NOT NULL REFERENCES agent_hosts(id) ON DELETE CASCADE,
    UNIQUE(path_id, sequence)
);
CREATE INDEX IF NOT EXISTS idx_relay_path_nodes_path ON relay_path_nodes(path_id);
CREATE INDEX IF NOT EXISTS idx_relay_path_nodes_agent ON relay_path_nodes(agent_host_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS relay_path_nodes;
DROP TABLE IF EXISTS relay_paths;
-- +goose StatementEnd
