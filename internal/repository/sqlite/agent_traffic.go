package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type agentTrafficPolicyRepo struct {
	db *sql.DB
}

func newAgentTrafficPolicyRepo(db *sql.DB) *agentTrafficPolicyRepo {
	return &agentTrafficPolicyRepo{db: db}
}

func (r *agentTrafficPolicyRepo) Upsert(ctx context.Context, policy *repository.AgentTrafficPolicy) (*repository.AgentTrafficPolicy, error) {
	if policy == nil {
		return nil, errors.New("agent traffic policy is nil")
	}
	if policy.AgentHostID <= 0 {
		return nil, errors.New("agent host id is required")
	}
	policy.LimitType = strings.TrimSpace(policy.LimitType)
	policy.ThresholdAction = strings.TrimSpace(policy.ThresholdAction)
	policy.ResetMode = strings.TrimSpace(policy.ResetMode)
	if policy.LimitType == "" {
		policy.LimitType = "sum"
	}
	if policy.ThresholdPercent == 0 {
		policy.ThresholdPercent = 100
	}
	if policy.ThresholdAction == "" {
		policy.ThresholdAction = "notify_only"
	}
	if policy.ResetMode == "" {
		policy.ResetMode = "off"
	}
	if policy.ResetDay == 0 {
		policy.ResetDay = 1
	}
	if policy.UpdatedAt == 0 {
		policy.UpdatedAt = time.Now().Unix()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_traffic_policies (
			agent_host_id, enabled, limit_bytes, limit_type, threshold_percent,
			threshold_action, threshold_reached, reset_mode, reset_day, interval_days,
			anchor_at, last_reset_at, last_reset_cycle_key, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_host_id) DO UPDATE SET
			enabled = excluded.enabled,
			limit_bytes = excluded.limit_bytes,
			limit_type = excluded.limit_type,
			threshold_percent = excluded.threshold_percent,
			threshold_action = excluded.threshold_action,
			threshold_reached = excluded.threshold_reached,
			reset_mode = excluded.reset_mode,
			reset_day = excluded.reset_day,
			interval_days = excluded.interval_days,
			anchor_at = excluded.anchor_at,
			last_reset_at = excluded.last_reset_at,
			last_reset_cycle_key = excluded.last_reset_cycle_key,
			updated_at = excluded.updated_at
	`,
		policy.AgentHostID,
		boolToInt(policy.Enabled),
		policy.LimitBytes,
		policy.LimitType,
		policy.ThresholdPercent,
		policy.ThresholdAction,
		boolToInt(policy.ThresholdReached),
		policy.ResetMode,
		policy.ResetDay,
		policy.IntervalDays,
		policy.AnchorAt,
		policy.LastResetAt,
		policy.LastResetCycleKey,
		policy.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return r.FindByAgentHostID(ctx, policy.AgentHostID)
}

func (r *agentTrafficPolicyRepo) FindByAgentHostID(ctx context.Context, agentHostID int64) (*repository.AgentTrafficPolicy, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT agent_host_id, enabled, limit_bytes, limit_type, threshold_percent,
			threshold_action, threshold_reached, reset_mode, reset_day, interval_days,
			anchor_at, last_reset_at, last_reset_cycle_key, updated_at
		FROM agent_traffic_policies
		WHERE agent_host_id = ?
		LIMIT 1
	`, agentHostID)
	return scanAgentTrafficPolicy(row)
}

