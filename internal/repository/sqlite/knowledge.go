// 文件路径: internal/repository/sqlite/knowledge.go
// 模块说明: 这是 internal 模块里的 knowledge 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// knowledgeRepo 负责 knowledge 表的 SQLite 实现。
type knowledgeRepo struct {
	db *sql.DB
}

func (r *knowledgeRepo) List(ctx context.Context) ([]*repository.Knowledge, error) {
	// 按排序字段与创建时间获取知识条目。
	rows, err := r.db.QueryContext(ctx, knowledgeListQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*repository.Knowledge
	for rows.Next() {
		knowledge, err := scanKnowledge(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, knowledge)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *knowledgeRepo) FindByID(ctx context.Context, id int64) (*repository.Knowledge, error) {
	// 根据主键读取单条知识记录。
	row := r.db.QueryRowContext(ctx, knowledgeByIDQuery, id)
	knowledge, err := scanKnowledge(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return knowledge, nil
}

func (r *knowledgeRepo) Create(ctx context.Context, knowledge *repository.Knowledge) (*repository.Knowledge, error) {
	// 新增知识记录，写入后回填 ID。
	if knowledge == nil {
		return nil, errors.New("knowledge is nil")
	}
	const stmt = `INSERT INTO knowledge(language, category, title, body, sort, show, created_at, updated_at)
                  VALUES(?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, stmt,
		strings.TrimSpace(knowledge.Language),
		strings.TrimSpace(knowledge.Category),
		strings.TrimSpace(knowledge.Title),
		knowledge.Body,
		sortValue(knowledge.Sort),
		boolToInt(knowledge.Show),
		knowledge.CreatedAt,
		knowledge.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if id, err := res.LastInsertId(); err == nil {
		knowledge.ID = id
	}
	return knowledge, nil
}

func (r *knowledgeRepo) Update(ctx context.Context, knowledge *repository.Knowledge) error {
	// 更新知识记录的字段内容。
	if knowledge == nil {
		return errors.New("knowledge is nil")
	}
	const stmt = `UPDATE knowledge
                  SET language = ?, category = ?, title = ?, body = ?, sort = ?, show = ?, updated_at = ?
                  WHERE id = ?`
	result, err := r.db.ExecContext(ctx, stmt,
		strings.TrimSpace(knowledge.Language),
		strings.TrimSpace(knowledge.Category),
		strings.TrimSpace(knowledge.Title),
		knowledge.Body,
		sortValue(knowledge.Sort),
		boolToInt(knowledge.Show),
		knowledge.UpdatedAt,
		knowledge.ID,
	)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(result); err != nil {
		return fmt.Errorf("knowledge: %w", err)
	}
	return nil
}

func (r *knowledgeRepo) Delete(ctx context.Context, id int64) error {
	// 按主键删除知识记录。
	result, err := r.db.ExecContext(ctx, `DELETE FROM knowledge WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *knowledgeRepo) Sort(ctx context.Context, ids []int64, updatedAt int64) error {
	// 批量更新排序顺序，保持事务一致性。
	if len(ids) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for idx, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE knowledge SET sort = ?, updated_at = ? WHERE id = ?`, int64(idx+1), updatedAt, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *knowledgeRepo) Categories(ctx context.Context) ([]string, error) {
	// 获取去重后的分类列表。
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT category FROM knowledge ORDER BY category ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]string, 0)
	for rows.Next() {
		var category sql.NullString
		if err := rows.Scan(&category); err != nil {
			return nil, err
		}
		if trimmed := strings.TrimSpace(category.String); trimmed != "" {
			categories = append(categories, trimmed)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *knowledgeRepo) ListVisible(ctx context.Context, filter repository.KnowledgeVisibleFilter) ([]*repository.Knowledge, error) {
	// 根据语言与关键字筛选可见知识条目。
	builder := strings.Builder{}
	builder.WriteString(`SELECT ` + knowledgeColumns + ` FROM knowledge WHERE show = 1`)
	var args []any
	if lang := strings.TrimSpace(filter.Language); lang != "" {
		builder.WriteString(` AND language = ?`)
		args = append(args, lang)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		builder.WriteString(` AND (title LIKE ? OR body LIKE ?)`)
		pattern := "%" + keyword + "%"
		args = append(args, pattern, pattern)
	}
	builder.WriteString(` ORDER BY CASE WHEN sort IS NULL OR sort = 0 THEN 1 ELSE 0 END, sort ASC, id DESC`)

	rows, err := r.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*repository.Knowledge
	for rows.Next() {
		knowledge, err := scanKnowledge(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, knowledge)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

type knowledgeScanner interface {
	Scan(dest ...any) error
}

// scanKnowledge 将查询行转换为 Knowledge 结构体。
func scanKnowledge(scanner knowledgeScanner) (*repository.Knowledge, error) {
	var (
		id        int64
		language  string
		category  string
		title     string
		body      string
		sort      sql.NullInt64
		showFlag  int64
		createdAt int64
		updatedAt int64
	)
	if err := scanner.Scan(&id, &language, &category, &title, &body, &sort, &showFlag, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return &repository.Knowledge{
		ID:        id,
		Language:  language,
		Category:  category,
		Title:     title,
		Body:      body,
		Sort:      sort.Int64,
		Show:      showFlag == 1,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// sortValue 将排序值转换为可写入数据库的类型。
func sortValue(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

const (
	knowledgeColumns   = `id, language, category, title, body, sort, show, created_at, updated_at`
	knowledgeListQuery = `SELECT ` + knowledgeColumns + ` FROM knowledge ORDER BY CASE WHEN sort IS NULL OR sort = 0 THEN 1 ELSE 0 END, sort ASC, id DESC`
	knowledgeByIDQuery = `SELECT ` + knowledgeColumns + ` FROM knowledge WHERE id = ? LIMIT 1`
)
