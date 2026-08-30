package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type forwardingRuleLogRepo struct {
	db *sql.DB
}

func newForwardingRuleLogRepo(db *sql.DB) *forwardingRuleLogRepo {
	return &forwardingRuleLogRepo{db: db}
}

func (r *forwardingRuleLogRepo) Create(ctx context.Context, log *repository.ForwardingRuleLog) error {
	log.CreatedAt = time.Now().Unix()

	result, err := execWithRetry(ctx, r.db, `
		INSERT INTO forwarding_rule_logs (
			rule_id, agent_host_id, action, operator_id, detail, created_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`,
		log.RuleID, log.AgentHostID, log.Action, log.OperatorID, log.Detail, log.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id
	return nil
}

func (r *forwardingRuleLogRepo) List(ctx context.Context, filter repository.ForwardingRuleLogFilter) ([]*repository.ForwardingRuleLog, error) {
	query := strings.Builder{}
	args := make([]interface{}, 0, 5)

	query.WriteString("SELECT id, rule_id, agent_host_id, action, operator_id, detail, created_at FROM forwarding_rule_logs WHERE agent_host_id = ?")
	args = append(args, filter.AgentHostID)

	if filter.RuleID != nil {
		query.WriteString(" AND rule_id = ?")
		args = append(args, *filter.RuleID)
	}
	if filter.StartAt != nil {
		query.WriteString(" AND created_at >= ?")
		args = append(args, *filter.StartAt)
	}
	if filter.EndAt != nil {
		query.WriteString(" AND created_at <= ?")
		args = append(args, *filter.EndAt)
	}

	query.WriteString(" ORDER BY created_at DESC")
	query.WriteString(" LIMIT ? OFFSET ?")
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLogs(rows)
}

func (r *forwardingRuleLogRepo) Count(ctx context.Context, filter repository.ForwardingRuleLogFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]interface{}, 0, 4)

	query.WriteString("SELECT COUNT(*) FROM forwarding_rule_logs WHERE agent_host_id = ?")
	args = append(args, filter.AgentHostID)

	if filter.RuleID != nil {
		query.WriteString(" AND rule_id = ?")
		args = append(args, *filter.RuleID)
	}
	if filter.StartAt != nil {
		query.WriteString(" AND created_at >= ?")
		args = append(args, *filter.StartAt)
	}
	if filter.EndAt != nil {
		query.WriteString(" AND created_at <= ?")
		args = append(args, *filter.EndAt)
	}

	var count int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&count)
	return count, err
}

func (r *forwardingRuleLogRepo) ListByRuleID(ctx context.Context, ruleID int64, limit int) ([]*repository.ForwardingRuleLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, rule_id, agent_host_id, action, operator_id, detail, created_at
		FROM forwarding_rule_logs
		WHERE rule_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, ruleID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLogs(rows)
}

func (r *forwardingRuleLogRepo) scanLogs(rows *sql.Rows) ([]*repository.ForwardingRuleLog, error) {
	var logs []*repository.ForwardingRuleLog
	for rows.Next() {
		var log repository.ForwardingRuleLog
		var ruleID sql.NullInt64
		var operatorID sql.NullInt64
		var detail sql.NullString

		err := rows.Scan(
			&log.ID, &ruleID, &log.AgentHostID, &log.Action,
			&operatorID, &detail, &log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if ruleID.Valid {
			log.RuleID = &ruleID.Int64
		}
		if operatorID.Valid {
			log.OperatorID = &operatorID.Int64
		}
		if detail.Valid {
			log.Detail = detail.String
		}

		logs = append(logs, &log)
	}
	return logs, rows.Err()
}
