-- +goose Up
ALTER TABLE inbound_specs ADD COLUMN relay_path_id INTEGER REFERENCES relay_paths(id) ON DELETE SET NULL;

-- +goose Down
SQLite 3.35+ 支持 DROP COLUMN；老版本需重建表，此处仅保留回滚语义说明。
