// 文件路径: internal/repository/sqlite/login_log.go
// 模块说明: 这是 internal 模块里的 login_log 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// loginLogRepo persists login attempts into SQLite for auditing.
type loginLogRepo struct {
	db *sql.DB
}

func (r *loginLogRepo) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	const stmt = `DELETE FROM login_logs WHERE created_at < unixepoch() - ? * 86400`
	res, err := execWithRetry(ctx, r.db, stmt, days)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *loginLogRepo) Create(ctx context.Context, logEntry *repository.LoginLog) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("login log repository not configured / 登录日志仓储未配置")
	}
	if logEntry == nil {
		return fmt.Errorf("login log entry is required / 登录日志条目不能为空")
	}
	if strings.TrimSpace(logEntry.Email) == "" {
		return fmt.Errorf("login log email is required / 登录日志邮箱不能为空")
	}
	const stmt = `INSERT INTO login_logs(user_id, email, ip, user_agent, success, reason, created_at, updated_at)
                  VALUES(?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now().Unix()
	created := logEntry.CreatedAt
	if created == 0 {
		created = now
	}
	updated := logEntry.UpdatedAt
	if updated == 0 {
		updated = created
	}
	var userID any
	if logEntry.UserID != nil && *logEntry.UserID > 0 {
		userID = *logEntry.UserID
	}
	_, err := execWithRetry(ctx, r.db,
		
		stmt,
		userID,
		logEntry.Email,
		nullableString(logEntry.IP),
		nullableString(logEntry.UserAgent),
		boolToInt(logEntry.Success),
		nullableString(logEntry.Reason),
		created,
		updated,
	)
	return err
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
