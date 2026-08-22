package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type agentCoreInstanceRepo struct {
	db *sql.DB
}

func newAgentCoreInstanceRepo(db *sql.DB) *agentCoreInstanceRepo {
	return &agentCoreInstanceRepo{db: db}
}

func (r *agentCoreInstanceRepo) Create(ctx context.Context, instance *repository.AgentCoreInstance) error {
	if instance == nil {
		return errors.New("instance is nil")
	}
	result, err := r.insertInstance(ctx, r.db, instance)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	instance.ID = id
	return nil
}

func (r *agentCoreInstanceRepo) Update(ctx context.Context, instance *repository.AgentCoreInstance) error {
	if instance == nil {
		return errors.New("instance is nil")
	}
	instance.UpdatedAt = time.Now().Unix()
	portsJSON, snapshotJSON, err := encodeInstanceSnapshot(instance)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_core_instances SET
			agent_host_id = ?, instance_id = ?, core_type = ?, status = ?, listen_ports = ?,
			config_template_id = ?, config_hash = ?, started_at = ?, last_heartbeat_at = ?,
			error_message = ?, core_snapshot = ?, updated_at = ?
		WHERE id = ?
	`,
		instance.AgentHostID,
		instance.InstanceID,
		instance.CoreType,
		instance.Status,
		portsJSON,
		optionalInt64(instance.ConfigTemplateID),
		instance.ConfigHash,
		optionalInt64(instance.StartedAt),
		optionalInt64(instance.LastHeartbeatAt),
		instance.ErrorMessage,
		snapshotJSON,
		instance.UpdatedAt,
		instance.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *agentCoreInstanceRepo) ReplaceSnapshot(ctx context.Context, agentHostID int64, instances []*repository.AgentCoreInstance) error {
	if agentHostID <= 0 {
		return errors.New("agent host id is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentRows, err := tx.QueryContext(ctx, `
		SELECT id, agent_host_id, instance_id, core_type, status, listen_ports,
			config_template_id, config_hash, started_at, last_heartbeat_at,
			error_message, core_snapshot, created_at, updated_at
		FROM agent_core_instances WHERE agent_host_id = ?
	`, agentHostID)
	if err != nil {
		return err
	}
	defer currentRows.Close()

	currentByInstanceID := make(map[string]*repository.AgentCoreInstance)
	for currentRows.Next() {
		inst, scanErr := r.scanInstanceRow(currentRows)
		if scanErr != nil {
			return scanErr
		}
		currentByInstanceID[inst.InstanceID] = inst
	}
	if err := currentRows.Err(); err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		if instance == nil || strings.TrimSpace(instance.InstanceID) == "" {
			continue
		}
		instance.AgentHostID = agentHostID
		seen[instance.InstanceID] = struct{}{}
		if existing, ok := currentByInstanceID[instance.InstanceID]; ok {
			instance.ID = existing.ID
			instance.CreatedAt = existing.CreatedAt
			if snapshotsEqual(existing, instance) {
				continue
			}
			if err := r.updateInstanceWithExecutor(ctx, tx, instance); err != nil {
				return err
			}
			continue
		}
		result, err := r.insertInstance(ctx, tx, instance)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		instance.ID = id
	}

	now := time.Now().Unix()
	for instanceID, existing := range currentByInstanceID {
		if _, ok := seen[instanceID]; ok {
			continue
		}
		if existing.Status == "stopped" || existing.Status == "offline" || existing.Status == "removed" {
			continue
		}
		existing.Status = "stopped"
		existing.ErrorMessage = "instance missing from latest snapshot"
		existing.LastHeartbeatAt = &now
		if err := r.updateInstanceWithExecutor(ctx, tx, existing); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *agentCoreInstanceRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM agent_core_instances WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *agentCoreInstanceRepo) FindByID(ctx context.Context, id int64) (*repository.AgentCoreInstance, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, agent_host_id, instance_id, core_type, status, listen_ports,
			config_template_id, config_hash, started_at, last_heartbeat_at,
			error_message, core_snapshot, created_at, updated_at
		FROM agent_core_instances WHERE id = ?
	`, id)
	return r.scanInstance(row)
}

