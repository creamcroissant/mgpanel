package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// sqliteBusyRetryLimit 是 SQLITE_BUSY / database is locked 写重试次数。
// 生产为 WAL 单写者模型，8 连接池下多写者并发时偶发锁冲突，
// busy_timeout(30s) 之外再做应用层重试，避免高频 agent 上报（traffic/
// inventory/operation log）周期性丢失或 500。
const sqliteBusyRetryLimit = 5

func isSQLiteBusyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sqlite_busy") || strings.Contains(msg, "database is locked") || strings.Contains(msg, "database table is locked")
}

// execWithRetry executes a write statement, retrying on SQLITE_BUSY.
func execWithRetry(ctx context.Context, db *sql.DB, query string, args ...any) (sql.Result, error) {
	var lastErr error
	for attempt := 0; attempt < sqliteBusyRetryLimit; attempt++ {
		result, err := db.ExecContext(ctx, query, args...)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isSQLiteBusyErr(err) {
			return result, err
		}
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
		}
	}
	return nil, lastErr
}

// txRunner runs fn inside a transaction, retrying BeginTx on SQLITE_BUSY.
func txRunner(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	var lastErr error
	for attempt := 0; attempt < sqliteBusyRetryLimit; attempt++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			lastErr = err
			if !isSQLiteBusyErr(err) {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
			}
			continue
		}
		if err := fn(tx); err != nil {
			_ = tx.Rollback()
			if isSQLiteBusyErr(err) {
				lastErr = err
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
				}
				continue
			}
			return err
		}
		if err := tx.Commit(); err != nil {
			if isSQLiteBusyErr(err) {
				lastErr = err
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
				}
				continue
			}
			return err
		}
		return nil
	}
	return lastErr
}

// beginTxWithRetry begins a transaction, retrying on SQLITE_BUSY.
func beginTxWithRetry(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	var lastErr error
	for attempt := 0; attempt < sqliteBusyRetryLimit; attempt++ {
		tx, err := db.BeginTx(ctx, nil)
		if err == nil {
			return tx, nil
		}
		lastErr = err
		if !isSQLiteBusyErr(err) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
		}
	}
	return nil, lastErr
}
