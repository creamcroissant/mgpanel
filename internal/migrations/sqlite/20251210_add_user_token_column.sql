-- +goose Up
ALTER TABLE users ADD COLUMN token TEXT NOT NULL DEFAULT '';
UPDATE users SET token = lower(hex(randomblob(16))) WHERE token IS NULL OR token = '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_token ON users(token) WHERE token <> '';

-- +goose Down
-- SQLite cannot drop columns or partial indexes without rebuilding the table; treat as irreversible.
SELECT 1;
