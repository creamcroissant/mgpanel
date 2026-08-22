-- +goose Up
CREATE TABLE IF NOT EXISTS agent_operation_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scope TEXT NOT NULL,
    target_id TEXT NOT NULL,
    agent_host_id INTEGER NOT NULL,
    sequence INTEGER NOT NULL DEFAULT 0,
    phase TEXT NOT NULL DEFAULT '',
    level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL DEFAULT '',
    payload_json BLOB NOT NULL DEFAULT '{}',
    source_event_id TEXT NOT NULL DEFAULT '',
    reported_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_operation_logs_scope_target_id
    ON agent_operation_logs(scope, target_id, id);
CREATE INDEX IF NOT EXISTS idx_agent_operation_logs_agent_created
    ON agent_operation_logs(agent_host_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_agent_operation_logs_scope_target_sequence
    ON agent_operation_logs(scope, target_id, sequence);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_operation_logs_source_event
    ON agent_operation_logs(scope, target_id, source_event_id)
    WHERE source_event_id <> '';

CREATE TABLE IF NOT EXISTS agent_binary_version_states (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    component TEXT NOT NULL,
    local_version TEXT NOT NULL DEFAULT '',
    remote_version TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'unknown',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    build_tags_json TEXT NOT NULL DEFAULT '[]',
    last_checked_at INTEGER NOT NULL DEFAULT 0,
    last_check_error TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE,
    UNIQUE(agent_host_id, component)
);

CREATE INDEX IF NOT EXISTS idx_agent_binary_version_states_agent_status
    ON agent_binary_version_states(agent_host_id, status);
CREATE INDEX IF NOT EXISTS idx_agent_binary_version_states_component_status
    ON agent_binary_version_states(component, status);

ALTER TABLE agent_hosts ADD COLUMN upload_rate_bps INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_hosts ADD COLUMN download_rate_bps INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_hosts ADD COLUMN raw_upload_total_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_hosts ADD COLUMN raw_download_total_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_hosts ADD COLUMN boot_id TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_hosts ADD COLUMN last_realtime_report_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_hosts ADD COLUMN last_restart_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_hosts ADD COLUMN agent_version TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_hosts ADD COLUMN current_core_type TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE agent_hosts DROP COLUMN current_core_type;
ALTER TABLE agent_hosts DROP COLUMN agent_version;
ALTER TABLE agent_hosts DROP COLUMN last_restart_at;
ALTER TABLE agent_hosts DROP COLUMN last_realtime_report_at;
ALTER TABLE agent_hosts DROP COLUMN boot_id;
ALTER TABLE agent_hosts DROP COLUMN raw_download_total_bytes;
ALTER TABLE agent_hosts DROP COLUMN raw_upload_total_bytes;
ALTER TABLE agent_hosts DROP COLUMN download_rate_bps;
ALTER TABLE agent_hosts DROP COLUMN upload_rate_bps;

DROP INDEX IF EXISTS idx_agent_binary_version_states_component_status;
DROP INDEX IF EXISTS idx_agent_binary_version_states_agent_status;
DROP TABLE IF EXISTS agent_binary_version_states;

DROP INDEX IF EXISTS idx_agent_operation_logs_source_event;
DROP INDEX IF EXISTS idx_agent_operation_logs_scope_target_sequence;
DROP INDEX IF EXISTS idx_agent_operation_logs_agent_created;
DROP INDEX IF EXISTS idx_agent_operation_logs_scope_target_id;
DROP TABLE IF EXISTS agent_operation_logs;
