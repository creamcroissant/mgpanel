-- +goose Up
ALTER TABLE cdn_sites ADD COLUMN asn_tags TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE cdn_sites DROP COLUMN asn_tags;
