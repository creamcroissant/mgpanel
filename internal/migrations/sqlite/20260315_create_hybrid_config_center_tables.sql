-- +goose Up
-- Hybrid Inbound Config Center tables (Desired / Applied / Drift)
-- Notes:
-- 1) This migration is forward-only and does not alter historical tables.
-- 2) For safety and compatibility, goose Down only drops objects created here.

-- Desired: inbound specs (tag-level source of truth)
CREATE TABLE IF NOT EXISTS inbound_specs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    core_type TEXT NOT NULL,                    -- sing-box | xray
    tag TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    semantic_spec TEXT NOT NULL DEFAULT '{}',   -- JSON
    core_specific TEXT NOT NULL DEFAULT '{}',   -- JSON
    desired_revision INTEGER NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL DEFAULT 0,
    updated_by INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_inbound_specs_agent_host ON inbound_specs(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_inbound_specs_core_type ON inbound_specs(core_type);
CREATE INDEX IF NOT EXISTS idx_inbound_specs_enabled ON inbound_specs(enabled);
CREATE UNIQUE INDEX IF NOT EXISTS idx_inbound_specs_unique_tag ON inbound_specs(agent_host_id, core_type, tag);

-- Desired history: immutable snapshots per revision
CREATE TABLE IF NOT EXISTS inbound_spec_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    spec_id INTEGER NOT NULL,
    revision INTEGER NOT NULL,
    snapshot TEXT NOT NULL,                     -- JSON snapshot
    change_note TEXT NOT NULL DEFAULT '',
    operator_id INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (spec_id) REFERENCES inbound_specs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_inbound_spec_revisions_spec_id ON inbound_spec_revisions(spec_id);
CREATE INDEX IF NOT EXISTS idx_inbound_spec_revisions_created_at ON inbound_spec_revisions(created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_inbound_spec_revisions_unique ON inbound_spec_revisions(spec_id, revision);

-- Deployable unit: rendered artifact files
CREATE TABLE IF NOT EXISTS desired_artifacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    core_type TEXT NOT NULL,
    desired_revision INTEGER NOT NULL,
    filename TEXT NOT NULL,
    source_tag TEXT NOT NULL,
    content BLOB NOT NULL,
    content_hash TEXT NOT NULL,
    generated_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_desired_artifacts_agent_host ON desired_artifacts(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_desired_artifacts_core_type ON desired_artifacts(core_type);
CREATE INDEX IF NOT EXISTS idx_desired_artifacts_revision ON desired_artifacts(desired_revision);
CREATE INDEX IF NOT EXISTS idx_desired_artifacts_source_tag ON desired_artifacts(source_tag);
CREATE INDEX IF NOT EXISTS idx_desired_artifacts_hash ON desired_artifacts(content_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_desired_artifacts_unique_file_in_batch
    ON desired_artifacts(agent_host_id, core_type, desired_revision, filename);

-- Apply execution audit/state
CREATE TABLE IF NOT EXISTS apply_runs (
    run_id TEXT PRIMARY KEY,
    agent_host_id INTEGER NOT NULL,
    core_type TEXT NOT NULL,
    target_revision INTEGER NOT NULL,
    status TEXT NOT NULL,                       -- pending | applying | success | failed | rolled_back
    error_message TEXT NOT NULL DEFAULT '',
    previous_revision INTEGER NOT NULL DEFAULT 0,
    rollback_revision INTEGER NOT NULL DEFAULT 0,
    operator_id INTEGER NOT NULL DEFAULT 0,
    started_at INTEGER NOT NULL,
    finished_at INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apply_runs_agent_host ON apply_runs(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_apply_runs_core_type ON apply_runs(core_type);
CREATE INDEX IF NOT EXISTS idx_apply_runs_status ON apply_runs(status);
CREATE INDEX IF NOT EXISTS idx_apply_runs_started_at ON apply_runs(started_at);

-- Applied inventory: file-level observation
CREATE TABLE IF NOT EXISTS agent_config_inventory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    core_type TEXT NOT NULL,
    source TEXT NOT NULL,                       -- legacy | managed | merged
    filename TEXT NOT NULL,
    hash_applied TEXT NOT NULL,
    parse_status TEXT NOT NULL DEFAULT 'ok',    -- ok | parse_error
    parse_error TEXT NOT NULL DEFAULT '',
    last_seen_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_config_inventory_agent_host ON agent_config_inventory(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_agent_config_inventory_core_type ON agent_config_inventory(core_type);
CREATE INDEX IF NOT EXISTS idx_agent_config_inventory_source ON agent_config_inventory(source);
CREATE INDEX IF NOT EXISTS idx_agent_config_inventory_last_seen_at ON agent_config_inventory(last_seen_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_config_inventory_unique_file
    ON agent_config_inventory(agent_host_id, core_type, source, filename);

-- Applied semantic index: inbound-level observation
CREATE TABLE IF NOT EXISTS inbound_index (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    core_type TEXT NOT NULL,
    source TEXT NOT NULL,                       -- legacy | managed | merged
    filename TEXT NOT NULL,
    tag TEXT NOT NULL,
    protocol TEXT NOT NULL,
    listen TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 0,
    tls TEXT NOT NULL DEFAULT '{}',             -- JSON
    transport TEXT NOT NULL DEFAULT '{}',       -- JSON
    multiplex TEXT NOT NULL DEFAULT '{}',       -- JSON
    last_seen_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_inbound_index_agent_host ON inbound_index(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_inbound_index_core_type ON inbound_index(core_type);
CREATE INDEX IF NOT EXISTS idx_inbound_index_source ON inbound_index(source);
CREATE INDEX IF NOT EXISTS idx_inbound_index_tag ON inbound_index(tag);
CREATE INDEX IF NOT EXISTS idx_inbound_index_last_seen_at ON inbound_index(last_seen_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_inbound_index_unique_inbound
    ON inbound_index(agent_host_id, core_type, source, filename, tag);

-- Drift state: desired vs applied mismatch tracking
CREATE TABLE IF NOT EXISTS drift_states (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    core_type TEXT NOT NULL,
    filename TEXT NOT NULL,
    tag TEXT NOT NULL DEFAULT '',
    desired_revision INTEGER NOT NULL DEFAULT 0,
    desired_hash TEXT NOT NULL DEFAULT '',
    applied_hash TEXT NOT NULL DEFAULT '',
    drift_type TEXT NOT NULL,                   -- hash_mismatch | missing_tag | tag_conflict | parse_error
    status TEXT NOT NULL DEFAULT 'drift',       -- drift | recovered
    detail TEXT NOT NULL DEFAULT '{}',          -- JSON
    first_detected_at INTEGER NOT NULL,
    last_changed_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_drift_states_agent_host ON drift_states(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_drift_states_core_type ON drift_states(core_type);
CREATE INDEX IF NOT EXISTS idx_drift_states_status ON drift_states(status);
CREATE INDEX IF NOT EXISTS idx_drift_states_drift_type ON drift_states(drift_type);
CREATE INDEX IF NOT EXISTS idx_drift_states_last_changed_at ON drift_states(last_changed_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_drift_states_unique_item
    ON drift_states(agent_host_id, core_type, filename, tag, drift_type);

-- +goose Down
DROP INDEX IF EXISTS idx_drift_states_unique_item;
DROP INDEX IF EXISTS idx_drift_states_last_changed_at;
DROP INDEX IF EXISTS idx_drift_states_drift_type;
DROP INDEX IF EXISTS idx_drift_states_status;
DROP INDEX IF EXISTS idx_drift_states_core_type;
DROP INDEX IF EXISTS idx_drift_states_agent_host;
DROP TABLE IF EXISTS drift_states;

DROP INDEX IF EXISTS idx_inbound_index_unique_inbound;
DROP INDEX IF EXISTS idx_inbound_index_last_seen_at;
DROP INDEX IF EXISTS idx_inbound_index_tag;
DROP INDEX IF EXISTS idx_inbound_index_source;
DROP INDEX IF EXISTS idx_inbound_index_core_type;
DROP INDEX IF EXISTS idx_inbound_index_agent_host;
DROP TABLE IF EXISTS inbound_index;

DROP INDEX IF EXISTS idx_agent_config_inventory_unique_file;
DROP INDEX IF EXISTS idx_agent_config_inventory_last_seen_at;
DROP INDEX IF EXISTS idx_agent_config_inventory_source;
DROP INDEX IF EXISTS idx_agent_config_inventory_core_type;
DROP INDEX IF EXISTS idx_agent_config_inventory_agent_host;
DROP TABLE IF EXISTS agent_config_inventory;

DROP INDEX IF EXISTS idx_apply_runs_started_at;
DROP INDEX IF EXISTS idx_apply_runs_status;
DROP INDEX IF EXISTS idx_apply_runs_core_type;
DROP INDEX IF EXISTS idx_apply_runs_agent_host;
DROP TABLE IF EXISTS apply_runs;

DROP INDEX IF EXISTS idx_desired_artifacts_unique_file_in_batch;
DROP INDEX IF EXISTS idx_desired_artifacts_hash;
DROP INDEX IF EXISTS idx_desired_artifacts_source_tag;
DROP INDEX IF EXISTS idx_desired_artifacts_revision;
DROP INDEX IF EXISTS idx_desired_artifacts_core_type;
DROP INDEX IF EXISTS idx_desired_artifacts_agent_host;
DROP TABLE IF EXISTS desired_artifacts;

DROP INDEX IF EXISTS idx_inbound_spec_revisions_unique;
DROP INDEX IF EXISTS idx_inbound_spec_revisions_created_at;
DROP INDEX IF EXISTS idx_inbound_spec_revisions_spec_id;
DROP TABLE IF EXISTS inbound_spec_revisions;

DROP INDEX IF EXISTS idx_inbound_specs_unique_tag;
DROP INDEX IF EXISTS idx_inbound_specs_enabled;
DROP INDEX IF EXISTS idx_inbound_specs_core_type;
DROP INDEX IF EXISTS idx_inbound_specs_agent_host;
DROP TABLE IF EXISTS inbound_specs;
