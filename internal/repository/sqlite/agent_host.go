package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/bootstrap"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

type agentHostRepo struct {
	db *sql.DB
}

func newAgentHostRepo(db *sql.DB) *agentHostRepo {
	return &agentHostRepo{db: db}
}

func (r *agentHostRepo) Create(ctx context.Context, host *repository.AgentHost) error {
	now := time.Now().Unix()
	host.CreatedAt = now
	host.UpdatedAt = now

	capsJSON, err := json.Marshal(host.Capabilities)
	if err != nil {
		return fmt.Errorf("encode capabilities: %w", err)
	}
	if host.Capabilities == nil {
		capsJSON = []byte("[]")
	}
	tagsJSON, err := json.Marshal(host.BuildTags)
	if err != nil {
		return fmt.Errorf("encode build tags: %w", err)
	}
	if host.BuildTags == nil {
		tagsJSON = []byte("[]")
	}

	result, err := execWithRetry(ctx, r.db, `
		INSERT INTO agent_hosts (
			name, host, token, status, provision_status, template_id, core_version, capabilities, build_tags,
			cpu_total, cpu_used, mem_total, mem_used,
			disk_total, disk_used, upload_total, download_total,
			upload_rate_bps, download_rate_bps, raw_upload_total_bytes, raw_download_total_bytes,
			boot_id, last_realtime_report_at, last_restart_at, agent_version, current_core_type, config_yaml, country, region,
			last_heartbeat_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		host.Name, host.Host, host.Token, host.Status, host.ProvisionStatus, host.TemplateID,
		host.CoreVersion, string(capsJSON), string(tagsJSON),
		host.CPUTotal, host.CPUUsed, host.MemTotal, host.MemUsed,
		host.DiskTotal, host.DiskUsed, host.UploadTotal, host.DownloadTotal,
		host.UploadRateBps, host.DownloadRateBps, host.RawUploadTotalBytes, host.RawDownloadTotalBytes,
		host.BootID, host.LastRealtimeReportAt, host.LastRestartAt, host.AgentVersion, host.CurrentCoreType,
		host.ConfigYAML, host.Country, host.Region,
		host.LastHeartbeatAt, host.CreatedAt, host.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	host.ID = id
	return nil
}

func (r *agentHostRepo) FindByID(ctx context.Context, id int64) (*repository.AgentHost, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, host, token, status, provision_status, template_id, core_version, capabilities, build_tags,
			cpu_total, cpu_used, mem_total, mem_used,
			disk_total, disk_used, upload_total, download_total,
			upload_rate_bps, download_rate_bps, raw_upload_total_bytes, raw_download_total_bytes,
			boot_id, last_realtime_report_at, last_restart_at, agent_version, current_core_type, config_yaml, country, region,
			last_heartbeat_at, created_at, updated_at
		FROM agent_hosts WHERE id = ?
	`, id)

	return r.scanHost(row)
}

func (r *agentHostRepo) FindByHost(ctx context.Context, host string) (*repository.AgentHost, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, host, token, status, provision_status, template_id, core_version, capabilities, build_tags,
			cpu_total, cpu_used, mem_total, mem_used,
			disk_total, disk_used, upload_total, download_total,
			upload_rate_bps, download_rate_bps, raw_upload_total_bytes, raw_download_total_bytes,
			boot_id, last_realtime_report_at, last_restart_at, agent_version, current_core_type, config_yaml, country, region,
			last_heartbeat_at, created_at, updated_at
		FROM agent_hosts WHERE host = ?
	`, host)

	return r.scanHost(row)
}

func (r *agentHostRepo) FindByToken(ctx context.Context, token string) (*repository.AgentHost, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, host, token, status, provision_status, template_id, core_version, capabilities, build_tags,
			cpu_total, cpu_used, mem_total, mem_used,
			disk_total, disk_used, upload_total, download_total,
			upload_rate_bps, download_rate_bps, raw_upload_total_bytes, raw_download_total_bytes,
			boot_id, last_realtime_report_at, last_restart_at, agent_version, current_core_type, config_yaml, country, region,
			last_heartbeat_at, created_at, updated_at
		FROM agent_hosts WHERE token = ?
	`, token)

	return r.scanHost(row)
}

