package mesh

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PeerConfig holds the configuration for a single WireGuard peer.
type PeerConfig struct {
	ID         string
	PublicKey  string
	Endpoint   string
	AllowedIPs []string
	Keepalive  int
}

// LocalConfig holds the local WireGuard interface configuration.
type LocalConfig struct {
	PrivateKey string
	Address    string
	ListenPort int
}

// ConfigManager manages the tinc-style configuration directory for WireGuard.
type ConfigManager struct {
	configDir string
}

// NewConfigManager creates a new ConfigManager with the given configuration directory.
func NewConfigManager(configDir string) *ConfigManager {
	return &ConfigManager{configDir: configDir}
}

// EnsureDirs creates the configuration directory and peers subdirectory.
func (cm *ConfigManager) EnsureDirs() error {
	if err := os.MkdirAll(cm.configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", cm.configDir, err)
	}
	peersDir := filepath.Join(cm.configDir, "peers")
	if err := os.MkdirAll(peersDir, 0o755); err != nil {
		return fmt.Errorf("create peers dir %s: %w", peersDir, err)
	}
	return nil
}

// WriteLocalConfig writes the local WireGuard interface configuration file.
func (cm *ConfigManager) WriteLocalConfig(cfg LocalConfig) error {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	if cfg.PrivateKey != "" {
		b.WriteString(fmt.Sprintf("PrivateKey = %s\n", cfg.PrivateKey))
	}
	if cfg.Address != "" {
		b.WriteString(fmt.Sprintf("Address = %s\n", cfg.Address))
	}
	if cfg.ListenPort > 0 {
		b.WriteString(fmt.Sprintf("ListenPort = %d\n", cfg.ListenPort))
	}

	path := filepath.Join(cm.configDir, "wg.conf")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write local config %s: %w", path, err)
	}
	return nil
}

// WritePeerConfig writes a peer configuration file.
func (cm *ConfigManager) WritePeerConfig(peer PeerConfig) error {
	var b strings.Builder
	b.WriteString("[Peer]\n")
	b.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))
	if peer.Endpoint != "" {
		b.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint))
	}
	if len(peer.AllowedIPs) > 0 {
		b.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(peer.AllowedIPs, ",")))
	}
	if peer.Keepalive > 0 {
		b.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.Keepalive))
	}

	path := filepath.Join(cm.configDir, "peers", peer.ID+".conf")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write peer config %s: %w", path, err)
	}
	return nil
}

// ReadAllPeers reads all peer configuration files from the peers directory.
func (cm *ConfigManager) ReadAllPeers() ([]PeerConfig, error) {
	peersDir := filepath.Join(cm.configDir, "peers")
	entries, err := os.ReadDir(peersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read peers dir %s: %w", peersDir, err)
	}

	var peers []PeerConfig
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".conf")
		data, err := os.ReadFile(filepath.Join(peersDir, entry.Name()))
		if err != nil {
			continue
		}
		peer := parsePeerConfig(string(data))
		peer.ID = id
		peers = append(peers, peer)
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].ID < peers[j].ID
	})
	return peers, nil
}

// RemovePeerConfig removes a peer configuration file.
func (cm *ConfigManager) RemovePeerConfig(peerID string) error {
	path := filepath.Join(cm.configDir, "peers", peerID+".conf")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove peer config %s: %w", path, err)
	}
	return nil
}

// RemoveLocalConfig removes the local WireGuard interface configuration file.
func (cm *ConfigManager) RemoveLocalConfig() error {
	path := filepath.Join(cm.configDir, "wg.conf")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove local config %s: %w", path, err)
	}
	return nil
}

// ReadLocalConfig reads the local WireGuard interface configuration file.
func (cm *ConfigManager) ReadLocalConfig() (*LocalConfig, error) {
	path := filepath.Join(cm.configDir, "wg.conf")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read local config %s: %w", path, err)
	}
	return parseLocalConfig(string(data)), nil
}

// parsePeerConfig parses a [Peer] section from configuration content.
func parsePeerConfig(content string) PeerConfig {
	var peer PeerConfig
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "PublicKey":
			peer.PublicKey = value
		case "Endpoint":
			peer.Endpoint = value
		case "AllowedIPs":
			peer.AllowedIPs = strings.Split(value, ",")
		case "PersistentKeepalive":
			fmt.Sscanf(value, "%d", &peer.Keepalive)
		}
	}
	return peer
}

// parseLocalConfig parses an [Interface] section from configuration content.
func parseLocalConfig(content string) *LocalConfig {
	cfg := &LocalConfig{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "PrivateKey":
			cfg.PrivateKey = value
		case "Address":
			cfg.Address = value
		case "ListenPort":
			fmt.Sscanf(value, "%d", &cfg.ListenPort)
		}
	}
	return cfg
}
