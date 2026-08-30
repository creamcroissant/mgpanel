package bootstrap

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type JWTSigningKeySource string

const (
	defaultJWTSigningKey    = "change-me"
	jwtSigningKeySettingKey = "auth_signing_key"
	jwtSigningKeyCategory   = "security"
	jwtSigningKeyBytes      = 32

	JWTSigningKeySourceConfig    JWTSigningKeySource = "config"
	JWTSigningKeySourceSettings  JWTSigningKeySource = "settings"
	JWTSigningKeySourceGenerated JWTSigningKeySource = "generated"
)

type jwtSigningKeyDeps struct {
	now        func() time.Time
	randReader io.Reader
}

// ResolveJWTSigningKey resolves JWT signing key with priority:
// config/env > settings > generate-and-persist.
func ResolveJWTSigningKey(ctx context.Context, db *sql.DB, configuredKey string, now func() time.Time) (string, JWTSigningKeySource, error) {
	return resolveJWTSigningKey(ctx, db, configuredKey, jwtSigningKeyDeps{
		now:        now,
		randReader: rand.Reader,
	})
}

func resolveJWTSigningKey(ctx context.Context, db *sql.DB, configuredKey string, deps jwtSigningKeyDeps) (string, JWTSigningKeySource, error) {
	normalizedConfiguredKey := strings.TrimSpace(configuredKey)
	if normalizedConfiguredKey != "" && normalizedConfiguredKey != defaultJWTSigningKey {
		return normalizedConfiguredKey, JWTSigningKeySourceConfig, nil
	}

	if db == nil {
		return "", "", fmt.Errorf("resolve jwt signing key: db is required when auth.signing_key uses default value; you can set XBOARD_AUTH_SIGNING_KEY")
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.randReader == nil {
		deps.randReader = rand.Reader
	}

	existingKey, err := readJWTSigningKeyFromSettings(ctx, db)
	if err != nil {
		return "", "", fmt.Errorf("read jwt signing key from settings: %w; you can set XBOARD_AUTH_SIGNING_KEY", err)
	}
	if existingKey != "" {
		return existingKey, JWTSigningKeySourceSettings, nil
	}

	generatedKey, err := generateJWTSigningKey(deps.randReader)
	if err != nil {
		return "", "", fmt.Errorf("generate jwt signing key: %w; you can set XBOARD_AUTH_SIGNING_KEY", err)
	}

	if err := insertJWTSigningKeyIfMissing(ctx, db, generatedKey, deps.now().Unix()); err != nil {
		return "", "", fmt.Errorf("persist jwt signing key to settings: %w; you can set XBOARD_AUTH_SIGNING_KEY", err)
	}

	resolvedKey, err := readJWTSigningKeyFromSettings(ctx, db)
	if err != nil {
		return "", "", fmt.Errorf("read jwt signing key after persistence: %w; you can set XBOARD_AUTH_SIGNING_KEY", err)
	}
	if resolvedKey == "" {
		return "", "", fmt.Errorf("jwt signing key not found after persistence; you can set XBOARD_AUTH_SIGNING_KEY")
	}

	if resolvedKey == generatedKey {
		return resolvedKey, JWTSigningKeySourceGenerated, nil
	}
	return resolvedKey, JWTSigningKeySourceSettings, nil
}

func readJWTSigningKeyFromSettings(ctx context.Context, db *sql.DB) (string, error) {
	const query = `SELECT value FROM settings WHERE key = ?`

	var value string
	if err := db.QueryRowContext(ctx, query, jwtSigningKeySettingKey).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(value), nil
}

func insertJWTSigningKeyIfMissing(ctx context.Context, db *sql.DB, key string, updatedAt int64) error {
	const statement = `INSERT INTO settings(key, value, category, updated_at) VALUES(?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, category = excluded.category, updated_at = excluded.updated_at
		WHERE TRIM(settings.value) = ''`
	_, err := db.ExecContext(ctx, statement, jwtSigningKeySettingKey, key, jwtSigningKeyCategory, updatedAt)
	return err
}

func generateJWTSigningKey(reader io.Reader) (string, error) {
	bytes := make([]byte, jwtSigningKeyBytes)
	if _, err := io.ReadFull(reader, bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
