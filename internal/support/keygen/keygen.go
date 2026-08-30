// Package keygen provides cryptographically-random key generation helpers
// used by the agent to produce node-level secrets on-device (Reality X25519
// keypairs, short IDs, passwords, and self-signed TLS certificates).
//
// Rationale (see docs/plans/20260828-inbound-v2rayapi.md): node-level secrets
// must never live in the config-center spec — they are generated on the agent
// side and persisted to a local secrets file instead.
package keygen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"golang.org/x/crypto/curve25519"
)

// GenerateX25519KeyPair generates a WireGuard-compatible X25519 key pair.
// It returns (privateKey, publicKey) both base64-std-encoded.
//
// The algorithm mirrors internal/service/meshkey.GenerateKeyPair (clamping per
// the WireGuard spec); it is intentionally duplicated here so that this low-level
// support package does not depend on the higher-level service layer.
func GenerateX25519KeyPair() (privateKey, publicKey string, err error) {
	var private [32]byte
	if _, err := rand.Read(private[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Clamp the private key per WireGuard spec.
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64

	public, err := curve25519.X25519(private[:], curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	return base64.StdEncoding.EncodeToString(private[:]),
		base64.StdEncoding.EncodeToString(public), nil
}

// GenerateShortID generates a random hex string of exactly n characters
// (n/2 random bytes), for use as a Reality short ID. n must be an even
// positive number; the resulting hex string is lowercase.
func GenerateShortID(n int) (string, error) {
	if n <= 0 || n%2 != 0 {
		return "", fmt.Errorf("short id length must be a positive even number, got %d", n)
	}
	buf := make([]byte, n/2)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate short id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// GeneratePassword generates a random URL-safe base64 string from n bytes of
// randomness. Callers that need a specific character count should pick n
// accordingly (base64 output length = ceil(n/3)*4).
func GeneratePassword(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("password byte length must be positive, got %d", n)
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// GenerateSelfSignedCert generates a self-signed ECDSA P-256 TLS certificate
// (PEM-encoded) valid for the given number of days, with the given Common Name
// and DNS SANs. It returns (certPEM, keyPEM).
func GenerateSelfSignedCert(commonName string, dnsNames []string, days int) (certPEM, keyPEM string, err error) {
	if days <= 0 {
		return "", "", fmt.Errorf("certificate validity days must be positive, got %d", days)
	}
	if commonName == "" && len(dnsNames) == 0 {
		return "", "", fmt.Errorf("certificate requires a common name or dns names")
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate ecdsa key: %w", err)
	}

	serial, err := randomSerialNumber()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	notBefore := time.Now().Add(-time.Hour)
	notAfter := notBefore.Add(time.Duration(days) * 24 * time.Hour)

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              append([]string(nil), dnsNames...),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal ecdsa key: %w", err)
	}
	keyPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return string(certPEMBytes), string(keyPEMBytes), nil
}

// randomSerialNumber generates a random 128-bit serial number for a certificate.
func randomSerialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}