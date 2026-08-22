-- +goose Up
-- Create server_client_configs table for storing client-side configurations
CREATE TABLE IF NOT EXISTS server_client_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    format TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
    UNIQUE(server_id, format)
);

-- Index for faster lookups
CREATE INDEX IF NOT EXISTS idx_server_client_configs_server_id ON server_client_configs(server_id);

-- +goose Down
DROP INDEX IF EXISTS idx_server_client_configs_server_id;
DROP TABLE IF EXISTS server_client_configs;
