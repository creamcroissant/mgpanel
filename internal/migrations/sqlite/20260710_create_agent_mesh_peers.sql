-- +goose Up
CREATE TABLE IF NOT EXISTS agent_mesh_peers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    wg_private_key TEXT NOT NULL,
    wg_public_key TEXT NOT NULL,
    wg_ip TEXT NOT NULL,
    wg_listen_port INTEGER NOT NULL DEFAULT 51820,
    network_id TEXT NOT NULL DEFAULT 'default',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_amp_agent ON agent_mesh_peers(agent_host_id);
CREATE INDEX idx_amp_network ON agent_mesh_peers(network_id);

-- +goose Down
DROP TABLE IF EXISTS agent_mesh_peers;
