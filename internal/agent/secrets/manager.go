// Package secrets manages the agent's node-level cryptographic secrets
// (Reality X25519 keypair, short ID, self-signed TLS certificate).
//
// Rationale (see docs/plans/20260828-inbound-v2rayapi.md): node-level secrets
// must never be stored in the config-center spec — they are generated on the
// agent side, persisted to a local secrets file with restrictive permissions,
// and injected into the rendered core config at apply time.
package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/creamcroissant/mgpanel/internal/support/keygen"
)

// SecretsData holds the node-level secrets for a single agent host.
// PublicKey is derivable from PrivateKey but stored for convenience so the
// agent can report it to the panel without re-deriving.
type SecretsData struct {
	RealityPrivateKey string `json:"reality_private_key"`
	RealityPublicKey  string `json:"reality_public_key"`
	RealityShortID    string `json:"reality_short_id"`
	TLSCertPEM        string `json:"tls_cert_pem"`
	TLSKeyPEM         string `json:"tls_key_pem"`
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
	return m.data
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

// Generate produces a fresh, complete set of node-level secrets.
func Generate() (SecretsData, error) {
	priv, pub, err := keygen.GenerateX25519KeyPair()
	if err != nil {
		return SecretsData{}, fmt.Errorf("failed to generate reality keypair: %w", err)
	}
	shortID, err := keygen.GenerateShortID(DefaultShortIDLength)
	if err != nil {
		return SecretsData{}, fmt.Errorf("failed to generate reality short id: %w", err)
	}
	certPEM, keyPEM, err := keygen.GenerateSelfSignedCert("mgpanel-agent", []string{"localhost"}, 825) // 825 days ~ 2.25 years
	if err != nil {
		return SecretsData{}, fmt.Errorf("failed to generate self-signed tls cert: %w", err)
	}
	return SecretsData{
		RealityPrivateKey: priv,
		RealityPublicKey:  pub,
		RealityShortID:    shortID,
		TLSCertPEM:        certPEM,
		TLSKeyPEM:         keyPEM,
	}, nil
}

// validateSecrets ensures a loaded secrets set is complete enough to be usable.
func validateSecrets(d *SecretsData) error {
	if d.RealityPrivateKey == "" {
		return fmt.Errorf("reality_private_key is empty")
	}
	if d.RealityPublicKey == "" {
		return fmt.Errorf("reality_public_key is empty")
	}
	if d.RealityShortID == "" {
		return fmt.Errorf("reality_short_id is empty")
	}
	if d.TLSCertPEM == "" {
		return fmt.Errorf("tls_cert_pem is empty")
	}
	if d.TLSKeyPEM == "" {
		return fmt.Errorf("tls_key_pem is empty")
	}
	return nil
}