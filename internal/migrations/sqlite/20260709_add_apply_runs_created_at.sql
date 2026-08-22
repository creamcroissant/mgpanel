-- +goose Up
-- ApplyRun 超时机制：增加 created_at 列用于超时判断
ALTER TABLE apply_runs ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0;
UPDATE apply_runs SET created_at = started_at WHERE created_at = 0;

-- +goose Down
ALTER TABLE apply_runs DROP COLUMN created_at;
