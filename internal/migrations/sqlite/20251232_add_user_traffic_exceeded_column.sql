-- +goose Up
-- Add traffic_exceeded column to users table for tracking exceeded status
ALTER TABLE users ADD COLUMN traffic_exceeded INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN directly, so we recreate the table
-- This is handled by copying data to a temp table and recreating
-- For simplicity, we'll leave this as a no-op since the column is backward compatible
