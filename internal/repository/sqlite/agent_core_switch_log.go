package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type agentCoreSwitchLogRepo struct {
	db *sql.DB
}

func newAgentCoreSwitchLogRepo(db *sql.DB) *agentCoreSwitchLogRepo {
	return &agentCoreSwitchLogRepo{db: db}
}

func (r *agentCoreSwitchLogRepo) Create(ctx context.Context, log *repository.AgentCoreSwitchLog) error {
	if log == nil {
		return errors.New("log is nil")
	}

	log.CreatedAt = time.Now().Unix()

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_core_switch_logs (
			agent_host_id, from_instance_id, from_core_type, to_instance_id, to_core_type,
			operator_id, status, detail, created_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		log.AgentHostID,
		nullableTextPtr(log.FromInstanceID),
		nullableTextPtr(log.FromCoreType),
		log.ToInstanceID,
		log.ToCoreType,
		optionalInt64(log.OperatorID),
		log.Status,
		log.Detail,
		log.CreatedAt,
		optionalInt64(log.CompletedAt),
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

func (r *agentCoreSwitchLogRepo) UpdateStatus(ctx context.Context, id int64, status string, detail string, completedAt *int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_core_switch_logs
		SET status = ?, detail = ?, completed_at = ?
		WHERE id = ?
	`, status, detail, optionalInt64(completedAt), id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *agentCoreSwitchLogRepo) List(ctx context.Context, filter repository.AgentCoreSwitchLogFilter) ([]*repository.AgentCoreSwitchLog, error) {
	query := strings.Builder{}
	args := make([]interface{}, 0, 5)

	query.WriteString("SELECT id, agent_host_id, from_instance_id, from_core_type, to_instance_id, to_core_type, operator_id, status, detail, created_at, completed_at FROM agent_core_switch_logs WHERE agent_host_id = ?")
	args = append(args, filter.AgentHostID)

	if filter.Status != nil {
		query.WriteString(" AND status = ?")
		args = append(args, *filter.Status)
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

func (r *agentCoreSwitchLogRepo) Count(ctx context.Context, filter repository.AgentCoreSwitchLogFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]interface{}, 0, 4)

	query.WriteString("SELECT COUNT(*) FROM agent_core_switch_logs WHERE agent_host_id = ?")
	args = append(args, filter.AgentHostID)

	if filter.Status != nil {
		query.WriteString(" AND status = ?")
		args = append(args, *filter.Status)
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

func (r *agentCoreSwitchLogRepo) scanLogs(rows *sql.Rows) ([]*repository.AgentCoreSwitchLog, error) {
	var logs []*repository.AgentCoreSwitchLog
	for rows.Next() {
		log, err := r.scanLogRow(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (r *agentCoreSwitchLogRepo) scanLogRow(rows *sql.Rows) (*repository.AgentCoreSwitchLog, error) {
	var log repository.AgentCoreSwitchLog
	var fromInstance sql.NullString
	var fromCore sql.NullString
	var operatorID sql.NullInt64
	var detail sql.NullString
	var completedAt sql.NullInt64

	if err := rows.Scan(
		&log.ID,
		&log.AgentHostID,
		&fromInstance,
		&fromCore,
		&log.ToInstanceID,
		&log.ToCoreType,
		&operatorID,
		&log.Status,
		&detail,
		&log.CreatedAt,
		&completedAt,
	); err != nil {
		return nil, err
	}

	if fromInstance.Valid {
		value := fromInstance.String
		log.FromInstanceID = &value
	}
	if fromCore.Valid {
		value := fromCore.String
		log.FromCoreType = &value
	}
	if operatorID.Valid {
		log.OperatorID = nullableIntPtr(operatorID)
	}
	if detail.Valid {
		log.Detail = detail.String
	}
	if completedAt.Valid {
		log.CompletedAt = nullableIntPtr(completedAt)
	}

	return &log, nil
}

func nullableTextPtr(v *string) any {
	if v == nil {
		return nil
	}
	if strings.TrimSpace(*v) == "" {
		return nil
	}
	return *v
}
