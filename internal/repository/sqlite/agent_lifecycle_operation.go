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

type agentLifecycleOperationRepo struct {
	db *sql.DB
}

func newAgentLifecycleOperationRepo(db *sql.DB) *agentLifecycleOperationRepo {
	return &agentLifecycleOperationRepo{db: db}
}

func (r *agentLifecycleOperationRepo) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	const stmt = `DELETE FROM agent_lifecycle_operations WHERE created_at < unixepoch() - ? * 86400`
	res, err := r.db.ExecContext(ctx, stmt, days)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *agentLifecycleOperationRepo) Create(ctx context.Context, operation *repository.AgentLifecycleOperation) error {
	if operation == nil {
		return errors.New("agent lifecycle operation is nil")
	}
	operation.ID = strings.TrimSpace(operation.ID)
	operation.OperationType = strings.TrimSpace(operation.OperationType)
	operation.Status = strings.TrimSpace(operation.Status)
	operation.Source = strings.TrimSpace(operation.Source)
	if operation.ID == "" {
		return errors.New("agent lifecycle operation id is required")
	}
	if operation.AgentHostID <= 0 {
		return errors.New("agent host id is required")
	}
	if operation.OperationType == "" {
		return errors.New("agent lifecycle operation type is required")
	}
	if operation.Status == "" {
		operation.Status = "pending"
	}
	if len(operation.RequestPayload) == 0 {
		operation.RequestPayload = json.RawMessage(`{}`)
	}
	if len(operation.ResultPayload) == 0 {
		operation.ResultPayload = json.RawMessage(`{}`)
	}
	now := time.Now().Unix()
	if operation.CreatedAt == 0 {
		operation.CreatedAt = now
	}
	if operation.UpdatedAt == 0 {
		operation.UpdatedAt = operation.CreatedAt
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_lifecycle_operations (
			id, agent_host_id, operation_type, status, request_payload, result_payload,
			error_message, claimed_by, claimed_at, started_at, finished_at,
			operator_id, source, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		operation.ID,
		operation.AgentHostID,
		operation.OperationType,
		operation.Status,
		[]byte(operation.RequestPayload),
		[]byte(operation.ResultPayload),
		operation.ErrorMessage,
		operation.ClaimedBy,
		optionalInt64(operation.ClaimedAt),
		optionalInt64(operation.StartedAt),
		optionalInt64(operation.FinishedAt),
		optionalInt64(operation.OperatorID),
		operation.Source,
		operation.CreatedAt,
		operation.UpdatedAt,
	)
	return err
}

func (r *agentLifecycleOperationRepo) UpdateStatus(ctx context.Context, id, status string, resultPayload json.RawMessage, errorMessage string, claimedBy string, claimedAt, startedAt, finishedAt *int64) error {
	if len(resultPayload) == 0 {
		resultPayload = json.RawMessage(`{}`)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_lifecycle_operations
		SET status = ?, result_payload = ?, error_message = ?, claimed_by = ?, claimed_at = ?, started_at = ?, finished_at = ?, updated_at = ?
		WHERE id = ?
	`, status, []byte(resultPayload), errorMessage, claimedBy, optionalInt64(claimedAt), optionalInt64(startedAt), optionalInt64(finishedAt), time.Now().Unix(), id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *agentLifecycleOperationRepo) UpdateClaimedStatus(ctx context.Context, id, claimedBy, status string, resultPayload json.RawMessage, errorMessage string, startedAt, finishedAt *int64) error {
	if len(resultPayload) == 0 {
		resultPayload = json.RawMessage(`{}`)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_lifecycle_operations
		SET status = ?, result_payload = ?, error_message = ?, started_at = ?, finished_at = ?, updated_at = ?
		WHERE id = ?
		  AND claimed_by = ?
		  AND status NOT IN ('success', 'failed', 'timeout', 'cancelled', 'unsupported_action', 'queue_full')
	`, status, []byte(resultPayload), errorMessage, optionalInt64(startedAt), optionalInt64(finishedAt), time.Now().Unix(), id, claimedBy)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *agentLifecycleOperationRepo) FindByID(ctx context.Context, id string) (*repository.AgentLifecycleOperation, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, agent_host_id, operation_type, status, request_payload, result_payload,
			error_message, claimed_by, claimed_at, started_at, finished_at,
			operator_id, source, created_at, updated_at
		FROM agent_lifecycle_operations
		WHERE id = ?
		LIMIT 1
	`, id)
	return scanAgentLifecycleOperation(row)
}

func (r *agentLifecycleOperationRepo) List(ctx context.Context, filter repository.AgentLifecycleOperationFilter) ([]*repository.AgentLifecycleOperation, error) {
	query := strings.Builder{}
	args := make([]any, 0, 12)
	query.WriteString(`
		SELECT id, agent_host_id, operation_type, status, request_payload, result_payload,
			error_message, claimed_by, claimed_at, started_at, finished_at,
			operator_id, source, created_at, updated_at
		FROM agent_lifecycle_operations WHERE 1 = 1
	`)
	appendAgentLifecycleOperationFilterConditions(&query, &args, filter)
	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	operations := make([]*repository.AgentLifecycleOperation, 0)
	for rows.Next() {
		operation, err := scanAgentLifecycleOperation(rows)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}

func (r *agentLifecycleOperationRepo) Count(ctx context.Context, filter repository.AgentLifecycleOperationFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 10)
	query.WriteString(`SELECT COUNT(*) FROM agent_lifecycle_operations WHERE 1 = 1`)
	appendAgentLifecycleOperationFilterConditions(&query, &args, filter)
	var total int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total)
	return total, err
}

func (r *agentLifecycleOperationRepo) ClaimNext(ctx context.Context, agentHostID int64, statuses []string, operationTypes []string, claimedBy string, claimedAt int64, reclaimBefore *int64, limit int) ([]*repository.AgentLifecycleOperation, error) {
	if limit <= 0 {
		limit = 1
	}
	operationTypes = normalizeClaimableLifecycleOperationTypes(operationTypes)
	if len(operationTypes) == 0 {
		return nil, repository.ErrNotFound
	}
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
		FROM agent_lifecycle_operations
		WHERE agent_host_id = ?
	`)
	args = append(args, agentHostID)
	appendClaimableLifecycleOperationTypes(&query, &args, operationTypes)
	appendClaimableLifecycleStatuses(&query, &args, statuses, reclaimBefore)
	query.WriteString(" ORDER BY created_at ASC, id ASC LIMIT ?")
	args = append(args, limit)

	rows, err := tx.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := make([]lifecycleClaimCandidate, 0, limit)
	for rows.Next() {
		var candidate lifecycleClaimCandidate
		if err := rows.Scan(&candidate.id, &candidate.status, &candidate.claimedAt); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, repository.ErrNotFound
	}

	claimedIDs := make([]string, 0, len(candidates))
	updatedAt := time.Now().Unix()
	for _, candidate := range candidates {
		result, err := updateLifecycleClaim(ctx, tx, candidate, claimedBy, claimedAt, updatedAt)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected > 0 {
			claimedIDs = append(claimedIDs, candidate.id)
		}
	}
	if len(claimedIDs) == 0 {
		return nil, repository.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	operations := make([]*repository.AgentLifecycleOperation, 0, len(claimedIDs))
	for _, id := range claimedIDs {
		operation, err := r.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	return operations, nil
}

type lifecycleClaimCandidate struct {
	id        string
	status    string
	claimedAt sql.NullInt64
}

func normalizeClaimableLifecycleOperationTypes(operationTypes []string) []string {
	seen := make(map[string]struct{}, len(operationTypes))
	out := make([]string, 0, len(operationTypes))
	for _, operationType := range operationTypes {
		operationType = strings.TrimSpace(operationType)
		if operationType == "" {
			continue
		}
		if _, ok := seen[operationType]; ok {
			continue
		}
		seen[operationType] = struct{}{}
		out = append(out, operationType)
	}
	return out
}

func appendClaimableLifecycleOperationTypes(query *strings.Builder, args *[]any, operationTypes []string) {
	query.WriteString(" AND operation_type IN (")
	for idx, operationType := range operationTypes {
		if idx > 0 {
			query.WriteString(",")
		}
		query.WriteString("?")
		*args = append(*args, operationType)
	}
	query.WriteString(")")
}

func appendClaimableLifecycleStatuses(query *strings.Builder, args *[]any, statuses []string, reclaimBefore *int64) {
	if len(statuses) == 0 {
		query.WriteString(" AND status = ?")
		*args = append(*args, "pending")
		return
	}
	query.WriteString(" AND (")
	for idx, status := range statuses {
		if idx > 0 {
			query.WriteString(" OR ")
		}
		if status == "claimed" && reclaimBefore != nil {
			query.WriteString("(status = ? AND claimed_at IS NOT NULL AND claimed_at <= ?)")
			*args = append(*args, status, *reclaimBefore)
			continue
		}
		query.WriteString("status = ?")
		*args = append(*args, status)
	}
	query.WriteString(")")
}

func updateLifecycleClaim(ctx context.Context, tx *sql.Tx, candidate lifecycleClaimCandidate, claimedBy string, claimedAt int64, updatedAt int64) (sql.Result, error) {
	if candidate.claimedAt.Valid {
		return tx.ExecContext(ctx, `
			UPDATE agent_lifecycle_operations
			SET status = ?, claimed_by = ?, claimed_at = ?, started_at = ?, updated_at = ?
			WHERE id = ? AND status = ? AND claimed_at = ?
		`, "claimed", claimedBy, claimedAt, claimedAt, updatedAt, candidate.id, candidate.status, candidate.claimedAt.Int64)
	}
	return tx.ExecContext(ctx, `
		UPDATE agent_lifecycle_operations
		SET status = ?, claimed_by = ?, claimed_at = ?, started_at = ?, updated_at = ?
		WHERE id = ? AND status = ? AND claimed_at IS NULL
	`, "claimed", claimedBy, claimedAt, claimedAt, updatedAt, candidate.id, candidate.status)
}

func appendAgentLifecycleOperationFilterConditions(query *strings.Builder, args *[]any, filter repository.AgentLifecycleOperationFilter) {
	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		*args = append(*args, *filter.AgentHostID)
	}
	if filter.OperationType != nil {
		query.WriteString(" AND operation_type = ?")
		*args = append(*args, *filter.OperationType)
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
	if filter.Source != nil {
		query.WriteString(" AND source = ?")
		*args = append(*args, *filter.Source)
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

type agentLifecycleOperationScanner interface {
	Scan(dest ...any) error
}

func scanAgentLifecycleOperation(scanner agentLifecycleOperationScanner) (*repository.AgentLifecycleOperation, error) {
	var operation repository.AgentLifecycleOperation
	var requestPayload []byte
	var resultPayload []byte
	var claimedAt sql.NullInt64
	var startedAt sql.NullInt64
	var finishedAt sql.NullInt64
	var operatorID sql.NullInt64
	if err := scanner.Scan(
		&operation.ID,
		&operation.AgentHostID,
		&operation.OperationType,
		&operation.Status,
		&requestPayload,
		&resultPayload,
		&operation.ErrorMessage,
		&operation.ClaimedBy,
		&claimedAt,
		&startedAt,
		&finishedAt,
		&operatorID,
		&operation.Source,
		&operation.CreatedAt,
		&operation.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if len(requestPayload) == 0 {
		operation.RequestPayload = json.RawMessage(`{}`)
	} else {
		operation.RequestPayload = append(json.RawMessage(nil), requestPayload...)
	}
	if len(resultPayload) == 0 {
		operation.ResultPayload = json.RawMessage(`{}`)
	} else {
		operation.ResultPayload = append(json.RawMessage(nil), resultPayload...)
	}
	operation.ClaimedAt = nullableIntPtr(claimedAt)
	operation.StartedAt = nullableIntPtr(startedAt)
	operation.FinishedAt = nullableIntPtr(finishedAt)
	operation.OperatorID = nullableIntPtr(operatorID)
	return &operation, nil
}
