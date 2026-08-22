-- +goose Up
-- Fix: original 20260603_create_cdn_tables.sql omitted the `status` column.
ALTER TABLE cdn_sites ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE cdn_sites ADD COLUMN provider TEXT NOT NULL DEFAULT '';
ALTER TABLE cdn_sites ADD COLUMN origin_path TEXT NOT NULL DEFAULT '';
ALTER TABLE cdn_sites ADD COLUMN origin_protocol TEXT NOT NULL DEFAULT 'http';
ALTER TABLE cdn_sites ADD COLUMN last_deployed_at INTEGER DEFAULT NULL;

-- +goose Down
ALTER TABLE cdn_sites DROP COLUMN status;
ALTER TABLE cdn_sites DROP COLUMN provider;
ALTER TABLE cdn_sites DROP COLUMN origin_path;
ALTER TABLE cdn_sites DROP COLUMN origin_protocol;
ALTER TABLE cdn_sites DROP COLUMN last_deployed_at;
