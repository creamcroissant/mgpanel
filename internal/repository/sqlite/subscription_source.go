package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

type subscriptionSourceRepo struct {
	db *sql.DB
}

func newSubscriptionSourceRepo(db *sql.DB) *subscriptionSourceRepo {
	return &subscriptionSourceRepo{db: db}
}

func (r *subscriptionSourceRepo) Create(ctx context.Context, source *repository.SubscriptionSource) (*repository.SubscriptionSource, error) {
	if source == nil {
		return nil, errors.New("subscription source is nil")
	}
	normalizeSubscriptionSource(source)
	if source.Type == "" {
		return nil, errors.New("subscription source type is required")
	}
	if source.Name == "" {
		return nil, errors.New("subscription source name is required")
	}
	now := time.Now().Unix()
	if source.CreatedAt == 0 {
		source.CreatedAt = now
	}
	if source.UpdatedAt == 0 {
		source.UpdatedAt = source.CreatedAt
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO subscription_sources (
			type, name, url, content, enabled, last_sync_at, last_sync_err, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		source.Type,
		source.Name,
		source.URL,
		source.Content,
		boolToInt(source.Enabled),
		source.LastSyncAt,
		source.LastSyncErr,
		source.CreatedAt,
		source.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *subscriptionSourceRepo) Update(ctx context.Context, source *repository.SubscriptionSource) error {
	if source == nil {
		return errors.New("subscription source is nil")
	}
	if source.ID <= 0 {
		return errors.New("subscription source id is required")
	}
	normalizeSubscriptionSource(source)
	if source.Type == "" {
		return errors.New("subscription source type is required")
	}
	if source.Name == "" {
		return errors.New("subscription source name is required")
	}
	if source.UpdatedAt == 0 {
		source.UpdatedAt = time.Now().Unix()
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE subscription_sources
		SET type = ?, name = ?, url = ?, content = ?, enabled = ?, last_sync_at = ?, last_sync_err = ?, updated_at = ?
		WHERE id = ?
	`,
		source.Type,
		source.Name,
		source.URL,
		source.Content,
		boolToInt(source.Enabled),
		source.LastSyncAt,
		source.LastSyncErr,
		source.UpdatedAt,
		source.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *subscriptionSourceRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM subscription_sources WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *subscriptionSourceRepo) FindByID(ctx context.Context, id int64) (*repository.SubscriptionSource, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, type, name, url, content, enabled, last_sync_at, last_sync_err, created_at, updated_at
		FROM subscription_sources
		WHERE id = ?
		LIMIT 1
	`, id)
	return scanSubscriptionSource(row)
}

func (r *subscriptionSourceRepo) List(ctx context.Context, filter repository.SubscriptionSourceFilter) ([]*repository.SubscriptionSource, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)
	query.WriteString(`
		SELECT id, type, name, url, content, enabled, last_sync_at, last_sync_err, created_at, updated_at
		FROM subscription_sources WHERE 1 = 1
	`)
	appendSubscriptionSourceFilterConditions(&query, &args, filter)
	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := make([]*repository.SubscriptionSource, 0)
	for rows.Next() {
		source, err := scanSubscriptionSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (r *subscriptionSourceRepo) Count(ctx context.Context, filter repository.SubscriptionSourceFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 6)
	query.WriteString(`SELECT COUNT(*) FROM subscription_sources WHERE 1 = 1`)
	appendSubscriptionSourceFilterConditions(&query, &args, filter)
	var total int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total)
	return total, err
}

func (r *subscriptionSourceRepo) UpdateSyncResult(ctx context.Context, id int64, content string, syncErr string, syncedAt int64) error {
	if syncedAt == 0 {
		syncedAt = time.Now().Unix()
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE subscription_sources
		SET content = ?, last_sync_at = ?, last_sync_err = ?, updated_at = ?
		WHERE id = ?
	`, content, syncedAt, strings.TrimSpace(syncErr), syncedAt, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func normalizeSubscriptionSource(source *repository.SubscriptionSource) {
	source.Type = strings.TrimSpace(source.Type)
	source.Name = strings.TrimSpace(source.Name)
	source.URL = strings.TrimSpace(source.URL)
	source.LastSyncErr = strings.TrimSpace(source.LastSyncErr)
}

func appendSubscriptionSourceFilterConditions(query *strings.Builder, args *[]any, filter repository.SubscriptionSourceFilter) {
	if filter.Type != nil {
		query.WriteString(" AND type = ?")
		*args = append(*args, *filter.Type)
	}
	if filter.Enabled != nil {
		query.WriteString(" AND enabled = ?")
		*args = append(*args, boolToInt(*filter.Enabled))
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		query.WriteString(" AND name LIKE ?")
		*args = append(*args, "%"+keyword+"%")
	}
}

type subscriptionSourceScanner interface {
	Scan(dest ...any) error
}

func scanSubscriptionSource(scanner subscriptionSourceScanner) (*repository.SubscriptionSource, error) {
	var source repository.SubscriptionSource
	var enabled int
	if err := scanner.Scan(
		&source.ID,
		&source.Type,
		&source.Name,
		&source.URL,
		&source.Content,
		&enabled,
		&source.LastSyncAt,
		&source.LastSyncErr,
		&source.CreatedAt,
		&source.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	source.Enabled = enabled != 0
	return &source, nil
}

type subscriptionFilterReasonRepo struct {
	db *sql.DB
}

func newSubscriptionFilterReasonRepo(db *sql.DB) *subscriptionFilterReasonRepo {
	return &subscriptionFilterReasonRepo{db: db}
}

func (r *subscriptionFilterReasonRepo) ReplaceForSource(ctx context.Context, sourceType string, sourceID int64, reasons []*repository.SubscriptionFilterReason) error {
	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" {
		return errors.New("subscription filter reason source type is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM subscription_filter_reasons WHERE source_type = ? AND source_id = ?`, sourceType, sourceID); err != nil {
		return err
	} // idempotent: no error if not found
	now := time.Now().Unix()
	for _, reason := range reasons {
		if reason == nil {
			return errors.New("subscription filter reason is nil")
		}
		reason.SourceType = sourceType
		reason.SourceID = sourceID
		reason.NodeName = strings.TrimSpace(reason.NodeName)
		reason.Reason = strings.TrimSpace(reason.Reason)
		reason.Detail = strings.TrimSpace(reason.Detail)
		if reason.Reason == "" {
			return errors.New("subscription filter reason is required")
		}
		if reason.CreatedAt == 0 {
			reason.CreatedAt = now
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO subscription_filter_reasons (
				source_type, source_id, server_id, node_name, reason, detail, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`, reason.SourceType, reason.SourceID, reason.ServerID, reason.NodeName, reason.Reason, reason.Detail, reason.CreatedAt)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		reason.ID = id
	}
	return tx.Commit()
}

func (r *subscriptionFilterReasonRepo) List(ctx context.Context, filter repository.SubscriptionFilterReasonFilter) ([]*repository.SubscriptionFilterReason, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)
	query.WriteString(`
		SELECT id, source_type, source_id, server_id, node_name, reason, detail, created_at
		FROM subscription_filter_reasons WHERE 1 = 1
	`)
	appendSubscriptionFilterReasonFilterConditions(&query, &args, filter)
	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reasons := make([]*repository.SubscriptionFilterReason, 0)
	for rows.Next() {
		reason, err := scanSubscriptionFilterReason(rows)
		if err != nil {
			return nil, err
		}
		reasons = append(reasons, reason)
	}
	return reasons, rows.Err()
}

func (r *subscriptionFilterReasonRepo) Count(ctx context.Context, filter repository.SubscriptionFilterReasonFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 6)
	query.WriteString(`SELECT COUNT(*) FROM subscription_filter_reasons WHERE 1 = 1`)
	appendSubscriptionFilterReasonFilterConditions(&query, &args, filter)
	var total int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total)
	return total, err
}

func (r *subscriptionFilterReasonRepo) DeleteBySource(ctx context.Context, sourceType string, sourceID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM subscription_filter_reasons WHERE source_type = ? AND source_id = ?`, strings.TrimSpace(sourceType), sourceID)
	// idempotent: no error if not found
	return err
}

func appendSubscriptionFilterReasonFilterConditions(query *strings.Builder, args *[]any, filter repository.SubscriptionFilterReasonFilter) {
	if filter.SourceType != nil {
		query.WriteString(" AND source_type = ?")
		*args = append(*args, *filter.SourceType)
	}
	if filter.SourceID != nil {
		query.WriteString(" AND source_id = ?")
		*args = append(*args, *filter.SourceID)
	}
	if filter.ServerID != nil {
		query.WriteString(" AND server_id = ?")
		*args = append(*args, *filter.ServerID)
	}
	if filter.Reason != nil {
		query.WriteString(" AND reason = ?")
		*args = append(*args, *filter.Reason)
	}
	if filter.CreatedAfter != nil {
		query.WriteString(" AND created_at >= ?")
		*args = append(*args, *filter.CreatedAfter)
	}
	if filter.CreatedBefore != nil {
		query.WriteString(" AND created_at <= ?")
		*args = append(*args, *filter.CreatedBefore)
	}
}

type subscriptionFilterReasonScanner interface {
	Scan(dest ...any) error
}

func scanSubscriptionFilterReason(scanner subscriptionFilterReasonScanner) (*repository.SubscriptionFilterReason, error) {
	var reason repository.SubscriptionFilterReason
	if err := scanner.Scan(
		&reason.ID,
		&reason.SourceType,
		&reason.SourceID,
		&reason.ServerID,
		&reason.NodeName,
		&reason.Reason,
		&reason.Detail,
		&reason.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &reason, nil
}
