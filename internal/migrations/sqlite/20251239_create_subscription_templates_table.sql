-- +goose Up
-- 创建订阅模板表，用于存储用户自定义的 Clash/Singbox 等客户端配置模板
CREATE TABLE IF NOT EXISTS subscription_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    type TEXT NOT NULL CHECK(type IN ('clash', 'singbox', 'surge', 'general')),
    content TEXT NOT NULL,
    is_default INTEGER DEFAULT 0,
    is_public INTEGER DEFAULT 1,
    sort_order INTEGER DEFAULT 0,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER DEFAULT (strftime('%s', 'now'))
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_subscription_templates_type ON subscription_templates(type);
CREATE INDEX IF NOT EXISTS idx_subscription_templates_is_default ON subscription_templates(is_default);
CREATE INDEX IF NOT EXISTS idx_subscription_templates_is_public ON subscription_templates(is_public);

-- 插入默认模板
INSERT INTO subscription_templates (name, description, type, content, is_default, is_public) VALUES
('默认 Clash 模板', '基础 Clash 配置模板', 'clash', '', 1, 1),
('默认 Singbox 模板', '基础 Sing-box 配置模板', 'singbox', '', 1, 1);

-- +goose Down
DROP TABLE IF EXISTS subscription_templates;
