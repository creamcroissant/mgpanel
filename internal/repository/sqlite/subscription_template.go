// 文件路径: internal/repository/sqlite/subscription_template.go
// 模块说明: 订阅模板仓库的 SQLite 实现
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type subscriptionTemplateRepo struct {
	db *sql.DB
}

func newSubscriptionTemplateRepo(db *sql.DB) *subscriptionTemplateRepo {
	return &subscriptionTemplateRepo{db: db}
}

func (r *subscriptionTemplateRepo) Create(ctx context.Context, tpl *repository.SubscriptionTemplate) error {
	now := time.Now().Unix()
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO subscription_templates (name, description, type, content, is_default, is_public, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, tpl.Name, tpl.Description, tpl.Type, tpl.Content, boolToInt(tpl.IsDefault), boolToInt(tpl.IsPublic), tpl.SortOrder, now, now)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	tpl.ID = id
	tpl.CreatedAt = now
	tpl.UpdatedAt = now
	return nil
}

func (r *subscriptionTemplateRepo) FindByID(ctx context.Context, id int64) (*repository.SubscriptionTemplate, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, type, content, is_default, is_public, sort_order, created_at, updated_at
		FROM subscription_templates WHERE id = ?
	`, id)
	return r.scanTemplate(row)
}

func (r *subscriptionTemplateRepo) FindDefaultByType(ctx context.Context, templateType string) (*repository.SubscriptionTemplate, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, type, content, is_default, is_public, sort_order, created_at, updated_at
		FROM subscription_templates WHERE type = ? AND is_default = 1 LIMIT 1
	`, templateType)
	return r.scanTemplate(row)
}

func (r *subscriptionTemplateRepo) ListByType(ctx context.Context, templateType string) ([]*repository.SubscriptionTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, type, content, is_default, is_public, sort_order, created_at, updated_at
		FROM subscription_templates WHERE type = ? ORDER BY sort_order ASC, id ASC
	`, templateType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanTemplates(rows)
}

func (r *subscriptionTemplateRepo) ListPublic(ctx context.Context) ([]*repository.SubscriptionTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, type, content, is_default, is_public, sort_order, created_at, updated_at
		FROM subscription_templates WHERE is_public = 1 ORDER BY type ASC, sort_order ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanTemplates(rows)
}

func (r *subscriptionTemplateRepo) Update(ctx context.Context, tpl *repository.SubscriptionTemplate) error {
	now := time.Now().Unix()
	result, err := r.db.ExecContext(ctx, `
		UPDATE subscription_templates
		SET name = ?, description = ?, type = ?, content = ?, is_default = ?, is_public = ?, sort_order = ?, updated_at = ?
		WHERE id = ?
	`, tpl.Name, tpl.Description, tpl.Type, tpl.Content, boolToInt(tpl.IsDefault), boolToInt(tpl.IsPublic), tpl.SortOrder, now, tpl.ID)
	if err != nil {
		return err
	}
	tpl.UpdatedAt = now
	return ensureRowsAffected(result)
}

func (r *subscriptionTemplateRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM subscription_templates WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *subscriptionTemplateRepo) SetDefault(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var templateType string
	if err := tx.QueryRowContext(ctx, `SELECT type FROM subscription_templates WHERE id = ?`, id).Scan(&templateType); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return err
	}

	now := time.Now().Unix()

	if _, err := tx.ExecContext(ctx, `
		UPDATE subscription_templates SET is_default = 0, updated_at = ? WHERE type = ?
	`, now, templateType); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE subscription_templates SET is_default = 1, updated_at = ? WHERE id = ?
	`, now, id); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *subscriptionTemplateRepo) scanTemplate(row *sql.Row) (*repository.SubscriptionTemplate, error) {
	var tpl repository.SubscriptionTemplate
	var isDefault, isPublic int
	err := row.Scan(
		&tpl.ID,
		&tpl.Name,
		&tpl.Description,
		&tpl.Type,
		&tpl.Content,
		&isDefault,
		&isPublic,
		&tpl.SortOrder,
		&tpl.CreatedAt,
		&tpl.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	tpl.IsDefault = isDefault == 1
	tpl.IsPublic = isPublic == 1
	return &tpl, nil
}

func (r *subscriptionTemplateRepo) scanTemplates(rows *sql.Rows) ([]*repository.SubscriptionTemplate, error) {
	var templates []*repository.SubscriptionTemplate
	for rows.Next() {
		var tpl repository.SubscriptionTemplate
		var isDefault, isPublic int
		err := rows.Scan(
			&tpl.ID,
			&tpl.Name,
			&tpl.Description,
			&tpl.Type,
			&tpl.Content,
			&isDefault,
			&isPublic,
			&tpl.SortOrder,
			&tpl.CreatedAt,
			&tpl.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tpl.IsDefault = isDefault == 1
		tpl.IsPublic = isPublic == 1
		templates = append(templates, &tpl)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return templates, nil
}
