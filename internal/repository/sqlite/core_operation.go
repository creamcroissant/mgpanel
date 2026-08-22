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

type coreOperationRepo struct {
	db *sql.DB
}

func newCoreOperationRepo(db *sql.DB) *coreOperationRepo {
	return &coreOperationRepo{db: db}
}

func (r *coreOperationRepo) Create(ctx context.Context, operation *repository.CoreOperation) error {
	if operation == nil {
		return errors.New("core operation is nil")
	}
	if strings.TrimSpace(operation.ID) == "" {
		return errors.New("core operation id is required")
	}
	now := time.Now().Unix()
	if operation.CreatedAt == 0 {
		operation.CreatedAt = now
	}
	if operation.UpdatedAt == 0 {
		operation.UpdatedAt = operation.CreatedAt
	}
	if len(operation.RequestPayload) == 0 {
		operation.RequestPayload = json.RawMessage(`{}`)
	}
	if len(operation.ResultPayload) == 0 {
		operation.ResultPayload = json.RawMessage(`{}`)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO core_operations (
			id, agent_host_id, operation_type, core_type, status,
			request_payload, result_payload, error_message, operator_id,
			claimed_by, claimed_at, started_at, finished_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		operation.ID,
		operation.AgentHostID,
		operation.OperationType,
		operation.CoreType,
		operation.Status,
		[]byte(operation.RequestPayload),
		[]byte(operation.ResultPayload),
		operation.ErrorMessage,
		optionalInt64(operation.OperatorID),
		operation.ClaimedBy,
		optionalInt64(operation.ClaimedAt),
		optionalInt64(operation.StartedAt),
		optionalInt64(operation.FinishedAt),
		operation.CreatedAt,
		operation.UpdatedAt,
	)
	return err
}

func (r *coreOperationRepo) UpdateStatus(ctx context.Context, id, status string, resultPayload json.RawMessage, errorMessage string, claimedBy string, claimedAt, startedAt, finishedAt *int64) error {
	if len(resultPayload) == 0 {
		resultPayload = json.RawMessage(`{}`)
	}
	updatedAt := time.Now().Unix()
	result, err := r.db.ExecContext(ctx, `
		UPDATE core_operations
		SET status = ?, result_payload = ?, error_message = ?, claimed_by = ?, claimed_at = ?, started_at = ?, finished_at = ?, updated_at = ?
		WHERE id = ?
	`, status, []byte(resultPayload), errorMessage, claimedBy, optionalInt64(claimedAt), optionalInt64(startedAt), optionalInt64(finishedAt), updatedAt, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *coreOperationRepo) FindByID(ctx context.Context, id string) (*repository.CoreOperation, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, agent_host_id, operation_type, core_type, status,
			request_payload, result_payload, error_message, operator_id,
			claimed_by, claimed_at, started_at, finished_at, created_at, updated_at
		FROM core_operations WHERE id = ? LIMIT 1
	`, id)
	return r.scanCoreOperation(row)
}

func (r *coreOperationRepo) List(ctx context.Context, filter repository.CoreOperationFilter) ([]*repository.CoreOperation, error) {
	query := strings.Builder{}
	args := make([]any, 0, 12)
	query.WriteString(`
		SELECT id, agent_host_id, operation_type, core_type, status,
			request_payload, result_payload, error_message, operator_id,
			claimed_by, claimed_at, started_at, finished_at, created_at, updated_at
		FROM core_operations WHERE 1 = 1
	`)
	appendCoreOperationFilterConditions(&query, &args, filter)
	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var operations []*repository.CoreOperation
	for rows.Next() {
		op, err := r.scanCoreOperation(rows)
		if err != nil {
			return nil, err
		}
		operations = append(operations, op)
	}
	return operations, rows.Err()
}

func (r *coreOperationRepo) Count(ctx context.Context, filter repository.CoreOperationFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 12)
	query.WriteString(`SELECT COUNT(*) FROM core_operations WHERE 1 = 1`)
	appendCoreOperationFilterConditions(&query, &args, filter)
	var total int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total)
	return total, err
}

func (r *coreOperationRepo) ClaimNext(ctx context.Context, agentHostID int64, statuses []string, claimedBy string, claimedAt int64, reclaimBefore *int64) (*repository.CoreOperation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	query := strings.Builder{}
	args := make([]any, 0, 12)
	query.WriteString(`
		SELECT id, status, claimed_at
		FROM core_operations
		WHERE agent_host_id = ?
	`)
	args = append(args, agentHostID)
	if len(statuses) > 0 {
		query.WriteString(" AND (")
		for idx, status := range statuses {
			if idx > 0 {
				query.WriteString(" OR ")
			}
			if status == "claimed" && reclaimBefore != nil {
				query.WriteString("(status = ? AND claimed_at IS NOT NULL AND claimed_at <= ?)")
				args = append(args, status, *reclaimBefore)
				continue
			}
			query.WriteString("status = ?")
			args = append(args, status)
		}
		query.WriteString(")")
	}
	query.WriteString(" ORDER BY created_at ASC, id ASC LIMIT 1")

	var operationID string
	var previousStatus string
	var previousClaimedAt sql.NullInt64
	if err := tx.QueryRowContext(ctx, query.String(), args...).Scan(&operationID, &previousStatus, &previousClaimedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	updatedAt := time.Now().Unix()
	startedAt := claimedAt
	var updateQuery string
	var updateArgs []any
	if previousClaimedAt.Valid {
		updateQuery = `
			UPDATE core_operations
			SET status = ?, claimed_by = ?, claimed_at = ?, started_at = ?, updated_at = ?
			WHERE id = ? AND status = ? AND claimed_at = ?`
		updateArgs = []any{"claimed", claimedBy, claimedAt, startedAt, updatedAt, operationID, previousStatus, previousClaimedAt.Int64}
	} else {
		updateQuery = `
			UPDATE core_operations
			SET status = ?, claimed_by = ?, claimed_at = ?, started_at = ?, updated_at = ?
			WHERE id = ? AND status = ? AND claimed_at IS NULL`
		updateArgs = []any{"claimed", claimedBy, claimedAt, startedAt, updatedAt, operationID, previousStatus}
	}
	result, err := tx.ExecContext(ctx, updateQuery, updateArgs...)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, repository.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, operationID)
}

type coreOperationScanner interface {
	Scan(dest ...any) error
}

func (r *coreOperationRepo) scanCoreOperation(scanner coreOperationScanner) (*repository.CoreOperation, error) {
	var op repository.CoreOperation
	var requestPayload []byte
	var resultPayload []byte
	var operatorID sql.NullInt64
	var claimedAt sql.NullInt64
	var startedAt sql.NullInt64
	var finishedAt sql.NullInt64
	if err := scanner.Scan(
		&op.ID,
		&op.AgentHostID,
		&op.OperationType,
		&op.CoreType,
		&op.Status,
		&requestPayload,
		&resultPayload,
		&op.ErrorMessage,
		&operatorID,
		&op.ClaimedBy,
		&claimedAt,
		&startedAt,
		&finishedAt,
		&op.CreatedAt,
		&op.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if len(requestPayload) == 0 {
		op.RequestPayload = json.RawMessage(`{}`)
	} else {
		op.RequestPayload = append(json.RawMessage(nil), requestPayload...)
	}
	if len(resultPayload) == 0 {
		op.ResultPayload = json.RawMessage(`{}`)
	} else {
		op.ResultPayload = append(json.RawMessage(nil), resultPayload...)
	}
	op.OperatorID = nullableIntPtr(operatorID)
	op.ClaimedAt = nullableIntPtr(claimedAt)
	op.StartedAt = nullableIntPtr(startedAt)
	op.FinishedAt = nullableIntPtr(finishedAt)
	return &op, nil
}

func appendCoreOperationFilterConditions(query *strings.Builder, args *[]any, filter repository.CoreOperationFilter) {
	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		*args = append(*args, *filter.AgentHostID)
	}
	if filter.OperationType != nil {
		query.WriteString(" AND operation_type = ?")
		*args = append(*args, *filter.OperationType)
	}
	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		*args = append(*args, *filter.CoreType)
	}
	if len(filter.Statuses) > 0 {
		query.WriteString(" AND status IN (")
		for idx, status := range filter.Statuses {
			if idx > 0 {
				query.WriteString(",")
			}
			query.WriteString("?")
			*args = append(*args, status)
		}
		query.WriteString(")")
	} else if filter.Status != nil {
		query.WriteString(" AND status = ?")
		*args = append(*args, *filter.Status)
	}
	if filter.ClaimedBy != nil {
		query.WriteString(" AND claimed_by = ?")
		*args = append(*args, *filter.ClaimedBy)
	}
	if filter.CreatedAfter != nil {
		query.WriteString(" AND created_at >= ?")
		*args = append(*args, *filter.CreatedAfter)
	}
	if filter.CreatedBefore != nil {
		query.WriteString(" AND created_at <= ?")
		*args = append(*args, *filter.CreatedBefore)
	}
}
