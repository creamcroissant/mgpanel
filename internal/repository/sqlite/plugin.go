// 文件路径: internal/repository/sqlite/plugin.go
// 模块说明: 这是 internal 模块里的 plugin 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// pluginRepo implements repository.PluginRepository backed by SQLite.
type pluginRepo struct {
	db *sql.DB
}

func (r *pluginRepo) FindEnabledByCode(ctx context.Context, code string) (*repository.Plugin, error) {
	if r == nil || r.db == nil {
		return nil, repository.ErrNotFound
	}

	const query = `SELECT id, code, type, is_enabled, config FROM v2_plugins WHERE code = ? AND is_enabled = 1 LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, code)

	var (
		plugin  repository.Plugin
		enabled int
	)
	if err := row.Scan(&plugin.ID, &plugin.Code, &plugin.Type, &enabled, &plugin.Config); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	plugin.IsEnabled = enabled == 1
	return &plugin, nil
}
