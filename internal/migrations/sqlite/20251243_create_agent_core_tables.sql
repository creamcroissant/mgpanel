-- +goose Up
-- 创建 agent_core_instances 表，用于记录 Agent 核心实例状态
CREATE TABLE IF NOT EXISTS agent_core_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    instance_id TEXT NOT NULL,
    core_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'stopped',
    listen_ports TEXT,                -- JSON array
    config_template_id INTEGER,       -- 关联配置模板（可为空）
    config_hash TEXT,
    started_at INTEGER,
    last_heartbeat_at INTEGER,
    error_message TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(agent_host_id, instance_id),
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_core_instances_agent ON agent_core_instances(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_agent_core_instances_status ON agent_core_instances(status);
CREATE INDEX IF NOT EXISTS idx_agent_core_instances_core ON agent_core_instances(core_type);

-- 创建 agent_core_switch_logs 表，用于记录核心切换审计日志
CREATE TABLE IF NOT EXISTS agent_core_switch_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    from_instance_id TEXT,
    from_core_type TEXT,
    to_instance_id TEXT NOT NULL,
    to_core_type TEXT NOT NULL,
    operator_id INTEGER,              -- 操作人 ID（管理员）
    status TEXT NOT NULL,             -- pending/in_progress/completed/failed
    detail TEXT,                      -- JSON: {reason, error, duration_ms}
    created_at INTEGER NOT NULL,
    completed_at INTEGER,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_core_switch_logs_agent ON agent_core_switch_logs(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_agent_core_switch_logs_status ON agent_core_switch_logs(status);
CREATE INDEX IF NOT EXISTS idx_agent_core_switch_logs_time ON agent_core_switch_logs(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_agent_core_switch_logs_time;
DROP INDEX IF EXISTS idx_agent_core_switch_logs_status;
DROP INDEX IF EXISTS idx_agent_core_switch_logs_agent;
DROP TABLE IF EXISTS agent_core_switch_logs;
DROP INDEX IF EXISTS idx_agent_core_instances_core;
DROP INDEX IF EXISTS idx_agent_core_instances_status;
DROP INDEX IF EXISTS idx_agent_core_instances_agent;
DROP TABLE IF EXISTS agent_core_instances;
