// 文件路径: internal/repository/sqlite/stat_user.go
// 模块说明: 这是 internal 模块里的 stat_user 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

type statUserRepo struct {
	db *sql.DB
	cache cache.Store
}

func (r *statUserRepo) Upsert(ctx context.Context, record repository.StatUserRecord) error {
	const stmt = `INSERT INTO stat_users(user_id, agent_host_id, server_rate, record_at, record_type, u, d, created_at, updated_at)
                  VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
                  ON CONFLICT(user_id, agent_host_id, record_type, record_at) DO UPDATE SET
                      u = stat_users.u + excluded.u,
                      d = stat_users.d + excluded.d,
                      server_rate = excluded.server_rate,
                      updated_at = excluded.updated_at`
	_, err := execWithRetry(ctx, r.db, stmt,
		record.UserID,
		record.AgentHostID,
		record.ServerRate,
		record.RecordAt,
		record.RecordType,
		record.Upload,
		record.Download,
		record.CreatedAt,
		record.UpdatedAt,
	)
	return err
}

func (r *statUserRepo) UpsertBatch(ctx context.Context, records []repository.StatUserRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := beginTxWithRetry(ctx, r.db)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const stmt = `INSERT INTO stat_users(user_id, agent_host_id, server_rate, record_at, record_type, u, d, created_at, updated_at)
	                  VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	                  ON CONFLICT(user_id, agent_host_id, record_type, record_at) DO UPDATE SET
	                      u = stat_users.u + excluded.u,
	                      d = stat_users.d + excluded.d,
	                      server_rate = excluded.server_rate,
	                      updated_at = excluded.updated_at`
	prepared, err := tx.PrepareContext(ctx, stmt)
	if err != nil {
		return err
	}
	defer prepared.Close()

	for _, record := range records {
		if _, err := prepared.ExecContext(ctx,
			record.UserID,
			record.AgentHostID,
			record.ServerRate,
			record.RecordAt,
			record.RecordType,
			record.Upload,
			record.Download,
			record.CreatedAt,
			record.UpdatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *statUserRepo) ListByRecord(ctx context.Context, recordType int, recordAt int64, agentHostID *int64, limit int) ([]repository.StatUserRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	var query string
	var args []any

	if agentHostID != nil {
		query = `SELECT user_id, agent_host_id, server_rate, record_at, record_type, u, d, created_at, updated_at
			FROM stat_users
			WHERE record_type = ? AND record_at = ? AND agent_host_id = ?
			ORDER BY (u + d) DESC
			LIMIT ?`
		args = []any{recordType, recordAt, *agentHostID, limit}
	} else {
		query = `SELECT user_id, agent_host_id, server_rate, record_at, record_type, u, d, created_at, updated_at
			FROM stat_users
			WHERE record_type = ? AND record_at = ?
			ORDER BY (u + d) DESC
			LIMIT ?`
		args = []any{recordType, recordAt, limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]repository.StatUserRecord, 0, limit)
	for rows.Next() {
		record := repository.StatUserRecord{}
		if err := rows.Scan(
			&record.UserID,
			&record.AgentHostID,
			&record.ServerRate,
			&record.RecordAt,
			&record.RecordType,
			&record.Upload,
			&record.Download,
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

func (r *statUserRepo) ListByUserSince(ctx context.Context, userID int64, since int64, limit int) ([]repository.StatUserRecord, error) {
	if limit <= 0 {
		limit = 62
	}
	if limit > 200 {
		limit = 200
	}
	const query = `SELECT user_id, agent_host_id, server_rate, record_at, record_type, u, d, created_at, updated_at
		FROM stat_users
		WHERE user_id = ? AND record_at >= ? AND record_type = 1
		ORDER BY record_at DESC
		LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, userID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]repository.StatUserRecord, 0, limit)
	for rows.Next() {
		record := repository.StatUserRecord{}
		if err := rows.Scan(
			&record.UserID,
			&record.AgentHostID,
			&record.ServerRate,
			&record.RecordAt,
			&record.RecordType,
			&record.Upload,
			&record.Download,
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

func (r *statUserRepo) SumByRange(ctx context.Context, filter repository.StatUserSumFilter) (repository.StatUserSumResult, error) {
	key := fmt.Sprintf("SumByRange_%d_%v_%v_%d_%d", filter.RecordType, ptrInt64(filter.UserID), ptrInt64(filter.AgentHostID), filter.StartAt, filter.EndAt)
	if r.cache != nil {
		var cached repository.StatUserSumResult
		if ok, _ := r.cache.Namespace("stat_user").GetJSON(ctx, key, &cached); ok {
			return cached, nil
		}
	}
	recordType := filter.RecordType
	if recordType == 0 {
		recordType = 1 // Default to daily
	}
	query := `SELECT COALESCE(SUM(u), 0) AS upload, COALESCE(SUM(d), 0) AS download
	          FROM stat_users
	          WHERE record_type = ?`
	args := make([]any, 0, 5)
	args = append(args, recordType)
	if filter.UserID != nil {
		query += ` AND user_id = ?`
		args = append(args, *filter.UserID)
	}
	if filter.AgentHostID != nil {
		query += ` AND agent_host_id = ?`
		args = append(args, *filter.AgentHostID)
	}
	if filter.StartAt > 0 {
		query += ` AND record_at >= ?`
		args = append(args, filter.StartAt)
	}
	if filter.EndAt > 0 {
		query += ` AND record_at < ?`
		args = append(args, filter.EndAt)
	}
	var result repository.StatUserSumResult
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&result.Upload, &result.Download); err != nil {
		return repository.StatUserSumResult{}, err
	}
	if r.cache != nil {
		_ = r.cache.Namespace("stat_user").SetJSON(ctx, key, result, 15*time.Second)
	}
	return result, nil
}

func (r *statUserRepo) TopByRange(ctx context.Context, filter repository.StatUserTopFilter) ([]repository.StatUserAggregate, error) {
	key := fmt.Sprintf("TopByRange_%d_%v_%d_%d_%d", filter.RecordType, ptrInt64(filter.AgentHostID), filter.StartAt, filter.EndAt, filter.Limit)
	if r.cache != nil {
		var cached []repository.StatUserAggregate
		if ok, _ := r.cache.Namespace("stat_user").GetJSON(ctx, key, &cached); ok && len(cached) > 0 {
			return cached, nil
		}
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	recordType := filter.RecordType
	if recordType == 0 {
		recordType = 1 // Default to daily
	}
	query := `SELECT user_id,
	                 COALESCE(SUM(u), 0) AS upload,
	                 COALESCE(SUM(d), 0) AS download
	          FROM stat_users
	          WHERE record_type = ?`
	args := make([]any, 0, 5)
	args = append(args, recordType)
	if filter.AgentHostID != nil {
		query += ` AND agent_host_id = ?`
		args = append(args, *filter.AgentHostID)
	}
	if filter.StartAt > 0 {
		query += ` AND record_at >= ?`
		args = append(args, filter.StartAt)
	}
	if filter.EndAt > 0 {
		query += ` AND record_at < ?`
		args = append(args, filter.EndAt)
	}
	query += ` GROUP BY user_id ORDER BY (upload + download) DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var aggregates []repository.StatUserAggregate
	for rows.Next() {
		var agg repository.StatUserAggregate
		if err := rows.Scan(&agg.UserID, &agg.Upload, &agg.Download); err != nil {
			return nil, err
		}
		aggregates = append(aggregates, agg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if r.cache != nil && len(aggregates) > 0 {
		_ = r.cache.Namespace("stat_user").SetJSON(ctx, key, aggregates, 15*time.Second)
	}
	return aggregates, nil
}

// ListByAgentHost returns traffic records for a specific agent host.
func (r *statUserRepo) ListByAgentHost(ctx context.Context, agentHostID int64, recordType int, since int64, limit int) ([]repository.StatUserRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	const query = `SELECT user_id, agent_host_id, server_rate, record_at, record_type, u, d, created_at, updated_at
		FROM stat_users
		WHERE agent_host_id = ? AND record_type = ? AND record_at >= ?
		ORDER BY record_at DESC
		LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, agentHostID, recordType, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]repository.StatUserRecord, 0, limit)
	for rows.Next() {
		record := repository.StatUserRecord{}
		if err := rows.Scan(
			&record.UserID,
			&record.AgentHostID,
			&record.ServerRate,
			&record.RecordAt,
			&record.RecordType,
			&record.Upload,
			&record.Download,
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

// SumByAgentHost returns total traffic for a specific agent host.
func (r *statUserRepo) SumByAgentHost(ctx context.Context, agentHostID int64, recordType int, startAt, endAt int64) (repository.StatUserSumResult, error) {
	if recordType == 0 {
		recordType = 1 // Default to daily
	}
	query := `SELECT COALESCE(SUM(u), 0) AS upload, COALESCE(SUM(d), 0) AS download
	          FROM stat_users
	          WHERE agent_host_id = ? AND record_type = ? AND record_at >= ? AND record_at < ?`
	var result repository.StatUserSumResult
	if err := r.db.QueryRowContext(ctx, query, agentHostID, recordType, startAt, endAt).Scan(&result.Upload, &result.Download); err != nil {
		return repository.StatUserSumResult{}, err
	}
	return result, nil
}