func (r *agentHostRepo) Update(ctx context.Context, host *repository.AgentHost) error {
	host.UpdatedAt = time.Now().Unix()

	capsJSON, err := json.Marshal(host.Capabilities)
	if err != nil {
		return fmt.Errorf("encode capabilities: %w", err)
	}
	if host.Capabilities == nil {
		capsJSON = []byte("[]")
	}
	tagsJSON, err := json.Marshal(host.BuildTags)
	if err != nil {
		return fmt.Errorf("encode build tags: %w", err)
	}
	if host.BuildTags == nil {
		tagsJSON = []byte("[]")
	}

	// NOTE: no RowsAffected check -- caller already holds the host from DB;
	// a zero-rows update indicates a concurrent delete, which is acceptable.
	_, err = execWithRetry(ctx, r.db, `
		UPDATE agent_hosts SET
			name = ?, host = ?, token = ?, status = ?, provision_status = ?, template_id = ?,
			core_version = ?, capabilities = ?, build_tags = ?, config_yaml = ?, country = ?, region = ?,
			cpu_total = ?, cpu_used = ?, mem_total = ?, mem_used = ?,
			disk_total = ?, disk_used = ?, upload_total = ?, download_total = ?,
			last_heartbeat_at = ?, updated_at = ?
		WHERE id = ?
	`,
		host.Name, host.Host, host.Token, host.Status, host.ProvisionStatus, host.TemplateID,
		host.CoreVersion, string(capsJSON), string(tagsJSON),
		host.ConfigYAML, host.Country, host.Region,
		host.CPUTotal, host.CPUUsed, host.MemTotal, host.MemUsed,
		host.DiskTotal, host.DiskUsed, host.UploadTotal, host.DownloadTotal,
		host.LastHeartbeatAt, host.UpdatedAt, host.ID,
	)
	return err
}

