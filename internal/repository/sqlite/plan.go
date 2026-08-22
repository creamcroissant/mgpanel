// 文件路径: internal/repository/sqlite/plan.go
// 模块说明: 这是 internal 模块里的 plan 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type planRepo struct {
	db *sql.DB
}

func (r *planRepo) ListVisible(ctx context.Context) ([]*repository.Plan, error) {
	rows, err := r.db.QueryContext(ctx, listVisibleQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []*repository.Plan
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return plans, nil
}

func (r *planRepo) ListAll(ctx context.Context) ([]*repository.Plan, error) {
	rows, err := r.db.QueryContext(ctx, listAllQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []*repository.Plan
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return plans, nil
}

func (r *planRepo) FindByID(ctx context.Context, id int64) (*repository.Plan, error) {
	row := r.db.QueryRowContext(ctx, planByIDQuery, id)
	plan, err := scanPlan(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return plan, nil
}

func (r *planRepo) Create(ctx context.Context, plan *repository.Plan) (*repository.Plan, error) {
	if plan == nil {
		return nil, errors.New("plan 不能为空")
	}
	const stmt = `INSERT INTO plans (
		group_id, name, prices, sell, transfer_enable, speed_limit, device_limit,
		show, renew, content, tags, reset_traffic_method, capacity_limit, sort, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	tags, err := encodeStringSlice(plan.Tags)
	if err != nil {
		return nil, fmt.Errorf("encode plan tags: %w", err)
	}
	pricesJSON, err := encodePriceMap(plan.Prices)
	if err != nil {
		return nil, fmt.Errorf("encode plan prices: %w", err)
	}
	result, err := r.db.ExecContext(ctx, stmt,
		optionalInt64(plan.GroupID),
		plan.Name,
		pricesJSON,
		boolToInt(plan.Sell),
		plan.TransferEnable,
		optionalInt64(plan.SpeedLimit),
		optionalInt64(plan.DeviceLimit),
		boolToInt(plan.Show),
		boolToInt(plan.Renew),
		plan.Content,
		tags,
		optionalInt64(plan.ResetTrafficMethod),
		optionalInt64(plan.CapacityLimit),
		plan.Sort,
		plan.CreatedAt,
		plan.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	plan.ID = id
	return plan, nil
}

func (r *planRepo) Update(ctx context.Context, plan *repository.Plan) error {
	if plan == nil {
		return errors.New("plan is nil / plan 为空")
	}
	const stmt = `UPDATE plans SET
		group_id = ?,
		name = ?,
		prices = ?,
		sell = ?,
		transfer_enable = ?,
		speed_limit = ?,
		device_limit = ?,
		show = ?,
		renew = ?,
		content = ?,
		tags = ?,
		reset_traffic_method = ?,
		capacity_limit = ?,
		sort = ?,
		updated_at = ?
	WHERE id = ?`
	tags, err := encodeStringSlice(plan.Tags)
	if err != nil {
		return fmt.Errorf("encode plan tags: %w", err)
	}
	pricesJSON, err := encodePriceMap(plan.Prices)
	if err != nil {
		return fmt.Errorf("encode plan prices: %w", err)
	}
	result, err := r.db.ExecContext(ctx, stmt,
		optionalInt64(plan.GroupID),
		plan.Name,
		pricesJSON,
		boolToInt(plan.Sell),
		plan.TransferEnable,
		optionalInt64(plan.SpeedLimit),
		optionalInt64(plan.DeviceLimit),
		boolToInt(plan.Show),
		boolToInt(plan.Renew),
		plan.Content,
		tags,
		optionalInt64(plan.ResetTrafficMethod),
		optionalInt64(plan.CapacityLimit),
		plan.Sort,
		plan.UpdatedAt,
		plan.ID,
	)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(result); err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	return nil
}

func (r *planRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM plans WHERE id = ?`, id)
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

func (r *planRepo) Sort(ctx context.Context, ids []int64, updatedAt int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for idx, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE plans SET sort = ?, updated_at = ? WHERE id = ?`, int64(idx+1), updatedAt, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *planRepo) BindGroups(ctx context.Context, planID int64, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO plan_server_groups (plan_id, group_id) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, groupID := range groupIDs {
		if _, err := stmt.ExecContext(ctx, planID, groupID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *planRepo) UnbindGroups(ctx context.Context, planID int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM plan_server_groups WHERE plan_id = ?", planID)
	// idempotent: no error if not found
	return err
}

func (r *planRepo) ReplaceGroups(ctx context.Context, planID int64, groupIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := replacePlanGroupsTx(ctx, tx, planID, groupIDs); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE plans SET updated_at = ? WHERE id = ?", time.Now().Unix(), planID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *planRepo) UpdateWithGroups(ctx context.Context, plan *repository.Plan, groupIDs []int64) error {
	if plan == nil {
		return errors.New("plan is nil / plan 为空")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := updatePlanTx(ctx, tx, plan); err != nil {
		return err
	}
	if err := replacePlanGroupsTx(ctx, tx, plan.ID, groupIDs); err != nil {
		return err
	}

	return tx.Commit()
}

func updatePlanTx(ctx context.Context, tx *sql.Tx, plan *repository.Plan) error {
	const stmt = `UPDATE plans SET
		group_id = ?,
		name = ?,
		prices = ?,
		sell = ?,
		transfer_enable = ?,
		speed_limit = ?,
		device_limit = ?,
		show = ?,
		renew = ?,
		content = ?,
		tags = ?,
		reset_traffic_method = ?,
		capacity_limit = ?,
		sort = ?,
		updated_at = ?
	WHERE id = ?`

	tags, err := encodeStringSlice(plan.Tags)
	if err != nil {
		return fmt.Errorf("encode plan tags: %w", err)
	}
	pricesJSON, err := encodePriceMap(plan.Prices)
	if err != nil {
		return fmt.Errorf("encode plan prices: %w", err)
	}

	result, err := tx.ExecContext(ctx, stmt,
		optionalInt64(plan.GroupID),
		plan.Name,
		pricesJSON,
		boolToInt(plan.Sell),
		plan.TransferEnable,
		optionalInt64(plan.SpeedLimit),
		optionalInt64(plan.DeviceLimit),
		boolToInt(plan.Show),
		boolToInt(plan.Renew),
		plan.Content,
		tags,
		optionalInt64(plan.ResetTrafficMethod),
		optionalInt64(plan.CapacityLimit),
		plan.Sort,
		plan.UpdatedAt,
		plan.ID,
	)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(result); err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	return nil
}

func replacePlanGroupsTx(ctx context.Context, tx *sql.Tx, planID int64, groupIDs []int64) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM plan_server_groups WHERE plan_id = ?", planID); err != nil {
		return err
	} // idempotent: no error if not found
	if len(groupIDs) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO plan_server_groups (plan_id, group_id) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, groupID := range groupIDs {
		if _, err := stmt.ExecContext(ctx, planID, groupID); err != nil {
			return err
		}
	}
	return nil
}

func (r *planRepo) GetGroups(ctx context.Context, planID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT group_id FROM plan_server_groups WHERE plan_id = ?", planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groupIDs []int64
	for rows.Next() {
		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, groupID)
	}
	return groupIDs, rows.Err()
}

type planScanner interface {
	Scan(dest ...any) error
}

func scanPlan(scanner planScanner) (*repository.Plan, error) {
	var (
		id             int64
		groupID        sql.NullInt64
		name           string
		prices         sql.NullString
		sellFlag       int64
		transferEnable int64
		speedLimit     sql.NullInt64
		deviceLimit    sql.NullInt64
		showFlag       int64
		renewFlag      int64
		content        sql.NullString
		tags           sql.NullString
		resetMethod    sql.NullInt64
		capacityLimit  sql.NullInt64
		sort           int64
		createdAt      int64
		updatedAt      int64
	)

	if err := scanner.Scan(
		&id,
		&groupID,
		&name,
		&prices,
		&sellFlag,
		&transferEnable,
		&speedLimit,
		&deviceLimit,
		&showFlag,
		&renewFlag,
		&content,
		&tags,
		&resetMethod,
		&capacityLimit,
		&sort,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}

	decodedTags, err := decodeJSONSlice(tags.String)
	if err != nil {
		return nil, fmt.Errorf("decode plan tags: %w", err)
	}

	pricesMap, err := decodePriceMap(prices.String)
	if err != nil {
		return nil, fmt.Errorf("decode plan prices: %w", err)
	}

	return &repository.Plan{
		ID:                 id,
		GroupID:            nullableIntPtr(groupID),
		Name:               name,
		Prices:             pricesMap,
		Sell:               sellFlag == 1,
		TransferEnable:     transferEnable,
		SpeedLimit:         nullableIntPtr(speedLimit),
		DeviceLimit:        nullableIntPtr(deviceLimit),
		Show:               showFlag == 1,
		Renew:              renewFlag == 1,
		Content:            content.String,
		Tags:               decodedTags,
		ResetTrafficMethod: nullableIntPtr(resetMethod),
		CapacityLimit:      nullableIntPtr(capacityLimit),
		Sort:               sort,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}, nil
}

const (
	planColumns = `id,
	       group_id,
	       name,
	       prices,
	       sell,
	       transfer_enable,
	       speed_limit,
	       device_limit,
	       show,
	       renew,
	       content,
	       tags,
	       reset_traffic_method,
	       capacity_limit,
	       sort,
	       created_at,
	       updated_at`
	listVisibleQuery = "SELECT " + planColumns + " FROM plans WHERE show = 1 AND sell = 1 ORDER BY sort ASC, id ASC"
	listAllQuery     = "SELECT " + planColumns + " FROM plans ORDER BY sort ASC, id ASC"
	planByIDQuery    = "SELECT " + planColumns + " FROM plans WHERE id = ? LIMIT 1"
)

// Removed duplicate nullableIntPtr declaration

func decodePriceMap(raw string) (map[string]float64, error) {
	if raw == "" {
		return nil, nil
	}
	var data map[string]float64
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func encodePriceMap(prices map[string]float64) (any, error) {
	if len(prices) == 0 {
		return nil, nil
	}
	buf, err := json.Marshal(prices)
	if err != nil {
		return nil, err
	}
	return string(buf), nil
}
