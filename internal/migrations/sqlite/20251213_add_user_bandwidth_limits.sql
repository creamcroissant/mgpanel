-- +goose Up
ALTER TABLE users ADD COLUMN speed_limit INTEGER;
ALTER TABLE users ADD COLUMN device_limit INTEGER;

-- +goose Down
-- SQLite cannot drop columns without rebuilding the table; noop for down migration.
SELECT 1;
