-- +goose Up
CREATE TABLE IF NOT EXISTS core_operations (
    id TEXT PRIMARY KEY,
    agent_host_id INTEGER NOT NULL,
    operation_type TEXT NOT NULL,
    core_type TEXT NOT NULL,
    status TEXT NOT NULL,
    request_payload BLOB NOT NULL DEFAULT '{}',
    result_payload BLOB NOT NULL DEFAULT '{}',
    error_message TEXT NOT NULL DEFAULT '',
    operator_id INTEGER,
    claimed_by TEXT NOT NULL DEFAULT '',
    claimed_at INTEGER,
    started_at INTEGER,
    finished_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_core_operations_agent_status_created
    ON core_operations(agent_host_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_core_operations_agent_type_status_created
    ON core_operations(agent_host_id, core_type, operation_type, status, created_at);
CREATE INDEX IF NOT EXISTS idx_core_operations_claimed_by_status
    ON core_operations(claimed_by, status);

ALTER TABLE agent_core_instances ADD COLUMN core_snapshot TEXT NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE agent_core_instances DROP COLUMN core_snapshot;
DROP INDEX IF EXISTS idx_core_operations_claimed_by_status;
DROP INDEX IF EXISTS idx_core_operations_agent_type_status_created;
DROP INDEX IF EXISTS idx_core_operations_agent_status_created;
DROP TABLE IF EXISTS core_operations;
