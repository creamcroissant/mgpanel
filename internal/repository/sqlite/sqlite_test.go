// 文件路径: internal/repository/sqlite/sqlite_test.go
// 模块说明: 这是 internal 模块里的 sqlite_test 逻辑。
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/creamcroissant/mgpanel/internal/migrations"
	"github.com/creamcroissant/mgpanel/internal/repository"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	if err := migrations.Up(db); err != nil {
		t.Fatalf("failed to migrate db: %v", err)
	}
	return db
}

func TestMigrationsObservabilitySchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	operationLogColumns := loadTableColumns(t, db, "agent_operation_logs")
	for _, column := range []string{"id", "scope", "target_id", "agent_host_id", "sequence", "phase", "level", "message", "payload_json", "source_event_id", "reported_at", "created_at"} {
		if _, ok := operationLogColumns[column]; !ok {
			t.Fatalf("expected agent_operation_logs.%s column", column)
		}
	}
	binaryVersionColumns := loadTableColumns(t, db, "agent_binary_version_states")
	for _, column := range []string{"id", "agent_host_id", "component", "local_version", "remote_version", "status", "capabilities_json", "build_tags_json", "last_checked_at", "last_check_error", "updated_at"} {
		if _, ok := binaryVersionColumns[column]; !ok {
			t.Fatalf("expected agent_binary_version_states.%s column", column)
		}
	}
	agentHostColumns := loadTableColumns(t, db, "agent_hosts")
	for _, column := range []string{"upload_rate_bps", "download_rate_bps", "raw_upload_total_bytes", "raw_download_total_bytes", "boot_id", "last_realtime_report_at", "last_restart_at", "agent_version", "current_core_type"} {
		if _, ok := agentHostColumns[column]; !ok {
			t.Fatalf("expected agent_hosts.%s column", column)
		}
	}
	for _, index := range []string{"idx_agent_operation_logs_scope_target_id", "idx_agent_operation_logs_agent_created", "idx_agent_operation_logs_scope_target_sequence", "idx_agent_operation_logs_source_event", "idx_agent_binary_version_states_agent_status", "idx_agent_binary_version_states_component_status"} {
		assertIndexExists(t, db, index)
	}

	_, err := db.Exec(`INSERT INTO agent_hosts (name, host, token, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, "agent", "127.0.0.1", "token", 1, 1)
	if err != nil {
		t.Fatalf("insert agent host: %v", err)
	}
	var uploadRate, downloadRate, rawUpload, rawDownload, realtimeAt, restartAt int64
	var bootID, agentVersion, currentCoreType string
	if err := db.QueryRow(`SELECT upload_rate_bps, download_rate_bps, raw_upload_total_bytes, raw_download_total_bytes, boot_id, last_realtime_report_at, last_restart_at, agent_version, current_core_type FROM agent_hosts WHERE host = ?`, "127.0.0.1").Scan(&uploadRate, &downloadRate, &rawUpload, &rawDownload, &bootID, &realtimeAt, &restartAt, &agentVersion, &currentCoreType); err != nil {
		t.Fatalf("query agent host defaults: %v", err)
	}
	if uploadRate != 0 || downloadRate != 0 || rawUpload != 0 || rawDownload != 0 || realtimeAt != 0 || restartAt != 0 || bootID != "" || agentVersion != "" || currentCoreType != "" {
		t.Fatalf("unexpected observability defaults: rates=(%d,%d) raw=(%d,%d) meta=(%q,%d,%d,%q,%q)", uploadRate, downloadRate, rawUpload, rawDownload, bootID, realtimeAt, restartAt, agentVersion, currentCoreType)
	}
}

func TestMigrationsAgentLifecycleAutomationSchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tables := map[string][]string{
		"agent_lifecycle_operations":  {"id", "agent_host_id", "operation_type", "status", "request_payload", "result_payload", "error_message", "claimed_by", "claimed_at", "started_at", "finished_at", "operator_id", "source", "created_at", "updated_at"},
		"agent_traffic_policies":      {"agent_host_id", "enabled", "limit_bytes", "limit_type", "threshold_percent", "threshold_action", "threshold_reached", "reset_mode", "reset_day", "interval_days", "anchor_at", "last_reset_at", "last_reset_cycle_key", "updated_at"},
		"agent_traffic_states":        {"agent_host_id", "boot_id", "last_raw_upload_bytes", "last_raw_download_bytes", "counter_seen", "cycle_upload_bytes", "cycle_download_bytes", "updated_at"},
		"subscription_sources":        {"id", "type", "name", "url", "content", "enabled", "last_sync_at", "last_sync_err", "created_at", "updated_at"},
		"subscription_filter_reasons": {"id", "source_type", "source_id", "server_id", "node_name", "reason", "detail", "created_at"},
	}
	for table, expectedColumns := range tables {
		columns := loadTableColumns(t, db, table)
		for _, column := range expectedColumns {
			if _, ok := columns[column]; !ok {
				t.Fatalf("expected %s.%s column", table, column)
			}
		}
	}
	for _, index := range []string{
		"idx_agent_lifecycle_operations_agent_status_created",
		"idx_agent_lifecycle_operations_agent_type_status_created",
		"idx_agent_lifecycle_operations_claimed_by_status",
		"idx_agent_lifecycle_operations_source_created",
		"idx_agent_traffic_policies_enabled_threshold",
		"idx_agent_traffic_policies_reset_mode",
		"idx_agent_traffic_policies_threshold_action",
		"idx_agent_traffic_states_boot_id",
		"idx_agent_traffic_states_updated_at",
		"idx_subscription_sources_type_enabled",
		"idx_subscription_sources_enabled",
		"idx_subscription_sources_name",
		"idx_subscription_filter_reasons_source",
		"idx_subscription_filter_reasons_server",
		"idx_subscription_filter_reasons_reason",
	} {
		assertIndexExists(t, db, index)
	}

	_, err := db.Exec(`INSERT INTO agent_hosts (name, host, token, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, "agent", "10.0.0.1", "token", 1, 1)
	if err != nil {
		t.Fatalf("insert lifecycle agent host: %v", err)
	}
	var agentHostID int64
	if err := db.QueryRow(`SELECT id FROM agent_hosts WHERE host = ?`, "10.0.0.1").Scan(&agentHostID); err != nil {
		t.Fatalf("query lifecycle agent host id: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO agent_lifecycle_operations (id, agent_host_id, operation_type, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, "op-1", agentHostID, "agent_update", "pending", 1, 1); err != nil {
		t.Fatalf("insert lifecycle operation defaults: %v", err)
	}
	var requestPayload, resultPayload, errorMessage, claimedBy, source string
	var claimedAt sql.NullInt64
	if err := db.QueryRow(`SELECT request_payload, result_payload, error_message, claimed_by, source, claimed_at FROM agent_lifecycle_operations WHERE id = ?`, "op-1").Scan(&requestPayload, &resultPayload, &errorMessage, &claimedBy, &source, &claimedAt); err != nil {
		t.Fatalf("query lifecycle operation defaults: %v", err)
	}
	if requestPayload != "{}" || resultPayload != "{}" || errorMessage != "" || claimedBy != "" || source != "" || claimedAt.Valid {
		t.Fatalf("unexpected lifecycle operation defaults: request=%q result=%q error=%q claimed_by=%q source=%q claimed_at=%v", requestPayload, resultPayload, errorMessage, claimedBy, source, claimedAt)
	}

	if _, err := db.Exec(`INSERT INTO agent_traffic_policies (agent_host_id) VALUES (?)`, agentHostID); err != nil {
		t.Fatalf("insert traffic policy defaults: %v", err)
	}
	var enabled, limitBytes, thresholdPercent, thresholdReached, resetDay, intervalDays, anchorAt, lastResetAt, updatedAt int64
	var limitType, thresholdAction, resetMode, lastResetCycleKey string
	if err := db.QueryRow(`SELECT enabled, limit_bytes, limit_type, threshold_percent, threshold_action, threshold_reached, reset_mode, reset_day, interval_days, anchor_at, last_reset_at, last_reset_cycle_key, updated_at FROM agent_traffic_policies WHERE agent_host_id = ?`, agentHostID).Scan(&enabled, &limitBytes, &limitType, &thresholdPercent, &thresholdAction, &thresholdReached, &resetMode, &resetDay, &intervalDays, &anchorAt, &lastResetAt, &lastResetCycleKey, &updatedAt); err != nil {
		t.Fatalf("query traffic policy defaults: %v", err)
	}
	if enabled != 0 || limitBytes != 0 || limitType != "sum" || thresholdPercent != 100 || thresholdAction != "notify_only" || thresholdReached != 0 || resetMode != "off" || resetDay != 1 || intervalDays != 0 || anchorAt != 0 || lastResetAt != 0 || lastResetCycleKey != "" || updatedAt != 0 {
		t.Fatalf("unexpected traffic policy defaults")
	}

	if _, err := db.Exec(`INSERT INTO agent_traffic_states (agent_host_id) VALUES (?)`, agentHostID); err != nil {
		t.Fatalf("insert traffic state defaults: %v", err)
	}
	var bootID string
	var lastRawUpload, lastRawDownload, counterSeen, cycleUpload, cycleDownload int64
	if err := db.QueryRow(`SELECT boot_id, last_raw_upload_bytes, last_raw_download_bytes, counter_seen, cycle_upload_bytes, cycle_download_bytes, updated_at FROM agent_traffic_states WHERE agent_host_id = ?`, agentHostID).Scan(&bootID, &lastRawUpload, &lastRawDownload, &counterSeen, &cycleUpload, &cycleDownload, &updatedAt); err != nil {
		t.Fatalf("query traffic state defaults: %v", err)
	}
	if bootID != "" || lastRawUpload != 0 || lastRawDownload != 0 || counterSeen != 0 || cycleUpload != 0 || cycleDownload != 0 || updatedAt != 0 {
		t.Fatalf("unexpected traffic state defaults")
	}

	if _, err := db.Exec(`INSERT INTO subscription_sources (type, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, "custom", "Manual", 1, 1); err != nil {
		t.Fatalf("insert subscription source defaults: %v", err)
	}
	var url, content, lastSyncErr string
	var lastSyncAt int64
	if err := db.QueryRow(`SELECT url, content, enabled, last_sync_at, last_sync_err FROM subscription_sources WHERE name = ?`, "Manual").Scan(&url, &content, &enabled, &lastSyncAt, &lastSyncErr); err != nil {
		t.Fatalf("query subscription source defaults: %v", err)
	}
	if url != "" || content != "" || enabled != 1 || lastSyncAt != 0 || lastSyncErr != "" {
		t.Fatalf("unexpected subscription source defaults")
	}

	if _, err := db.Exec(`INSERT INTO subscription_filter_reasons (source_type, reason, created_at) VALUES (?, ?, ?)`, "self_hosted", "threshold_reached", 1); err != nil {
		t.Fatalf("insert filter reason defaults: %v", err)
	}
	var sourceID, serverID int64
	var nodeName, detail string
	if err := db.QueryRow(`SELECT source_id, server_id, node_name, detail FROM subscription_filter_reasons WHERE reason = ?`, "threshold_reached").Scan(&sourceID, &serverID, &nodeName, &detail); err != nil {
		t.Fatalf("query filter reason defaults: %v", err)
	}
	if sourceID != 0 || serverID != 0 || nodeName != "" || detail != "" {
		t.Fatalf("unexpected filter reason defaults")
	}

	if err := migrations.Up(db); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}
}

func loadTableColumns(t *testing.T, db *sql.DB, table string) map[string]struct{} {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		t.Fatalf("load columns for %s: %v", table, err)
	}
	defer rows.Close()
	columns := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan column for %s: %v", table, err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate columns for %s: %v", table, err)
	}
	return columns
}

func assertIndexExists(t *testing.T, db *sql.DB, index string) {
	t.Helper()
	var name string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&name); err != nil {
		t.Fatalf("expected index %s: %v", index, err)
	}
}

func TestUserRepositoryCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := &userRepo{db: db}
	ctx := context.Background()

	// Create
	user := &repository.User{
		Email:     "test@example.com",
		Password:  "hashed_pw",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("expected valid ID")
	}

	// Read
	fetched, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to find user: %v", err)
	}
	if fetched.Email != "test@example.com" {
		t.Fatalf("expected email match")
	}

	// Update
	fetched.Status = 1
	if err := repo.Save(ctx, fetched); err != nil {
		t.Fatalf("failed to update user: %v", err)
	}
	updated, _ := repo.FindByID(ctx, created.ID)
	if updated.Status != 1 {
		t.Fatalf("expected status updated")
	}
}

func TestUserRepositoryFilter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := &userRepo{db: db}
	ctx := context.Background()

	_ = createTestUser(ctx, repo, "a@test.com", 1)
	_ = createTestUser(ctx, repo, "b@test.com", 2)
	_ = createTestUser(ctx, repo, "c@other.com", 1)

	// Filter by Status
	status := 1
	users, err := repo.Search(ctx, repository.UserSearchFilter{Status: &status})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users with status 1, got %d", len(users))
	}

	// Filter by Keyword
	users, err = repo.Search(ctx, repository.UserSearchFilter{Keyword: "test.com"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users matching query, got %d", len(users))
	}
}

func createTestUser(ctx context.Context, repo repository.UserRepository, email string, status int) *repository.User {
	user := &repository.User{
		Email:     email,
		Status:    status,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	created, _ := repo.Create(ctx, user)
	return created
}

func TestServerRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := &serverRepo{db: db}
	ctx := context.Background()

	// Manually insert server since repo is read-only
	_, err := db.Exec(`INSERT INTO servers (group_id, route_id, parent_id, tags, name, rate, host, port, server_port, cipher, obfs, obfs_settings, "show", sort, status, type, settings, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, 1, 0, "[]", "Node 1", "1.0", "1.1.1.1", 443, 443, "aes-256-gcm", "http", "{}", 1, 0, 1, "vless", "{}", time.Now().Unix(), time.Now().Unix())
	if err != nil {
		t.Fatalf("failed to insert server: %v", err)
	}

	// List
	servers, err := repo.FindAllVisible(ctx)
	if err != nil {
		t.Fatalf("failed to list servers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Name != "Node 1" {
		t.Fatalf("expected name Node 1, got %s", servers[0].Name)
	}

	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("failed to count servers: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 server count, got %d", count)
	}
}
