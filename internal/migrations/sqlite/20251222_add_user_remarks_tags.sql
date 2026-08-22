-- +goose Up
-- Add remarks and tags columns to users table
ALTER TABLE users ADD COLUMN remarks TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN tags TEXT DEFAULT '[]';

-- +goose Down
ALTER TABLE users DROP COLUMN remarks;
ALTER TABLE users DROP COLUMN tags;