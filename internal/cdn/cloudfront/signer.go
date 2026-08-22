// Package cloudfront 提供 CloudFront CDN Provider 实现。
package cloudfront

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// AWS Signature V4 — minimal implementation for CloudFront API
// ---------------------------------------------------------------------------

const (
	awsService = "cloudfront"
	awsRegion  = "us-east-1" // CloudFront is a global service
)

// Credentials holds AWS access credentials for signing.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

// String returns a masked representation of Credentials for safe logging.
func (c Credentials) String() string {
	if c.SecretAccessKey == "" {
		return "Credentials{empty}"
	}
	return fmt.Sprintf("Credentials{AccessKeyID=%s, SecretAccessKey=***}", c.AccessKeyID)
}

// signer signs HTTP requests with AWS Signature V4.
type signer struct {
	cred Credentials
	now  func() time.Time // injectable for testing
}

func newSigner(cred Credentials) (*signer, error) {
	if cred.AccessKeyID == "" {
		cred.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
		if cred.AccessKeyID != "" {
			slog.Warn("cloudfront: using AWS_ACCESS_KEY_ID from env var")
		}
	}
	if cred.SecretAccessKey == "" {
		cred.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		if cred.SecretAccessKey != "" {
			slog.Warn("cloudfront: using AWS_SECRET_ACCESS_KEY from env var")
		}
	}
	if cred.AccessKeyID == "" || cred.SecretAccessKey == "" {
		return nil, errors.New("cloudfront: credentials not configured (set AccessKeyID/SecretAccessKey or AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY)")
	}
	return &signer{cred: cred, now: time.Now}, nil
}

// Sign returns the Authorization header value and the x-amz-date header value
// for a given HTTP method, URI path, query string, headers, and payload body.
func (s *signer) Sign(method, path, query string, headers map[string]string, body []byte) (authHeader, amzDate string) {
	t := s.now().UTC()
	amzDate = t.Format("20060102T150405Z")
	dateStr := t.Format("20060102")

	// Payload hash
	payloadHash := sha256Hex(body)

	// Canonical headers
	canonicalHeaders := map[string]string{}
	for k, v := range headers {
		canonicalHeaders[strings.ToLower(k)] = strings.TrimSpace(v)
	}
	canonicalHeaders["host"] = "cloudfront.amazonaws.com"
	canonicalHeaders["x-amz-date"] = amzDate

	var sortedKeys []string
	for k := range canonicalHeaders {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var headerLines strings.Builder
	var signedHeaders strings.Builder
	for i, k := range sortedKeys {
		headerLines.WriteString(k)
		headerLines.WriteString(":")
		headerLines.WriteString(canonicalHeaders[k])
		headerLines.WriteString("\n")
		if i > 0 {
			signedHeaders.WriteString(";")
		}
		signedHeaders.WriteString(k)
	}
	sh := signedHeaders.String()

	// Canonical request
	cr := strings.Join([]string{
		method,
		path,
		query,
		headerLines.String(),
		sh,
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStr, awsRegion, awsService)

	// String to sign
	sts := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(cr)),
	}, "\n")

	// Signing key
	signingKey := s.deriveKey(dateStr)

	// Signature
	sig := hmacSHA256Hex(signingKey, sts)

	auth := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cred.AccessKeyID, credentialScope, sh, sig,
	)
	return auth, amzDate
}

// deriveKey computes the AWS Signature V4 signing key.
func (s *signer) deriveKey(date string) []byte {
	kSecret := []byte("AWS4" + s.cred.SecretAccessKey)
	kDate := hmacSHA256(kSecret, date)
	kRegion := hmacSHA256(kDate, awsRegion)
	kService := hmacSHA256(kRegion, awsService)
	return hmacSHA256(kService, "aws4_request")
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

func hmacSHA256Hex(key []byte, data string) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}
