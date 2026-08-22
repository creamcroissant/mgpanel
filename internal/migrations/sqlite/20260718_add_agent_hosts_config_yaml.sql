-- +goose Up
-- +goose StatementBegin
ALTER TABLE agent_hosts ADD COLUMN config_yaml TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agent_hosts DROP COLUMN config_yaml;
-- +goose StatementEnd
