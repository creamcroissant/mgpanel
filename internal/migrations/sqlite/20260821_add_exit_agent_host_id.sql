-- +goose Up
-- +goose StatementBegin
ALTER TABLE inbound_specs ADD COLUMN exit_agent_host_id INTEGER REFERENCES agent_hosts(id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE inbound_specs DROP COLUMN exit_agent_host_id;
-- +goose StatementEnd
