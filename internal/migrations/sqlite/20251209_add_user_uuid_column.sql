-- +goose Up
ALTER TABLE users ADD COLUMN uuid TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite cannot drop columns without rebuilding the table; noop for down migration.
SELECT 1;
