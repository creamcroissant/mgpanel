package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type inboundSpecRepo struct {
	db *sql.DB
}

func newInboundSpecRepo(db *sql.DB) *inboundSpecRepo {
	return &inboundSpecRepo{db: db}
}

func (r *inboundSpecRepo) Create(ctx context.Context, spec *repository.InboundSpec) error {
	if spec == nil {
		return errors.New("inbound spec is nil")
	}

	now := time.Now().Unix()
	if spec.CreatedAt == 0 {
		spec.CreatedAt = now
	}
	spec.UpdatedAt = now

	semanticSpec := normalizeJSONObject(spec.SemanticSpec)
	coreSpecific := normalizeJSONObject(spec.CoreSpecific)

	result, err := execWithRetry(ctx, r.db, `
		INSERT INTO inbound_specs (
			agent_host_id, exit_agent_host_id, exit_node_set_id, relay_path_id, core_type, tag, enabled, semantic_spec, core_specific,
			desired_revision, created_by, updated_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		optionalInt64(spec.AgentHostID),
		optionalInt64(spec.ExitAgentHostID),
		optionalInt64(spec.ExitNodeSetID),
		optionalInt64(spec.RelayPathID),
		spec.CoreType,
		spec.Tag,
		boolToInt(spec.Enabled),
		string(semanticSpec),
		string(coreSpecific),
		spec.DesiredRevision,
		spec.CreatedBy,
		spec.UpdatedBy,
		spec.CreatedAt,
		spec.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	spec.ID = id
	spec.SemanticSpec = semanticSpec
	spec.CoreSpecific = coreSpecific
	return nil
}

func (r *inboundSpecRepo) Update(ctx context.Context, spec *repository.InboundSpec) error {
	if spec == nil {
		return errors.New("inbound spec is nil")
	}

	spec.UpdatedAt = time.Now().Unix()
	semanticSpec := normalizeJSONObject(spec.SemanticSpec)
	coreSpecific := normalizeJSONObject(spec.CoreSpecific)

	result, err := execWithRetry(ctx, r.db, `
		UPDATE inbound_specs
		SET agent_host_id = ?, exit_agent_host_id = ?, exit_node_set_id = ?, relay_path_id = ?, core_type = ?, tag = ?, enabled = ?, semantic_spec = ?,
			core_specific = ?, desired_revision = ?, created_by = ?, updated_by = ?, updated_at = ?
		WHERE id = ?
	`,
		optionalInt64(spec.AgentHostID),
		optionalInt64(spec.ExitAgentHostID),
		optionalInt64(spec.ExitNodeSetID),
		optionalInt64(spec.RelayPathID),
		spec.CoreType,
		spec.Tag,
		boolToInt(spec.Enabled),
		string(semanticSpec),
		string(coreSpecific),
		spec.DesiredRevision,
		spec.CreatedBy,
		spec.UpdatedBy,
		spec.UpdatedAt,
		spec.ID,
	)
	if err != nil {
		return err
	}

	spec.SemanticSpec = semanticSpec
	spec.CoreSpecific = coreSpecific
	return ensureRowsAffected(result)
}

func (r *inboundSpecRepo) UpdateWithRevision(ctx context.Context, spec *repository.InboundSpec, expectedRevision int64) error {
	if spec == nil {
		return errors.New("inbound spec is nil")
	}

	spec.UpdatedAt = time.Now().Unix()
	semanticSpec := normalizeJSONObject(spec.SemanticSpec)
	coreSpecific := normalizeJSONObject(spec.CoreSpecific)

	result, err := execWithRetry(ctx, r.db, `
		UPDATE inbound_specs
		SET agent_host_id = ?, exit_agent_host_id = ?, exit_node_set_id = ?, relay_path_id = ?, core_type = ?, tag = ?, enabled = ?, semantic_spec = ?,
			core_specific = ?, desired_revision = ?, created_by = ?, updated_by = ?, updated_at = ?
		WHERE id = ? AND desired_revision = ?
	`,
		optionalInt64(spec.AgentHostID),
		optionalInt64(spec.ExitAgentHostID),
		optionalInt64(spec.ExitNodeSetID),
		optionalInt64(spec.RelayPathID),
		spec.CoreType,
		spec.Tag,
		boolToInt(spec.Enabled),
		string(semanticSpec),
		string(coreSpecific),
		spec.DesiredRevision,
		spec.CreatedBy,
		spec.UpdatedBy,
		spec.UpdatedAt,
		spec.ID,
		expectedRevision,
	)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected(); if err != nil { return err }
	if n == 0 {
		return repository.ErrConflict
	}

	spec.SemanticSpec = semanticSpec
	spec.CoreSpecific = coreSpecific
	return nil
}

func (r *inboundSpecRepo) Delete(ctx context.Context, id int64) error {
	result, err := execWithRetry(ctx, r.db, `DELETE FROM inbound_specs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *inboundSpecRepo) FindByID(ctx context.Context, id int64) (*repository.InboundSpec, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, agent_host_id, exit_agent_host_id, exit_node_set_id, relay_path_id, core_type, tag, enabled, semantic_spec, core_specific,
			desired_revision, created_by, updated_by, created_at, updated_at
		FROM inbound_specs
		WHERE id = ?
	`, id)

	return r.scanInboundSpec(row)
}

func (r *inboundSpecRepo) FindByHostCoreTag(ctx context.Context, agentHostID int64, coreType, tag string) (*repository.InboundSpec, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, agent_host_id, exit_agent_host_id, exit_node_set_id, relay_path_id, core_type, tag, enabled, semantic_spec, core_specific,
			desired_revision, created_by, updated_by, created_at, updated_at
		FROM inbound_specs
		WHERE agent_host_id = ? AND core_type = ? AND tag = ?
		LIMIT 1
	`, agentHostID, coreType, tag)

	return r.scanInboundSpec(row)
}

