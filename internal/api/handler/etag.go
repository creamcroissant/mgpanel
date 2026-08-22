// 文件路径: internal/api/handler/etag.go
// 模块说明: 这是 internal 模块里的 etag 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import "strings"

func formatETag(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	safe := strings.ReplaceAll(trimmed, `"`, `\"`)
	return `"` + safe + `"`
}
