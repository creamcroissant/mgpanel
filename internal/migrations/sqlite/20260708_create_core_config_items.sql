-- +goose Up
-- 新增核心配置项表，管理 outbound/routing/DNS/核心设置等非 inbound 配置
-- 每个配置项单独一行，支持模板/主机绑定模式

CREATE TABLE IF NOT EXISTS core_config_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER DEFAULT NULL,     -- NULL = 模板项，不绑定特定主机
    core_type TEXT NOT NULL,                -- 'sing-box' | 'xray'
    config_type TEXT NOT NULL,              -- 'outbound' | 'routing' | 'dns' | 'core_settings'
    tag TEXT NOT NULL,                       -- 配置块标识（outbound tag、或固定标识如 '_routing'）
    enabled INTEGER NOT NULL DEFAULT 1,
    config_data TEXT NOT NULL DEFAULT '{}', -- 结构化 JSON 配置
    desired_revision INTEGER NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL DEFAULT 0,
    updated_by INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cci_host_core ON core_config_items(agent_host_id, core_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cci_host_core_type_tag ON core_config_items(agent_host_id, core_type, config_type, tag);

-- +goose Down
DROP TABLE IF EXISTS core_config_items;