func (r *agentTrafficPolicyRepo) List(ctx context.Context, filter repository.AgentTrafficPolicyFilter) ([]*repository.AgentTrafficPolicy, error) {
	query := strings.Builder{}
	args := make([]any, 0, 8)
	query.WriteString(`
		SELECT agent_host_id, enabled, limit_bytes, limit_type, threshold_percent,
			threshold_action, threshold_reached, reset_mode, reset_day, interval_days,
			anchor_at, last_reset_at, last_reset_cycle_key, updated_at
		FROM agent_traffic_policies WHERE 1 = 1
	`)
	appendAgentTrafficPolicyFilterConditions(&query, &args, filter)
	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY agent_host_id ASC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := make([]*repository.AgentTrafficPolicy, 0)
	for rows.Next() {
		policy, err := scanAgentTrafficPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}

func (r *agentTrafficPolicyRepo) UpdateThresholdReached(ctx context.Context, agentHostID int64, reached bool, updatedAt int64) error {
	if updatedAt == 0 {
		updatedAt = time.Now().Unix()
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_traffic_policies
		SET threshold_reached = ?, updated_at = ?
		WHERE agent_host_id = ?
	`, boolToInt(reached), updatedAt, agentHostID)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *agentTrafficPolicyRepo) UpdateResetState(ctx context.Context, agentHostID int64, lastResetAt int64, cycleKey string, updatedAt int64) error {
	if updatedAt == 0 {
		updatedAt = time.Now().Unix()
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_traffic_policies
		SET threshold_reached = 0, last_reset_at = ?, last_reset_cycle_key = ?, updated_at = ?
		WHERE agent_host_id = ?
	`, lastResetAt, cycleKey, updatedAt, agentHostID)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func appendAgentTrafficPolicyFilterConditions(query *strings.Builder, args *[]any, filter repository.AgentTrafficPolicyFilter) {
	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		*args = append(*args, *filter.AgentHostID)
	}
	if filter.Enabled != nil {
		query.WriteString(" AND enabled = ?")
		*args = append(*args, boolToInt(*filter.Enabled))
	}
	if filter.ThresholdReached != nil {
		query.WriteString(" AND threshold_reached = ?")
		*args = append(*args, boolToInt(*filter.ThresholdReached))
	}
	if filter.ResetMode != nil {
		query.WriteString(" AND reset_mode = ?")
		*args = append(*args, *filter.ResetMode)
	}
}

type agentTrafficPolicyScanner interface {
	Scan(dest ...any) error
}

func scanAgentTrafficPolicy(scanner agentTrafficPolicyScanner) (*repository.AgentTrafficPolicy, error) {
	var policy repository.AgentTrafficPolicy
	var enabled int
	var thresholdReached int
	if err := scanner.Scan(
		&policy.AgentHostID,
		&enabled,
		&policy.LimitBytes,
		&policy.LimitType,
		&policy.ThresholdPercent,
		&policy.ThresholdAction,
		&thresholdReached,
		&policy.ResetMode,
		&policy.ResetDay,
		&policy.IntervalDays,
		&policy.AnchorAt,
		&policy.LastResetAt,
		&policy.LastResetCycleKey,
		&policy.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	policy.Enabled = enabled != 0
	policy.ThresholdReached = thresholdReached != 0
	return &policy, nil
}

type agentTrafficStateRepo struct {
	db *sql.DB
}

func newAgentTrafficStateRepo(db *sql.DB) *agentTrafficStateRepo {
	return &agentTrafficStateRepo{db: db}
}

func (r *agentTrafficStateRepo) Upsert(ctx context.Context, state *repository.AgentTrafficState) (*repository.AgentTrafficState, error) {
	if state == nil {
		return nil, errors.New("agent traffic state is nil")
	}
	if state.AgentHostID <= 0 {
		return nil, errors.New("agent host id is required")
	}
	state.BootID = strings.TrimSpace(state.BootID)
	if state.UpdatedAt == 0 {
		state.UpdatedAt = time.Now().Unix()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_traffic_states (
			agent_host_id, boot_id, last_raw_upload_bytes, last_raw_download_bytes,
			counter_seen, cycle_upload_bytes, cycle_download_bytes, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_host_id) DO UPDATE SET
			boot_id = excluded.boot_id,
			last_raw_upload_bytes = excluded.last_raw_upload_bytes,
			last_raw_download_bytes = excluded.last_raw_download_bytes,
			counter_seen = excluded.counter_seen,
			cycle_upload_bytes = excluded.cycle_upload_bytes,
			cycle_download_bytes = excluded.cycle_download_bytes,
			updated_at = excluded.updated_at
	`,
		state.AgentHostID,
		state.BootID,
		state.LastRawUploadBytes,
		state.LastRawDownloadBytes,
		boolToInt(state.CounterSeen),
		state.CycleUploadBytes,
		state.CycleDownloadBytes,
		state.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return r.FindByAgentHostID(ctx, state.AgentHostID)
}

func (r *agentTrafficStateRepo) FindByAgentHostID(ctx context.Context, agentHostID int64) (*repository.AgentTrafficState, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT agent_host_id, boot_id, last_raw_upload_bytes, last_raw_download_bytes,
			counter_seen, cycle_upload_bytes, cycle_download_bytes, updated_at
		FROM agent_traffic_states
		WHERE agent_host_id = ?
		LIMIT 1
	`, agentHostID)
	return scanAgentTrafficState(row)
}

func (r *agentTrafficStateRepo) List(ctx context.Context, filter repository.AgentTrafficStateFilter) ([]*repository.AgentTrafficState, error) {
	query := strings.Builder{}
	args := make([]any, 0, 6)
	query.WriteString(`
		SELECT agent_host_id, boot_id, last_raw_upload_bytes, last_raw_download_bytes,
			counter_seen, cycle_upload_bytes, cycle_download_bytes, updated_at
		FROM agent_traffic_states WHERE 1 = 1
	`)
	appendAgentTrafficStateFilterConditions(&query, &args, filter)
	limit, offset := normalizePagination(filter.Limit, filter.Offset, 100)
	query.WriteString(" ORDER BY agent_host_id ASC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]*repository.AgentTrafficState, 0)
	for rows.Next() {
		state, err := scanAgentTrafficState(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

func (r *agentTrafficStateRepo) ResetCycle(ctx context.Context, agentHostID int64, resetAt int64) error {
	if resetAt == 0 {
		resetAt = time.Now().Unix()
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_traffic_states
		SET cycle_upload_bytes = 0, cycle_download_bytes = 0, updated_at = ?
		WHERE agent_host_id = ?
	`, resetAt, agentHostID)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func appendAgentTrafficStateFilterConditions(query *strings.Builder, args *[]any, filter repository.AgentTrafficStateFilter) {
	if filter.AgentHostID != nil {
		query.WriteString(" AND agent_host_id = ?")
		*args = append(*args, *filter.AgentHostID)
	}
	if filter.BootID != nil {
		query.WriteString(" AND boot_id = ?")
		*args = append(*args, *filter.BootID)
	}
}

type agentTrafficStateScanner interface {
	Scan(dest ...any) error
}

func scanAgentTrafficState(scanner agentTrafficStateScanner) (*repository.AgentTrafficState, error) {
	var state repository.AgentTrafficState
	var counterSeen int
	if err := scanner.Scan(
		&state.AgentHostID,
		&state.BootID,
		&state.LastRawUploadBytes,
		&state.LastRawDownloadBytes,
		&counterSeen,
		&state.CycleUploadBytes,
		&state.CycleDownloadBytes,
		&state.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	state.CounterSeen = counterSeen != 0
	return &state, nil
}
