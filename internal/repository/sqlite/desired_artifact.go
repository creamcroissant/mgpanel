package sqlite

import (
	"context"
	"fmt"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type desiredArtifactRepo struct {
	db *sql.DB
}

func newDesiredArtifactRepo(db *sql.DB) *desiredArtifactRepo {
	return &desiredArtifactRepo{db: db}
}

func (r *desiredArtifactRepo) CreateBatch(ctx context.Context, artifacts []*repository.DesiredArtifact) error {
	if len(artifacts) == 0 {
		return nil
	}
	for _, artifact := range artifacts {
		if artifact == nil {
			return errors.New("desired artifact is nil")
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO desired_artifacts (
			agent_host_id, core_type, desired_revision, filename, source_tag,
			content, content_hash, generated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, artifact := range artifacts {
		if artifact.GeneratedAt == 0 {
			artifact.GeneratedAt = now
		}

		result, err := stmt.ExecContext(ctx,
			artifact.AgentHostID,
			artifact.CoreType,
			artifact.DesiredRevision,
			artifact.Filename,
			artifact.SourceTag,
			artifact.Content,
			artifact.ContentHash,
			artifact.GeneratedAt,
		)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		artifact.ID = id
	}

	return tx.Commit()
}

// PruneOldRevisions 按 (agent_host_id, core_type) 分组仅保留最近 keep 个
// revision 批次，删除更早批次，约束表无限膨胀。keep<=0 视为非法参数。
func (r *desiredArtifactRepo) PruneOldRevisions(ctx context.Context, keep int) (int64, error) {
	if keep <= 0 {
		return 0, fmt.Errorf("desired artifact prune: keep must be positive / 保留版本数必须为正")
	}
	const stmt = `DELETE FROM desired_artifacts WHERE (agent_host_id, core_type, desired_revision) IN (
		SELECT agent_host_id, core_type, desired_revision FROM (
			SELECT agent_host_id, core_type, desired_revision,
			       DENSE_RANK() OVER (PARTITION BY agent_host_id, core_type ORDER BY desired_revision DESC) AS rk
			FROM desired_artifacts
			GROUP BY agent_host_id, core_type, desired_revision
		)
		WHERE rk > ?
	)`
	res, err := r.db.ExecContext(ctx, stmt, keep)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *desiredArtifactRepo) DeleteByHostCoreRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, sourceTags ...string) error {
	query := "DELETE FROM desired_artifacts WHERE agent_host_id = ? AND core_type = ? AND desired_revision = ?"
	args := []any{agentHostID, coreType, desiredRevision}
	if len(sourceTags) > 0 {
		placeholders := make([]string, len(sourceTags))
		for i, tag := range sourceTags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		query += " AND source_tag IN (" + strings.Join(placeholders, ",") + ")"
	}
	_, err := r.db.ExecContext(ctx, query, args...) // idempotent: no error if not found
	return err
}

// ReplaceRevision 在单事务内删除指定维度(host+core+revision，可选 sourceTags)
// 的旧 artifacts 并写入新批次；任一步失败整体回滚，杜绝"删完未写入"的中间态。
func (r *desiredArtifactRepo) ReplaceRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, artifacts []*repository.DesiredArtifact, sourceTags ...string) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := "DELETE FROM desired_artifacts WHERE agent_host_id = ? AND core_type = ? AND desired_revision = ?"
	args := []any{agentHostID, coreType, desiredRevision}
	if len(sourceTags) > 0 {
		placeholders := make([]string, len(sourceTags))
		for i, tag := range sourceTags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		query += " AND source_tag IN (" + strings.Join(placeholders, ",") + ")"
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return 0, fmt.Errorf("delete old artifacts: %w", err)
	}

	inserted := int64(0)
	for i, a := range artifacts {
		if a == nil {
			return 0, fmt.Errorf("insert artifact[%d]: nil", i)
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO desired_artifacts(agent_host_id, core_type, desired_revision, filename, source_tag, content, content_hash, generated_at)
			 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			a.AgentHostID, a.CoreType, a.DesiredRevision, a.Filename, a.SourceTag, a.Content, a.ContentHash, a.GeneratedAt,
		)
		if err != nil {
			return 0, fmt.Errorf("insert artifact %s: %w", a.Filename, err)
		}
		if id, rerr := res.LastInsertId(); rerr == nil {
			a.ID = id
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}

func (r *desiredArtifactRepo) List(ctx context.Context, filter repository.DesiredArtifactFilter) ([]*repository.DesiredArtifact, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)

	query.WriteString("SELECT id, agent_host_id, core_type, desired_revision, filename, source_tag,")
	if filter.ExcludeContent {
		query.WriteString(" content_hash, generated_at")
	} else {
		query.WriteString(" content, content_hash, generated_at")
	}
	query.WriteString(" FROM desired_artifacts WHERE agent_host_id = ?")
	args = append(args, filter.AgentHostID)
	r.appendDesiredArtifactWhere(&query, &args, filter)

	limit, offset := normalizePagination(filter.Limit, filter.Offset, 200)
	query.WriteString(" ORDER BY desired_revision DESC, filename ASC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []*repository.DesiredArtifact
	for rows.Next() {
		var artifact *repository.DesiredArtifact
		if filter.ExcludeContent {
			artifact, err = r.scanDesiredArtifactWithoutContent(rows)
		} else {
			artifact, err = r.scanDesiredArtifact(rows)
		}
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (r *desiredArtifactRepo) Count(ctx context.Context, filter repository.DesiredArtifactFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)

	query.WriteString("SELECT COUNT(1) FROM desired_artifacts WHERE agent_host_id = ?")
	args = append(args, filter.AgentHostID)
	r.appendDesiredArtifactWhere(&query, &args, filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *desiredArtifactRepo) appendDesiredArtifactWhere(query *strings.Builder, args *[]any, filter repository.DesiredArtifactFilter) {
	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		*args = append(*args, *filter.CoreType)
	}
	if filter.DesiredRevision != nil {
		query.WriteString(" AND desired_revision = ?")
		*args = append(*args, *filter.DesiredRevision)
	}
	if filter.SourceTag != nil {
		query.WriteString(" AND source_tag = ?")
		*args = append(*args, *filter.SourceTag)
	}
	if filter.Filename != nil {
		query.WriteString(" AND filename = ?")
		*args = append(*args, *filter.Filename)
	}
}

func (r *desiredArtifactRepo) GetLatestRevision(ctx context.Context, agentHostID int64, coreType string) (int64, error) {
	var latest sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT MAX(desired_revision)
		FROM desired_artifacts
		WHERE agent_host_id = ? AND core_type = ?
	`, agentHostID, coreType).Scan(&latest)
	if err != nil {
		return 0, err
	}
	if !latest.Valid {
		return 0, nil
	}
	return latest.Int64, nil
}

func (r *desiredArtifactRepo) FindByHostCoreRevisionFilename(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, filename string) (*repository.DesiredArtifact, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, agent_host_id, core_type, desired_revision, filename, source_tag,
			content, content_hash, generated_at
		FROM desired_artifacts
		WHERE agent_host_id = ? AND core_type = ? AND desired_revision = ? AND filename = ?
		LIMIT 1
	`, agentHostID, coreType, desiredRevision, filename)

	return r.scanDesiredArtifact(row)
}

type desiredArtifactScanner interface {
	Scan(dest ...any) error
}

func (r *desiredArtifactRepo) scanDesiredArtifact(scanner desiredArtifactScanner) (*repository.DesiredArtifact, error) {
	var artifact repository.DesiredArtifact

	err := scanner.Scan(
		&artifact.ID,
		&artifact.AgentHostID,
		&artifact.CoreType,
		&artifact.DesiredRevision,
		&artifact.Filename,
		&artifact.SourceTag,
		&artifact.Content,
		&artifact.ContentHash,
		&artifact.GeneratedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if artifact.Content == nil {
		artifact.Content = []byte{}
	}

	return &artifact, nil
}

func (r *desiredArtifactRepo) scanDesiredArtifactWithoutContent(scanner desiredArtifactScanner) (*repository.DesiredArtifact, error) {
	var artifact repository.DesiredArtifact

	err := scanner.Scan(
		&artifact.ID,
		&artifact.AgentHostID,
		&artifact.CoreType,
		&artifact.DesiredRevision,
		&artifact.Filename,
		&artifact.SourceTag,
		&artifact.ContentHash,
		&artifact.GeneratedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	artifact.Content = nil
	return &artifact, nil
}
