package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

type statServerRepo struct {
	db *sql.DB
	cache cache.Store
}

func (r *statServerRepo) Upsert(ctx context.Context, record repository.StatServerRecord) error {
	const stmt = `INSERT INTO stat_servers(server_id, record_at, record_type, upload, download, cpu_avg, mem_used, mem_total, disk_used, disk_total, online_users, created_at, updated_at)
                  VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                  ON CONFLICT(server_id, record_type, record_at) DO UPDATE SET
                      upload = stat_servers.upload + excluded.upload,
                      download = stat_servers.download + excluded.download,
                      cpu_avg = excluded.cpu_avg,
                      mem_used = excluded.mem_used,
                      mem_total = excluded.mem_total,
                      disk_used = excluded.disk_used,
                      disk_total = excluded.disk_total,
                      online_users = excluded.online_users,
                      updated_at = excluded.updated_at`
	_, err := r.db.ExecContext(ctx, stmt,
		record.ServerID,
		record.RecordAt,
		record.RecordType,
		record.Upload,
		record.Download,
		record.CPUAvg,
		record.MemUsed,
		record.MemTotal,
		record.DiskUsed,
		record.DiskTotal,
		record.OnlineUsers,
		record.CreatedAt,
		record.UpdatedAt,
	)
	return err
}

func (r *statServerRepo) ListByServer(ctx context.Context, serverID int64, recordType int, since int64, limit int) ([]repository.StatServerRecord, error) {
	if limit <= 0 {
		limit = 62
	}
	if limit > 500 {
		limit = 500
	}
	const query = `SELECT id, server_id, record_at, record_type, upload, download, cpu_avg, mem_used, mem_total, disk_used, disk_total, online_users, created_at, updated_at
		FROM stat_servers
		WHERE server_id = ? AND record_type = ? AND record_at >= ?
		ORDER BY record_at DESC
		LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, serverID, recordType, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]repository.StatServerRecord, 0, limit)
	for rows.Next() {
		record := repository.StatServerRecord{}
		if err := rows.Scan(
			&record.ID,
			&record.ServerID,
			&record.RecordAt,
			&record.RecordType,
			&record.Upload,
			&record.Download,
			&record.CPUAvg,
			&record.MemUsed,
			&record.MemTotal,
			&record.DiskUsed,
			&record.DiskTotal,
			&record.OnlineUsers,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *statServerRepo) SumByRange(ctx context.Context, filter repository.StatServerSumFilter) (repository.StatServerSumResult, error) {
	key := fmt.Sprintf("SumByRange_%d_%v_%d_%d", filter.RecordType, ptrInt64(filter.ServerID), filter.StartAt, filter.EndAt)
	if r.cache != nil {
		var cached repository.StatServerSumResult
		if ok, _ := r.cache.Namespace("stat_server").GetJSON(ctx, key, &cached); ok {
			return cached, nil
		}
	}
	query := `SELECT COALESCE(SUM(upload), 0) AS upload, COALESCE(SUM(download), 0) AS download
	          FROM stat_servers
	          WHERE record_type = ?`
	args := make([]any, 0, 4)
	args = append(args, filter.RecordType)

	if filter.ServerID != nil {
		query += ` AND server_id = ?`
		args = append(args, *filter.ServerID)
	}
	if filter.StartAt > 0 {
		query += ` AND record_at >= ?`
		args = append(args, filter.StartAt)
	}
	if filter.EndAt > 0 {
		query += ` AND record_at < ?`
		args = append(args, filter.EndAt)
	}

	var result repository.StatServerSumResult
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&result.Upload, &result.Download); err != nil {
		return repository.StatServerSumResult{}, err
	}
	if r.cache != nil {
		_ = r.cache.Namespace("stat_server").SetJSON(ctx, key, result, 15*time.Second)
	}
	return result, nil
}

func (r *statServerRepo) TopByRange(ctx context.Context, filter repository.StatServerTopFilter) ([]repository.StatServerAggregate, error) {
	key := fmt.Sprintf("TopByRange_%d_%d_%d_%d", filter.RecordType, filter.StartAt, filter.EndAt, filter.Limit)
	if r.cache != nil {
		var cached []repository.StatServerAggregate
		if ok, _ := r.cache.Namespace("stat_server").GetJSON(ctx, key, &cached); ok && len(cached) > 0 {
			return cached, nil
		}
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	query := `SELECT server_id,
	                 COALESCE(SUM(upload), 0) AS upload,
	                 COALESCE(SUM(download), 0) AS download
	          FROM stat_servers
	          WHERE record_type = ?`
	args := make([]any, 0, 4)
	args = append(args, filter.RecordType)

	if filter.StartAt > 0 {
		query += ` AND record_at >= ?`
		args = append(args, filter.StartAt)
	}
	if filter.EndAt > 0 {
		query += ` AND record_at < ?`
		args = append(args, filter.EndAt)
	}

	query += ` GROUP BY server_id ORDER BY (upload + download) DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var aggregates []repository.StatServerAggregate
	for rows.Next() {
		var agg repository.StatServerAggregate
		if err := rows.Scan(&agg.ServerID, &agg.Upload, &agg.Download); err != nil {
			return nil, err
		}
		aggregates = append(aggregates, agg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if r.cache != nil && len(aggregates) > 0 {
		_ = r.cache.Namespace("stat_server").SetJSON(ctx, key, aggregates, 15*time.Second)
	}
	return aggregates, nil
}
