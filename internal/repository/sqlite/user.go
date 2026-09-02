// 文件路径: internal/repository/sqlite/user.go
// 模块说明: 这是 internal 模块里的 user 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

// userRepo 负责 users 表的 SQLite 实现。
type userRepo struct {
	db *sql.DB
	cache cache.Store
}

func (r *userRepo) FindByID(ctx context.Context, id int64) (*repository.User, error) {
	if r.cache != nil {
		var cached *repository.User
		if ok, _ := r.cache.Namespace("user").GetJSON(ctx, "FindByID_"+fmt.Sprint(id), &cached); ok && cached != nil {
			return cached, nil
		}
	}
	// 按 ID 查询用户。
	query, err := userSelectBy("id")
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, query, id)
	user, err := scanUser(row)
	if err == nil && r.cache != nil {
		_ = r.cache.Namespace("user").SetJSON(ctx, "FindByID_"+fmt.Sprint(id), user, 30*time.Second)
	}
	return user, err
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*repository.User, error) {
	// 按邮箱查询用户。
	query, err := userSelectBy("email")
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, query, email)
	return scanUser(row)
}

func (r *userRepo) FindByUsername(ctx context.Context, username string) (*repository.User, error) {
	// 按用户名查询用户。
	query, err := userSelectBy("username")
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, query, username)
	return scanUser(row)
}

func (r *userRepo) FindByToken(ctx context.Context, token string) (*repository.User, error) {
	if r.cache != nil && token != "" {
		var cached *repository.User
		if ok, _ := r.cache.Namespace("user").GetJSON(ctx, "FindByToken_"+token, &cached); ok && cached != nil {
			return cached, nil
		}
	}
	// 按订阅 token 查询用户。
	query, err := userSelectBy("token")
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, query, token)
	user, err := scanUser(row)
	if err == nil && r.cache != nil && token != "" {
		_ = r.cache.Namespace("user").SetJSON(ctx, "FindByToken_"+token, user, 30*time.Second)
	}
	return user, err
}

