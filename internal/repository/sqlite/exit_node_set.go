package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type exitNodeSetRepo struct {
	db *sql.DB
}

func newExitNodeSetRepo(db *sql.DB) *exitNodeSetRepo {
	return &exitNodeSetRepo{db: db}
}

func (r *exitNodeSetRepo) Create(ctx context.Context, set *repository.ExitNodeSet) error {
	res, err := execWithRetry(ctx, r.db, `
		INSERT INTO exit_node_sets (name, description, tags, strategy, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, set.Name, set.Description, set.Tags, set.Strategy, boolToInt(set.Enabled), set.CreatedAt, set.UpdatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	set.ID = id
	return nil
}

func (r *exitNodeSetRepo) Update(ctx context.Context, set *repository.ExitNodeSet) error {
	_, err := execWithRetry(ctx, r.db, `
		UPDATE exit_node_sets
		SET name = ?, description = ?, tags = ?, strategy = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, set.Name, set.Description, set.Tags, set.Strategy, boolToInt(set.Enabled), set.UpdatedAt, set.ID)
	return err
}

func (r *exitNodeSetRepo) Delete(ctx context.Context, id int64) error {
	// 级联删除成员
	_, err := execWithRetry(ctx, r.db, `DELETE FROM exit_node_sets WHERE id = ?`, id)
	return err
}

func (r *exitNodeSetRepo) FindByID(ctx context.Context, id int64) (*repository.ExitNodeSet, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, tags, strategy, enabled, created_at, updated_at
		FROM exit_node_sets WHERE id = ?
	`, id)
	return scanExitNodeSet(row)
}

func (r *exitNodeSetRepo) FindByName(ctx context.Context, name string) (*repository.ExitNodeSet, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, tags, strategy, enabled, created_at, updated_at
		FROM exit_node_sets WHERE name = ?
	`, name)
	return scanExitNodeSet(row)
}

func (r *exitNodeSetRepo) List(ctx context.Context) ([]*repository.ExitNodeSet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, tags, strategy, enabled, created_at, updated_at
		FROM exit_node_sets ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sets []*repository.ExitNodeSet
	for rows.Next() {
		set, err := scanExitNodeSet(rows)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	if sets == nil {
		sets = []*repository.ExitNodeSet{}
	}
	return sets, rows.Err()
}

func (r *exitNodeSetRepo) AddMember(ctx context.Context, m *repository.ExitNodeSetMember) error {
	res, err := execWithRetry(ctx, r.db, `
		INSERT INTO exit_node_set_members (set_id, agent_host_id, weight, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(set_id, agent_host_id) DO UPDATE SET weight = excluded.weight, enabled = excluded.enabled, updated_at = excluded.updated_at
	`, m.SetID, m.AgentHostID, m.Weight, boolToInt(m.Enabled), m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if id > 0 {
		m.ID = id
	}
	return nil
}

func (r *exitNodeSetRepo) RemoveMember(ctx context.Context, setID, agentHostID int64) error {
	_, err := execWithRetry(ctx, r.db, `DELETE FROM exit_node_set_members WHERE set_id = ? AND agent_host_id = ?`, setID, agentHostID)
	return err
}

func (r *exitNodeSetRepo) UpdateMember(ctx context.Context, m *repository.ExitNodeSetMember) error {
	_, err := execWithRetry(ctx, r.db, `
		UPDATE exit_node_set_members SET weight = ?, enabled = ?, updated_at = ? WHERE set_id = ? AND agent_host_id = ?
	`, m.Weight, boolToInt(m.Enabled), m.UpdatedAt, m.SetID, m.AgentHostID)
	return err
}

func (r *exitNodeSetRepo) ListMembers(ctx context.Context, setID int64) ([]*repository.ExitNodeSetMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, set_id, agent_host_id, weight, enabled, created_at, updated_at
		FROM exit_node_set_members WHERE set_id = ? ORDER BY agent_host_id
	`, setID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExitNodeSetMembers(rows)
}

func (r *exitNodeSetRepo) ListMembersByAgent(ctx context.Context, agentHostID int64) ([]*repository.ExitNodeSetMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, set_id, agent_host_id, weight, enabled, created_at, updated_at
		FROM exit_node_set_members WHERE agent_host_id = ? ORDER BY set_id
	`, agentHostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExitNodeSetMembers(rows)
}

func scanExitNodeSet(scanner interface {
	Scan(dest ...any) error
}) (*repository.ExitNodeSet, error) {
	var set repository.ExitNodeSet
	var enabled int
	err := scanner.Scan(&set.ID, &set.Name, &set.Description, &set.Tags, &set.Strategy, &enabled, &set.CreatedAt, &set.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	set.Enabled = enabled != 0
	return &set, nil
}

func scanExitNodeSetMembers(rows *sql.Rows) ([]*repository.ExitNodeSetMember, error) {
	var members []*repository.ExitNodeSetMember
	for rows.Next() {
		var m repository.ExitNodeSetMember
		var enabled int
		if err := rows.Scan(&m.ID, &m.SetID, &m.AgentHostID, &m.Weight, &enabled, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Enabled = enabled != 0
		members = append(members, &m)
	}
	if members == nil {
		members = []*repository.ExitNodeSetMember{}
	}
	return members, rows.Err()
}

var _ repository.ExitNodeSetRepository = (*exitNodeSetRepo)(nil)
var _ = errors.New
