package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type routingPolicyRepo struct {
	db *sql.DB
}

func newRoutingPolicyRepo(db *sql.DB) *routingPolicyRepo {
	return &routingPolicyRepo{db: db}
}

func (r *routingPolicyRepo) Create(ctx context.Context, p *repository.RoutingPolicy) error {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO routing_policies (name, core_type, priority, match_type, match_value, action, target_set_id, spec_id, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.Name, p.CoreType, p.Priority, p.MatchType, p.MatchValue, p.Action, optionalInt64(p.TargetSetID), optionalInt64(p.SpecID), boolToInt(p.Enabled), p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

func (r *routingPolicyRepo) Update(ctx context.Context, p *repository.RoutingPolicy) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE routing_policies
		SET name = ?, core_type = ?, priority = ?, match_type = ?, match_value = ?, action = ?, target_set_id = ?, spec_id = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.CoreType, p.Priority, p.MatchType, p.MatchValue, p.Action, optionalInt64(p.TargetSetID), optionalInt64(p.SpecID), boolToInt(p.Enabled), p.UpdatedAt, p.ID)
	return err
}

func (r *routingPolicyRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM routing_policies WHERE id = ?`, id)
	return err
}

func (r *routingPolicyRepo) FindByID(ctx context.Context, id int64) (*repository.RoutingPolicy, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, core_type, priority, match_type, match_value, action, target_set_id, spec_id, enabled, created_at, updated_at
		FROM routing_policies WHERE id = ?
	`, id)
	return scanRoutingPolicy(row)
}

func (r *routingPolicyRepo) List(ctx context.Context, filter repository.RoutingPolicyFilter) ([]*repository.RoutingPolicy, error) {
	query := "SELECT id, name, core_type, priority, match_type, match_value, action, target_set_id, spec_id, enabled, created_at, updated_at FROM routing_policies WHERE 1=1"
	args := make([]any, 0, 5)
	if filter.CoreType != nil {
		query += " AND core_type = ?"
		args = append(args, *filter.CoreType)
	}
	if filter.Enabled != nil {
		query += " AND enabled = ?"
		args = append(args, boolToInt(*filter.Enabled))
	}
	if filter.SpecID != nil {
		query += " AND spec_id = ?"
		args = append(args, *filter.SpecID)
	}
	if filter.OnlyGlobal != nil && *filter.OnlyGlobal {
		query += " AND spec_id IS NULL"
	}
	query += " ORDER BY priority ASC, id ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoutingPolicies(rows)
}

func (r *routingPolicyRepo) ListEnabledByCore(ctx context.Context, coreType string) ([]*repository.RoutingPolicy, error) {
	enabled := true
	return r.List(ctx, repository.RoutingPolicyFilter{CoreType: &coreType, Enabled: &enabled})
}

// ReorderPriorities 事务内按给定 ID 顺序重写 priority：首项 100，逐项递增 100。
// 编译器按 ORDER BY priority ASC 求值，升序赋值保证数组顺序即求值顺序。
func (r *routingPolicyRepo) ReorderPriorities(ctx context.Context, orderedIDs []int64) (int64, error) {
	if len(orderedIDs) == 0 {
		return 0, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin reorder tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, "UPDATE routing_policies SET priority = ?, updated_at = ? WHERE id = ?")
	if err != nil {
		return 0, fmt.Errorf("prepare reorder stmt: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	var updated int64
	for i, id := range orderedIDs {
		priority := (i + 1) * 100 // 首项 100（如 3 项→100/200/300），ASC 求值即数组顺序
		res, err := stmt.ExecContext(ctx, priority, now, id)
		if err != nil {
			return 0, fmt.Errorf("reorder policy id=%d: %w", id, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			updated += n
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit reorder: %w", err)
	}
	return updated, nil
}

func scanRoutingPolicy(scanner interface {
	Scan(dest ...any) error
}) (*repository.RoutingPolicy, error) {
	var p repository.RoutingPolicy
	var enabled int
	var targetSetID sql.NullInt64
	var specID sql.NullInt64
	err := scanner.Scan(&p.ID, &p.Name, &p.CoreType, &p.Priority, &p.MatchType, &p.MatchValue, &p.Action, &targetSetID, &specID, &enabled, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Enabled = enabled != 0
	if targetSetID.Valid {
		p.TargetSetID = &targetSetID.Int64
	}
	if specID.Valid {
		p.SpecID = &specID.Int64
	}
	return &p, nil
}

func scanRoutingPolicies(rows *sql.Rows) ([]*repository.RoutingPolicy, error) {
	var policies []*repository.RoutingPolicy
	for rows.Next() {
		p, err := scanRoutingPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	if policies == nil {
		policies = []*repository.RoutingPolicy{}
	}
	return policies, rows.Err()
}

var _ repository.RoutingPolicyRepository = (*routingPolicyRepo)(nil)