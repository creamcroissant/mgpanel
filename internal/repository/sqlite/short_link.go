package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/creamcroissant/xboard/internal/repository"
)

type shortLinkRepo struct {
	db *sql.DB
}

// NewShortLinkRepository creates a new SQLite-backed short link repository.
func NewShortLinkRepository(db *sql.DB) repository.ShortLinkRepository {
	return &shortLinkRepo{db: db}
}

func (r *shortLinkRepo) Create(ctx context.Context, link *repository.ShortLink) error {
	query := `
		INSERT INTO short_links (code, user_id, target_path, custom_params, expires_at, access_count, last_accessed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		link.Code,
		link.UserID,
		link.TargetPath,
		nullString(link.CustomParams),
		nullInt64(link.ExpiresAt),
		link.AccessCount,
		nullInt64(link.LastAccessedAt),
		link.CreatedAt,
		link.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	link.ID = id
	return nil
}

func (r *shortLinkRepo) FindByCode(ctx context.Context, code string) (*repository.ShortLink, error) {
	query := `
		SELECT id, code, user_id, target_path, custom_params, expires_at, access_count, last_accessed_at, created_at, updated_at
		FROM short_links
		WHERE code = ?
	`
	link := &repository.ShortLink{}
	var customParams sql.NullString
	var expiresAtInt, lastAccessedAtInt sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&link.ID,
		&link.Code,
		&link.UserID,
		&link.TargetPath,
		&customParams,
		&expiresAtInt,
		&link.AccessCount,
		&lastAccessedAtInt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	if customParams.Valid {
		link.CustomParams = customParams.String
	}
	if expiresAtInt.Valid {
		link.ExpiresAt = expiresAtInt.Int64
	}
	if lastAccessedAtInt.Valid {
		link.LastAccessedAt = lastAccessedAtInt.Int64
	}

	return link, nil
}

func (r *shortLinkRepo) FindByID(ctx context.Context, id int64) (*repository.ShortLink, error) {
	query := `
		SELECT id, code, user_id, target_path, custom_params, expires_at, access_count, last_accessed_at, created_at, updated_at
		FROM short_links
		WHERE id = ?
	`
	link := &repository.ShortLink{}
	var customParams sql.NullString
	var expiresAtInt, lastAccessedAtInt sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&link.ID,
		&link.Code,
		&link.UserID,
		&link.TargetPath,
		&customParams,
		&expiresAtInt,
		&link.AccessCount,
		&lastAccessedAtInt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	if customParams.Valid {
		link.CustomParams = customParams.String
	}
	if expiresAtInt.Valid {
		link.ExpiresAt = expiresAtInt.Int64
	}
	if lastAccessedAtInt.Valid {
		link.LastAccessedAt = lastAccessedAtInt.Int64
	}

	return link, nil
}

func (r *shortLinkRepo) FindByUserID(ctx context.Context, userID int64) ([]*repository.ShortLink, error) {
	query := `
		SELECT id, code, user_id, target_path, custom_params, expires_at, access_count, last_accessed_at, created_at, updated_at
		FROM short_links
		WHERE user_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*repository.ShortLink
	for rows.Next() {
		link := &repository.ShortLink{}
		var customParams sql.NullString
		var expiresAtInt, lastAccessedAtInt sql.NullInt64

		err := rows.Scan(
			&link.ID,
			&link.Code,
			&link.UserID,
			&link.TargetPath,
			&customParams,
			&expiresAtInt,
			&link.AccessCount,
			&lastAccessedAtInt,
			&link.CreatedAt,
			&link.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if customParams.Valid {
			link.CustomParams = customParams.String
		}
		if expiresAtInt.Valid {
			link.ExpiresAt = expiresAtInt.Int64
		}
		if lastAccessedAtInt.Valid {
			link.LastAccessedAt = lastAccessedAtInt.Int64
		}

		links = append(links, link)
	}

	return links, rows.Err()
}

func (r *shortLinkRepo) Update(ctx context.Context, link *repository.ShortLink) error {
	query := `
		UPDATE short_links
		SET target_path = ?, custom_params = ?, expires_at = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := r.db.ExecContext(ctx, query,
		link.TargetPath,
		nullString(link.CustomParams),
		nullInt64(link.ExpiresAt),
		link.UpdatedAt,
		link.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *shortLinkRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM short_links WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *shortLinkRepo) DeleteByUserID(ctx context.Context, userID int64) error {
	query := `DELETE FROM short_links WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, userID) // idempotent: no error if not found
	return err
}

func (r *shortLinkRepo) IncrementAccessCount(ctx context.Context, id int64, accessTime int64) error {
	query := `
		UPDATE short_links
		SET access_count = access_count + 1, last_accessed_at = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query, accessTime, accessTime, id)
	return err
}

func (r *shortLinkRepo) CodeExists(ctx context.Context, code string) (bool, error) {
	query := `SELECT 1 FROM short_links WHERE code = ? LIMIT 1`
	var exists int
	err := r.db.QueryRowContext(ctx, query, code).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Helper functions
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}
