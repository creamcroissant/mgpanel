// 文件路径: internal/migrations/sqlite_embed.go
// 模块说明: 这是 internal 模块里的 sqlite_embed 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package migrations

import "embed"

// SQLite embeds all SQLite-specific migration files.
//
//go:embed sqlite/*.sql
var SQLite embed.FS