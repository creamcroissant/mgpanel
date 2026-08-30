// 文件路径: internal/repository/sqlite/user_notice_reads.go
// 模块说明: 用户公告已读状态的 SQLite 实现
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type userNoticeReadsRepo struct {
	db *sql.DB
}

func newUserNoticeReadsRepo(db *sql.DB) repository.UserNoticeReadsRepository {
	return &userNoticeReadsRepo{db: db}
}

// MarkRead records that a user has read a notice (idempotent using INSERT OR IGNORE)
func (r *userNoticeReadsRepo) MarkRead(ctx context.Context, userID, noticeID int64) error {
	query := `INSERT OR IGNORE INTO user_notice_reads (user_id, notice_id, read_at) VALUES (?, ?, ?)`
	_, err := execWithRetry(ctx, r.db, query, userID, noticeID, time.Now().Unix())
	return err
}

// HasRead checks if a user has read a specific notice
func (r *userNoticeReadsRepo) HasRead(ctx context.Context, userID, noticeID int64) (bool, error) {
	query := `SELECT 1 FROM user_notice_reads WHERE user_id = ? AND notice_id = ? LIMIT 1`
	var exists int
	err := r.db.QueryRowContext(ctx, query, userID, noticeID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetUnreadPopupNoticeIDs returns IDs of unread popup notices for a user
// Only returns notices where popup=1 AND show=1
func (r *userNoticeReadsRepo) GetUnreadPopupNoticeIDs(ctx context.Context, userID int64) ([]int64, error) {
	query := `
		SELECT n.id
		FROM notices n
		LEFT JOIN user_notice_reads unr ON n.id = unr.notice_id AND unr.user_id = ?
		WHERE n.popup = 1 AND n.show = 1 AND unr.id IS NULL
		ORDER BY n.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
