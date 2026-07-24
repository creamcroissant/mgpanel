package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

type serverClientConfigRepo struct {
	db *sql.DB
}

func newServerClientConfigRepo(db *sql.DB) *serverClientConfigRepo {
	return &serverClientConfigRepo{db: db}
}

// Create inserts a new client config record.
func (r *serverClientConfigRepo) Create(ctx context.Context, cfg *repository.ServerClientConfig) error {
	const query = `INSERT INTO server_client_configs (server_id, format, content, content_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	now := time.Now().Unix()
	result, err := r.db.ExecContext(ctx, query,
		cfg.ServerID,
		cfg.Format,
		cfg.Content,
		cfg.ContentHash,
		now,
		now,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	cfg.ID = id
	cfg.CreatedAt = now
	cfg.UpdatedAt = now
	return nil
}

// FindByServerID returns all client configs for a server.
func (r *serverClientConfigRepo) FindByServerID(ctx context.Context, serverID int64) ([]*repository.ServerClientConfig, error) {
	const query = `SELECT id, server_id, format, content, content_hash, created_at, updated_at
		FROM server_client_configs
		WHERE server_id = ?
		ORDER BY format`

	rows, err := r.db.QueryContext(ctx, query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*repository.ServerClientConfig
	for rows.Next() {
		cfg := &repository.ServerClientConfig{}
		if err := rows.Scan(
			&cfg.ID,
			&cfg.ServerID,
			&cfg.Format,
			&cfg.Content,
			&cfg.ContentHash,
			&cfg.CreatedAt,
			&cfg.UpdatedAt,
		); err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

// FindByServerIDAndFormat returns a specific format config for a server.
func (r *serverClientConfigRepo) FindByServerIDAndFormat(ctx context.Context, serverID int64, format string) (*repository.ServerClientConfig, error) {
	const query = `SELECT id, server_id, format, content, content_hash, created_at, updated_at
		FROM server_client_configs
		WHERE server_id = ? AND format = ?`

	cfg := &repository.ServerClientConfig{}
	err := r.db.QueryRowContext(ctx, query, serverID, format).Scan(
		&cfg.ID,
		&cfg.ServerID,
		&cfg.Format,
		&cfg.Content,
		&cfg.ContentHash,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return cfg, nil
}

// Upsert creates or updates a client config.
func (r *serverClientConfigRepo) Upsert(ctx context.Context, cfg *repository.ServerClientConfig) error {
	const query = `INSERT INTO server_client_configs (server_id, format, content, content_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id, format) DO UPDATE SET
			content = excluded.content,
			content_hash = excluded.content_hash,
			updated_at = excluded.updated_at`

	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, query,
		cfg.ServerID,
		cfg.Format,
		cfg.Content,
		cfg.ContentHash,
		now,
		now,
	)
	if err != nil {
		return err
	}
	cfg.UpdatedAt = now
	return nil
}

// DeleteByServerID removes all client configs for a server.
func (r *serverClientConfigRepo) DeleteByServerID(ctx context.Context, serverID int64) error {
	const query = `DELETE FROM server_client_configs WHERE server_id = ?`
	_, err := r.db.ExecContext(ctx, query, serverID) // idempotent: no error if not found
	return err
}

// DeleteByServerIDAndFormat removes a specific format config.
func (r *serverClientConfigRepo) DeleteByServerIDAndFormat(ctx context.Context, serverID int64, format string) error {
	const query = `DELETE FROM server_client_configs WHERE server_id = ? AND format = ?`
	_, err := r.db.ExecContext(ctx, query, serverID, format) // idempotent: no error if not found
	return err
}
