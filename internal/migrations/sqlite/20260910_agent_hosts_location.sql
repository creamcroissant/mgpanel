-- +goose Up
-- +goose StatementBegin
ALTER TABLE agent_hosts ADD COLUMN country TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE agent_hosts ADD COLUMN region TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agent_hosts DROP COLUMN region;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE agent_hosts DROP COLUMN country;
-- +goose StatementEnd
