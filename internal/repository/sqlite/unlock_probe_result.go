package sqlite

import (
	"context"
	"database/sql"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type unlockProbeResultRepo struct {
	db *sql.DB
}

func newUnlockProbeResultRepo(db *sql.DB) *unlockProbeResultRepo {
	return &unlockProbeResultRepo{db: db}
}

func (r *unlockProbeResultRepo) Upsert(ctx context.Context, result *repository.UnlockProbeResult) error {
	_, err := execWithRetry(ctx, r.db, `
		INSERT INTO unlock_probe_results (agent_host_id, service, status, region, detail, probed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_host_id, service)
		DO UPDATE SET status=excluded.status, region=excluded.region, detail=excluded.detail,
		              probed_at=excluded.probed_at, created_at=excluded.created_at
	`, result.AgentHostID, result.Service, result.Status, result.Region, result.Detail, result.ProbedAt, result.CreatedAt)
	return err
}

func (r *unlockProbeResultRepo) ListByAgentHost(ctx context.Context, agentHostID int64) ([]*repository.UnlockProbeResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, agent_host_id, service, status, region, detail, probed_at, created_at
		FROM unlock_probe_results
		WHERE agent_host_id = ?
		ORDER BY service
	`, agentHostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUnlockProbeResults(rows)
}

func (r *unlockProbeResultRepo) ListAll(ctx context.Context) ([]*repository.UnlockProbeResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, agent_host_id, service, status, region, detail, probed_at, created_at
		FROM unlock_probe_results
		ORDER BY agent_host_id, service
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUnlockProbeResults(rows)
}

func (r *unlockProbeResultRepo) CountByAgentHost(ctx context.Context, agentHostID int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM unlock_probe_results WHERE agent_host_id = ?`, agentHostID).Scan(&count)
	return count, err
}

func scanUnlockProbeResults(rows *sql.Rows) ([]*repository.UnlockProbeResult, error) {
	var results []*repository.UnlockProbeResult
	for rows.Next() {
		var r repository.UnlockProbeResult
		if err := rows.Scan(&r.ID, &r.AgentHostID, &r.Service, &r.Status, &r.Region, &r.Detail, &r.ProbedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, &r)
	}
	if results == nil {
		results = []*repository.UnlockProbeResult{}
	}
	return results, rows.Err()
}

var _ repository.UnlockProbeResultRepository = (*unlockProbeResultRepo)(nil)

// Ensure strings import is used if needed
