-- +goose Up
-- Add agent_host_id column to track traffic source node
ALTER TABLE stat_users ADD COLUMN agent_host_id INTEGER NOT NULL DEFAULT 0;

-- Drop old unique index
DROP INDEX IF EXISTS idx_stat_users_unique;

-- Create new unique index including agent_host_id
CREATE UNIQUE INDEX IF NOT EXISTS idx_stat_users_unique
    ON stat_users(user_id, agent_host_id, record_type, record_at);

-- Create auxiliary index for querying by agent host
CREATE INDEX IF NOT EXISTS idx_stat_users_agent_host
    ON stat_users(agent_host_id, record_at);

-- +goose Down
-- Drop new indexes
DROP INDEX IF EXISTS idx_stat_users_agent_host;
DROP INDEX IF EXISTS idx_stat_users_unique;

-- SQLite does not support DROP COLUMN, need to rebuild table
CREATE TABLE stat_users_backup AS
    SELECT id, user_id, server_rate, record_at, record_type, u, d, created_at, updated_at
    FROM stat_users;
DROP TABLE stat_users;
ALTER TABLE stat_users_backup RENAME TO stat_users;

-- Recreate original unique index
CREATE UNIQUE INDEX IF NOT EXISTS idx_stat_users_unique
    ON stat_users(user_id, record_type, record_at);
