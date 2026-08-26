-- +goose Up
ALTER TABLE relay_path_nodes ADD COLUMN private_key TEXT NOT NULL DEFAULT '';
ALTER TABLE relay_path_nodes ADD COLUMN public_key TEXT NOT NULL DEFAULT '';

-- +goose Down
SQLite 3.35+ 支持 DROP COLUMN；老版本需重建表，此处仅保留回滚语义说明。
ALTER TABLE relay_path_nodes DROP COLUMN public_key;
ALTER TABLE relay_path_nodes DROP COLUMN private_key;
