package cloudfront

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/creamcroissant/xboard/internal/cdn"
)

// ---------------------------------------------------------------------------
// Provider types
// ---------------------------------------------------------------------------

// DistributionCfg describes the desired state of a CloudFront distribution.
// This is the public config type accepted by SyncDistribution.
type DistributionCfg struct {
	// Domain is the domain name for the distribution (CNAME).
	Domain string

	// Origin is the origin server domain name.
	Origin string

	// OriginPath is an optional path prefix on the origin.
	OriginPath string

	// Comment is an optional comment for the distribution.
	Comment string

	// Enabled controls whether the distribution is enabled.
	Enabled bool
}

// DistributionStatus represents the current state of a distribution.
type DistributionStatus struct {
	ID         string
	Status     string // "Deployed" or "InProgress"
	DomainName string
	Enabled    bool
	ETag       string
}

// ---------------------------------------------------------------------------
// CloudFrontProvider implements the CDN Provider interface via CloudFront API.
// ---------------------------------------------------------------------------

// CloudFrontProvider wraps the CloudFront API client and implements cdn.Provider.
type CloudFrontProvider struct {
	client *Client
}

// NewCloudFrontProvider creates a new CloudFrontProvider.
func NewCloudFrontProvider(accessKey, secretKey string) (*CloudFrontProvider, error) {
	client, err := NewClient(Credentials{
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
	})
	if err != nil {
		return nil, err
	}
	return &CloudFrontProvider{
		client: client,
	}, nil
}

// WithHTTPClient sets a custom HTTP client on the underlying API client.
func (p *CloudFrontProvider) WithHTTPClient(hc *http.Client) *CloudFrontProvider {
	p.client.WithHTTPClient(hc)
	return p
}

// WithBaseURL sets a custom base URL on the underlying API client.
func (p *CloudFrontProvider) WithBaseURL(baseURL string) *CloudFrontProvider {
	p.client.WithBaseURL(baseURL)
	return p
}

// Name returns the provider name.
func (p *CloudFrontProvider) Name() string {
	return "cloudfront"
}

// SyncDistribution creates a CloudFront distribution from the given site config.
// site should be a *DistributionCfg. edges is ignored.
func (p *CloudFrontProvider) SyncDistribution(ctx context.Context, site interface{}, _ interface{}) (interface{}, error) {
	cfg, ok := site.(*DistributionCfg)
	if !ok {
		return nil, &cdn.ErrProviderConfig{
			Provider: "cloudfront",
			Message:  "site must be *cloudfront.DistributionCfg",
		}
	}

	// Build origin config
	origin := Origin{
		ID:         cfg.Domain,
		DomainName: cfg.Origin,
		OriginPath: cfg.OriginPath,
		CustomOriginConfig: &CustomOriginConfig{
			HTTPPort:             80,
			HTTPSPort:            443,
			OriginProtocolPolicy: "https-only",
		},
	}

	aliases := struct {
		Quantity int      `xml:"Quantity"`
		Items    []string `xml:"Items>CNAME,omitempty"`
	}{
		Quantity: 1,
		Items:    []string{cfg.Domain},
	}

	apiConfig := DistributionConfig{
		CallerReference: fmt.Sprintf("xboard-%s-%d", cfg.Domain, time.Now().UnixNano()),
		Comment:         cfg.Comment,
		Enabled:         cfg.Enabled,
		Aliases:         aliases,
		Origins: Origins{
			Quantity: 1,
			Items:    []Origin{origin},
		},
		DefaultCacheBehavior: DefaultCacheBehavior{
			TargetOriginID:       cfg.Domain,
			ViewerProtocolPolicy: "redirect-to-https",
			MinTTL:               0,
			DefaultTTL:           86400,
			MaxTTL:               31536000,
		},
	}

	dist, err := p.client.CreateDistribution(ctx, &CreateDistributionInput{
		Config: apiConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create distribution: %w", err)
	}

	return &DistributionStatus{
		ID:         dist.ID,
		Status:     dist.Status,
		DomainName: dist.DomainName,
		Enabled:    dist.Enabled,
		ETag:       dist.ETag,
	}, nil
}

// DeleteDistribution deletes a CloudFront distribution.
func (p *CloudFrontProvider) DeleteDistribution(ctx context.Context, id string) error {
	return p.client.DeleteDistribution(ctx, id)
}

// GetDistributionStatus returns the status of a distribution.
func (p *CloudFrontProvider) GetDistributionStatus(ctx context.Context, id string) (interface{}, error) {

	dist, err := p.client.GetDistribution(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get distribution: %w", err)
	}

	return &DistributionStatus{
		ID:         dist.ID,
		Status:     dist.Status,
		DomainName: dist.DomainName,
		Enabled:    dist.Enabled,
		ETag:       dist.ETag,
	}, nil
}

// SyncDNS returns ErrNotSupported — CloudFront does not manage DNS.
func (p *CloudFrontProvider) SyncDNS(_ context.Context, _ string, _ []string, _ bool) (interface{}, error) {
	return nil, cdn.ErrNotSupported
}

// SyncCacheRules returns ErrNotSupported — CloudFront cache behavior is set
// at distribution config time and updated via DistributionConfig updates.
func (p *CloudFrontProvider) SyncCacheRules(_ context.Context, _ string, _ interface{}) error {
	return cdn.ErrNotSupported
}

// Invalidate creates a CloudFront cache invalidation for the given paths.
func (p *CloudFrontProvider) Invalidate(ctx context.Context, distID string, paths []string) error {
	_, err := p.client.CreateInvalidation(ctx, &CreateInvalidationInput{
		DistributionID: distID,
		Paths:          paths,
	})
	if err != nil {
		return fmt.Errorf("create invalidation: %w", err)
	}
	return nil
}
