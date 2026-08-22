package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type accessLogRepo struct {
	db *sql.DB
}

func newAccessLogRepo(db *sql.DB) *accessLogRepo {
	return &accessLogRepo{db: db}
}

func (r *accessLogRepo) Create(ctx context.Context, log *repository.AccessLog) error {
	log.CreatedAt = time.Now().Unix()

	query := `
		INSERT INTO access_logs (
			user_id, user_email, agent_host_id, source_ip, target_domain,
			target_ip, target_port, protocol, upload, download,
			connection_start, connection_end, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		log.UserID, log.UserEmail, log.AgentHostID, log.SourceIP, log.TargetDomain,
		log.TargetIP, log.TargetPort, log.Protocol, log.Upload, log.Download,
		log.ConnectionStart, log.ConnectionEnd, log.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id
	return nil
}

func (r *accessLogRepo) BatchCreate(ctx context.Context, logs []*repository.AccessLog) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO access_logs (
			user_id, user_email, agent_host_id, source_ip, target_domain,
			target_ip, target_port, protocol, upload, download,
			connection_start, connection_end, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, log := range logs {
		log.CreatedAt = now
		_, err := stmt.ExecContext(ctx,
			log.UserID, log.UserEmail, log.AgentHostID, log.SourceIP, log.TargetDomain,
			log.TargetIP, log.TargetPort, log.Protocol, log.Upload, log.Download,
			log.ConnectionStart, log.ConnectionEnd, log.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *accessLogRepo) buildFilter(filter repository.AccessLogFilter) (string, []interface{}) {
	query := strings.Builder{}
	args := make([]interface{}, 0)

	query.WriteString(" WHERE 1=1")

	if filter.UserID != nil {
		query.WriteString(" AND user_id = ?")
		args = append(args, *filter.UserID)
	}
	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		args = append(args, *filter.AgentHostID)
	}
	if filter.TargetDomain != nil && *filter.TargetDomain != "" {
		query.WriteString(" AND target_domain LIKE ?")
		args = append(args, "%"+*filter.TargetDomain+"%")
	}
	if filter.SourceIP != nil && *filter.SourceIP != "" {
		query.WriteString(" AND source_ip = ?")
		args = append(args, *filter.SourceIP)
	}
	if filter.Protocol != nil && *filter.Protocol != "" {
		query.WriteString(" AND protocol = ?")
		args = append(args, *filter.Protocol)
	}
	if filter.StartAt != nil {
		query.WriteString(" AND created_at >= ?")
		args = append(args, *filter.StartAt)
	}
	if filter.EndAt != nil {
		query.WriteString(" AND created_at <= ?")
		args = append(args, *filter.EndAt)
	}

	return query.String(), args
}

func (r *accessLogRepo) List(ctx context.Context, filter repository.AccessLogFilter) ([]*repository.AccessLog, error) {
	where, args := r.buildFilter(filter)

	query := `
		SELECT id, user_id, user_email, agent_host_id, source_ip, target_domain,
		       target_ip, target_port, protocol, upload, download,
		       connection_start, connection_end, created_at
		FROM access_logs
	` + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLogs(rows)
}

func (r *accessLogRepo) Count(ctx context.Context, filter repository.AccessLogFilter) (int64, error) {
	where, args := r.buildFilter(filter)
	query := "SELECT COUNT(*) FROM access_logs" + where

	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *accessLogRepo) DeleteByRetentionDays(ctx context.Context, days int) (int64, error) {
	retentionTime := time.Now().AddDate(0, 0, -days).Unix()

	result, err := r.db.ExecContext(ctx, "DELETE FROM access_logs WHERE created_at < ?", retentionTime)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (r *accessLogRepo) GetStats(ctx context.Context, filter repository.AccessLogFilter) (*repository.AccessLogStats, error) {
	where, args := r.buildFilter(filter)

	query := `
		SELECT COUNT(*), COALESCE(SUM(upload), 0), COALESCE(SUM(download), 0)
		FROM access_logs
	` + where

	stats := &repository.AccessLogStats{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&stats.TotalCount, &stats.TotalUpload, &stats.TotalDownload)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *accessLogRepo) scanLogs(rows *sql.Rows) ([]*repository.AccessLog, error) {
	var logs []*repository.AccessLog
	for rows.Next() {
		var log repository.AccessLog
		var userID sql.NullInt64
		var connStart sql.NullInt64
		var connEnd sql.NullInt64

		err := rows.Scan(
			&log.ID, &userID, &log.UserEmail, &log.AgentHostID, &log.SourceIP, &log.TargetDomain,
			&log.TargetIP, &log.TargetPort, &log.Protocol, &log.Upload, &log.Download,
			&connStart, &connEnd, &log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if userID.Valid {
			log.UserID = &userID.Int64
		}
		if connStart.Valid {
			log.ConnectionStart = &connStart.Int64
		}
		if connEnd.Valid {
			log.ConnectionEnd = &connEnd.Int64
		}

		logs = append(logs, &log)
	}
	return logs, rows.Err()
}
