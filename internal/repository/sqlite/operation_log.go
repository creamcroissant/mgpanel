package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type operationLogRepo struct {
	db *sql.DB
}

func newOperationLogRepo(db *sql.DB) *operationLogRepo {
	return &operationLogRepo{db: db}
}

func (r *operationLogRepo) Append(ctx context.Context, entry *repository.OperationLogEntry) (*repository.OperationLogEntry, error) {
	if entry == nil {
		return nil, errors.New("operation log entry is nil")
	}
	entry.Scope = strings.TrimSpace(entry.Scope)
	entry.TargetID = strings.TrimSpace(entry.TargetID)
	entry.SourceEventID = strings.TrimSpace(entry.SourceEventID)
	if entry.Scope == "" {
		return nil, errors.New("operation log scope is required")
	}
	if entry.TargetID == "" {
		return nil, errors.New("operation log target id is required")
	}
	if entry.SourceEventID != "" {
		existing, err := r.findBySourceEventID(ctx, entry.Scope, entry.TargetID, entry.SourceEventID)
		if err == nil {
			return existing, nil
		}
		if err != repository.ErrNotFound {
			return nil, err
		}
	}
	if len(entry.Payload) == 0 {
		entry.Payload = json.RawMessage(`{}`)
	}
	if entry.Level == "" {
		entry.Level = "info"
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_operation_logs (
			scope, target_id, agent_host_id, sequence, phase, level,
			message, payload_json, source_event_id, reported_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.Scope,
		entry.TargetID,
		entry.AgentHostID,
		entry.Sequence,
		entry.Phase,
		entry.Level,
		entry.Message,
		[]byte(entry.Payload),
		entry.SourceEventID,
		entry.ReportedAt,
		entry.CreatedAt,
	)
	if err != nil {
		if entry.SourceEventID != "" {
			existing, findErr := r.findBySourceEventID(ctx, entry.Scope, entry.TargetID, entry.SourceEventID)
			if findErr == nil {
				return existing, nil
			}
		}
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	entry.ID = id
	return entry, nil
}

func (r *operationLogRepo) List(ctx context.Context, filter repository.OperationLogFilter) ([]*repository.OperationLogEntry, error) {
	query := strings.Builder{}
	args := make([]any, 0, 10)
	query.WriteString(`
		SELECT id, scope, target_id, agent_host_id, sequence, phase, level,
			message, payload_json, source_event_id, reported_at, created_at
		FROM agent_operation_logs WHERE 1 = 1
	`)
	appendOperationLogFilterConditions(&query, &args, filter)
	limit, offset := normalizePagination(filter.Limit, filter.Offset, 200)
	query.WriteString(" ORDER BY id ASC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]*repository.OperationLogEntry, 0)
	for rows.Next() {
		entry, err := scanOperationLogEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *operationLogRepo) Count(ctx context.Context, filter repository.OperationLogFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)
	query.WriteString(`SELECT COUNT(*) FROM agent_operation_logs WHERE 1 = 1`)
	appendOperationLogFilterConditions(&query, &args, filter)

	var total int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total)
	return total, err
}

func (r *operationLogRepo) findBySourceEventID(ctx context.Context, scope, targetID, sourceEventID string) (*repository.OperationLogEntry, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scope, target_id, agent_host_id, sequence, phase, level,
			message, payload_json, source_event_id, reported_at, created_at
		FROM agent_operation_logs
		WHERE scope = ? AND target_id = ? AND source_event_id = ?
		LIMIT 1
	`, scope, targetID, sourceEventID)
	return scanOperationLogEntry(row)
}

func appendOperationLogFilterConditions(query *strings.Builder, args *[]any, filter repository.OperationLogFilter) {
	if filter.Scope != nil {
		query.WriteString(" AND scope = ?")
		*args = append(*args, *filter.Scope)
	}
	if filter.TargetID != nil {
		query.WriteString(" AND target_id = ?")
		*args = append(*args, *filter.TargetID)
	}
	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		*args = append(*args, *filter.AgentHostID)
	}
	if filter.AfterID != nil {
		query.WriteString(" AND id > ?")
		*args = append(*args, *filter.AfterID)
	}
	if filter.Level != nil {
		query.WriteString(" AND level = ?")
		*args = append(*args, *filter.Level)
	}
}

type operationLogScanner interface {
	Scan(dest ...any) error
}

func scanOperationLogEntry(scanner operationLogScanner) (*repository.OperationLogEntry, error) {
	entry := repository.OperationLogEntry{}
	var payload []byte
	if err := scanner.Scan(
		&entry.ID,
		&entry.Scope,
		&entry.TargetID,
		&entry.AgentHostID,
		&entry.Sequence,
		&entry.Phase,
		&entry.Level,
		&entry.Message,
		&payload,
		&entry.SourceEventID,
		&entry.ReportedAt,
		&entry.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if len(payload) == 0 {
		entry.Payload = json.RawMessage(`{}`)
	} else {
		entry.Payload = append(json.RawMessage(nil), payload...)
	}
	return &entry, nil
}
