-- +goose Up
-- 创建流量上报幂等去重表，避免同一 agent_host 的同一 report_id 重复入账
CREATE TABLE IF NOT EXISTS traffic_report_dedups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    report_id TEXT NOT NULL,
    handled_at INTEGER NOT NULL,
    UNIQUE(agent_host_id, report_id)
);

CREATE INDEX IF NOT EXISTS idx_traffic_report_dedups_agent_host ON traffic_report_dedups(agent_host_id);

-- +goose Down
DROP INDEX IF EXISTS idx_traffic_report_dedups_agent_host;
DROP TABLE IF EXISTS traffic_report_dedups;
