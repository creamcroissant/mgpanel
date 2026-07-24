package cloudflare

import (
	"context"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// Provider interface
// ---------------------------------------------------------------------------

// Provider defines the cloud CDN operations that a provider must implement.
type Provider interface {
	// SyncDistribution ensures the zone and DNS records match the desired
	// distribution configuration (domain, origin, proxy settings).
	SyncDistribution(ctx context.Context, cfg DistributionConfig) error

	// SyncCacheRules applies cache rules for the given zone.
	SyncCacheRules(ctx context.Context, zoneID string, rules []CacheRuleConfig) error

	// Invalidate purges cached content for the given zone. When urls is empty
	// the entire cache is invalidated.
	Invalidate(ctx context.Context, zoneID string, urls []string) error
}

// ---------------------------------------------------------------------------
// Config types
// ---------------------------------------------------------------------------

// DistributionConfig describes the desired state of a CDN distribution.
type DistributionConfig struct {
	// Domain is the CDN-accelerated domain name (e.g. cdn.example.com).
	Domain string

	// Origin is the upstream origin server address.
	Origin string

	// OriginType describes the origin type: "http", "https", "hostname".
	OriginType string

	// ProxyEnabled controls whether Cloudflare proxy (orange cloud) is on.
	ProxyEnabled bool

	// SSLMode is the desired SSL/TLS setting ("off", "flexible", "full", "strict").
	SSLMode string

	// TTL is the default DNS record TTL in seconds (1 = automatic).
	TTL int
}

// CacheRuleConfig describes a single cache rule.
type CacheRuleConfig struct {
	// Expression is the Cloudflare Rules language expression (e.g.
	// `http.host eq "cdn.example.com" and starts_with(http.request.uri.path, "/static")`).
	Expression string

	// Action is the cache action: "bypass", "cache", "set_cache_ttl".
	Action string

	// ActionParameters holds action-specific key-value pairs.
	// For "set_cache_ttl": {"ttl": 86400, "mode": "respect_origin"}
	ActionParameters map[string]any

	// Enabled controls whether the rule is active.
	Enabled bool

	// Priority controls evaluation order (lower number = higher priority).
	Priority int

	// Description is an optional human-readable label.
	Description string
}

// ---------------------------------------------------------------------------
// CloudflareProvider implements Provider via the Cloudflare API.
// ---------------------------------------------------------------------------

// CloudflareProvider uses the Cloudflare REST API v4 to manage CDN
// distributions, DNS records, cache rules, and cache invalidation.
type CloudflareProvider struct {
	client *Client
}

// NewCloudflareProvider creates a new CloudflareProvider.
func NewCloudflareProvider(apiToken string) *CloudflareProvider {
	return &CloudflareProvider{
		client: NewClient(apiToken),
	}
}

// WithHTTPClient sets a custom HTTP client on the underlying API client.
func (p *CloudflareProvider) WithHTTPClient(hc *http.Client) *CloudflareProvider {
	p.client.WithHTTPClient(hc)
	return p
}

// WithBaseURL sets a custom base URL on the underlying API client.
func (p *CloudflareProvider) WithBaseURL(baseURL string) *CloudflareProvider {
	p.client.WithBaseURL(baseURL)
	return p
}

// SyncDistribution ensures the zone and DNS A/AAAA/CNAME record exist and
// are configured according to the DistributionConfig.
//
// It looks up the zone by domain, finds or creates an appropriate DNS record
// pointing to the origin, and updates the SSL setting.
func (p *CloudflareProvider) SyncDistribution(ctx context.Context, cfg DistributionConfig) error {
	// 1. Resolve zone – we use the apex of the domain.
	zoneName := apexDomain(cfg.Domain)
	zones, err := p.client.ListZones(ctx)
	if err != nil {
		return fmt.Errorf("list zones: %w", err)
	}
	var zone *CFZone
	for i := range zones {
		if zones[i].Name == zoneName {
			zone = &zones[i]
			break
		}
	}
	if zone == nil {
		return fmt.Errorf("zone %q not found in Cloudflare account", zoneName)
	}

	// 2. Find or create the DNS record.
	records, err := p.client.ListDNSRecords(ctx, zone.ID)
	if err != nil {
		return fmt.Errorf("list dns records: %w", err)
	}

	proxied := cfg.ProxyEnabled
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 1 // 1 = automatic
	}
	recordType := dnsRecordType(cfg.OriginType)

	var existing *CFDNSRecord
	for i := range records {
		if records[i].Name == cfg.Domain && records[i].Type == recordType {
			existing = &records[i]
			break
		}
	}

	if existing != nil {
		// Update the existing record.
		updated := CFDNSRecord{
			Name:    cfg.Domain,
			Type:    recordType,
			Content: cfg.Origin,
			Proxied: &proxied,
			TTL:     ttl,
		}
		if err := p.client.UpdateDNSRecord(ctx, zone.ID, existing.ID, updated); err != nil {
			return fmt.Errorf("update dns record: %w", err)
		}
	} else {
		// Create a new record.
		record := CFDNSRecord{
			Name:    cfg.Domain,
			Type:    recordType,
			Content: cfg.Origin,
			Proxied: &proxied,
			TTL:     ttl,
		}
		if err := p.client.CreateDNSRecord(ctx, zone.ID, record); err != nil {
			return fmt.Errorf("create dns record: %w", err)
		}
	}

	// 3. Sync SSL setting if requested.
	if cfg.SSLMode != "" {
		if err := p.updateSSLSetting(ctx, zone.ID, cfg.SSLMode); err != nil {
			return fmt.Errorf("update ssl setting: %w", err)
		}
	}

	return nil
}

