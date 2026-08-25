-- +goose Up
INSERT INTO settings(key, value, category) VALUES ('desired_artifact.retention_revisions', '10', 'retention') ON CONFLICT(key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key = 'desired_artifact.retention_revisions';
