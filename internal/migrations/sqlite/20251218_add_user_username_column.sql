-- +goose Up
ALTER TABLE users ADD COLUMN username TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE username <> '';

-- +goose Down
-- SQLite cannot drop columns without rebuilding the table; treat as irreversible.
SELECT 1;
