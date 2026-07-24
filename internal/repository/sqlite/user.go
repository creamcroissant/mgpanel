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

	"github.com/creamcroissant/xboard/internal/repository"
)

// userRepo 负责 users 表的 SQLite 实现。
type userRepo struct {
	db *sql.DB
}

func (r *userRepo) FindByID(ctx context.Context, id int64) (*repository.User, error) {
	// 按 ID 查询用户。
	query, err := userSelectBy("id")
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, query, id)
	return scanUser(row)
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
	// 按订阅 token 查询用户。
	query, err := userSelectBy("token")
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, query, token)
	return scanUser(row)
}

func (r *userRepo) Save(ctx context.Context, user *repository.User) error {
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
		plan_id,
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
		              VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
	                plan_id = excluded.plan_id,
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
	_, err = r.db.ExecContext(ctx, stmt,
		user.ID,
		user.UUID,
		user.Token,
		user.Username,
		user.Email,
		user.Password,
		user.PasswordAlgo,
		user.PasswordSalt,
		user.BalanceCents,
		user.PlanID,
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
		plan_id,
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
		              VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now().Unix()
	user.CreatedAt = now
	user.UpdatedAt = now

	tags, err := encodeStringSlice(user.Tags)
	if err != nil {
		return nil, fmt.Errorf("encode user tags: %w", err)
	}
	res, err := r.db.ExecContext(ctx, stmt,
		user.UUID,
		user.Token,
		user.Username,
		user.Email,
		user.Password,
		user.PasswordAlgo,
		user.PasswordSalt,
		user.BalanceCents,
		user.PlanID,
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

func (r *userRepo) ActiveCountByPlan(ctx context.Context, planID int64, nowUnix int64) (int64, error) {
	// 统计套餐下仍处于有效期的用户数量。
	query := `SELECT COUNT(*) FROM users WHERE plan_id = ? AND (expired_at = 0 OR expired_at > ?)`
	var count int64
	err := r.db.QueryRowContext(ctx, query, planID, nowUnix).Scan(&count)
	return count, err
}

func (r *userRepo) AdjustBalance(ctx context.Context, userID int64, deltaCents int64) (bool, error) {
	// 调整余额并确保不为负。
	res, err := r.db.ExecContext(ctx, `UPDATE users SET balance = balance + ? WHERE id = ? AND (balance + ?) >= 0`, deltaCents, userID, deltaCents)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected(); if err != nil { return false, err }
	return affected > 0, nil
}

func (r *userRepo) IncrementTraffic(ctx context.Context, userID int64, uploadDelta, downloadDelta int64) error {
	// NOTE: no RowsAffected check — incrementing traffic is fire-and-forget;
	// a missing user row is a no-op and not treated as error.
	_, err := r.db.ExecContext(ctx, `UPDATE users SET u = u + ?, d = d + ? WHERE id = ?`, uploadDelta, downloadDelta, userID)
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
	// This is tricky: users are related to plans, plans are related to groups (now via junction table).
	// But `users` table also has `group_id` legacy field?
	// The requirement is usually: User has Plan -> Plan has Groups -> User can access Groups.
	// But V2bX/XBoard often syncs users to nodes based on Plan/Group permissions.
	// The original query likely used `users.group_id` or `users.plan_id`.
	// Let's assume we fetch users whose Plan allows access to any of the groupIDs.
	// This requires joining users, plans, plan_server_groups.

	placeholders := make([]string, len(groupIDs))
	args := make([]any, len(groupIDs)+1)
	for i, id := range groupIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	args[len(groupIDs)] = nowUnix

	// Join users -> plans -> plan_server_groups
	// We want users where plan_server_groups.group_id IN (...)
	query := `
		SELECT u.id, u.uuid, u.email, u.speed_limit, u.device_limit
		FROM users u
		JOIN plan_server_groups psg ON u.plan_id = psg.plan_id
		WHERE psg.group_id IN (` + strings.Join(placeholders, ",") + `)
		  AND (u.expired_at = 0 OR u.expired_at > ?)
		  AND u.banned = 0
		  AND u.status = 1
	`
	// Wait, we also need to consider legacy `group_id` on user?
	// If user.group_id matches? Usually user.group_id is for manual override or admin?
	// Let's stick to plan-based logic for now as `plan_server_groups` is the new standard.

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*repository.NodeUser
	for rows.Next() {
		var u repository.NodeUser
		var speed, device sql.NullInt64
		if err := rows.Scan(&u.ID, &u.UUID, &u.Email, &speed, &device); err != nil {
			return nil, err
		}
		u.SpeedLimit = nullableIntPtr(speed)
		u.DeviceLimit = nullableIntPtr(device)
		users = append(users, &u)
	}
	return users, rows.Err()
}

func (r *userRepo) PlanCounts(ctx context.Context, planIDs []int64, nowUnix int64) (map[int64]repository.PlanUserCount, error) {
	if len(planIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(planIDs))
	args := make([]any, len(planIDs))
	for i, id := range planIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `SELECT plan_id, COUNT(*), SUM(CASE WHEN (expired_at = 0 OR expired_at > ?) AND banned = 0 THEN 1 ELSE 0 END)
	          FROM users WHERE plan_id IN (` + strings.Join(placeholders, ",") + `) GROUP BY plan_id`
	
	// Prepend nowUnix to args for the SUM condition
	finalArgs := append([]any{nowUnix}, args...)
	
	rows, err := r.db.QueryContext(ctx, query, finalArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]repository.PlanUserCount)
	for rows.Next() {
		var planID int64
		var total, active int64
		if err := rows.Scan(&planID, &total, &active); err != nil {
			return nil, err
		}
		result[planID] = repository.PlanUserCount{Total: total, Active: active}
	}
	return result, rows.Err()
}

func (r *userRepo) Search(ctx context.Context, filter repository.UserSearchFilter) ([]*repository.User, error) {
	baseQuery := `SELECT id, uuid, token, username, email, password, password_algo, password_salt, balance, plan_id, group_id, expired_at, u, d, transfer_enable, speed_limit, device_limit, is_admin, status, banned, traffic_exceeded, last_login_at, remarks, tags, created_at, updated_at FROM users`
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
	if filter.PlanID != nil {
		conds = append(conds, "plan_id = ?")
		args = append(args, *filter.PlanID)
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
	if filter.PlanID != nil {
		conds = append(conds, "plan_id = ?")
		args = append(args, *filter.PlanID)
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
		&u.PlanID,
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
	const cols = `id, uuid, token, username, email, password, password_algo, password_salt, balance, plan_id, group_id, expired_at, u, d, transfer_enable, speed_limit, device_limit, is_admin, status, banned, traffic_exceeded, last_login_at, remarks, tags, created_at, updated_at`
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
	_, err := r.db.ExecContext(ctx, `UPDATE users SET traffic_exceeded = ? WHERE id = ?`, val, userID)
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
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
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
