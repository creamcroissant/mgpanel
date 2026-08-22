// 文件路径: internal/repository/errors.go
// 模块说明: 这是 internal 模块里的 errors 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package repository

import "errors"

var (
	// ErrNotFound 表示查询未返回数据。
	ErrNotFound = errors.New("not found / 未找到数据")
	// ErrConflict 表示并发写入冲突（乐观锁）。
	ErrConflict = errors.New("conflict / 并发冲突")
)
