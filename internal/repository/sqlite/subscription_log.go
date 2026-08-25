package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type subscriptionLogRepo struct {
	db *sql.DB
}

func (r *subscriptionLogRepo) Log(ctx context.Context, log *repository.SubscriptionLog) error {
	const query = `INSERT INTO subscription_logs (
		user_id, client_ip, user_agent, request_type, request_url, created_at
	) VALUES (?, ?, ?, ?, ?, ?)`

	now := time.Now().Unix()
	log.CreatedAt = now

	res, err := r.db.ExecContext(ctx, query,
		log.UserID,
		log.IP,
		log.UserAgent,
		log.Type,
		log.URL,
		log.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id
	return nil
}

func (r *subscriptionLogRepo) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	const stmt = `DELETE FROM subscription_logs WHERE created_at < unixepoch() - ? * 86400`
	res, err := r.db.ExecContext(ctx, stmt, days)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *subscriptionLogRepo) GetRecentLogs(ctx context.Context, userID int64, limit int) ([]*repository.SubscriptionLog, error) {
	const query = `SELECT
		id, user_id, client_ip, user_agent, request_type, request_url, created_at
		FROM subscription_logs
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*repository.SubscriptionLog
	for rows.Next() {
		var log repository.SubscriptionLog
		if err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.IP,
			&log.UserAgent,
			&log.Type,
			&log.URL,
			&log.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, &log)
	}
	return logs, rows.Err()
}
