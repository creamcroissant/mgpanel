-- +goose Up
CREATE TABLE IF NOT EXISTS agent_lifecycle_operations (
    id TEXT PRIMARY KEY,
    agent_host_id INTEGER NOT NULL,
    operation_type TEXT NOT NULL,
    status TEXT NOT NULL,
    request_payload BLOB NOT NULL DEFAULT '{}',
    result_payload BLOB NOT NULL DEFAULT '{}',
    error_message TEXT NOT NULL DEFAULT '',
    claimed_by TEXT NOT NULL DEFAULT '',
    claimed_at INTEGER,
    started_at INTEGER,
    finished_at INTEGER,
    operator_id INTEGER,
    source TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_lifecycle_operations_agent_status_created
    ON agent_lifecycle_operations(agent_host_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_lifecycle_operations_agent_type_status_created
    ON agent_lifecycle_operations(agent_host_id, operation_type, status, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_lifecycle_operations_claimed_by_status
    ON agent_lifecycle_operations(claimed_by, status);
CREATE INDEX IF NOT EXISTS idx_agent_lifecycle_operations_source_created
    ON agent_lifecycle_operations(source, created_at);

CREATE TABLE IF NOT EXISTS agent_traffic_policies (
    agent_host_id INTEGER PRIMARY KEY,
    enabled INTEGER NOT NULL DEFAULT 0,
    limit_bytes INTEGER NOT NULL DEFAULT 0,
    limit_type TEXT NOT NULL DEFAULT 'sum',
    threshold_percent INTEGER NOT NULL DEFAULT 100,
    threshold_action TEXT NOT NULL DEFAULT 'notify_only',
    threshold_reached INTEGER NOT NULL DEFAULT 0,
    reset_mode TEXT NOT NULL DEFAULT 'off',
    reset_day INTEGER NOT NULL DEFAULT 1,
    interval_days INTEGER NOT NULL DEFAULT 0,
    anchor_at INTEGER NOT NULL DEFAULT 0,
    last_reset_at INTEGER NOT NULL DEFAULT 0,
    last_reset_cycle_key TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_traffic_policies_enabled_threshold
    ON agent_traffic_policies(enabled, threshold_reached);
CREATE INDEX IF NOT EXISTS idx_agent_traffic_policies_reset_mode
    ON agent_traffic_policies(reset_mode, last_reset_at);
CREATE INDEX IF NOT EXISTS idx_agent_traffic_policies_threshold_action
    ON agent_traffic_policies(threshold_action);

CREATE TABLE IF NOT EXISTS agent_traffic_states (
    agent_host_id INTEGER PRIMARY KEY,
    boot_id TEXT NOT NULL DEFAULT '',
    last_raw_upload_bytes INTEGER NOT NULL DEFAULT 0,
    last_raw_download_bytes INTEGER NOT NULL DEFAULT 0,
    counter_seen INTEGER NOT NULL DEFAULT 0,
    cycle_upload_bytes INTEGER NOT NULL DEFAULT 0,
    cycle_download_bytes INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_traffic_states_boot_id
    ON agent_traffic_states(boot_id);
CREATE INDEX IF NOT EXISTS idx_agent_traffic_states_updated_at
    ON agent_traffic_states(updated_at);

CREATE TABLE IF NOT EXISTS subscription_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    last_sync_at INTEGER NOT NULL DEFAULT 0,
    last_sync_err TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_subscription_sources_type_enabled
    ON subscription_sources(type, enabled);
CREATE INDEX IF NOT EXISTS idx_subscription_sources_enabled
    ON subscription_sources(enabled);
CREATE INDEX IF NOT EXISTS idx_subscription_sources_name
    ON subscription_sources(name);

CREATE TABLE IF NOT EXISTS subscription_filter_reasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,
    source_id INTEGER NOT NULL DEFAULT 0,
    server_id INTEGER NOT NULL DEFAULT 0,
    node_name TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_subscription_filter_reasons_source
    ON subscription_filter_reasons(source_type, source_id, created_at);
CREATE INDEX IF NOT EXISTS idx_subscription_filter_reasons_server
    ON subscription_filter_reasons(server_id, created_at);
CREATE INDEX IF NOT EXISTS idx_subscription_filter_reasons_reason
    ON subscription_filter_reasons(reason, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_subscription_filter_reasons_reason;
DROP INDEX IF EXISTS idx_subscription_filter_reasons_server;
DROP INDEX IF EXISTS idx_subscription_filter_reasons_source;
DROP TABLE IF EXISTS subscription_filter_reasons;

DROP INDEX IF EXISTS idx_subscription_sources_name;
DROP INDEX IF EXISTS idx_subscription_sources_enabled;
DROP INDEX IF EXISTS idx_subscription_sources_type_enabled;
DROP TABLE IF EXISTS subscription_sources;

DROP INDEX IF EXISTS idx_agent_traffic_states_updated_at;
DROP INDEX IF EXISTS idx_agent_traffic_states_boot_id;
DROP TABLE IF EXISTS agent_traffic_states;

DROP INDEX IF EXISTS idx_agent_traffic_policies_threshold_action;
DROP INDEX IF EXISTS idx_agent_traffic_policies_reset_mode;
DROP INDEX IF EXISTS idx_agent_traffic_policies_enabled_threshold;
DROP TABLE IF EXISTS agent_traffic_policies;

DROP INDEX IF EXISTS idx_agent_lifecycle_operations_source_created;
DROP INDEX IF EXISTS idx_agent_lifecycle_operations_claimed_by_status;
DROP INDEX IF EXISTS idx_agent_lifecycle_operations_agent_type_status_created;
DROP INDEX IF EXISTS idx_agent_lifecycle_operations_agent_status_created;
DROP TABLE IF EXISTS agent_lifecycle_operations;
