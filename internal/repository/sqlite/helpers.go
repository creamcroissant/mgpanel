// 文件路径: internal/repository/sqlite/helpers.go
// 模块说明: 这是 internal 模块里的 helpers 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"database/sql"
	"encoding/json"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func optionalInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func nullableIntPtr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	value := v.Int64
	return &value
}

func encodeStringSlice(s []string) (sql.NullString, error) {
	if len(s) == 0 {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func decodeJSONSlice(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	var res []string
	if err := json.Unmarshal([]byte(s), &res); err != nil {
		return nil, err
	}
	return res, nil
}

func normalizePagination(limit, offset, defaultLimit int) (int, int) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func ensureRowsAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return repository.ErrNotFound
	}
	return nil
}
