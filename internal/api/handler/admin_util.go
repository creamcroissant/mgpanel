// 文件路径: internal/api/handler/admin_util.go
// 模块说明: 这是 internal 模块里的 admin_util 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func clampQueryInt(raw string, def int) int {
	if strings.TrimSpace(raw) == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if value < 0 {
		return 0
	}
	if value > 200 {
		return 200
	}
	return value
}

func clampNonNegativeQueryInt(raw string, def int) int {
	if strings.TrimSpace(raw) == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if value < 0 {
		return 0
	}
	return value
}

func parseInt64(raw string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
}
