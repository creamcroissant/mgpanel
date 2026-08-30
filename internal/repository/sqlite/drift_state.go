package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type driftStateRepo struct {
	db *sql.DB
}

func newDriftStateRepo(db *sql.DB) *driftStateRepo {
	return &driftStateRepo{db: db}
}

func (r *driftStateRepo) Upsert(ctx context.Context, drift *repository.DriftState) error {
	if drift == nil {
		return errors.New("drift state is nil")
	}

	result, err := execWithRetry(ctx, r.db, `
		INSERT INTO drift_states (
			agent_host_id, core_type, filename, tag, desired_revision,
			desired_hash, applied_hash, drift_type, status, detail,
			first_detected_at, last_changed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_host_id, core_type, filename, tag, drift_type)
		DO UPDATE SET
			desired_revision = excluded.desired_revision,
			desired_hash = excluded.desired_hash,
			applied_hash = excluded.applied_hash,
			status = excluded.status,
			detail = excluded.detail,
			last_changed_at = excluded.last_changed_at
	`,
		drift.AgentHostID,
		drift.CoreType,
		drift.Filename,
		drift.Tag,
		drift.DesiredRevision,
		drift.DesiredHash,
		drift.AppliedHash,
		drift.DriftType,
		drift.Status,
		string(normalizeJSONObject(drift.Detail)),
		drift.FirstDetectedAt,
		drift.LastChangedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	if id > 0 {
		drift.ID = id
	}
	return nil
}

func (r *driftStateRepo) MarkRecoveredByHostCore(ctx context.Context, agentHostID int64, coreType string, recoveredAt int64) error {
	result, err := execWithRetry(ctx, r.db, `
		UPDATE drift_states
		SET status = 'recovered', last_changed_at = ?
		WHERE agent_host_id = ? AND core_type = ? AND status != 'recovered'
	`, recoveredAt, agentHostID, coreType)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *driftStateRepo) List(ctx context.Context, filter repository.DriftStateFilter) ([]*repository.DriftState, error) {
	query := strings.Builder{}
	args := make([]any, 0, 10)

	query.WriteString(`
		SELECT
			id, agent_host_id, core_type, filename, tag, desired_revision,
			desired_hash, applied_hash, drift_type, status, detail,
			first_detected_at, last_changed_at
		FROM drift_states
		WHERE 1 = 1
	`)

	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		args = append(args, *filter.AgentHostID)
	}
	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		args = append(args, *filter.CoreType)
	}
	if filter.Status != nil {
		query.WriteString(" AND status = ?")
		args = append(args, *filter.Status)
	}
	if filter.DriftType != nil {
		query.WriteString(" AND drift_type = ?")
		args = append(args, *filter.DriftType)
	}
	if filter.Tag != nil {
		query.WriteString(" AND tag = ?")
		args = append(args, *filter.Tag)
	}
	if filter.Filename != nil {
		query.WriteString(" AND filename = ?")
		args = append(args, *filter.Filename)
	}

	limit, offset := normalizePagination(filter.Limit, filter.Offset, 200)
	query.WriteString(" ORDER BY last_changed_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var driftStates []*repository.DriftState
	for rows.Next() {
		drift, err := r.scanDriftState(rows)
		if err != nil {
			return nil, err
		}
		driftStates = append(driftStates, drift)
	}
	return driftStates, rows.Err()
}

func (r *driftStateRepo) Count(ctx context.Context, filter repository.DriftStateFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)

	query.WriteString(`SELECT COUNT(*) FROM drift_states WHERE 1 = 1`)
	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		args = append(args, *filter.AgentHostID)
	}
	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		args = append(args, *filter.CoreType)
	}
	if filter.Status != nil {
		query.WriteString(" AND status = ?")
		args = append(args, *filter.Status)
	}
	if filter.DriftType != nil {
		query.WriteString(" AND drift_type = ?")
		args = append(args, *filter.DriftType)
	}
	if filter.Tag != nil {
		query.WriteString(" AND tag = ?")
		args = append(args, *filter.Tag)
	}
	if filter.Filename != nil {
		query.WriteString(" AND filename = ?")
		args = append(args, *filter.Filename)
	}

	var total int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total)
	return total, err
}

type driftStateScanner interface {
	Scan(dest ...any) error
}

func (r *driftStateRepo) scanDriftState(scanner driftStateScanner) (*repository.DriftState, error) {
	var drift repository.DriftState
	var detail sql.NullString

	err := scanner.Scan(
		&drift.ID,
		&drift.AgentHostID,
		&drift.CoreType,
		&drift.Filename,
		&drift.Tag,
		&drift.DesiredRevision,
		&drift.DesiredHash,
		&drift.AppliedHash,
		&drift.DriftType,
		&drift.Status,
		&detail,
		&drift.FirstDetectedAt,
		&drift.LastChangedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	drift.Detail = parseJSONObject(detail)
	return &drift, nil
}
