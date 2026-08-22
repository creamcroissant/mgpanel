-- +goose Up
ALTER TABLE servers ADD COLUMN last_heartbeat_at INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE servers DROP COLUMN last_heartbeat_at;
