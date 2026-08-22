-- +goose Up
-- 创建 forwarding_rules 表，用于存储 nftables 端口转发规则
CREATE TABLE IF NOT EXISTS forwarding_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,           -- 关联的 Agent
    name TEXT NOT NULL,                       -- 规则名称
    protocol TEXT NOT NULL DEFAULT 'tcp',     -- tcp/udp/both
    listen_port INTEGER NOT NULL,             -- 本地监听端口
    target_address TEXT NOT NULL,             -- 目标地址 (IP 或域名)
    target_port INTEGER NOT NULL,             -- 目标端口
    enabled INTEGER NOT NULL DEFAULT 1,       -- 是否启用
    priority INTEGER NOT NULL DEFAULT 100,    -- 优先级（越小越优先）
    remark TEXT,                              -- 备注
    version INTEGER NOT NULL DEFAULT 1,       -- 规则版本
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_forwarding_rules_agent ON forwarding_rules(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_forwarding_rules_enabled ON forwarding_rules(enabled);
CREATE UNIQUE INDEX IF NOT EXISTS idx_forwarding_rules_unique_port ON forwarding_rules(agent_host_id, listen_port, protocol);

-- 创建 forwarding_rule_logs 表，用于审计日志
CREATE TABLE IF NOT EXISTS forwarding_rule_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER,                          -- 关联规则 ID（删除后可为空）
    agent_host_id INTEGER NOT NULL,
    action TEXT NOT NULL,                     -- create/update/delete/apply/fail
    operator_id INTEGER,                      -- 操作人 ID（管理员）
    detail TEXT,                              -- 变更详情 (JSON)
    created_at INTEGER NOT NULL
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_forwarding_rule_logs_agent ON forwarding_rule_logs(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_forwarding_rule_logs_time ON forwarding_rule_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_forwarding_rule_logs_rule ON forwarding_rule_logs(rule_id);

-- +goose Down
DROP INDEX IF EXISTS idx_forwarding_rule_logs_rule;
DROP INDEX IF EXISTS idx_forwarding_rule_logs_time;
DROP INDEX IF EXISTS idx_forwarding_rule_logs_agent;
DROP TABLE IF EXISTS forwarding_rule_logs;
DROP INDEX IF EXISTS idx_forwarding_rules_unique_port;
DROP INDEX IF EXISTS idx_forwarding_rules_enabled;
DROP INDEX IF EXISTS idx_forwarding_rules_agent;
DROP TABLE IF EXISTS forwarding_rules;
