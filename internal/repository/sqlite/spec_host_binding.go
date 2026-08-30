package sqlite

import (
	"context"
	"database/sql"
	"time"
)

type specHostBindingRepo struct {
	db *sql.DB
}

func newSpecHostBindingRepo(db *sql.DB) *specHostBindingRepo {
	return &specHostBindingRepo{db: db}
}

func (r *specHostBindingRepo) Bind(ctx context.Context, specID, agentHostID int64) error {
	_, err := execWithRetry(ctx, r.db,
		`INSERT OR IGNORE INTO spec_host_bindings (spec_id, agent_host_id, created_at) VALUES (?, ?, ?)`,
		specID, agentHostID, time.Now().Unix())
	return err
}

func (r *specHostBindingRepo) Unbind(ctx context.Context, specID, agentHostID int64) error {
	_, err := execWithRetry(ctx, r.db,
		`DELETE FROM spec_host_bindings WHERE spec_id = ? AND agent_host_id = ?`,
		specID, agentHostID)
	return err
}

func (r *specHostBindingRepo) UnbindAll(ctx context.Context, specID int64) error {
	_, err := execWithRetry(ctx, r.db, `DELETE FROM spec_host_bindings WHERE spec_id = ?`, specID)
	// idempotent: no error if not found
	return err
}

func (r *specHostBindingRepo) ListBySpec(ctx context.Context, specID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT agent_host_id FROM spec_host_bindings WHERE spec_id = ? ORDER BY agent_host_id`, specID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *specHostBindingRepo) ListByHost(ctx context.Context, agentHostID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT spec_id FROM spec_host_bindings WHERE agent_host_id = ? ORDER BY spec_id`, agentHostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
