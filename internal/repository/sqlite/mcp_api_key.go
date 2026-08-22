// 文件路径: internal/repository/sqlite/mcp_api_key.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type mcpApiKeyRepo struct {
	db *sql.DB
}

func newMCPApiKeyRepo(db *sql.DB) *mcpApiKeyRepo {
	return &mcpApiKeyRepo{db: db}
}

func (r *mcpApiKeyRepo) Create(ctx context.Context, key *repository.MCPApiKey) error {
	query := `INSERT INTO mcp_api_keys (name, prefix, key_hash, enabled, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, strftime('%s','now'), strftime('%s','now'))`
	result, err := r.db.ExecContext(ctx, query, key.Name, key.Prefix, key.KeyHash, boolToInt(key.Enabled), key.CreatedBy)
	if err != nil {
		return fmt.Errorf("create mcp api key: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	key.ID = id
	return nil
}

func (r *mcpApiKeyRepo) GetByID(ctx context.Context, id int64) (*repository.MCPApiKey, error) {
	query := `SELECT id, name, prefix, key_hash, enabled, last_used_at, created_by, created_at, updated_at
		FROM mcp_api_keys WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanMCPApiKey(row)
}

func (r *mcpApiKeyRepo) GetByPrefix(ctx context.Context, prefix string) (*repository.MCPApiKey, error) {
	query := `SELECT id, name, prefix, key_hash, enabled, last_used_at, created_by, created_at, updated_at
		FROM mcp_api_keys WHERE prefix = ?`
	row := r.db.QueryRowContext(ctx, query, prefix)
	return scanMCPApiKey(row)
}

func (r *mcpApiKeyRepo) List(ctx context.Context) ([]*repository.MCPApiKey, error) {
	query := `SELECT id, name, prefix, key_hash, enabled, last_used_at, created_by, created_at, updated_at
		FROM mcp_api_keys ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list mcp api keys: %w", err)
	}
	defer rows.Close()

	var keys []*repository.MCPApiKey
	for rows.Next() {
		key, err := scanMCPApiKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return keys, nil
}

func (r *mcpApiKeyRepo) Update(ctx context.Context, key *repository.MCPApiKey) error {
	query := `UPDATE mcp_api_keys SET name = ?, enabled = ?, updated_at = strftime('%s','now') WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, key.Name, boolToInt(key.Enabled), key.ID)
	if err != nil {
		return fmt.Errorf("update mcp api key: %w", err)
	}
	return ensureRowsAffected(result)
}

func (r *mcpApiKeyRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM mcp_api_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete mcp api key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *mcpApiKeyRepo) UpdateLastUsed(ctx context.Context, id int64, at int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE mcp_api_keys SET last_used_at = ? WHERE id = ?`, at, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanMCPApiKey(row scanner) (*repository.MCPApiKey, error) {
	var key repository.MCPApiKey
	var enabled int
	err := row.Scan(&key.ID, &key.Name, &key.Prefix, &key.KeyHash, &enabled, &key.LastUsedAt, &key.CreatedBy, &key.CreatedAt, &key.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan mcp api key: %w", err)
	}
	key.Enabled = enabled != 0
	return &key, nil
}
