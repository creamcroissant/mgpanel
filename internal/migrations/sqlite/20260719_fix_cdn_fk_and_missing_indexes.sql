-- +goose Up
-- Restore index lost during 20260703 table rebuild
CREATE INDEX IF NOT EXISTS idx_inbound_specs_enabled ON inbound_specs(core_type, enabled);

-- +goose Down
DROP INDEX IF EXISTS idx_inbound_specs_enabled;
