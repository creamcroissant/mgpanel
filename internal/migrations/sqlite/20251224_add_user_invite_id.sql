-- +goose Up
ALTER TABLE users ADD COLUMN invite_user_id INTEGER DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_users_invite_user_id ON users(invite_user_id);

-- +goose Down
-- SQLite does not support dropping columns easily before recent versions, but we can try or ignore for now as it is additive.
-- ALTER TABLE users DROP COLUMN invite_user_id;
