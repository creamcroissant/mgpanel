-- +goose Up
-- Extend config_templates table with version compatibility and validation fields

-- Add minimum version requirement column
ALTER TABLE config_templates ADD COLUMN min_version TEXT NOT NULL DEFAULT '';

-- Add description column for template documentation
ALTER TABLE config_templates ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- Add schema_version column to track template format version
ALTER TABLE config_templates ADD COLUMN schema_version INTEGER NOT NULL DEFAULT 1;

-- Add is_valid column to cache validation status
ALTER TABLE config_templates ADD COLUMN is_valid INTEGER NOT NULL DEFAULT 1;

-- Add validation_error column to store last validation error
ALTER TABLE config_templates ADD COLUMN validation_error TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE config_templates DROP COLUMN validation_error;
ALTER TABLE config_templates DROP COLUMN is_valid;
ALTER TABLE config_templates DROP COLUMN schema_version;
ALTER TABLE config_templates DROP COLUMN description;
ALTER TABLE config_templates DROP COLUMN min_version;

