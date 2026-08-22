-- +goose Up
-- Add telegram_id column to users table for alert notifications
ALTER TABLE users ADD COLUMN telegram_id TEXT;

-- +goose Down
-- SQLite doesn't support DROP COLUMN in older versions, but modern SQLite 3.35+ does
-- For compatibility, we leave this empty (the column will remain)
