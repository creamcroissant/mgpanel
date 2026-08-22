-- +goose Up
-- Create config_templates table
CREATE TABLE IF NOT EXISTS config_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT 'sing-box',
    content TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0
);

-- Add template_id to agent_hosts table
ALTER TABLE agent_hosts ADD COLUMN template_id INTEGER DEFAULT 0;

-- +goose Down
ALTER TABLE agent_hosts DROP COLUMN template_id;
DROP TABLE IF EXISTS config_templates;
