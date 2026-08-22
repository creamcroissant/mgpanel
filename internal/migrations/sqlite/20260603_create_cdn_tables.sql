-- +goose Up
-- 创建 CDN 功能相关表

-- CDN 站点表
CREATE TABLE IF NOT EXISTS cdn_sites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    domain TEXT NOT NULL,
    origin_type TEXT NOT NULL DEFAULT 'reverse_proxy',  -- reverse_proxy / static_files / s3
    origin_url TEXT NOT NULL DEFAULT '',
    cache_ttl INTEGER NOT NULL DEFAULT 3600,
    ssl_mode TEXT NOT NULL DEFAULT 'auto_acme',  -- auto_acme / custom / none / cloudflare
    custom_cert_pem TEXT DEFAULT '',
    custom_key_pem TEXT DEFAULT '',
    acceleration_mode TEXT NOT NULL DEFAULT 'none',  -- none / xhttp
    inbound_spec_id INTEGER DEFAULT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cdn_sites_domain ON cdn_sites(domain);
CREATE INDEX IF NOT EXISTS idx_cdn_sites_inbound_spec ON cdn_sites(inbound_spec_id);
CREATE INDEX IF NOT EXISTS idx_cdn_sites_enabled ON cdn_sites(enabled);

-- CDN 边缘节点关联表（站点与 Agent 的绑定）
CREATE TABLE IF NOT EXISTS cdn_edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER NOT NULL REFERENCES cdn_sites(id) ON DELETE CASCADE,
    agent_host_id INTEGER NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'pending',  -- pending / deploying / active / error / removing
    last_error TEXT DEFAULT '',
    deployed_at INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cdn_edges_site ON cdn_edges(site_id);
CREATE INDEX IF NOT EXISTS idx_cdn_edges_agent ON cdn_edges(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_cdn_edges_status ON cdn_edges(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cdn_edges_unique ON cdn_edges(site_id, agent_host_id);

-- CDN 缓存规则表
CREATE TABLE IF NOT EXISTS cdn_cache_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER NOT NULL REFERENCES cdn_sites(id) ON DELETE CASCADE,
    match_type TEXT NOT NULL,  -- path / extension / query / header
    match_value TEXT NOT NULL,
    cache_ttl INTEGER NOT NULL DEFAULT 3600,
    bypass INTEGER NOT NULL DEFAULT 0,
    priority INTEGER NOT NULL DEFAULT 10,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cdn_cache_rules_site ON cdn_cache_rules(site_id);

-- Cloudflare zone 绑定表
CREATE TABLE IF NOT EXISTS cdn_cloudflare_zones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,  -- example.com
    zone_id TEXT NOT NULL,
    api_token_encrypted TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_cdn_cf_zones_name ON cdn_cloudflare_zones(name);

-- Cloudflare DNS 记录表
CREATE TABLE IF NOT EXISTS cdn_cloudflare_dns_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id INTEGER NOT NULL REFERENCES cdn_cloudflare_zones(id) ON DELETE CASCADE,
    name TEXT NOT NULL,  -- cdn.example.com
    type TEXT NOT NULL DEFAULT 'A',  -- A / CNAME / AAAA
    content TEXT NOT NULL,  -- IP 地址或 CNAME 目标
    proxied INTEGER NOT NULL DEFAULT 1,
    ttl INTEGER NOT NULL DEFAULT 1,
    cf_record_id TEXT NOT NULL DEFAULT '',
    synced_at INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cdn_cf_dns_zone ON cdn_cloudflare_dns_records(zone_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cdn_cf_dns_name ON cdn_cloudflare_dns_records(zone_id, name, type);

-- CloudFront Distribution 表
CREATE TABLE IF NOT EXISTS cdn_cloudfront_distributions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER NOT NULL REFERENCES cdn_sites(id) ON DELETE CASCADE,
    distribution_id TEXT NOT NULL,  -- CloudFront Distribution ID (E1234567890)
    distribution_arn TEXT NOT NULL,
    domain_name TEXT NOT NULL,   -- dxxx.cloudfront.net
    cert_arn TEXT DEFAULT '',
    price_class TEXT DEFAULT 'PriceClass_100',
    enabled INTEGER NOT NULL DEFAULT 1,
    status TEXT DEFAULT 'InProgress',
    last_synced_at INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cdn_cf_dist_site ON cdn_cloudfront_distributions(site_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cdn_cf_dist_id ON cdn_cloudfront_distributions(distribution_id);

-- +goose Down
DROP TABLE IF EXISTS cdn_cloudfront_distributions;
DROP TABLE IF EXISTS cdn_cloudflare_dns_records;
DROP TABLE IF EXISTS cdn_cloudflare_zones;
DROP TABLE IF EXISTS cdn_cache_rules;
DROP TABLE IF EXISTS cdn_edges;
DROP TABLE IF EXISTS cdn_sites;
