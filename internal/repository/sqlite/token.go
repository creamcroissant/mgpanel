// 文件路径: internal/repository/sqlite/token.go
// 模块说明: 这是 internal 模块里的 token 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// tokenRepo stores issued access/refresh tokens.
type tokenRepo struct {
	db *sql.DB
}

func (r *tokenRepo) Create(ctx context.Context, token *repository.AccessToken) (*repository.AccessToken, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("token 仓储未配置")
	}
	if token == nil {
		return nil, fmt.Errorf("access token 数据为空")
	}
	if token.UserID == 0 || strings.TrimSpace(token.RefreshToken) == "" {
		return nil, fmt.Errorf("userID 和 refresh token 不能为空")
	}
	now := time.Now().Unix()
	if token.CreatedAt == 0 {
		token.CreatedAt = now
	}
	if token.UpdatedAt == 0 {
		token.UpdatedAt = token.CreatedAt
	}
	const stmt = `INSERT INTO tokens(user_id, token, refresh_token, expires_at, refresh_expires_at, ip, user_agent, revoked, created_at, updated_at)
                  VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := execWithRetry(ctx, r.db,
		
		stmt,
		token.UserID,
		token.Token,
		token.RefreshToken,
		token.ExpiresAt,
		token.RefreshExpiresAt,
		nullableString(token.IP),
		nullableString(token.UserAgent),
		boolToInt(token.Revoked),
		token.CreatedAt,
		token.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if id, err := res.LastInsertId(); err == nil {
		token.ID = id
	}
	return token, nil
}

func (r *tokenRepo) FindByRefreshToken(ctx context.Context, refreshToken string) (*repository.AccessToken, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("token 仓储未配置")
	}
	trimmed := strings.TrimSpace(refreshToken)
	if trimmed == "" {
		return nil, repository.ErrNotFound
	}
	const query = `SELECT id, user_id, token, refresh_token, expires_at, refresh_expires_at, ip, user_agent, revoked, created_at, updated_at
                   FROM tokens WHERE refresh_token = ? LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, trimmed)
	var (
		rec     repository.AccessToken
		ip      sql.NullString
		ua      sql.NullString
		revoked sql.NullInt64
	)
	if err := row.Scan(
		&rec.ID,
		&rec.UserID,
		&rec.Token,
		&rec.RefreshToken,
		&rec.ExpiresAt,
		&rec.RefreshExpiresAt,
		&ip,
		&ua,
		&revoked,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if ip.Valid {
		rec.IP = ip.String
	}
	if ua.Valid {
		rec.UserAgent = ua.String
	}
	if revoked.Valid {
		rec.Revoked = revoked.Int64 == 1
	}
	return &rec, nil
}

func (r *tokenRepo) DeleteByRefreshToken(ctx context.Context, refreshToken string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("tokenRepo is not configured")
	}
	trimmed := strings.TrimSpace(refreshToken)
	if trimmed == "" {
		return nil
	}
	_, err := execWithRetry(ctx, r.db, `DELETE FROM tokens WHERE refresh_token = ?`, trimmed)
	// idempotent: no error if not found
	return err
}

func (r *tokenRepo) DeleteByUser(ctx context.Context, userID int64) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("tokenRepo is not configured")
	}
	if userID <= 0 {
		return nil
	}
	_, err := execWithRetry(ctx, r.db, `DELETE FROM tokens WHERE user_id = ?`, userID)
	// idempotent: no error if not found
	return err
}
