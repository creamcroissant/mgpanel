package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type binaryVersionStateRepo struct {
	db *sql.DB
}

func newBinaryVersionStateRepo(db *sql.DB) *binaryVersionStateRepo {
	return &binaryVersionStateRepo{db: db}
}

func (r *binaryVersionStateRepo) Upsert(ctx context.Context, state *repository.BinaryVersionState) (*repository.BinaryVersionState, error) {
	if state == nil {
		return nil, errors.New("binary version state is nil")
	}
	if state.AgentHostID <= 0 {
		return nil, errors.New("agent host id is required")
	}
	state.Component = strings.TrimSpace(state.Component)
	if state.Component == "" {
		return nil, errors.New("binary version component is required")
	}
	if state.Status == "" {
		state.Status = "unknown"
	}
	if state.CapabilitiesJSON == "" {
		state.CapabilitiesJSON = "[]"
	}
	if state.BuildTagsJSON == "" {
		state.BuildTagsJSON = "[]"
	}
	if state.UpdatedAt == 0 {
		state.UpdatedAt = time.Now().Unix()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_binary_version_states (
			agent_host_id, component, local_version, remote_version, status,
			capabilities_json, build_tags_json, last_checked_at, last_check_error, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_host_id, component) DO UPDATE SET
			local_version = excluded.local_version,
			status = excluded.status,
			capabilities_json = excluded.capabilities_json,
			build_tags_json = excluded.build_tags_json,
			updated_at = excluded.updated_at
	`,
		state.AgentHostID,
		state.Component,
		state.LocalVersion,
		state.RemoteVersion,
		state.Status,
		state.CapabilitiesJSON,
		state.BuildTagsJSON,
		state.LastCheckedAt,
		state.LastCheckError,
		state.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return r.FindByHostComponent(ctx, state.AgentHostID, state.Component)
}

func (r *binaryVersionStateRepo) FindByHostComponent(ctx context.Context, agentHostID int64, component string) (*repository.BinaryVersionState, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, agent_host_id, component, local_version, remote_version, status,
			capabilities_json, build_tags_json, last_checked_at, last_check_error, updated_at
		FROM agent_binary_version_states
		WHERE agent_host_id = ? AND component = ?
		LIMIT 1
	`, agentHostID, component)
	return scanBinaryVersionState(row)
}

func (r *binaryVersionStateRepo) List(ctx context.Context, filter repository.BinaryVersionFilter) ([]*repository.BinaryVersionState, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)
	query.WriteString(`
		SELECT id, agent_host_id, component, local_version, remote_version, status,
			capabilities_json, build_tags_json, last_checked_at, last_check_error, updated_at
		FROM agent_binary_version_states WHERE 1 = 1
	`)
	appendBinaryVersionFilterConditions(&query, &args, filter)
	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY agent_host_id ASC, component ASC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]*repository.BinaryVersionState, 0)
	for rows.Next() {
		state, err := scanBinaryVersionState(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

func (r *binaryVersionStateRepo) UpdateCheckResult(ctx context.Context, agentHostID int64, component, remoteVersion, status, checkError string, checkedAt int64) error {
	updatedAt := time.Now().Unix()
	if checkError != "" {
		return r.updateCheckFailure(ctx, agentHostID, component, checkError, checkedAt, updatedAt)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_binary_version_states
		SET remote_version = ?, status = ?, last_check_error = ?, last_checked_at = ?, updated_at = ?
		WHERE agent_host_id = ? AND component = ?
	`, remoteVersion, status, checkError, checkedAt, updatedAt, agentHostID, component)
	if err != nil {
		return err
	}
	return ensureBinaryVersionStateUpdated(result)
}

func (r *binaryVersionStateRepo) updateCheckFailure(ctx context.Context, agentHostID int64, component, checkError string, checkedAt, updatedAt int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_binary_version_states
		SET last_check_error = ?, last_checked_at = ?, updated_at = ?
		WHERE agent_host_id = ? AND component = ?
	`, checkError, checkedAt, updatedAt, agentHostID, component)
	if err != nil {
		return err
	}
	return ensureBinaryVersionStateUpdated(result)
}

func ensureBinaryVersionStateUpdated(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func appendBinaryVersionFilterConditions(query *strings.Builder, args *[]any, filter repository.BinaryVersionFilter) {
	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		*args = append(*args, *filter.AgentHostID)
	}
	if filter.Component != nil {
		query.WriteString(" AND component = ?")
		*args = append(*args, *filter.Component)
	}
	if filter.Status != nil {
		query.WriteString(" AND status = ?")
		*args = append(*args, *filter.Status)
	}
}

type binaryVersionStateScanner interface {
	Scan(dest ...any) error
}

func scanBinaryVersionState(scanner binaryVersionStateScanner) (*repository.BinaryVersionState, error) {
	state := repository.BinaryVersionState{}
	if err := scanner.Scan(
		&state.ID,
		&state.AgentHostID,
		&state.Component,
		&state.LocalVersion,
		&state.RemoteVersion,
		&state.Status,
		&state.CapabilitiesJSON,
		&state.BuildTagsJSON,
		&state.LastCheckedAt,
		&state.LastCheckError,
		&state.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &state, nil
}
