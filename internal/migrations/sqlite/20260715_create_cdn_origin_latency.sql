-- +goose Up
CREATE TABLE IF NOT EXISTS cdn_origin_latency (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER NOT NULL,
    stack TEXT NOT NULL CHECK(stack IN ('v4','v6')),
    latency_ms INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL,
    UNIQUE(site_id, stack)
);

CREATE INDEX IF NOT EXISTS idx_cdn_origin_latency_site ON cdn_origin_latency(site_id);

-- +goose Down
DROP TABLE IF EXISTS cdn_origin_latency;
