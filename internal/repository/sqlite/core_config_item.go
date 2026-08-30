package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type coreConfigItemRepo struct {
	db *sql.DB
}

func newCoreConfigItemRepo(db *sql.DB) *coreConfigItemRepo {
	return &coreConfigItemRepo{db: db}
}

const coreConfigItemColumns = `id, agent_host_id, core_type, config_type, tag, enabled, config_data, desired_revision, created_by, updated_by, created_at, updated_at`

func scanCoreConfigItem(scanner interface {
	Scan(dest ...interface{}) error
}) (*repository.CoreConfigItem, error) {
	var item repository.CoreConfigItem
	var agentHostID sql.NullInt64
	var configData string
	err := scanner.Scan(
		&item.ID, &agentHostID, &item.CoreType, &item.ConfigType, &item.Tag,
		&item.Enabled, &configData, &item.DesiredRevision,
		&item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if agentHostID.Valid {
		item.AgentHostID = &agentHostID.Int64
	}
	item.ConfigData = json.RawMessage(configData)
	return &item, nil
}

func (r *coreConfigItemRepo) Create(ctx context.Context, item *repository.CoreConfigItem) error {
	now := time.Now().Unix()
	item.CreatedAt = now
	item.UpdatedAt = now
	if item.DesiredRevision == 0 {
		item.DesiredRevision = 1
	}
	configData := string(item.ConfigData)
	if configData == "" {
		configData = "{}"
	}

	result, err := execWithRetry(ctx, r.db, `
		INSERT INTO core_config_items (agent_host_id, core_type, config_type, tag, enabled, config_data, desired_revision, created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.AgentHostID, item.CoreType, item.ConfigType, item.Tag, item.Enabled, configData, item.DesiredRevision,
		item.CreatedBy, item.UpdatedBy, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert core_config_item: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	item.ID = id
	return nil
}

func (r *coreConfigItemRepo) Update(ctx context.Context, item *repository.CoreConfigItem) error {
	item.UpdatedAt = time.Now().Unix()
	configData := string(item.ConfigData)
	if configData == "" {
		configData = "{}"
	}

	result, err := execWithRetry(ctx, r.db, `
		UPDATE core_config_items SET core_type=?, config_type=?, tag=?, enabled=?, config_data=?, desired_revision=?, updated_by=?, updated_at=?
		WHERE id=?
	`, item.CoreType, item.ConfigType, item.Tag, item.Enabled, configData, item.DesiredRevision,
		item.UpdatedBy, item.UpdatedAt, item.ID)
	if err != nil {
		return fmt.Errorf("update core_config_item: %w", err)
	}
	return ensureRowsAffected(result)
}

func (r *coreConfigItemRepo) Delete(ctx context.Context, id int64) error {
	result, err := execWithRetry(ctx, r.db, `DELETE FROM core_config_items WHERE id=?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *coreConfigItemRepo) FindByID(ctx context.Context, id int64) (*repository.CoreConfigItem, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+coreConfigItemColumns+` FROM core_config_items WHERE id=?`, id)
	return scanCoreConfigItem(row)
}

func (r *coreConfigItemRepo) FindByHostCoreTypeTag(ctx context.Context, agentHostID int64, coreType, configType, tag string) (*repository.CoreConfigItem, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+coreConfigItemColumns+` FROM core_config_items WHERE agent_host_id=? AND core_type=? AND config_type=? AND tag=?`, agentHostID, coreType, configType, tag)
	return scanCoreConfigItem(row)
}

func (r *coreConfigItemRepo) FindByCoreTypeTag(ctx context.Context, coreType, configType, tag string) (*repository.CoreConfigItem, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+coreConfigItemColumns+` FROM core_config_items WHERE agent_host_id IS NULL AND core_type=? AND config_type=? AND tag=?`, coreType, configType, tag)
	return scanCoreConfigItem(row)
}

func (r *coreConfigItemRepo) ListByHost(ctx context.Context, agentHostID int64, coreType string, configType *string) ([]*repository.CoreConfigItem, error) {
	query := `SELECT ` + coreConfigItemColumns + ` FROM core_config_items WHERE (agent_host_id=? OR agent_host_id IS NULL) AND core_type=?`
	args := []interface{}{agentHostID, coreType}
	if configType != nil {
		query += ` AND config_type=?`
		args = append(args, *configType)
	}
	query += ` ORDER BY config_type, tag`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*repository.CoreConfigItem
	for rows.Next() {
		item, err := scanCoreConfigItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *coreConfigItemRepo) buildListQuery(filter repository.CoreConfigItemFilter) (string, []interface{}) {
	var clauses []string
	var args []interface{}

	if filter.AgentHostID != nil {
		clauses = append(clauses, "agent_host_id=?")
		args = append(args, *filter.AgentHostID)
	}
	if filter.CoreType != nil {
		clauses = append(clauses, "core_type=?")
		args = append(args, *filter.CoreType)
	}
	if filter.ConfigType != nil {
		clauses = append(clauses, "config_type=?")
		args = append(args, *filter.ConfigType)
	}
	if filter.Tag != nil {
		clauses = append(clauses, "tag=?")
		args = append(args, *filter.Tag)
	}
	if filter.Enabled != nil {
		clauses = append(clauses, "enabled=?")
		args = append(args, *filter.Enabled)
	}
	if filter.IsTemplate != nil && *filter.IsTemplate {
		clauses = append(clauses, "agent_host_id IS NULL")
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	return where, args
}

func (r *coreConfigItemRepo) List(ctx context.Context, filter repository.CoreConfigItemFilter) ([]*repository.CoreConfigItem, error) {
	where, args := r.buildListQuery(filter)
	query := `SELECT ` + coreConfigItemColumns + ` FROM core_config_items` + where + ` ORDER BY config_type, tag`
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*repository.CoreConfigItem
	for rows.Next() {
		item, err := scanCoreConfigItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *coreConfigItemRepo) Count(ctx context.Context, filter repository.CoreConfigItemFilter) (int64, error) {
	where, args := r.buildListQuery(filter)
	query := `SELECT COUNT(*) FROM core_config_items` + where
	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}
