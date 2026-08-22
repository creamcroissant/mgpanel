package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type trafficReportDedupRepo struct {
	db *sql.DB
}

func newTrafficReportDedupRepo(db *sql.DB) repository.TrafficReportDedupRepository {
	return &trafficReportDedupRepo{db: db}
}

func (r *trafficReportDedupRepo) MarkHandled(ctx context.Context, agentHostID int64, reportID string, handledAt int64) (bool, error) {
	if agentHostID <= 0 {
		return false, errors.New("agent_host_id must be positive")
	}
	reportID = strings.TrimSpace(reportID)
	if reportID == "" {
		return false, errors.New("report_id is required")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO traffic_report_dedups (agent_host_id, report_id, handled_at)
		VALUES (?, ?, ?)
	`, agentHostID, reportID, handledAt)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
