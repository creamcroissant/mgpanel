// 文件路径: internal/repository/sqlite/notice.go
// 模块说明: 这是 internal 模块里的 notice 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/creamcroissant/xboard/internal/repository"
)

type noticeRepo struct {
	db *sql.DB
}

func (r *noticeRepo) List(ctx context.Context) ([]*repository.Notice, error) {
	rows, err := r.db.QueryContext(ctx, listNoticesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notices []*repository.Notice
	for rows.Next() {
		notice, err := scanNotice(rows)
		if err != nil {
			return nil, err
		}
		notices = append(notices, notice)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notices, nil
}

func (r *noticeRepo) FindByID(ctx context.Context, id int64) (*repository.Notice, error) {
	row := r.db.QueryRowContext(ctx, noticeByIDQuery, id)
	notice, err := scanNotice(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return notice, nil
}

func (r *noticeRepo) Create(ctx context.Context, notice *repository.Notice) (*repository.Notice, error) {
	if notice == nil {
		return nil, errors.New("notice is nil")
	}
	const stmt = `INSERT INTO notices(sort, title, content, img_url, tags, show, popup, created_at, updated_at)
                  VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, stmt,
		nullableSort(notice.Sort),
		notice.Title,
		notice.Content,
		nullableText(notice.ImgURL),
		encodeNoticeTags(notice.Tags),
		boolToInt(notice.Show),
		boolToInt(notice.Popup),
		notice.CreatedAt,
		notice.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if id, err := res.LastInsertId(); err == nil {
		notice.ID = id
	}
	return notice, nil
}

func (r *noticeRepo) Update(ctx context.Context, notice *repository.Notice) error {
	if notice == nil {
		return errors.New("notice is nil")
	}
	const stmt = `UPDATE notices
                  SET sort = ?, title = ?, content = ?, img_url = ?, tags = ?, show = ?, popup = ?, updated_at = ?
                  WHERE id = ?`
	result, err := r.db.ExecContext(ctx, stmt,
		nullableSort(notice.Sort),
		notice.Title,
		notice.Content,
		nullableText(notice.ImgURL),
		encodeNoticeTags(notice.Tags),
		boolToInt(notice.Show),
		boolToInt(notice.Popup),
		notice.UpdatedAt,
		notice.ID,
	)
	if err != nil { return err }
		return ensureRowsAffected(result)
}

func (r *noticeRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM notices WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *noticeRepo) Sort(ctx context.Context, ids []int64, updatedAt int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for idx, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE notices SET sort = ?, updated_at = ? WHERE id = ?`, int64(idx+1), updatedAt, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type noticeScanner interface {
	Scan(dest ...any) error
}

func scanNotice(scanner noticeScanner) (*repository.Notice, error) {
	var (
		id        int64
		sort      sql.NullInt64
		title     string
		content   string
		imgURL    sql.NullString
		tags      sql.NullString
		showFlag  int64
		popupFlag int64
		createdAt int64
		updatedAt int64
	)
	if err := scanner.Scan(&id, &sort, &title, &content, &imgURL, &tags, &showFlag, &popupFlag, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return &repository.Notice{
		ID:        id,
		Sort:      sort.Int64,
		Title:     title,
		Content:   content,
		ImgURL:    imgURL.String,
		Tags:      decodeNoticeTags(tags.String),
		Show:      showFlag == 1,
		Popup:     popupFlag == 1,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func encodeNoticeTags(values []string) any {
	clean := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	if len(clean) == 0 {
		return nil
	}
	buf, err := json.Marshal(clean)
	if err != nil {
		return nil
	}
	return string(buf)
}

func decodeNoticeTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

func nullableSort(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func nullableText(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

const (
	noticeColumns    = `id, sort, title, content, img_url, tags, show, popup, created_at, updated_at`
	listNoticesQuery = `SELECT ` + noticeColumns + `
        FROM notices
        ORDER BY CASE WHEN sort IS NULL OR sort = 0 THEN 1 ELSE 0 END, sort ASC, id DESC`
	noticeByIDQuery = `SELECT ` + noticeColumns + ` FROM notices WHERE id = ? LIMIT 1`
)