func (r *agentCoreInstanceRepo) FindByInstanceID(ctx context.Context, agentHostID int64, instanceID string) (*repository.AgentCoreInstance, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, agent_host_id, instance_id, core_type, status, listen_ports,
			config_template_id, config_hash, started_at, last_heartbeat_at,
			error_message, core_snapshot, created_at, updated_at
		FROM agent_core_instances WHERE agent_host_id = ? AND instance_id = ?
		LIMIT 1
	`, agentHostID, instanceID)
	return r.scanInstance(row)
}

func (r *agentCoreInstanceRepo) ListByAgentHostID(ctx context.Context, agentHostID int64) ([]*repository.AgentCoreInstance, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, agent_host_id, instance_id, core_type, status, listen_ports,
			config_template_id, config_hash, started_at, last_heartbeat_at,
			error_message, core_snapshot, created_at, updated_at
		FROM agent_core_instances
		WHERE agent_host_id = ?
		ORDER BY id DESC
	`, agentHostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []*repository.AgentCoreInstance
	for rows.Next() {
		instance, err := r.scanInstanceRow(rows)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	return instances, rows.Err()
}

func (r *agentCoreInstanceRepo) UpdateHeartbeat(ctx context.Context, agentHostID int64, instanceID string, heartbeatAt int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE agent_core_instances
		SET last_heartbeat_at = ?, updated_at = ?
		WHERE agent_host_id = ? AND instance_id = ?
	`, heartbeatAt, time.Now().Unix(), agentHostID, instanceID)
	return err
}

type agentCoreInstanceScanner interface {
	Scan(dest ...any) error
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (r *agentCoreInstanceRepo) insertInstance(ctx context.Context, execer sqlExecutor, instance *repository.AgentCoreInstance) (sql.Result, error) {
	now := time.Now().Unix()
	if instance.CreatedAt == 0 {
		instance.CreatedAt = now
	}
	if instance.UpdatedAt == 0 {
		instance.UpdatedAt = now
	}
	portsJSON, snapshotJSON, err := encodeInstanceSnapshot(instance)
	if err != nil {
		return nil, err
	}
	return execer.ExecContext(ctx, `
		INSERT INTO agent_core_instances (
			agent_host_id, instance_id, core_type, status, listen_ports,
			config_template_id, config_hash, started_at, last_heartbeat_at,
			error_message, core_snapshot, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		instance.AgentHostID,
		instance.InstanceID,
		instance.CoreType,
		instance.Status,
		portsJSON,
		optionalInt64(instance.ConfigTemplateID),
		instance.ConfigHash,
		optionalInt64(instance.StartedAt),
		optionalInt64(instance.LastHeartbeatAt),
		instance.ErrorMessage,
		snapshotJSON,
		instance.CreatedAt,
		instance.UpdatedAt,
	)
}

func (r *agentCoreInstanceRepo) updateInstanceWithExecutor(ctx context.Context, execer sqlExecutor, instance *repository.AgentCoreInstance) error {
	instance.UpdatedAt = time.Now().Unix()
	portsJSON, snapshotJSON, err := encodeInstanceSnapshot(instance)
	if err != nil {
		return err
	}
	_, err = execer.ExecContext(ctx, `
		UPDATE agent_core_instances SET
			agent_host_id = ?, instance_id = ?, core_type = ?, status = ?, listen_ports = ?,
			config_template_id = ?, config_hash = ?, started_at = ?, last_heartbeat_at = ?,
			error_message = ?, core_snapshot = ?, updated_at = ?
		WHERE id = ?
	`,
		instance.AgentHostID,
		instance.InstanceID,
		instance.CoreType,
		instance.Status,
		portsJSON,
		optionalInt64(instance.ConfigTemplateID),
		instance.ConfigHash,
		optionalInt64(instance.StartedAt),
		optionalInt64(instance.LastHeartbeatAt),
		instance.ErrorMessage,
		snapshotJSON,
		instance.UpdatedAt,
		instance.ID,
	)
	return err
}

func (r *agentCoreInstanceRepo) scanInstance(scanner agentCoreInstanceScanner) (*repository.AgentCoreInstance, error) {
	var instance repository.AgentCoreInstance
	var listenPorts sql.NullString
	var templateID sql.NullInt64
	var startedAt sql.NullInt64
	var heartbeatAt sql.NullInt64
	var errorMessage sql.NullString
	var coreSnapshot sql.NullString

	err := scanner.Scan(
		&instance.ID,
		&instance.AgentHostID,
		&instance.InstanceID,
		&instance.CoreType,
		&instance.Status,
		&listenPorts,
		&templateID,
		&instance.ConfigHash,
		&startedAt,
		&heartbeatAt,
		&errorMessage,
		&coreSnapshot,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if listenPorts.Valid {
		if err := json.Unmarshal([]byte(listenPorts.String), &instance.ListenPorts); err != nil {
			return nil, fmt.Errorf("decode listen ports: %w", err)
		}
	}
	if instance.ListenPorts == nil {
		instance.ListenPorts = []int{}
	}
	if templateID.Valid {
		instance.ConfigTemplateID = nullableIntPtr(templateID)
	}
	if startedAt.Valid {
		instance.StartedAt = nullableIntPtr(startedAt)
	}
	if heartbeatAt.Valid {
		instance.LastHeartbeatAt = nullableIntPtr(heartbeatAt)
	}
	if errorMessage.Valid {
		instance.ErrorMessage = errorMessage.String
	}
	if coreSnapshot.Valid {
		snapshot, err := decodeCoreSnapshot(coreSnapshot.String)
		if err != nil {
			return nil, err
		}
		instance.CoreSnapshot = snapshot
	}

	return &instance, nil
}

func (r *agentCoreInstanceRepo) scanInstanceRow(rows *sql.Rows) (*repository.AgentCoreInstance, error) {
	var instance repository.AgentCoreInstance
	var listenPorts sql.NullString
	var templateID sql.NullInt64
	var startedAt sql.NullInt64
	var heartbeatAt sql.NullInt64
	var errorMessage sql.NullString
	var coreSnapshot sql.NullString

	if err := rows.Scan(
		&instance.ID,
		&instance.AgentHostID,
		&instance.InstanceID,
		&instance.CoreType,
		&instance.Status,
		&listenPorts,
		&templateID,
		&instance.ConfigHash,
		&startedAt,
		&heartbeatAt,
		&errorMessage,
		&coreSnapshot,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if listenPorts.Valid {
		if err := json.Unmarshal([]byte(listenPorts.String), &instance.ListenPorts); err != nil {
			return nil, fmt.Errorf("decode listen ports: %w", err)
		}
	}
	if instance.ListenPorts == nil {
		instance.ListenPorts = []int{}
	}
	if templateID.Valid {
		instance.ConfigTemplateID = nullableIntPtr(templateID)
	}
	if startedAt.Valid {
		instance.StartedAt = nullableIntPtr(startedAt)
	}
	if heartbeatAt.Valid {
		instance.LastHeartbeatAt = nullableIntPtr(heartbeatAt)
	}
	if errorMessage.Valid {
		instance.ErrorMessage = errorMessage.String
	}
	if coreSnapshot.Valid {
		snapshot, err := decodeCoreSnapshot(coreSnapshot.String)
		if err != nil {
			return nil, err
		}
		instance.CoreSnapshot = snapshot
	}
	return &instance, nil
}

func encodeInstanceSnapshot(instance *repository.AgentCoreInstance) (string, string, error) {
	portsJSON, err := json.Marshal(instance.ListenPorts)
	if err != nil {
		return "", "", err
	}
	if instance.ListenPorts == nil {
		portsJSON = []byte("[]")
	}
	snapshotJSON, err := encodeCoreSnapshot(instance.CoreSnapshot)
	if err != nil {
		return "", "", err
	}
	return string(portsJSON), snapshotJSON, nil
}

func encodeCoreSnapshot(snapshot *repository.CoreStatusSnapshot) (string, error) {
	if snapshot == nil {
		return `{}`, nil
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode core snapshot: %w", err)
	}
	return string(payload), nil
}

func decodeCoreSnapshot(raw string) (*repository.CoreStatusSnapshot, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == `{}` {
		return nil, nil
	}
	var snapshot repository.CoreStatusSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil, fmt.Errorf("decode core snapshot: %w", err)
	}
	return &snapshot, nil
}

func snapshotsEqual(current, next *repository.AgentCoreInstance) bool {
	if current == nil || next == nil {
		return false
	}
	if current.AgentHostID != next.AgentHostID || current.InstanceID != next.InstanceID || current.CoreType != next.CoreType || current.Status != next.Status || current.ConfigHash != next.ConfigHash || current.ErrorMessage != next.ErrorMessage {
		return false
	}
	if !equalIntPtr(current.ConfigTemplateID, next.ConfigTemplateID) || !equalIntPtr(current.StartedAt, next.StartedAt) || !equalIntPtr(current.LastHeartbeatAt, next.LastHeartbeatAt) {
		return false
	}
	if len(current.ListenPorts) != len(next.ListenPorts) {
		return false
	}
	for i := range current.ListenPorts {
		if current.ListenPorts[i] != next.ListenPorts[i] {
			return false
		}
	}
	return coreSnapshotsEqual(current.CoreSnapshot, next.CoreSnapshot)
}

func equalIntPtr(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func coreSnapshotsEqual(left, right *repository.CoreStatusSnapshot) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	if left.Type != right.Type || left.Version != right.Version || left.Installed != right.Installed {
		return false
	}
	if len(left.Capabilities) != len(right.Capabilities) {
		return false
	}
	for i := range left.Capabilities {
		if left.Capabilities[i] != right.Capabilities[i] {
			return false
		}
	}
	return true
}
