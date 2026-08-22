-- +goose Up
-- Create user_server_selections table for user-node many-to-many relationship
CREATE TABLE IF NOT EXISTS user_server_selections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    server_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
    UNIQUE(user_id, server_id)
);

-- Indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_user_server_selections_user_id ON user_server_selections(user_id);
CREATE INDEX IF NOT EXISTS idx_user_server_selections_server_id ON user_server_selections(server_id);

-- +goose Down
DROP INDEX IF EXISTS idx_user_server_selections_server_id;
DROP INDEX IF EXISTS idx_user_server_selections_user_id;
DROP TABLE IF EXISTS user_server_selections;
