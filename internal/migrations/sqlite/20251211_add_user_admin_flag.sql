-- +goose Up
ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0;
UPDATE users SET is_admin = 1 WHERE id = 1;

-- +goose Down
-- SQLite cannot drop columns without table rebuild; mark as irreversible.
SELECT 1;
