package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type forwardingRuleRepo struct {
	db *sql.DB
}

func newForwardingRuleRepo(db *sql.DB) *forwardingRuleRepo {
	return &forwardingRuleRepo{db: db}
}

func (r *forwardingRuleRepo) Create(ctx context.Context, rule *repository.ForwardingRule) error {
	now := time.Now().Unix()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	if rule.Version == 0 {
		rule.Version = time.Now().UnixNano()
	}

	result, err := execWithRetry(ctx, r.db, `
		INSERT INTO forwarding_rules (
			agent_host_id, name, protocol, listen_port, target_address,
			target_port, enabled, priority, remark, version, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		rule.AgentHostID, rule.Name, rule.Protocol, rule.ListenPort, rule.TargetAddress,
		rule.TargetPort, boolToInt(rule.Enabled), rule.Priority, rule.Remark,
		rule.Version, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	rule.ID = id
	return nil
}

func (r *forwardingRuleRepo) Update(ctx context.Context, rule *repository.ForwardingRule) error {
	rule.UpdatedAt = time.Now().Unix()
	rule.Version = time.Now().UnixNano()

	result, err := execWithRetry(ctx, r.db, `
		UPDATE forwarding_rules SET
			name = ?, protocol = ?, listen_port = ?, target_address = ?,
			target_port = ?, enabled = ?, priority = ?, remark = ?,
			version = ?, updated_at = ?
		WHERE id = ?
	`,
		rule.Name, rule.Protocol, rule.ListenPort, rule.TargetAddress,
		rule.TargetPort, boolToInt(rule.Enabled), rule.Priority, rule.Remark,
		rule.Version, rule.UpdatedAt, rule.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *forwardingRuleRepo) Delete(ctx context.Context, id int64) error {
	result, err := execWithRetry(ctx, r.db, `DELETE FROM forwarding_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *forwardingRuleRepo) FindByID(ctx context.Context, id int64) (*repository.ForwardingRule, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, agent_host_id, name, protocol, listen_port, target_address,
			target_port, enabled, priority, remark, version, created_at, updated_at
		FROM forwarding_rules WHERE id = ?
	`, id)

	return r.scanRule(row)
}

func (r *forwardingRuleRepo) ListByAgentHostID(ctx context.Context, agentHostID int64) ([]*repository.ForwardingRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, agent_host_id, name, protocol, listen_port, target_address,
			target_port, enabled, priority, remark, version, created_at, updated_at
		FROM forwarding_rules
		WHERE agent_host_id = ?
		ORDER BY priority ASC, id ASC
	`, agentHostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRules(rows)
}

func (r *forwardingRuleRepo) ListEnabledByAgentHostID(ctx context.Context, agentHostID int64) ([]*repository.ForwardingRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, agent_host_id, name, protocol, listen_port, target_address,
			target_port, enabled, priority, remark, version, created_at, updated_at
		FROM forwarding_rules
		WHERE agent_host_id = ? AND enabled = 1
		ORDER BY priority ASC, id ASC
	`, agentHostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRules(rows)
}

func (r *forwardingRuleRepo) GetMaxVersion(ctx context.Context, agentHostID int64) (int64, error) {
	var version sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT MAX(version) FROM forwarding_rules WHERE agent_host_id = ?
	`, agentHostID).Scan(&version)
	if err != nil {
		return 0, err
	}
	if !version.Valid {
		return 0, nil
	}
	return version.Int64, nil
}

func (r *forwardingRuleRepo) CheckPortConflict(ctx context.Context, agentHostID int64, listenPort int, protocol string, excludeID int64) (bool, error) {
	var count int64

	// For 'both' protocol, check against tcp, udp, and both
	// For tcp/udp, check against the same protocol and 'both'
	var query string
	var args []interface{}

	if protocol == "both" {
		query = `
			SELECT COUNT(*) FROM forwarding_rules
			WHERE agent_host_id = ? AND listen_port = ? AND id != ?
		`
		args = []interface{}{agentHostID, listenPort, excludeID}
	} else {
		query = `
			SELECT COUNT(*) FROM forwarding_rules
			WHERE agent_host_id = ? AND listen_port = ? AND id != ?
			AND (protocol = ? OR protocol = 'both')
		`
		args = []interface{}{agentHostID, listenPort, excludeID, protocol}
	}

	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *forwardingRuleRepo) scanRule(row *sql.Row) (*repository.ForwardingRule, error) {
	var rule repository.ForwardingRule
	var enabled int

	err := row.Scan(
		&rule.ID, &rule.AgentHostID, &rule.Name, &rule.Protocol,
		&rule.ListenPort, &rule.TargetAddress, &rule.TargetPort,
		&enabled, &rule.Priority, &rule.Remark, &rule.Version,
		&rule.CreatedAt, &rule.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rule.Enabled = enabled == 1
	return &rule, nil
}

func (r *forwardingRuleRepo) scanRules(rows *sql.Rows) ([]*repository.ForwardingRule, error) {
	var rules []*repository.ForwardingRule
	for rows.Next() {
		var rule repository.ForwardingRule
		var enabled int

		err := rows.Scan(
			&rule.ID, &rule.AgentHostID, &rule.Name, &rule.Protocol,
			&rule.ListenPort, &rule.TargetAddress, &rule.TargetPort,
			&enabled, &rule.Priority, &rule.Remark, &rule.Version,
			&rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		rule.Enabled = enabled == 1
		rules = append(rules, &rule)
	}
	return rules, rows.Err()
}
