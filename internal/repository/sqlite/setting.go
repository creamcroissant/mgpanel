// 文件路径: internal/repository/sqlite/setting.go
// 模块说明: 这是 internal 模块里的 setting 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

type settingRepo struct {
	db    *sql.DB
	cache cache.Store
}

func (r *settingRepo) Get(ctx context.Context, key string) (*repository.Setting, error) {
	if r.cache != nil {
		if cached, ok := r.cache.Get(ctx, cacheKey(key)); ok {
			if s, ok := cached.(repository.Setting); ok {
				return &s, nil
			}
		}
	}

	const query = `SELECT key, value, category, updated_at FROM settings WHERE key = ?`
	row := r.db.QueryRowContext(ctx, query, key)
	var s repository.Setting
	if err := row.Scan(&s.Key, &s.Value, &s.Category, &s.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	if r.cache != nil {
		_ = r.cache.Set(ctx, cacheKey(key), s, 0)
	}
	return &s, nil
}

func cacheKey(key string) string {
	return "setting:" + key
}

func (r *settingRepo) Upsert(ctx context.Context, setting *repository.Setting) error {
	const stmt = `INSERT INTO settings(key, value, category, updated_at) VALUES(?, ?, ?, ?)
                  ON CONFLICT(key) DO UPDATE SET value = excluded.value, category = excluded.category, updated_at = excluded.updated_at`
	_, err := execWithRetry(ctx, r.db, stmt, setting.Key, setting.Value, setting.Category, setting.UpdatedAt)
	if err != nil {
		return err
	}
	if r.cache != nil {
		r.cache.Delete(ctx, cacheKey(setting.Key))
	}
	return nil
}

func (r *settingRepo) List(ctx context.Context) ([]repository.Setting, error) {
	const query = `SELECT key, value, category, updated_at FROM settings ORDER BY key`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []repository.Setting
	for rows.Next() {
		var s repository.Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.Category, &s.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *settingRepo) ListByCategory(ctx context.Context, category string) ([]repository.Setting, error) {
	const query = `SELECT key, value, category, updated_at FROM settings WHERE category = ? ORDER BY key`
	rows, err := r.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []repository.Setting
	for rows.Next() {
		var s repository.Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.Category, &s.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}
