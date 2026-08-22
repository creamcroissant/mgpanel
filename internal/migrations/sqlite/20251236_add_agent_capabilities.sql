-- +goose Up
-- Add capabilities columns to agent_hosts table
ALTER TABLE agent_hosts ADD COLUMN core_version TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_hosts ADD COLUMN capabilities TEXT NOT NULL DEFAULT '[]';
ALTER TABLE agent_hosts ADD COLUMN build_tags TEXT NOT NULL DEFAULT '[]';

-- Add capabilities column to config_templates table
ALTER TABLE config_templates ADD COLUMN capabilities TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE config_templates DROP COLUMN capabilities;
ALTER TABLE agent_hosts DROP COLUMN build_tags;
ALTER TABLE agent_hosts DROP COLUMN capabilities;
ALTER TABLE agent_hosts DROP COLUMN core_version;
