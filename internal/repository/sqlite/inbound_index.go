package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/creamcroissant/xboard/internal/repository"
)

type inboundIndexRepo struct {
	db *sql.DB
}

func newInboundIndexRepo(db *sql.DB) *inboundIndexRepo {
	return &inboundIndexRepo{db: db}
}

func (r *inboundIndexRepo) UpsertBatch(ctx context.Context, indexes []*repository.InboundIndex) error {
	if len(indexes) == 0 {
		return nil
	}
	for _, item := range indexes {
		if item == nil {
			return errors.New("inbound index item is nil")
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO inbound_index (
			agent_host_id, core_type, source, filename, tag,
			protocol, listen, port, tls, transport, multiplex, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_host_id, core_type, source, filename, tag)
		DO UPDATE SET
			protocol = excluded.protocol,
			listen = excluded.listen,
			port = excluded.port,
			tls = excluded.tls,
			transport = excluded.transport,
			multiplex = excluded.multiplex,
			last_seen_at = excluded.last_seen_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range indexes {
		_, err := stmt.ExecContext(ctx,
			item.AgentHostID,
			item.CoreType,
			item.Source,
			item.Filename,
			item.Tag,
			item.Protocol,
			item.Listen,
			item.Port,
			string(normalizeJSONObject(item.TLS)),
			string(normalizeJSONObject(item.Transport)),
			string(normalizeJSONObject(item.Multiplex)),
			item.LastSeenAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *inboundIndexRepo) List(ctx context.Context, filter repository.InboundIndexFilter) ([]*repository.InboundIndex, error) {
	query := strings.Builder{}
	args := make([]any, 0, 10)

	query.WriteString(`
		SELECT
			id, agent_host_id, core_type, source, filename, tag,
			protocol, listen, port, tls, transport, multiplex, last_seen_at
		FROM inbound_index
		WHERE 1 = 1
	`)

	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		args = append(args, *filter.AgentHostID)
	}
	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		args = append(args, *filter.CoreType)
	}
	if filter.Source != nil {
		query.WriteString(" AND source = ?")
		args = append(args, *filter.Source)
	}
	if filter.Tag != nil {
		query.WriteString(" AND tag = ?")
		args = append(args, *filter.Tag)
	}
	if filter.Protocol != nil {
		query.WriteString(" AND protocol = ?")
		args = append(args, *filter.Protocol)
	}
	if filter.Filename != nil {
		query.WriteString(" AND filename = ?")
		args = append(args, *filter.Filename)
	}

	limit, offset := normalizePagination(filter.Limit, filter.Offset, 200)
	query.WriteString(" ORDER BY last_seen_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*repository.InboundIndex
	for rows.Next() {
		entry, err := r.scanInboundIndex(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *inboundIndexRepo) DeleteStaleByHostCoreBefore(ctx context.Context, agentHostID int64, coreType string, beforeLastSeenAt int64) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM inbound_index
		WHERE agent_host_id = ? AND core_type = ? AND last_seen_at < ?
	`, agentHostID, coreType, beforeLastSeenAt) // idempotent: no error if not found
	return err
}

type inboundIndexScanner interface {
	Scan(dest ...any) error
}

func (r *inboundIndexRepo) scanInboundIndex(scanner inboundIndexScanner) (*repository.InboundIndex, error) {
	var item repository.InboundIndex
	var tls sql.NullString
	var transport sql.NullString
	var multiplex sql.NullString

	err := scanner.Scan(
		&item.ID,
		&item.AgentHostID,
		&item.CoreType,
		&item.Source,
		&item.Filename,
		&item.Tag,
		&item.Protocol,
		&item.Listen,
		&item.Port,
		&tls,
		&transport,
		&multiplex,
		&item.LastSeenAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	item.TLS = parseJSONObject(tls)
	item.Transport = parseJSONObject(transport)
	item.Multiplex = parseJSONObject(multiplex)
	return &item, nil
}
