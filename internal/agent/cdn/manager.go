package cdn

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/creamcroissant/xboard/internal/agent/config"
	corecdn "github.com/creamcroissant/xboard/internal/cdn"
)

// Manager manages CDN sites and the Caddy reverse-proxy process.
//
// Site lifecycle:
//  1. DeploySite  - generates site config, writes sites/{domain}.caddy,
//     rebuilds the main Caddyfile from all managed sites, reloads Caddy.
//  2. RemoveSite  - deletes sites/{domain}.caddy, rebuilds the main
//     Caddyfile, reloads Caddy.
//  3. ListSites   - returns a snapshot of all managed CDNSiteConfig.
//
// On Start, the manager ensures the config directory and sites/ subdirectory
// exist, writes an initial empty Caddyfile if none exists, then launches the
// Caddy process.  Reloads are skipped when Caddy is not running.
//
// Install methods (Install, EnsureInstalled) use a CaddyInstaller to
// download and verify the Caddy binary.
type Manager struct {
	process   *CaddyProcess
	configDir string
	binPath   string
	builder   *CaddyfileBuilder
	sites     map[string]*CDNSiteConfig
	installer *CaddyInstaller
	mu        sync.RWMutex
}

// NewManager creates a Manager with the given caddy binary path and config
// directory.  If configDir is empty, /opt/work/xboard/caddy/ is used.
func NewManager(binPath, configDir string) *Manager {
	if configDir == "" {
		configDir = "/opt/work/xboard/caddy/"
	}
	installer := NewCaddyInstaller()
	installer.binDir = filepath.Dir(binPath)
	installer.binPath = binPath

	return &Manager{
		process:   NewCaddyProcess(binPath, configDir),
		configDir: configDir,
		binPath:   binPath,
		builder:   &CaddyfileBuilder{},
		sites:     make(map[string]*CDNSiteConfig),
		installer: installer,
	}
}

// NewManagerFromConfig creates a Manager from a CDNConfig.
func NewManagerFromConfig(cfg config.CDNConfig) *Manager {
	return NewManager(cfg.BinPath, cfg.ConfigDir)
}

// Init creates the config directory and the sites/ subdirectory.
func (m *Manager) Init(ctx context.Context) error {
	dirs := []string{
		m.configDir,
		filepath.Join(m.configDir, "sites"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	_ = ctx // satisfied interface shape
	return nil
}

// Start initialises the directory structure, writes an initial empty Caddyfile
// if one does not already exist, and starts the Caddy process.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.Init(ctx); err != nil {
		return fmt.Errorf("manager init: %w", err)
	}

	// Scan sites/ directory to restore in-memory site map on restart.
	sitesDir := filepath.Join(m.configDir, "sites")
	if entries, err := os.ReadDir(sitesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".caddy") {
				continue
			}
			domain := strings.TrimSuffix(entry.Name(), ".caddy")
			m.mu.Lock()
			m.sites[domain] = &CDNSiteConfig{Domain: domain}
			m.mu.Unlock()
		}
		slog.Info("restored sites from disk", "count", len(m.sites))
	}

	mainFile := filepath.Join(m.configDir, "Caddyfile")
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		empty, buildErr := m.builder.BuildSites(nil)
		if buildErr != nil {
			return fmt.Errorf("build initial caddyfile: %w", buildErr)
		}
		if err := os.WriteFile(mainFile, empty, 0o644); err != nil {
			return fmt.Errorf("write initial caddyfile: %w", err)
		}
	}

	return m.process.Start(ctx)
}

// Stop gracefully stops the Caddy process.
func (m *Manager) Stop(ctx context.Context) error {
	return m.process.Stop(ctx)
}

// IsRunning reports whether the Caddy process is currently running.
func (m *Manager) IsRunning() bool {
	return m.process.Running()
}

