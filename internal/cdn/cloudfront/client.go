package cloudfront

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// API constants
// ---------------------------------------------------------------------------

const (
	defaultBaseURL = "https://cloudfront.amazonaws.com"
	apiVersion     = "2020-05-31"
)

// ---------------------------------------------------------------------------
// XML types — CloudFront REST API
// ---------------------------------------------------------------------------

// Distribution represents a CloudFront distribution.
type Distribution struct {
	XMLName                      xml.Name `xml:"Distribution"`
	ID                           string   `xml:"Id"`
	ARN                          string   `xml:"ARN"`
	Status                       string   `xml:"Status"`
	LastModifiedTime             string   `xml:"LastModifiedTime"`
	DomainName                   string   `xml:"DomainName"`
	InProgressInvalidationBatches int     `xml:"InProgressInvalidationBatches"`
	DistributionConfig           DistributionConfig `xml:"DistributionConfig"`
	ETag                         string   `xml:"-"` // from response header
	Enabled                      bool     `xml:"-"` // convenience from DistributionConfig
}

// DistributionSummary is a lighter representation returned in list responses.
type DistributionSummary struct {
	ID                           string   `xml:"Id"`
	ARN                          string   `xml:"ARN"`
	Status                       string   `xml:"Status"`
	LastModifiedTime             string   `xml:"LastModifiedTime"`
	DomainName                   string   `xml:"DomainName"`
	Aliases                      struct {
		Quantity int      `xml:"Quantity"`
		Items    []string `xml:"Items>CNAME,omitempty"`
	} `xml:"Aliases"`
	Origins             Origins             `xml:"Origins"`
	DefaultCacheBehavior DefaultCacheBehavior `xml:"DefaultCacheBehavior"`
	Comment             string              `xml:"Comment"`
	PriceClass          string              `xml:"PriceClass,omitempty"`
	Enabled             bool                `xml:"Enabled"`
}

func (s *DistributionSummary) toDistribution() *Distribution {
	return &Distribution{
		ID:             s.ID,
		ARN:            s.ARN,
		Status:         s.Status,
		LastModifiedTime: s.LastModifiedTime,
		DomainName:     s.DomainName,
		Enabled:        s.Enabled,
	}
}

// DistributionConfig is the configuration for a distribution.
type DistributionConfig struct {
	XMLName            xml.Name `xml:"DistributionConfig"`
	CallerReference    string   `xml:"CallerReference"`
	Aliases            struct {
		Quantity int      `xml:"Quantity"`
		Items    []string `xml:"Items>CNAME,omitempty"`
	} `xml:"Aliases"`
	DefaultRootObject   string              `xml:"DefaultRootObject,omitempty"`
	Origins             Origins             `xml:"Origins"`
	DefaultCacheBehavior DefaultCacheBehavior `xml:"DefaultCacheBehavior"`
	Comment             string              `xml:"Comment"`
	PriceClass          string              `xml:"PriceClass,omitempty"`
	Enabled             bool                `xml:"Enabled"`
}

// Origins holds a list of origins.
type Origins struct {
	Quantity int      `xml:"Quantity"`
	Items    []Origin `xml:"Items>Origin,omitempty"`
}

// Origin represents an origin server.
type Origin struct {
	ID                string `xml:"Id"`
	DomainName        string `xml:"DomainName"`
	OriginPath        string `xml:"OriginPath,omitempty"`
	CustomOriginConfig *CustomOriginConfig `xml:"CustomOriginConfig,omitempty"`
}

// CustomOriginConfig describes a custom origin (not S3).
type CustomOriginConfig struct {
	HTTPPort             int    `xml:"HTTPPort"`
	HTTPSPort            int    `xml:"HTTPSPort"`
	OriginProtocolPolicy string `xml:"OriginProtocolPolicy"`
}

// DefaultCacheBehavior is the default cache behavior.
type DefaultCacheBehavior struct {
	TargetOriginID       string `xml:"TargetOriginId"`
	ViewerProtocolPolicy string `xml:"ViewerProtocolPolicy"`
	MinTTL               int    `xml:"MinTTL"`
	DefaultTTL           int    `xml:"DefaultTTL,omitempty"`
	MaxTTL               int    `xml:"MaxTTL,omitempty"`
}

// Invalidation represents a cache invalidation request.
type Invalidation struct {
	XMLName          xml.Name          `xml:"Invalidation"`
	ID               string            `xml:"Id"`
	Status           string            `xml:"Status"`
	CreateTime       string            `xml:"CreateTime"`
	InvalidationBatch InvalidationBatch `xml:"InvalidationBatch"`
}

// InvalidationBatch contains the paths to invalidate.
type InvalidationBatch struct {
	Paths struct {
		Quantity int      `xml:"Quantity"`
		Items    []string `xml:"Items>Path,omitempty"`
	} `xml:"Paths"`
	CallerReference string `xml:"CallerReference"`
}

// ---------------------------------------------------------------------------
// CloudFront Client
// ---------------------------------------------------------------------------

