-- +goose Up
-- Add limit and expire_at columns to invite_codes table
ALTER TABLE v2_invite_code ADD COLUMN "limit" INTEGER DEFAULT 1;
ALTER TABLE v2_invite_code ADD COLUMN expire_at INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN invite_limit INTEGER DEFAULT -1;
ALTER TABLE plans ADD COLUMN invite_limit INTEGER DEFAULT -1;

-- +goose Down
ALTER TABLE v2_invite_code DROP COLUMN "limit";
ALTER TABLE v2_invite_code DROP COLUMN expire_at;
ALTER TABLE users DROP COLUMN invite_limit;
ALTER TABLE plans DROP COLUMN invite_limit;