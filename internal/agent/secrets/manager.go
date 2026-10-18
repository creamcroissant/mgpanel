// Package secrets manages the agent's node-local cryptographic secrets.
//
// 设计（2026-10-03 收紧）：Reality 密钥不再由面板 spec 下发、也不在机器间复用，
// 而是由 agent 按「配置（inbound tag）」在本地独立生成并持久化（0600），
// 下发配置写盘前注入。私钥永不出机器；面板通过 agent 上报拿到对应公钥生成订阅。
// 这样一台机器的一个配置泄露，不会影响其它机器、其它配置。
package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/creamcroissant/mgpanel/internal/support/keygen"
)

// RealitySecret is one inbound's Reality credentials (X25519 keypair + short id).
type RealitySecret struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	ShortID    string `json:"short_id"`
}

// SecretsData holds the agent-local secrets for a single machine.
type SecretsData struct {
	TLSCertPEM string `json:"tls_cert_pem"`
	TLSKeyPEM  string `json:"tls_key_pem"`
	// Inbounds 按 inbound tag 索引：每个配置一套独立 Reality 密钥，互不复用。
	Inbounds map[string]RealitySecret `json:"inbounds,omitempty"`
}

// DefaultShortIDLength is the number of hex characters for a Reality short ID.
const DefaultShortIDLength = 8

// Manager loads and serves the node-level secrets file. It is safe for
// concurrent use: Get is guarded by an RWMutex and LoadOrCreate writes once.
type Manager struct {
	path string
	mu   sync.RWMutex
	data SecretsData
}

// NewManager returns a Manager for the given secrets file path.
func NewManager(path string) *Manager {
	return &Manager{path: path}
}

// Path returns the secrets file path this manager operates on.
func (m *Manager) Path() string {
	return m.path
}

// LoadOrCreate loads the secrets file if it exists; otherwise it generates a
// fresh set of secrets and persists them to the file with 0600 permissions.
// The parent directory is created with 0700 if missing.
func (m *Manager) LoadOrCreate() error {
	if m == nil {
		return fmt.Errorf("secrets manager is nil")
	}
	if m.path == "" {
		return fmt.Errorf("secrets file path is empty")
	}

	if data, err := os.ReadFile(m.path); err == nil {
		var loaded SecretsData
		if err := json.Unmarshal(data, &loaded); err != nil {
			return fmt.Errorf("failed to parse secrets file %s: %w", m.path, err)
		}
		if err := validateSecrets(&loaded); err != nil {
			return fmt.Errorf("secrets file %s is incomplete: %w", m.path, err)
		}
		m.mu.Lock()
		m.data = loaded
		m.mu.Unlock()
		return nil
	}
	// Fall through to generation when the file does not exist. Other read
	// errors (permissions) are also treated as "regenerate", which surfaces
	// any permission issue at write time with a clear message.

	generated, err := Generate()
	if err != nil {
		return err
	}
	if err := m.save(generated); err != nil {
		return err
	}
	m.mu.Lock()
	m.data = generated
	m.mu.Unlock()
	return nil
}

// Get returns a copy of the currently loaded secrets.
func (m *Manager) Get() SecretsData {
	if m == nil {
		return SecretsData{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSecrets(m.data)
}

// RealityFor returns the Reality credentials for one inbound tag, generating and
// persisting a fresh keypair on first use. Same tag always returns the same
// keypair (so client public keys stay stable); different tags never share keys.
func (m *Manager) RealityFor(tag string) (RealitySecret, error) {
	if m == nil {
		return RealitySecret{}, fmt.Errorf("secrets manager is nil")
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return RealitySecret{}, fmt.Errorf("inbound tag is empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.data.Inbounds[tag]; ok && existing.PrivateKey != "" {
		return existing, nil
	}

	secret, err := GenerateRealitySecret()
	if err != nil {
		return RealitySecret{}, err
	}
	if m.data.Inbounds == nil {
		m.data.Inbounds = make(map[string]RealitySecret)
	}
	m.data.Inbounds[tag] = secret
	if err := m.save(m.data); err != nil {
		delete(m.data.Inbounds, tag)
		return RealitySecret{}, err
	}
	return secret, nil
}

// cloneSecrets deep-copies the secrets so callers cannot mutate manager state.
func cloneSecrets(data SecretsData) SecretsData {
	out := data
	if len(data.Inbounds) == 0 {
		return out
	}
	out.Inbounds = make(map[string]RealitySecret, len(data.Inbounds))
	for tag, secret := range data.Inbounds {
		out.Inbounds[tag] = secret
	}
	return out
}

// save writes the given secrets to disk with 0600 permissions, creating the
// parent directory (0700) if it does not exist.
func (m *Manager) save(data SecretsData) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("failed to create secrets directory %s: %w", filepath.Dir(m.path), err)
	}
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}
	if err := os.WriteFile(m.path, payload, 0o600); err != nil {
		return fmt.Errorf("failed to write secrets file %s: %w", m.path, err)
	}
	return nil
}

// Generate produces a fresh set of machine-level secrets (TLS certificate).
// Reality keypairs are generated lazily per inbound tag via RealityFor.
func Generate() (SecretsData, error) {
	certPEM, keyPEM, err := keygen.GenerateSelfSignedCert("mgpanel-agent", []string{"localhost"}, 825) // 825 days ~ 2.25 years
	if err != nil {
		return SecretsData{}, fmt.Errorf("failed to generate self-signed tls cert: %w", err)
	}
	return SecretsData{
		TLSCertPEM: certPEM,
		TLSKeyPEM:  keyPEM,
		Inbounds:   map[string]RealitySecret{},
	}, nil
}

// GenerateRealitySecret produces one independent Reality keypair + short id.
// The keypair uses Reality's own wire format (base64url, no padding), which both
// xray (privateKey) and sing-box (tls.reality.private_key) accept verbatim.
func GenerateRealitySecret() (RealitySecret, error) {
	priv, pub, err := keygen.GenerateRealityKeyPair()
	if err != nil {
		return RealitySecret{}, fmt.Errorf("failed to generate reality keypair: %w", err)
	}
	shortID, err := keygen.GenerateShortID(DefaultShortIDLength)
	if err != nil {
		return RealitySecret{}, fmt.Errorf("failed to generate reality short id: %w", err)
	}
	return RealitySecret{PrivateKey: priv, PublicKey: pub, ShortID: shortID}, nil
}

// validateSecrets ensures a loaded secrets set is complete enough to be usable.
func validateSecrets(d *SecretsData) error {
	if d.TLSCertPEM == "" {
		return fmt.Errorf("tls_cert_pem is empty")
	}
	if d.TLSKeyPEM == "" {
		return fmt.Errorf("tls_key_pem is empty")
	}
	for tag, secret := range d.Inbounds {
		if strings.TrimSpace(tag) == "" || secret.PrivateKey == "" || secret.PublicKey == "" || secret.ShortID == "" {
			return fmt.Errorf("inbound secret %q is incomplete", tag)
		}
	}
	return nil
}
