package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type relayPathRepo struct {
	db *sql.DB
}

func newRelayPathRepo(db *sql.DB) *relayPathRepo {
	return &relayPathRepo{db: db}
}

// Create 插入链路主记录并在同一事务内写入全部节点。
func (r *relayPathRepo) Create(ctx context.Context, p *repository.RelayPath) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO relay_paths (name, description, core_type, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, p.Name, p.Description, p.CoreType, boolToInt(p.Enabled), p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := insertRelayNodes(ctx, tx, id, p.Nodes); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	p.ID = id
	return id, nil
}

// Update 更新主记录并以"先删后插"整体替换节点序列。
func (r *relayPathRepo) Update(ctx context.Context, p *repository.RelayPath) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE relay_paths
		SET name = ?, description = ?, core_type = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.Description, p.CoreType, boolToInt(p.Enabled), p.UpdatedAt, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repository.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM relay_path_nodes WHERE path_id = ?`, p.ID); err != nil {
		return err
	}
	if err := insertRelayNodes(ctx, tx, p.ID, p.Nodes); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *relayPathRepo) Delete(ctx context.Context, id int64) error {
	// relay_path_nodes 级联删除（FK ON DELETE CASCADE）
	_, err := execWithRetry(ctx, r.db, `DELETE FROM relay_paths WHERE id = ?`, id)
	return err
}

func (r *relayPathRepo) GetByID(ctx context.Context, id int64) (*repository.RelayPath, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, core_type, enabled, created_at, updated_at
		FROM relay_paths WHERE id = ?
	`, id)
	p, err := scanRelayPath(row)
	if err != nil {
		return nil, err
	}
	nodes, err := r.listNodes(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.Nodes = nodes
	return p, nil
}

func (r *relayPathRepo) List(ctx context.Context, coreType string) ([]*repository.RelayPath, error) {
	var rows *sql.Rows
	var err error
	if coreType == "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, description, core_type, enabled, created_at, updated_at
			FROM relay_paths ORDER BY id
		`)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, description, core_type, enabled, created_at, updated_at
			FROM relay_paths WHERE core_type = ? ORDER BY id
		`, coreType)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []*repository.RelayPath
	for rows.Next() {
		p, err := scanRelayPath(rows)
		if err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, p := range paths {
		nodes, err := r.listNodes(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		p.Nodes = nodes
	}
	if paths == nil {
		paths = []*repository.RelayPath{}
	}
	return paths, nil
}

func (r *relayPathRepo) listNodes(ctx context.Context, pathID int64) ([]repository.RelayPathNode, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sequence, agent_host_id, private_key, public_key FROM relay_path_nodes WHERE path_id = ? ORDER BY sequence
	`, pathID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := []repository.RelayPathNode{}
	for rows.Next() {
		var n repository.RelayPathNode
		if err := rows.Scan(&n.Sequence, &n.AgentHostID, &n.PrivateKey, &n.PublicKey); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// insertRelayNodes 在给定事务内批量插入节点。
func insertRelayNodes(ctx context.Context, tx *sql.Tx, pathID int64, nodes []repository.RelayPathNode) error {
	for _, n := range nodes {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO relay_path_nodes (path_id, sequence, agent_host_id, private_key, public_key)
			VALUES (?, ?, ?, ?, ?)
		`, pathID, n.Sequence, n.AgentHostID, n.PrivateKey, n.PublicKey)
		if err != nil {
			return fmt.Errorf("insert relay node seq=%d: %w", n.Sequence, err)
		}
		if _, err := res.RowsAffected(); err != nil {
			return err
		}
	}
	return nil
}

func scanRelayPath(row interface{ Scan(...any) error }) (*repository.RelayPath, error) {
	var p repository.RelayPath
	var enabled int
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.CoreType, &enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	p.Enabled = enabled != 0
	p.Nodes = []repository.RelayPathNode{}
	return &p, nil
}
