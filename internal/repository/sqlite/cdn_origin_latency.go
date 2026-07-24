package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

// ----------------------------------------------------------------
// CDN Origin Latency
// ----------------------------------------------------------------

type cdnOriginLatencyRepo struct {
	db *sql.DB
}

func newCDNOriginLatencyRepo(db *sql.DB) *cdnOriginLatencyRepo {
	return &cdnOriginLatencyRepo{db: db}
}

func (r *cdnOriginLatencyRepo) Upsert(ctx context.Context, siteID int64, stack string, latencyMs int64) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cdn_origin_latency (site_id, stack, latency_ms, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(site_id, stack) DO UPDATE SET
			latency_ms = excluded.latency_ms,
			updated_at = excluded.updated_at
	`, siteID, stack, latencyMs, now)
	return err
}

func (r *cdnOriginLatencyRepo) ListBySiteID(ctx context.Context, siteID int64) ([]*repository.CDNOriginLatency, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, site_id, stack, latency_ms, updated_at
		FROM cdn_origin_latency
		WHERE site_id = ?
		ORDER BY stack ASC
	`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*repository.CDNOriginLatency
	for rows.Next() {
		lat, err := r.scanLatencies(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, lat)
	}
	return results, rows.Err()
}

func (r *cdnOriginLatencyRepo) ListAll(ctx context.Context) ([]*repository.CDNOriginLatency, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, site_id, stack, latency_ms, updated_at
		FROM cdn_origin_latency
		ORDER BY site_id ASC, stack ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*repository.CDNOriginLatency
	for rows.Next() {
		lat, err := r.scanLatencies(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, lat)
	}
	return results, rows.Err()
}

func (r *cdnOriginLatencyRepo) scanLatencies(rows *sql.Rows) (*repository.CDNOriginLatency, error) {
	var lat repository.CDNOriginLatency
	err := rows.Scan(
		&lat.ID, &lat.SiteID, &lat.Stack,
		&lat.LatencyMs, &lat.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &lat, nil
}
