package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ---------------------------------------------------------------------------
// API types
// ---------------------------------------------------------------------------

// apiResponse is the top-level envelope for all Cloudflare API v4 responses.
type apiResponse struct {
	Success  bool            `json:"success"`
	Errors   []apiError      `json:"errors"`
	Messages []apiMessage    `json:"messages"`
	Result   json.RawMessage `json:"result"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e apiError) Error() string {
	return fmt.Sprintf("cf error %d: %s", e.Code, e.Message)
}

type apiMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CFZone represents a Cloudflare zone.
type CFZone struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Paused     bool   `json:"paused"`
	Type       string `json:"type"`
	Plan       any    `json:"plan"`
	CreatedOn  string `json:"created_on"`
	ModifiedOn string `json:"modified_on"`
}

// CFDNSRecord represents a Cloudflare DNS record.
type CFDNSRecord struct {
	ID         string `json:"id"`
	ZoneID     string `json:"zone_id"`
	ZoneName   string `json:"zone_name"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Content    string `json:"content"`
	Proxied    *bool  `json:"proxied,omitempty"`
	TTL        int    `json:"ttl"`
	Priority   *int   `json:"priority,omitempty"`
	Comment    string `json:"comment,omitempty"`
	CreatedOn  string `json:"created_on"`
	ModifiedOn string `json:"modified_on"`
}

// CFCacheRule is a simplified representation of a Cloudflare cache rule
// (Rules List / Cache Rules product).
type CFCacheRule struct {
	ID               string         `json:"id,omitempty"`
	ZoneID           string         `json:"zone_id,omitempty"`
	Expression       string         `json:"expression"`
	Action           string         `json:"action"`
	ActionParameters map[string]any `json:"action_parameters,omitempty"`
	Enabled          bool           `json:"enabled"`
	Priority         int            `json:"priority"`
	Description      string         `json:"description,omitempty"`
}

// CFSSLSetting represents the SSL/TLS encryption mode for a zone.
type CFSSLSetting struct {
	ID         string `json:"id"`
	Value      string `json:"value"`
	Editable   bool   `json:"editable"`
	ModifiedOn string `json:"modified_on"`
}

// Cloudflare SSL setting values:
//   "off"     – Off
//   "flexible" – Flexible
//   "full"    – Full
//   "strict"  – Full (strict)

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

const defaultBaseURL = "https://api.cloudflare.com/client/v4"

// Client is a lightweight Cloudflare API v4 client.
type Client struct {
	apiToken   string
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Cloudflare API client.
func NewClient(apiToken string) *Client {
	return &Client{
		apiToken:   apiToken,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
	}
}

// WithHTTPClient sets a custom http.Client (useful for testing with httptest.Server).
func (c *Client) WithHTTPClient(hc *http.Client) *Client {
	c.httpClient = hc
	return c
}

// WithBaseURL overrides the base URL (useful for testing).
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = baseURL
	return c
}

// ---------------------------------------------------------------------------
// Zone operations
// ---------------------------------------------------------------------------

// ListZones returns all zones on the account. Use name query param for filtering.
func (c *Client) ListZones(ctx context.Context) ([]CFZone, error) {
	res, err := c.do(ctx, http.MethodGet, "/zones", nil)
	if err != nil {
		return nil, err
	}
	var zones []CFZone
	if err := json.Unmarshal(res, &zones); err != nil {
		return nil, fmt.Errorf("unmarshal zones: %w", err)
	}
	return zones, nil
}

// ---------------------------------------------------------------------------
// DNS record operations
// ---------------------------------------------------------------------------

// CreateDNSRecord creates a DNS record in the given zone.
func (c *Client) CreateDNSRecord(ctx context.Context, zoneID string, record CFDNSRecord) error {
	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal dns record: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/dns_records", zoneID), body)
	return err
}

// UpdateDNSRecord updates an existing DNS record.
func (c *Client) UpdateDNSRecord(ctx context.Context, zoneID, recordID string, record CFDNSRecord) error {
	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal dns record: %w", err)
	}
	_, err = c.do(ctx, http.MethodPut, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID), body)
	return err
}

// DeleteDNSRecord deletes a DNS record.
func (c *Client) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID), nil)
	return err
}

// ListDNSRecords returns all DNS records for a zone.
func (c *Client) ListDNSRecords(ctx context.Context, zoneID string) ([]CFDNSRecord, error) {
	res, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/zones/%s/dns_records", zoneID), nil)
	if err != nil {
		return nil, err
	}
	var records []CFDNSRecord
	if err := json.Unmarshal(res, &records); err != nil {
		return nil, fmt.Errorf("unmarshal dns records: %w", err)
	}
	return records, nil
}

// ---------------------------------------------------------------------------
// Cache Rules (Cloudflare Rules API)
// ---------------------------------------------------------------------------

// CreateCacheRule creates a cache rule for the zone using the Cloudflare Rules API.
func (c *Client) CreateCacheRule(ctx context.Context, zoneID string, rule CFCacheRule) error {
	body, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal cache rule: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/rulesets/cache", zoneID), body)
	return err
}

// ---------------------------------------------------------------------------
// SSL settings
// ---------------------------------------------------------------------------

// GetSSLSetting returns the current SSL/TLS encryption mode for the zone.
func (c *Client) GetSSLSetting(ctx context.Context, zoneID string) (*CFSSLSetting, error) {
	res, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/zones/%s/settings/ssl", zoneID), nil)
	if err != nil {
		return nil, err
	}
	var setting CFSSLSetting
	if err := json.Unmarshal(res, &setting); err != nil {
		return nil, fmt.Errorf("unmarshal ssl setting: %w", err)
	}
	return &setting, nil
}

// ---------------------------------------------------------------------------
// Purge (cache invalidation)
// ---------------------------------------------------------------------------

// PurgeCache purges the cache for the given zone. When urls is empty the entire
// cache is purged; otherwise only the specified URLs are purged.
func (c *Client) PurgeCache(ctx context.Context, zoneID string, urls []string) error {
	payload := map[string]any{}
	if len(urls) > 0 {
		payload["files"] = urls
	} else {
		payload["purge_everything"] = true
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal purge payload: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/purge_cache", zoneID), body)
	return err
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

// do performs an HTTP request, handles auth, and returns the raw "result"
// portion of the response.  It returns an error for non-2xx responses or when
// the API envelope indicates failure.
func (c *Client) do(ctx context.Context, method, path string, body []byte) (json.RawMessage, error) {
	u, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(raw) >= 10<<20 {
		return nil, fmt.Errorf("cloudflare response too large: %d bytes", len(raw))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(raw))
	}

	var envelope apiResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	if !envelope.Success {
		if len(envelope.Errors) > 0 {
			return nil, envelope.Errors[0]
		}
		return nil, fmt.Errorf("api returned success=false: %s", string(raw))
	}

	return envelope.Result, nil
}
