package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type inboundSpecRevisionRepo struct {
	db *sql.DB
}

func newInboundSpecRevisionRepo(db *sql.DB) *inboundSpecRevisionRepo {
	return &inboundSpecRevisionRepo{db: db}
}

func (r *inboundSpecRevisionRepo) Create(ctx context.Context, revision *repository.InboundSpecRevision) error {
	if revision == nil {
		return errors.New("inbound spec revision is nil")
	}

	if revision.CreatedAt == 0 {
		revision.CreatedAt = time.Now().Unix()
	}
	snapshot := normalizeJSONObject(revision.Snapshot)

	result, err := execWithRetry(ctx, r.db, `
		INSERT INTO inbound_spec_revisions (
			spec_id, revision, snapshot, change_note, operator_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`,
		revision.SpecID,
		revision.Revision,
		string(snapshot),
		revision.ChangeNote,
		revision.OperatorID,
		revision.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	revision.ID = id
	revision.Snapshot = snapshot
	return nil
}

func (r *inboundSpecRevisionRepo) FindBySpecAndRevision(ctx context.Context, specID int64, revision int64) (*repository.InboundSpecRevision, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, spec_id, revision, snapshot, change_note, operator_id, created_at
		FROM inbound_spec_revisions
		WHERE spec_id = ? AND revision = ?
		LIMIT 1
	`, specID, revision)

	return r.scanInboundSpecRevision(row)
}

func (r *inboundSpecRevisionRepo) ListBySpecID(ctx context.Context, specID int64, limit, offset int) ([]*repository.InboundSpecRevision, error) {
	limit, offset = normalizePagination(limit, offset, 100)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, spec_id, revision, snapshot, change_note, operator_id, created_at
		FROM inbound_spec_revisions
		WHERE spec_id = ?
		ORDER BY revision DESC, id DESC
		LIMIT ? OFFSET ?
	`, specID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []*repository.InboundSpecRevision
	for rows.Next() {
		rev, err := r.scanInboundSpecRevision(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, rev)
	}
	return revisions, rows.Err()
}

func (r *inboundSpecRevisionRepo) GetMaxRevision(ctx context.Context, specID int64) (int64, error) {
	var maxRevision sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT MAX(revision)
		FROM inbound_spec_revisions
		WHERE spec_id = ?
	`, specID).Scan(&maxRevision)
	if err != nil {
		return 0, err
	}
	if !maxRevision.Valid {
		return 0, nil
	}
	return maxRevision.Int64, nil
}

type inboundSpecRevisionScanner interface {
	Scan(dest ...any) error
}

func (r *inboundSpecRevisionRepo) scanInboundSpecRevision(scanner inboundSpecRevisionScanner) (*repository.InboundSpecRevision, error) {
	var rev repository.InboundSpecRevision
	var snapshot sql.NullString

	err := scanner.Scan(
		&rev.ID,
		&rev.SpecID,
		&rev.Revision,
		&snapshot,
		&rev.ChangeNote,
		&rev.OperatorID,
		&rev.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rev.Snapshot = parseJSONObject(snapshot)

	return &rev, nil
}