func (r *userRepo) Save(ctx context.Context, user *repository.User) error {
	// 换发 Token 时旧订阅地址必须立即失效：先读库中原 Token，新旧双清
	var oldToken string
	if r.cache != nil && user.Token != "" {
		if old, err := r.FindByID(ctx, user.ID); err == nil && old != nil && old.Token != "" && old.Token != user.Token {
			oldToken = old.Token
		}
	}
	if r.cache != nil {
		r.cache.Namespace("user").Delete(ctx, "FindByID_"+fmt.Sprint(user.ID))
		if user.Token != "" {
			r.cache.Namespace("user").Delete(ctx, "FindByToken_"+user.Token)
		}
		if oldToken != "" {
			r.cache.Namespace("user").Delete(ctx, "FindByToken_"+oldToken)
		}
	}
	// Upsert 用户记录，维护更新时间。
	const stmt = `INSERT INTO users(
		id,
		uuid,
		token,
		username,
		email,
		password,
		password_algo,
		password_salt,
		balance,
		group_id,
		expired_at,
		u,
		d,
		transfer_enable,
		speed_limit,
		device_limit,
		is_admin,
		status,
		banned,
		traffic_exceeded,
		telegram_id,
		last_login_at,
		remarks,
		tags,
		created_at,
		updated_at)
		              VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	              ON CONFLICT(id) DO UPDATE SET
	                uuid = excluded.uuid,
	                is_admin = excluded.is_admin,
	                token = excluded.token,
	                username = excluded.username,
	                email = excluded.email,
	                password = excluded.password,
	                password_algo = excluded.password_algo,
	                password_salt = excluded.password_salt,
	                balance = excluded.balance,
	                group_id = excluded.group_id,
	                expired_at = excluded.expired_at,
	                u = excluded.u,
	                d = excluded.d,
	                transfer_enable = excluded.transfer_enable,
	                speed_limit = excluded.speed_limit,
	                device_limit = excluded.device_limit,
	                status = excluded.status,
	                banned = excluded.banned,
	                traffic_exceeded = excluded.traffic_exceeded,
					telegram_id = excluded.telegram_id,
	                last_login_at = excluded.last_login_at,
					remarks = excluded.remarks,
					tags = excluded.tags,
	                updated_at = excluded.updated_at`

	now := time.Now().Unix()
	if user.CreatedAt == 0 {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	tags, err := encodeStringSlice(user.Tags)
	if err != nil {
		return fmt.Errorf("encode user tags: %w", err)
	}
	_, err = execWithRetry(ctx, r.db, stmt,
		user.ID,
		user.UUID,
		user.Token,
		user.Username,
		user.Email,
		user.Password,
		user.PasswordAlgo,
		user.PasswordSalt,
		user.BalanceCents,
		user.GroupID,
		user.ExpiredAt,
		user.U,
		user.D,
		user.TransferEnable,
		nullableInt(user.SpeedLimit),
		nullableInt(user.DeviceLimit),
		boolToInt(user.IsAdmin),
		user.Status,
		boolToInt(user.Banned),
		boolToInt(user.TrafficExceeded),
		user.TelegramID,
		user.LastLoginAt,
		user.Remarks,
		tags,
		user.CreatedAt,
		user.UpdatedAt,
	)
	return err
}

func (r *userRepo) Create(ctx context.Context, user *repository.User) (*repository.User, error) {
	if r.cache != nil {
		r.cache.Namespace("user").Delete(ctx, "FindByID_"+fmt.Sprint(user.ID))
		if user.Token != "" {
			r.cache.Namespace("user").Delete(ctx, "FindByToken_"+user.Token)
		}
	}
	// 新增用户记录并回填主键。
	const stmt = `INSERT INTO users(
		uuid,
		token,
		username,
		email,
		password,
		password_algo,
		password_salt,
		balance,
		group_id,
		expired_at,
		u,
		d,
		transfer_enable,
		speed_limit,
		device_limit,
		is_admin,
		status,
		banned,
		last_login_at,
		remarks,
		tags,
		created_at,
		updated_at)
		              VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now().Unix()
	user.CreatedAt = now
	user.UpdatedAt = now

	tags, err := encodeStringSlice(user.Tags)
	if err != nil {
		return nil, fmt.Errorf("encode user tags: %w", err)
	}
	res, err := execWithRetry(ctx, r.db, stmt,
		user.UUID,
		user.Token,
		user.Username,
		user.Email,
		user.Password,
		user.PasswordAlgo,
		user.PasswordSalt,
		user.BalanceCents,
		user.GroupID,
		user.ExpiredAt,
		user.U,
		user.D,
		user.TransferEnable,
		nullableInt(user.SpeedLimit),
		nullableInt(user.DeviceLimit),
		boolToInt(user.IsAdmin),
		user.Status,
		boolToInt(user.Banned),
		user.LastLoginAt,
		user.Remarks,
		tags,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if id, err := res.LastInsertId(); err == nil {
		user.ID = id
	}
	return user, nil
}

func (r *userRepo) HasAdmin(ctx context.Context) (bool, error) {
	// 判断是否已有管理员用户。
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE is_admin = 1").Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *userRepo) AdjustBalance(ctx context.Context, userID int64, deltaCents int64) (bool, error) {
	// 调整余额并确保不为负。
	res, err := execWithRetry(ctx, r.db, `UPDATE users SET balance = balance + ? WHERE id = ? AND (balance + ?) >= 0`, deltaCents, userID, deltaCents)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected(); if err != nil { return false, err }
	return affected > 0, nil
}

// IncrementTrafficBatch 在单个事务内批量累加多个用户的流量增量，
// 减少逐条 UPDATE 带来的 fsync 次数（写放大优化）。
func (r *userRepo) IncrementTrafficBatch(ctx context.Context, deltas map[int64][2]int64) error {
	if len(deltas) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `UPDATE users SET u = u + ?, d = d + ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for userID, delta := range deltas {
		if _, err := stmt.ExecContext(ctx, delta[0], delta[1], userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *userRepo) IncrementTraffic(ctx context.Context, userID int64, uploadDelta, downloadDelta int64) error {
	// NOTE: no RowsAffected check — incrementing traffic is fire-and-forget;
	// a missing user row is a no-op and not treated as error.
	_, err := execWithRetry(ctx, r.db, `UPDATE users SET u = u + ?, d = d + ? WHERE id = ?`, uploadDelta, downloadDelta, userID)
	return err
}

func (r *userRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (r *userRepo) CountActive(ctx context.Context, nowUnix int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE expired_at > ? OR expired_at = 0", nowUnix).Scan(&count)
	return count, err
}

func (r *userRepo) CountCreatedBetween(ctx context.Context, startUnix, endUnix int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE created_at >= ? AND created_at <= ?", startUnix, endUnix).Scan(&count)
	return count, err
}

func (r *userRepo) ListActiveForGroups(ctx context.Context, groupIDs []int64, nowUnix int64) ([]*repository.NodeUser, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	// 用户通过自身 group_id 关联节点分组。
	placeholders := make([]string, len(groupIDs))
	args := make([]any, 0, len(groupIDs)+1)
	for i, id := range groupIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, nowUnix)

	query := `
		SELECT u.id, u.uuid, u.email, u.speed_limit, u.device_limit
		FROM users u
		WHERE u.group_id IN (` + strings.Join(placeholders, ",") + `)
		  AND (u.expired_at = 0 OR u.expired_at > ?)
		  AND u.banned = 0
		  AND u.status = 1
	`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*repository.NodeUser
	for rows.Next() {
		var nu repository.NodeUser
		var speedLimit, deviceLimit sql.NullInt64
		if err := rows.Scan(&nu.ID, &nu.UUID, &nu.Email, &speedLimit, &deviceLimit); err != nil {
			return nil, err
		}
		if speedLimit.Valid {
			v := speedLimit.Int64
			nu.SpeedLimit = &v
		}
		if deviceLimit.Valid {
			v := deviceLimit.Int64
			nu.DeviceLimit = &v
		}
		result = append(result, &nu)
	}
	return result, rows.Err()
}

func (r *userRepo) Search(ctx context.Context, filter repository.UserSearchFilter) ([]*repository.User, error) {
	baseQuery := `SELECT id, uuid, token, username, email, password, password_algo, password_salt, balance, group_id, expired_at, u, d, transfer_enable, speed_limit, device_limit, is_admin, status, banned, traffic_exceeded, last_login_at, remarks, tags, created_at, updated_at FROM users`
	var conds []string
	var args []any

	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		conds = append(conds, "(email LIKE ? OR username LIKE ? OR remarks LIKE ?)")
		args = append(args, like, like, like)
	}
	if filter.Status != nil {
		conds = append(conds, "status = ?")
		args = append(args, *filter.Status)
	}

	query := baseQuery
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}

	// Pagination
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*repository.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *userRepo) CountFiltered(ctx context.Context, filter repository.UserSearchFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM users"
	var conds []string
	var args []any

	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		conds = append(conds, "(email LIKE ? OR username LIKE ? OR remarks LIKE ?)")
		args = append(args, like, like, like)
	}
	if filter.Status != nil {
		conds = append(conds, "status = ?")
		args = append(args, *filter.Status)
	}

	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}

	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(row userScanner) (*repository.User, error) {
	var user repository.User
	var speedLimit, deviceLimit sql.NullInt64
	var remarks, tags sql.NullString
	var uuid, token, username, algo, salt string
	var lastLogin int64
	var trafficExceeded int

	var u = &user
	if err := row.Scan(
		&u.ID,
		&uuid,
		&token,
		&username,
		&u.Email,
		&u.Password,
		&algo,
		&salt,
		&u.BalanceCents,
		&u.GroupID,
		&u.ExpiredAt,
		&u.U,
		&u.D,
		&u.TransferEnable,
		&speedLimit,
		&deviceLimit,
		&u.IsAdmin,
		&u.Status,
		&u.Banned,
		&trafficExceeded,
		&lastLogin,
		&remarks,
		&tags,
		&u.CreatedAt,
		&u.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	u.UUID = uuid
	u.Token = token
	u.Username = username
	u.PasswordAlgo = algo
	u.PasswordSalt = salt
	u.LastLoginAt = lastLogin
	u.TrafficExceeded = trafficExceeded == 1
	user.SpeedLimit = nullableIntPtr(speedLimit)
	user.DeviceLimit = nullableIntPtr(deviceLimit)
	if remarks.Valid {
		user.Remarks = remarks.String
	}
	decodedTags, err := decodeJSONSlice(tags.String)
	if err != nil {
		return nil, fmt.Errorf("decode user tags: %w", err)
	}
	user.Tags = decodedTags
	return &user, nil
}

var userSelectByMap = map[string]string{
	"id":       "id",
	"email":    "email",
	"username": "username",
	"token":    "token",
}

func userSelectBy(field string) (string, error) {
	col, ok := userSelectByMap[field]
	if !ok {
		return "", fmt.Errorf("invalid user select field: %s", field)
	}
	const cols = `id, uuid, token, username, email, password, password_algo, password_salt, balance, group_id, expired_at, u, d, transfer_enable, speed_limit, device_limit, is_admin, status, banned, traffic_exceeded, last_login_at, remarks, tags, created_at, updated_at`
	return fmt.Sprintf("SELECT %s FROM users WHERE %s = ?", cols, col), nil
}


// SetTrafficExceeded updates the traffic_exceeded flag for a user.
// NOTE: no RowsAffected check — this is a best-effort flag; a missing user
// row is silently ignored.
func (r *userRepo) SetTrafficExceeded(ctx context.Context, userID int64, exceeded bool) error {
	val := 0
	if exceeded {
		val = 1
	}
	_, err := execWithRetry(ctx, r.db, `UPDATE users SET traffic_exceeded = ? WHERE id = ?`, val, userID)
	return err
}

// GetExceededUserIDs returns all user IDs with traffic_exceeded = 1.
func (r *userRepo) GetExceededUserIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM users WHERE traffic_exceeded = 1`)
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

// Delete removes a user by ID.
func (r *userRepo) Delete(ctx context.Context, id int64) error {
	// 删除前先取旧 Token，用于同时失效订阅缓存键，避免被删用户 30s 内仍可拉订阅
	var oldToken string
	if r.cache != nil {
		if old, err := r.FindByID(ctx, id); err == nil && old != nil {
			oldToken = old.Token
		}
	}
	if r.cache != nil {
		r.cache.Namespace("user").Delete(ctx, "FindByID_"+fmt.Sprint(id))
		if oldToken != "" {
			r.cache.Namespace("user").Delete(ctx, "FindByToken_"+oldToken)
		}
	}
	result, err := execWithRetry(ctx, r.db, `DELETE FROM users WHERE id = ?`, id)
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