func (r *inboundSpecRepo) FindByCoreTag(ctx context.Context, coreType, tag string) (*repository.InboundSpec, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, agent_host_id, exit_agent_host_id, exit_node_set_id, relay_path_id, core_type, tag, enabled, semantic_spec, core_specific,
			desired_revision, created_by, updated_by, created_at, updated_at
		FROM inbound_specs
		WHERE core_type = ? AND tag = ?
		LIMIT 1
	`, coreType, tag)

	return r.scanInboundSpec(row)
}

func (r *inboundSpecRepo) ListByAgentHost(ctx context.Context, agentHostID int64, filter repository.InboundSpecFilter) ([]*repository.InboundSpec, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)

	query.WriteString(`
		SELECT
			id, agent_host_id, exit_agent_host_id, exit_node_set_id, relay_path_id, core_type, tag, enabled, semantic_spec, core_specific,
			desired_revision, created_by, updated_by, created_at, updated_at
		FROM inbound_specs
		WHERE (agent_host_id = ?`)
	args = append(args, agentHostID)

	query.WriteString(` OR (agent_host_id IS NULL AND id IN (SELECT spec_id FROM spec_host_bindings WHERE agent_host_id = ?))`)
	args = append(args, agentHostID)

	query.WriteString(`)`)

	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		args = append(args, *filter.CoreType)
	}
	if filter.Tag != nil {
		query.WriteString(" AND tag = ?")
		args = append(args, *filter.Tag)
	}
	if filter.Enabled != nil {
		query.WriteString(" AND enabled = ?")
		args = append(args, boolToInt(*filter.Enabled))
	}

	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var specs []*repository.InboundSpec
	for rows.Next() {
		spec, err := r.scanInboundSpec(rows)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, rows.Err()
}

func (r *inboundSpecRepo) CountByAgentHost(ctx context.Context, agentHostID int64, filter repository.InboundSpecFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 6)

	query.WriteString(`SELECT COUNT(*) FROM inbound_specs WHERE (agent_host_id = ?`)
	args = append(args, agentHostID)
	query.WriteString(` OR (agent_host_id IS NULL AND id IN (SELECT spec_id FROM spec_host_bindings WHERE agent_host_id = ?))`)
	args = append(args, agentHostID)
	query.WriteString(`)`)

	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		args = append(args, *filter.CoreType)
	}
	if filter.Tag != nil {
		query.WriteString(" AND tag = ?")
		args = append(args, *filter.Tag)
	}
	if filter.Enabled != nil {
		query.WriteString(" AND enabled = ?")
		args = append(args, boolToInt(*filter.Enabled))
	}

	var total int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total)
	return total, err
}

func (r *inboundSpecRepo) List(ctx context.Context, filter repository.InboundSpecFilter) ([]*repository.InboundSpec, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)

	query.WriteString(`
		SELECT
			id, agent_host_id, exit_agent_host_id, exit_node_set_id, relay_path_id, core_type, tag, enabled, semantic_spec, core_specific,
			desired_revision, created_by, updated_by, created_at, updated_at
		FROM inbound_specs
		WHERE 1 = 1
	`)

	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		args = append(args, *filter.AgentHostID)
	}
	if filter.IsTemplate != nil && *filter.IsTemplate {
		query.WriteString(" AND agent_host_id IS NULL")
	}
	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		args = append(args, *filter.CoreType)
	}
	if filter.Tag != nil {
		query.WriteString(" AND tag = ?")
		args = append(args, *filter.Tag)
	}
	if filter.Enabled != nil {
		query.WriteString(" AND enabled = ?")
		args = append(args, boolToInt(*filter.Enabled))
	}

	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var specs []*repository.InboundSpec
	for rows.Next() {
		spec, err := r.scanInboundSpec(rows)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, rows.Err()
}

func (r *inboundSpecRepo) Count(ctx context.Context, filter repository.InboundSpecFilter) (int64, error) {
	query := strings.Builder{}
	args := make([]any, 0, 6)

	query.WriteString(`SELECT COUNT(*) FROM inbound_specs WHERE 1 = 1`)

	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		args = append(args, *filter.AgentHostID)
	}
	if filter.IsTemplate != nil && *filter.IsTemplate {
		query.WriteString(" AND agent_host_id IS NULL")
	}
	if filter.CoreType != nil {
		query.WriteString(" AND core_type = ?")
		args = append(args, *filter.CoreType)
	}
	if filter.Tag != nil {
		query.WriteString(" AND tag = ?")
		args = append(args, *filter.Tag)
	}
	if filter.Enabled != nil {
		query.WriteString(" AND enabled = ?")
		args = append(args, boolToInt(*filter.Enabled))
	}

	var total int64
	err := r.db.QueryRowContext(ctx, query.String(), args...).Scan(&total)
	return total, err
}

type inboundSpecScanner interface {
	Scan(dest ...any) error
}

func (r *inboundSpecRepo) scanInboundSpec(scanner inboundSpecScanner) (*repository.InboundSpec, error) {
	var spec repository.InboundSpec
	var enabled int
	var semanticSpec sql.NullString
	var coreSpecific sql.NullString
	var agentHostID sql.NullInt64
	var exitAgentHostID sql.NullInt64
	var exitNodeSetID sql.NullInt64
	var relayPathID sql.NullInt64

	err := scanner.Scan(
		&spec.ID,
		&agentHostID,
		&exitAgentHostID,
		&exitNodeSetID,
		&relayPathID,
		&spec.CoreType,
		&spec.Tag,
		&enabled,
		&semanticSpec,
		&coreSpecific,
		&spec.DesiredRevision,
		&spec.CreatedBy,
		&spec.UpdatedBy,
		&spec.CreatedAt,
		&spec.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if agentHostID.Valid {
		spec.AgentHostID = &agentHostID.Int64
	}
	if exitAgentHostID.Valid {
		spec.ExitAgentHostID = &exitAgentHostID.Int64
	}
	if exitNodeSetID.Valid {
		spec.ExitNodeSetID = &exitNodeSetID.Int64
	}
	if relayPathID.Valid {
		spec.RelayPathID = &relayPathID.Int64
	}
	spec.Enabled = enabled != 0
	spec.SemanticSpec = parseJSONObject(semanticSpec)
	spec.CoreSpecific = parseJSONObject(coreSpecific)

	return &spec, nil
}

func normalizeJSONObject(raw []byte) []byte {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return []byte("{}")
	}
	return raw
}

func parseJSONObject(value sql.NullString) []byte {
	if !value.Valid || len(strings.TrimSpace(value.String)) == 0 {
		return []byte("{}")
	}
	return []byte(value.String)
}
