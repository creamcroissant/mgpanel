-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS unlock_probe_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    service TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown',
    region TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT '',
    probed_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE(agent_host_id, service),
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS unlock_probe_results;
-- +goose StatementEnd
