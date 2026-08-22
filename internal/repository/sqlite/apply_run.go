package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type applyRunRepo struct {
	db *sql.DB
}

func newApplyRunRepo(db *sql.DB) *applyRunRepo {
	return &applyRunRepo{db: db}
}

func (r *applyRunRepo) Create(ctx context.Context, run *repository.ApplyRun) error {
	if run == nil {
		return errors.New("apply run is nil")
	}
	if strings.TrimSpace(run.RunID) == "" {
		return errors.New("run_id is required")
	}

	now := time.Now().Unix()
	if run.StartedAt == 0 {
		run.StartedAt = now
	}
	if run.CreatedAt == 0 {
		run.CreatedAt = now
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO apply_runs (
			run_id, agent_host_id, core_type, target_revision, status,
			error_message, previous_revision, rollback_revision, operator_id,
			started_at, finished_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		run.RunID,
		run.AgentHostID,
		run.CoreType,
		run.TargetRevision,
		run.Status,
		run.ErrorMessage,
		run.PreviousRevision,
		run.RollbackRevision,
		run.OperatorID,
		run.StartedAt,
		run.FinishedAt,
		run.CreatedAt,
	)
	return err
}

func (r *applyRunRepo) MarkStarted(ctx context.Context, runID, status string, startedAt int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE apply_runs
		SET status = ?, started_at = ?
		WHERE run_id = ? AND status = 'pending'
	`, status, startedAt, runID)
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

func (r *applyRunRepo) UpdateStatus(ctx context.Context, runID, status, errorMessage string, rollbackRevision int64, finishedAt int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE apply_runs
		SET status = ?, error_message = ?, rollback_revision = ?, finished_at = ?
		WHERE run_id = ?
	`, status, errorMessage, rollbackRevision, finishedAt, runID)
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

func (r *applyRunRepo) FindByRunID(ctx context.Context, runID string) (*repository.ApplyRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			run_id, agent_host_id, core_type, target_revision, status,
			error_message, previous_revision, rollback_revision, operator_id,
			started_at, finished_at, created_at
		FROM apply_runs
		WHERE run_id = ?
		LIMIT 1
	`, runID)
	return r.scanApplyRun(row)
}

func (r *applyRunRepo) List(ctx context.Context, filter repository.ApplyRunFilter) ([]*repository.ApplyRun, error) {
	query := strings.Builder{}
	args := make([]any, 0, 10)

	query.WriteString(`
		SELECT
			run_id, agent_host_id, core_type, target_revision, status,
			error_message, previous_revision, rollback_revision, operator_id,
			started_at, finished_at, created_at
		FROM apply_runs
		WHERE 1 = 1
	`)

	appendApplyRunFilterConditions(&query, &args, filter)

	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY started_at DESC, run_id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*repository.ApplyRun
	for rows.Next() {
		run, err := r.scanApplyRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *applyRunRepo) Count(ctx context.Context, filter repository.ApplyRunFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)

	query.WriteString(`SELECT COUNT(*) FROM apply_runs WHERE 1 = 1`)
	appendApplyRunFilterConditions(&query, &args, filter)

	var total int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total)
	return total, err
}

func appendApplyRunFilterConditions(query *strings.Builder, args *[]any, filter repository.ApplyRunFilter) {
	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		*args = append(*args, *filter.AgentHostID)
	}
	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		*args = append(*args, *filter.CoreType)
	}
	if filter.TargetRevision != nil {
		query.WriteString(" AND target_revision = ?")
		*args = append(*args, *filter.TargetRevision)
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
}

type applyRunScanner interface {
	Scan(dest ...any) error
}

func (r *applyRunRepo) scanApplyRun(scanner applyRunScanner) (*repository.ApplyRun, error) {
	var run repository.ApplyRun
	err := scanner.Scan(
		&run.RunID,
		&run.AgentHostID,
		&run.CoreType,
		&run.TargetRevision,
		&run.Status,
		&run.ErrorMessage,
		&run.PreviousRevision,
		&run.RollbackRevision,
		&run.OperatorID,
		&run.StartedAt,
		&run.FinishedAt,
		&run.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *applyRunRepo) DeleteByClaimAge(ctx context.Context, maxAge time.Duration) (int64, error) {
	deadline := time.Now().Unix() - int64(maxAge.Seconds())
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM apply_runs WHERE started_at IS NOT NULL AND started_at < ?`, deadline)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
