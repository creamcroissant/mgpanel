package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type agentConfigInventoryRepo struct {
	db *sql.DB
}

func newAgentConfigInventoryRepo(db *sql.DB) *agentConfigInventoryRepo {
	return &agentConfigInventoryRepo{db: db}
}

func (r *agentConfigInventoryRepo) UpsertBatch(ctx context.Context, inventories []*repository.AgentConfigInventory) error {
	if len(inventories) == 0 {
		return nil
	}
	for _, item := range inventories {
		if item == nil {
			return errors.New("agent config inventory item is nil")
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO agent_config_inventory (
			agent_host_id, core_type, source, filename, hash_applied,
			parse_status, parse_error, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_host_id, core_type, source, filename)
		DO UPDATE SET
			hash_applied = excluded.hash_applied,
			parse_status = excluded.parse_status,
			parse_error = excluded.parse_error,
			last_seen_at = excluded.last_seen_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range inventories {
		_, err := stmt.ExecContext(ctx,
			item.AgentHostID,
			item.CoreType,
			item.Source,
			item.Filename,
			item.HashApplied,
			item.ParseStatus,
			item.ParseError,
			item.LastSeenAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *agentConfigInventoryRepo) List(ctx context.Context, filter repository.AgentConfigInventoryFilter) ([]*repository.AgentConfigInventory, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)

	query.WriteString(`
		SELECT
			id, agent_host_id, core_type, source, filename,
			hash_applied, parse_status, parse_error, last_seen_at
		FROM agent_config_inventory
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
	if filter.Filename != nil {
		query.WriteString(" AND filename = ?")
		args = append(args, *filter.Filename)
	}
	if filter.ParseStatus != nil {
		query.WriteString(" AND parse_status = ?")
		args = append(args, *filter.ParseStatus)
	}

	limit, offset := normalizePagination(filter.Limit, filter.Offset, 200)
	query.WriteString(" ORDER BY last_seen_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inventories []*repository.AgentConfigInventory
	for rows.Next() {
		item, err := r.scanAgentConfigInventory(rows)
		if err != nil {
			return nil, err
		}
		inventories = append(inventories, item)
	}
	return inventories, rows.Err()
}

func (r *agentConfigInventoryRepo) DeleteStaleByHostCoreBefore(ctx context.Context, agentHostID int64, coreType string, beforeLastSeenAt int64) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM agent_config_inventory
		WHERE agent_host_id = ? AND core_type = ? AND last_seen_at < ?
	`, agentHostID, coreType, beforeLastSeenAt) // idempotent: no error if not found
	return err
}

type agentConfigInventoryScanner interface {
	Scan(dest ...any) error
}

func (r *agentConfigInventoryRepo) scanAgentConfigInventory(scanner agentConfigInventoryScanner) (*repository.AgentConfigInventory, error) {
	var item repository.AgentConfigInventory
	err := scanner.Scan(
		&item.ID,
		&item.AgentHostID,
		&item.CoreType,
		&item.Source,
		&item.Filename,
		&item.HashApplied,
		&item.ParseStatus,
		&item.ParseError,
		&item.LastSeenAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