// Client is a lightweight CloudFront REST API client using AWS Signature V4.
type Client struct {
	signer     *signer
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new CloudFront API client.
func NewClient(cred Credentials) (*Client, error) {
	s, err := newSigner(cred)
	if err != nil {
		return nil, err
	}
	return &Client{
		signer:     s,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
	}, nil
}

// WithHTTPClient sets a custom HTTP client (useful for testing).
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
// Distribution operations
// ---------------------------------------------------------------------------

// CreateDistributionInput contains the config for creating a distribution.
type CreateDistributionInput struct {
	Config DistributionConfig
}

// CreateDistribution creates a new CloudFront distribution.
func (c *Client) CreateDistribution(ctx context.Context, input *CreateDistributionInput) (*Distribution, error) {
	body, err := xml.Marshal(input.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal distribution config: %w", err)
	}

	raw, etag, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/%s/distribution", apiVersion), nil, body)
	if err != nil {
		return nil, err
	}

	var dist Distribution
	if err := xml.Unmarshal(raw, &dist); err != nil {
		return nil, fmt.Errorf("unmarshal distribution: %w", err)
	}
	dist.ETag = etag
	dist.Enabled = dist.DistributionConfig.Enabled
	return &dist, nil
}

// GetDistribution retrieves a distribution by ID.
func (c *Client) GetDistribution(ctx context.Context, id string) (*Distribution, error) {
	raw, etag, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/%s/distribution/%s", apiVersion, id), nil, nil)
	if err != nil {
		return nil, err
	}

	var dist Distribution
	if err := xml.Unmarshal(raw, &dist); err != nil {
		return nil, fmt.Errorf("unmarshal distribution: %w", err)
	}
	dist.ETag = etag
	dist.Enabled = dist.DistributionConfig.Enabled
	return &dist, nil
}

// DeleteDistribution deletes a distribution by ID. It first fetches the
// current ETag, then deletes with the If-Match header. Retries on ETag
// mismatch (412 PreconditionFailed) which can occur if the distribution
// is modified between the Get and Delete calls.
func (c *Client) DeleteDistribution(ctx context.Context, id string) error {
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		dist, err := c.GetDistribution(ctx, id)
		if err != nil {
			return err
		}

		_, _, err = c.doRequest(ctx, http.MethodDelete,
			fmt.Sprintf("/%s/distribution/%s", apiVersion, id),
			map[string]string{"If-Match": dist.ETag}, nil)

		if err == nil {
			return nil
		}

		// Retry on ETag mismatch (412 PreconditionFailed)
		if isETagMismatch(err) && attempt+1 < maxRetries {
			continue
		}
		return err
	}
	return fmt.Errorf("delete distribution %s: max retries exceeded", id)
}

func isETagMismatch(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status 412")
}

// ListDistributions lists all distributions.
func (c *Client) ListDistributions(ctx context.Context) ([]*Distribution, error) {
	raw, _, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/%s/distribution", apiVersion), nil, nil)
	if err != nil {
		return nil, err
	}

	// Parse using XML decoder to avoid issues with > path in nested structs.
	var summaries []*DistributionSummary
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parse distribution list XML: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "DistributionSummary" {
			var s DistributionSummary
			if err := dec.DecodeElement(&s, &se); err != nil {
				return nil, fmt.Errorf("decode DistributionSummary: %w", err)
			}
			summaries = append(summaries, &s)
		}
	}

	dists := make([]*Distribution, len(summaries))
	for i := range summaries {
		dists[i] = summaries[i].toDistribution()
	}
	return dists, nil
}

// CreateInvalidationInput contains the paths to invalidate.
type CreateInvalidationInput struct {
	DistributionID string
	Paths          []string
}

// CreateInvalidation creates a cache invalidation.
func (c *Client) CreateInvalidation(ctx context.Context, input *CreateInvalidationInput) (*Invalidation, error) {
	batch := InvalidationBatch{
		CallerReference: fmt.Sprintf("inv-%d", time.Now().UnixNano()),
	}
	batch.Paths.Quantity = len(input.Paths)
	if batch.Paths.Quantity > 0 {
		batch.Paths.Items = input.Paths
	}

	body, err := xml.Marshal(struct {
		XMLName xml.Name `xml:"InvalidationBatch"`
		InvalidationBatch
	}{InvalidationBatch: batch})
	if err != nil {
		return nil, fmt.Errorf("marshal invalidation batch: %w", err)
	}

	raw, _, err := c.doRequest(ctx, http.MethodPost,
		fmt.Sprintf("/%s/distribution/%s/invalidation", apiVersion, input.DistributionID), nil, body)
	if err != nil {
		return nil, err
	}

	var inv Invalidation
	if err := xml.Unmarshal(raw, &inv); err != nil {
		return nil, fmt.Errorf("unmarshal invalidation: %w", err)
	}
	return &inv, nil
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

// doRequest signs and sends an HTTP request to the CloudFront API.
// It returns the response body and the ETag response header.
func (c *Client) doRequest(ctx context.Context, method, path string, extraHeaders map[string]string, body []byte) ([]byte, string, error) {
	u := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}

	// Build headers for signing
	headers := map[string]string{}
	for k, v := range extraHeaders {
		headers[k] = v
	}

	authHeader, amzDate := c.signer.Sign(method, path, "", headers, body)

	req.Header.Set("Authorization", authHeader)
	req.Header.Set("x-amz-date", amzDate)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/xml")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}
	if len(raw) >= 10<<20 {
		return nil, "", fmt.Errorf("cloudfront response too large: %d bytes", len(raw))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("cloudfront API error: status %d body: %s", resp.StatusCode, string(raw))
	}

	etag := resp.Header.Get("ETag")
	return raw, etag, nil
}
