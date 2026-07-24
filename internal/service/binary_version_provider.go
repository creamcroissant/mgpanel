package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	githubAPICacheTTL = 5 * time.Minute
	githubAPIBaseURL  = "https://api.github.com"
)

type githubReleaseResponse struct {
	TagName string `json:"tag_name"`
}

type cachedVersion struct {
	version   string
	expiresAt time.Time
}

type gitHubVersionProvider struct {
	client    *http.Client
	repos     map[string]string // component -> owner/repo
	cache     map[string]*cachedVersion
	mu        sync.RWMutex
	now       func() time.Time
}

// NewGitHubVersionProvider creates a BinaryVersionRemoteProvider that fetches
// the latest release tag from GitHub Releases API for each component.
func NewGitHubVersionProvider() BinaryVersionRemoteProvider {
	return NewGitHubVersionProviderWithRepos(map[string]string{
		BinaryVersionComponentAgent:   "creamcroissant/xboard2p",
		BinaryVersionComponentSingBox: "SagerNet/sing-box",
		BinaryVersionComponentXray:    "XTLS/Xray-core",
	})
}

// NewGitHubVersionProviderWithRepos creates a provider with custom repos map.
// The map key is the component name (agent/sing-box/xray), value is "owner/repo".
func NewGitHubVersionProviderWithRepos(repos map[string]string) BinaryVersionRemoteProvider {
	copied := make(map[string]string, len(repos))
	for k, v := range repos {
		if strings.TrimSpace(v) != "" {
			copied[k] = strings.TrimSpace(v)
		}
	}
	return &gitHubVersionProvider{
		client: &http.Client{Timeout: 15 * time.Second},
		repos:  copied,
		cache:  make(map[string]*cachedVersion),
		now:    time.Now,
	}
}

func (p *gitHubVersionProvider) LatestVersion(ctx context.Context, component string) (string, error) {
	repo, ok := p.repos[component]
	if !ok {
		return "", fmt.Errorf("unknown component: %s", component)
	}

	// Check cache
	normalized := strings.ToLower(strings.TrimSpace(component))
	p.mu.RLock()
	cached, found := p.cache[normalized]
	p.mu.RUnlock()
	if found && p.now().Before(cached.expiresAt) {
		return cached.version, nil
	}

	// Fetch from GitHub API
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBaseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "xboard/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("GitHub API rate limited (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d for %s", resp.StatusCode, url)
	}

	var release githubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	version := strings.TrimSpace(release.TagName)
	if version == "" {
		return "", errBinaryVersionRemoteUnavailable
	}

	// Update cache
	p.mu.Lock()
	p.cache[normalized] = &cachedVersion{version: version, expiresAt: p.now().Add(githubAPICacheTTL)}
	p.mu.Unlock()

	return version, nil
}