// DeploySite deploys a CDN site: generates the Caddyfile for the site, writes
// sites/{domain}.caddy, rebuilds the main Caddyfile from all managed sites,
// and reloads Caddy if it is running.
func (m *Manager) DeploySite(ctx context.Context, site *CDNSiteConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := validateCaddySite(site); err != nil {
		return err
	}

	// Write individual site fragment.
	siteFile := filepath.Join(m.configDir, "sites", site.Domain+".caddy")
	if err := os.WriteFile(siteFile, m.buildSiteBlock(site), 0o644); err != nil {
		return fmt.Errorf("write site file %s: %w", site.Domain, err)
	}

	m.sites[site.Domain] = site

	if err := m.writeMainCaddyfile(); err != nil {
		return fmt.Errorf("write main caddyfile: %w", err)
	}

	if err := m.reload(ctx); err != nil {
		return fmt.Errorf("reload caddy: %w", err)
	}

	return nil
}

// RemoveSite removes the CDN site for the given domain: deletes
// sites/{domain}.caddy, rebuilds the main Caddyfile from remaining sites,
// and reloads Caddy if it is running.  It is not an error if the site file
// does not exist or the domain is unknown.
func (m *Manager) RemoveSite(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := corecdn.ValidateCaddySiteDomain(domain); err != nil {
		return err
	}

	// Delete the individual site file (if it exists).
	siteFile := filepath.Join(m.configDir, "sites", domain+".caddy")
	if err := os.Remove(siteFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove site file %s: %w", domain, err)
	}

	delete(m.sites, domain)

	if err := m.writeMainCaddyfile(); err != nil {
		return fmt.Errorf("write main caddyfile: %w", err)
	}

	if err := m.reload(ctx); err != nil {
		return fmt.Errorf("reload caddy: %w", err)
	}

	return nil
}

// ListSites returns a snapshot of all managed site configurations.
func (m *Manager) ListSites() []CDNSiteConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]CDNSiteConfig, 0, len(m.sites))
	for _, s := range m.sites {
		result = append(result, *s)
	}
	return result
}

// EnsureInstalled installs the Caddy binary if not already present.
func (m *Manager) EnsureInstalled(ctx context.Context) error {
	return m.installer.Install(ctx)
}

// Install installs Caddy with an optional custom download URL.
// If downloadURL is empty, the default release URL is used.
func (m *Manager) Install(ctx context.Context, downloadURL string) error {
	if downloadURL != "" {
		m.installer.SetDownloadURL(downloadURL)
	}
	return m.installer.Install(ctx)
}

// Reload reloads the current Caddyfile via the admin API.
// If Caddy is not running it will be started first.
func (m *Manager) Reload(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	caddyfilePath := filepath.Join(m.configDir, "Caddyfile")
	data, err := os.ReadFile(caddyfilePath)
	if err != nil {
		return fmt.Errorf("read caddyfile for reload: %w", err)
	}

	if m.process.Running() {
		return m.process.Reload(ctx, data)
	}
	return m.process.Start(ctx)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildSiteBlock generates the Caddy site block for a single site configuration.
func (m *Manager) buildSiteBlock(site *CDNSiteConfig) []byte {
	var buf bytes.Buffer
	writeSiteBlock(&buf, site)
	return buf.Bytes()
}

// writeMainCaddyfile rebuilds the main Caddyfile from the in-memory sites map.
// The caller must hold m.mu (at least for reads on the map).
func (m *Manager) writeMainCaddyfile() error {
	all := make([]*CDNSiteConfig, 0, len(m.sites))
	for _, s := range m.sites {
		all = append(all, s)
	}

	cfg, err := m.builder.BuildSites(all)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.configDir, "Caddyfile"), cfg, 0o644)
}

// reload triggers a Caddy hot reload by reading the main Caddyfile from disk
// and POSTing it to the admin /load endpoint.  It is a no-op when Caddy is
// not running.
func (m *Manager) reload(ctx context.Context) error {
	if !m.process.Running() {
		slog.Debug("caddy not running, skipping reload")
		return nil
	}
	mainFile := filepath.Join(m.configDir, "Caddyfile")
	data, err := os.ReadFile(mainFile)
	if err != nil {
		return fmt.Errorf("read caddyfile for reload: %w", err)
	}
	return m.process.Reload(ctx, data)
}