func (r *agentHostRepo) Delete(ctx context.Context, id int64) error {
	result, err := execWithRetry(ctx, r.db, `DELETE FROM agent_hosts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *agentHostRepo) ListAll(ctx context.Context) ([]*repository.AgentHost, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, host, token, status, provision_status, template_id, core_version, capabilities, build_tags,
			cpu_total, cpu_used, mem_total, mem_used,
			disk_total, disk_used, upload_total, download_total,
			upload_rate_bps, download_rate_bps, raw_upload_total_bytes, raw_download_total_bytes,
			boot_id, last_realtime_report_at, last_restart_at, agent_version, current_core_type, config_yaml, country, region,
			last_heartbeat_at, created_at, updated_at
		FROM agent_hosts ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("agent_hosts list query: %w", err)
	}
	defer rows.Close()

	var hosts []*repository.AgentHost
	for rows.Next() {
		host, err := r.scanHostFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("agent_hosts scan row: %w", err)
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("agent_hosts rows iteration: %w", err)
	}
	return hosts, nil
}

	// NOTE: no RowsAffected check -- this is a periodic heartbeat update;
	// the host may have been deleted concurrently, which is acceptable.
func (r *agentHostRepo) UpdateStatus(ctx context.Context, id int64, status int, heartbeatAt int64) error {
	return bootstrap.WithSQLiteBusyRetry(func() error {
		_, err := execWithRetry(ctx, r.db, `
			UPDATE agent_hosts SET
				status = ?,
				last_heartbeat_at = ?,
				updated_at = ?
			WHERE id = ?
		`, status, heartbeatAt, time.Now().Unix(), id)
		return err
	})
}

func (r *agentHostRepo) UpdateMetrics(ctx context.Context, id int64, metrics repository.AgentHostMetrics) error {
	return bootstrap.WithSQLiteBusyRetry(func() error {
		updatedAt := time.Now().Unix()
		_, err := execWithRetry(ctx, r.db, `
			UPDATE agent_hosts SET
				cpu_total = ?, cpu_used = ?,
				mem_total = ?, mem_used = ?,
				disk_total = ?, disk_used = ?,
				upload_total = ?, download_total = ?,
				upload_rate_bps = ?, download_rate_bps = ?,
				raw_upload_total_bytes = ?, raw_download_total_bytes = ?,
				boot_id = ?, last_realtime_report_at = ?, last_restart_at = ?,
				agent_version = ?, current_core_type = ?,
				last_heartbeat_at = ?,
				status = 1,
				updated_at = ?
			WHERE id = ?
		`,
			metrics.CPUTotal, metrics.CPUUsed,
			metrics.MemTotal, metrics.MemUsed,
			metrics.DiskTotal, metrics.DiskUsed,
			metrics.UploadTotal, metrics.DownloadTotal,
			metrics.UploadRateBps, metrics.DownloadRateBps,
			metrics.RawUploadTotalBytes, metrics.RawDownloadTotalBytes,
			metrics.BootID, metrics.LastRealtimeReportAt, metrics.LastRestartAt,
			metrics.AgentVersion, metrics.CurrentCoreType,
			updatedAt, updatedAt, id,
		)
		return err
	})
	// NOTE: no RowsAffected check -- metrics updates are fire-and-forget; the host may be deleted concurrently.
}

func (r *agentHostRepo) scanHost(row *sql.Row) (*repository.AgentHost, error) {
	var h repository.AgentHost
	var capsJSON, tagsJSON string

	err := row.Scan(
		&h.ID, &h.Name, &h.Host, &h.Token, &h.Status, &h.ProvisionStatus, &h.TemplateID,
		&h.CoreVersion, &capsJSON, &tagsJSON,
		&h.CPUTotal, &h.CPUUsed, &h.MemTotal, &h.MemUsed,
		&h.DiskTotal, &h.DiskUsed, &h.UploadTotal, &h.DownloadTotal,
		&h.UploadRateBps, &h.DownloadRateBps, &h.RawUploadTotalBytes, &h.RawDownloadTotalBytes,
		&h.BootID, &h.LastRealtimeReportAt, &h.LastRestartAt, &h.AgentVersion, &h.CurrentCoreType,
		&h.ConfigYAML, &h.Country, &h.Region,
		&h.LastHeartbeatAt, &h.CreatedAt, &h.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	// Parse JSON arrays
	if capsJSON != "" {
		if err := json.Unmarshal([]byte(capsJSON), &h.Capabilities); err != nil {
			return nil, fmt.Errorf("decode capabilities: %w", err)
		}
	}
	if h.Capabilities == nil {
		h.Capabilities = []string{}
	}
	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &h.BuildTags); err != nil {
			return nil, fmt.Errorf("decode build tags: %w", err)
		}
	}
	if h.BuildTags == nil {
		h.BuildTags = []string{}
	}

	return &h, nil
}

func (r *agentHostRepo) scanHostFromRows(rows *sql.Rows) (*repository.AgentHost, error) {
	var h repository.AgentHost
	var capsJSON, tagsJSON string

	err := rows.Scan(
		&h.ID, &h.Name, &h.Host, &h.Token, &h.Status, &h.ProvisionStatus, &h.TemplateID,
		&h.CoreVersion, &capsJSON, &tagsJSON,
		&h.CPUTotal, &h.CPUUsed, &h.MemTotal, &h.MemUsed,
		&h.DiskTotal, &h.DiskUsed, &h.UploadTotal, &h.DownloadTotal,
		&h.UploadRateBps, &h.DownloadRateBps, &h.RawUploadTotalBytes, &h.RawDownloadTotalBytes,
		&h.BootID, &h.LastRealtimeReportAt, &h.LastRestartAt, &h.AgentVersion, &h.CurrentCoreType,
		&h.ConfigYAML, &h.Country, &h.Region,
		&h.LastHeartbeatAt, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse JSON arrays
	if capsJSON != "" {
		if err := json.Unmarshal([]byte(capsJSON), &h.Capabilities); err != nil {
			return nil, fmt.Errorf("decode capabilities: %w", err)
		}
	}
	if h.Capabilities == nil {
		h.Capabilities = []string{}
	}
	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &h.BuildTags); err != nil {
			return nil, fmt.Errorf("decode build tags: %w", err)
		}
	}
	if h.BuildTags == nil {
		h.BuildTags = []string{}
	}

	return &h, nil
}

// UpdateCapabilities updates agent capabilities.
func (r *agentHostRepo) UpdateCapabilities(ctx context.Context, id int64, coreVersion string, capabilities, buildTags []string) error {
	capsJSON, err := json.Marshal(capabilities)
	if err != nil {
		return fmt.Errorf("encode capabilities: %w", err)
	}
	if capabilities == nil {
		capsJSON = []byte("[]")
	}
	tagsJSON, err := json.Marshal(buildTags)
	if err != nil {
		return fmt.Errorf("encode build tags: %w", err)
	}
	if buildTags == nil {
		tagsJSON = []byte("[]")
	}

	return bootstrap.WithSQLiteBusyRetry(func() error {
		_, err := execWithRetry(ctx, r.db, `
			UPDATE agent_hosts SET
				core_version = ?, capabilities = ?, build_tags = ?, updated_at = ?
			WHERE id = ?
		`, coreVersion, string(capsJSON), string(tagsJSON), time.Now().Unix(), id)
		return err
	})
	// NOTE: no RowsAffected check -- capability updates are fire-and-forget;
	// the host may be deleted concurrently.
}

func (r *agentHostRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_hosts").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *agentHostRepo) CountOnline(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_hosts WHERE status = 1").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}