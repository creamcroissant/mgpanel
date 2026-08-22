package bootstrap

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteBusyRetryLimit = 3

// ResolveSQLitePath normalizes the SQLite file path to an absolute path.
func ResolveSQLitePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("SQLite 路径不能为空 / SQLite path is required")
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve sqlite path: %w", err)
	}
	return filepath.Clean(absPath), nil
}

// OpenSQLite ensures the parent directory exists, then opens a SQLite connection with sane pragmas.
func OpenSQLite(path string) (*sql.DB, error) {
	resolvedPath, err := ResolveSQLitePath(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_busy_timeout=30000&_journal_mode=WAL", resolvedPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err := WithSQLiteBusyRetry(func() error {
		_, err := db.Exec("PRAGMA journal_mode=WAL;")
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("set wal mode: %w", err)
	}
	if err := WithSQLiteBusyRetry(func() error {
		_, err := db.Exec("PRAGMA busy_timeout=30000;")
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

func WithSQLiteBusyRetry(fn func() error) error {
	if fn == nil {
		return nil
	}
	var lastErr error
	for attempt := 0; attempt < sqliteBusyRetryLimit; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			if !isSQLiteBusyError(err) {
				return err
			}
			time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "sqlite_busy") || strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked")
}
