-- +goose Up
ALTER TABLE users ADD COLUMN group_id INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support dropping columns without rebuilding the table; treat as irreversible.
SELECT 1;
