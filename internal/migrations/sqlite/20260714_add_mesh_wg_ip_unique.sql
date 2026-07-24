-- +goose Up
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_mesh_peers_wg_ip ON agent_mesh_peers(wg_ip);

-- +goose Down
DROP INDEX IF EXISTS idx_agent_mesh_peers_wg_ip;
