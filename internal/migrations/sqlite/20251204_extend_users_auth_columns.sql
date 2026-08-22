-- +goose Up
ALTER TABLE users ADD COLUMN password_algo TEXT;
ALTER TABLE users ADD COLUMN password_salt TEXT;
ALTER TABLE users ADD COLUMN banned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN last_login_at INTEGER;

-- +goose Down
-- SQLite cannot drop columns without rebuilding the table; this migration is irreversible.
SELECT 1;