// SyncCacheRules applies cache rules for the given zone. This replaces
// existing cache rules by creating new ones.
func (p *CloudflareProvider) SyncCacheRules(ctx context.Context, zoneID string, rules []CacheRuleConfig) error {
	for _, rule := range rules {
		cfRule := CFCacheRule{
			Expression:       rule.Expression,
			Action:           rule.Action,
			ActionParameters: rule.ActionParameters,
			Enabled:          rule.Enabled,
			Priority:         rule.Priority,
			Description:      rule.Description,
		}
		if err := p.client.CreateCacheRule(ctx, zoneID, cfRule); err != nil {
			return fmt.Errorf("create cache rule: %w", err)
		}
	}
	return nil
}

// Invalidate purges cached content for the given zone. When urls is empty
// the entire cache is purged.
func (p *CloudflareProvider) Invalidate(ctx context.Context, zoneID string, urls []string) error {
	return p.client.PurgeCache(ctx, zoneID, urls)
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

// updateSSLSetting sets the SSL/TLS encryption mode for a zone by using the
// zone settings API.
func (p *CloudflareProvider) updateSSLSetting(ctx context.Context, zoneID, mode string) error {
	// Re-use the client's do() to PATCH the ssl setting.
	body := fmt.Sprintf(`{"value":%q}`, mode)
	_, err := p.client.do(ctx, "PATCH", fmt.Sprintf("/zones/%s/settings/ssl", zoneID), []byte(body))
	return err
}

// dnsRecordType maps the origin type to a DNS record type.
func dnsRecordType(originType string) string {
	switch originType {
	case "http", "https":
		return "CNAME"
	case "hostname":
		return "A"
	default:
		return "CNAME"
	}
}

// apexDomain returns the apex (registered domain) of a full domain name.
// For "cdn.example.com" it returns "example.com".
// This is a simple heuristic; a production implementation should use the
// Public Suffix List.
// TODO: use github.com/weppos/publicsuffix-go/publicsuffix for proper
// apex domain extraction instead of the simple heuristic below.
func apexDomain(domain string) string {
	// Find the last two dot-separated parts.
	dotCount := 0
	for i := len(domain) - 1; i >= 0; i-- {
		if domain[i] == '.' {
			dotCount++
			if dotCount == 2 {
				return domain[i+1:]
			}
		}
	}
	// Single label or no dot — return as-is.
	return domain
}
