// Package security 提供 AES-256-GCM 加密/解密工具。
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidKeyLength = errors.New("security: encryption key must be 16, 24, or 32 bytes")
	ErrCiphertextTooShort = errors.New("security: ciphertext too short")
)

// Encrypt 使用 AES-256-GCM 加密明文，返回 Base64 编码的密文。
// key 必须是 16/24/32 字节（AES-128/192/256）。
// 若 key 为 64 字符 hex 字符串，会自动解码为 32 字节。
func Encrypt(plaintext []byte, key []byte) (string, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 Base64 编码的 AES-256-GCM 密文。
func Decrypt(encoded string, key []byte) ([]byte, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// normalizeKey 将密钥标准化为 []byte。
// 如果 key 是 64 字符 hex 编码，解码为 32 字节；
// 如果是 16/24/32 bytes，直接使用。
func normalizeKey(key []byte) ([]byte, error) {
	// hex 编码的 32 字节 key（64 hex 字符）
	if len(key) == 64 {
		decoded := make([]byte, 32)
		if _, err := hex.Decode(decoded, key); err == nil {
			return decoded, nil
		}
	}
	// 已经是 16/24/32 字节的二进制 key
	if len(key) == 16 || len(key) == 24 || len(key) == 32 {
		return key, nil
	}
	return nil, ErrInvalidKeyLength
}
