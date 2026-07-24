package meshkey

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// GenerateKeyPair generates a WireGuard-compatible key pair.
// Returns (privateKey, publicKey, error).
func GenerateKeyPair() (string, string, error) {
	var private [32]byte
	if _, err := rand.Read(private[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Clamp the private key per WireGuard spec
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64

	public, err := curve25519.X25519(private[:], curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	return base64.StdEncoding.EncodeToString(private[:]), base64.StdEncoding.EncodeToString(public), nil
}
