// 文件路径: internal/service/server_telemetry.go
// 模块说明: 这是 internal 模块里的 server_telemetry 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/creamcroissant/mgpanel/internal/async"
	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ServerTelemetryService tracks node heartbeats, online counts, and load metrics.
type ServerTelemetryService interface {
	TrackUserPull(ctx context.Context, server *repository.Server, userCount int) error
	RecordPush(ctx context.Context, server *repository.Server, samples []UniProxyPushSample) error
	RecordAlive(ctx context.Context, server *repository.Server, payload map[int64][]string) error
	AliveCounts(ctx context.Context, userIDs []int64) (map[int64]int, error)
	RecordStatus(ctx context.Context, server *repository.Server, status ServerStatusReport) error
	IsNodeOnline(ctx context.Context, server *repository.Server) bool
	RecordHeartbeat(ctx context.Context, server *repository.Server) error
}

// UniProxyPushSample is an alias to async.UniProxyPushSample for backward compatibility.
// The canonical definition is in internal/async to avoid import cycles.
type UniProxyPushSample = async.UniProxyPushSample

// ServerStatusReport describes node load metrics submitted via `/status`.
type ServerStatusReport struct {
	CPU             float64
	Mem             StatusCapacity
	Swap            StatusCapacity
	Disk            StatusCapacity
	TrafficUpload   int64 // Node-level traffic delta since last report (bytes)
	TrafficDownload int64 // Node-level traffic delta since last report (bytes)
	Taken           time.Time
}

// StatusCapacity captures total and used capacity for a hardware resource.
type StatusCapacity struct {
	Total int64
	Used  int64
}

type serverTelemetryService struct {
	cache             cache.Store
	settings          repository.SettingRepository
	servers           repository.ServerRepository
	statServers       repository.StatServerRepository
	logger            *slog.Logger
	deviceModeValue   atomic.Int64
	deviceModeExpires atomic.Int64
}

const (
	nodeCacheTTL    = time.Hour
	userAlivePrefix = "ALIVE_IP_USER"
	userAliveTTL    = 120 * time.Second
	userAliveExpiry = 100 * time.Second
	deviceModeTTL   = 60
)

// NewServerTelemetryService wires telemetry tracking backed by cache + settings.
func NewServerTelemetryService(cacheStore cache.Store, settings repository.SettingRepository, servers repository.ServerRepository, statServers repository.StatServerRepository) ServerTelemetryService {
	return NewServerTelemetryServiceWithLogger(cacheStore, settings, servers, statServers, nil)
}

func NewServerTelemetryServiceWithLogger(cacheStore cache.Store, settings repository.SettingRepository, servers repository.ServerRepository, statServers repository.StatServerRepository, logger *slog.Logger) ServerTelemetryService {
	if logger == nil {
		logger = slog.Default()
	}
	svc := &serverTelemetryService{cache: cacheStore, settings: settings, servers: servers, statServers: statServers, logger: logger}
	svc.deviceModeValue.Store(0)
	svc.deviceModeExpires.Store(0)
	return svc
}

func (s *serverTelemetryService) TrackUserPull(ctx context.Context, server *repository.Server, userCount int) error {
	if err := ensureServer(server); err != nil {
		return err
	}
	if s.cache == nil {
		return fmt.Errorf("server telemetry cache unavailable / 节点遥测缓存不可用")
	}
	if err := s.RecordHeartbeat(ctx, server); err != nil {
		s.logger.Warn("failed to record server heartbeat", "error", err, "server_id", server.ID)
	}
	key := serverCacheKey(server, "LAST_CHECK_AT")
	return s.cache.Set(ctx, key, time.Now().Unix(), nodeCacheTTL)
}

func (s *serverTelemetryService) RecordPush(ctx context.Context, server *repository.Server, samples []UniProxyPushSample) error {
	if err := ensureServer(server); err != nil {
		return err
	}
	if s.cache == nil {
		return fmt.Errorf("server telemetry cache unavailable / 节点遥测缓存不可用")
	}
	if err := s.RecordHeartbeat(ctx, server); err != nil {
		s.logger.Warn("failed to record server heartbeat", "error", err, "server_id", server.ID)
	}
	filtered := samples
	if len(filtered) == 0 {
		return nil
	}
	countKey := serverCacheKey(server, "ONLINE_USER")
	if err := s.cache.Set(ctx, countKey, len(filtered), nodeCacheTTL); err != nil {
		return err
	}
	pushKey := serverCacheKey(server, "LAST_PUSH_AT")
	return s.cache.Set(ctx, pushKey, time.Now().Unix(), nodeCacheTTL)
}

func (s *serverTelemetryService) RecordAlive(ctx context.Context, server *repository.Server, payload map[int64][]string) error {
	if err := ensureServer(server); err != nil {
		return err
	}
	if s.cache == nil {
		return fmt.Errorf("server telemetry cache unavailable / 节点遥测缓存不可用")
	}
	if err := s.RecordHeartbeat(ctx, server); err != nil {
		s.logger.Warn("failed to record server heartbeat", "error", err, "server_id", server.ID)
	}
	if len(payload) == 0 {
		return nil
	}
	now := time.Now().Unix()
	nodeKey := fmt.Sprintf("%s%d", strings.ToLower(strings.TrimSpace(server.Type)), server.ID)
	mode := s.deviceLimitMode(ctx)

	for userID, ips := range payload {
		if userID <= 0 {
			continue
		}
		cacheKey := fmt.Sprintf("%s_%d", userAlivePrefix, userID)
		snapshot := aliveCache{Nodes: map[string]aliveNode{}}
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &snapshot); err != nil {
			return err
		} else if !ok || snapshot.Nodes == nil {
			snapshot.Nodes = make(map[string]aliveNode)
		}
		snapshot.prune(now)
		snapshot.Nodes[nodeKey] = aliveNode{
			AliveIPs: uniqueStrings(ips),
			Updated:  now,
		}
		snapshot.AliveIP = snapshot.count(mode)
		if err := s.cache.SetJSON(ctx, cacheKey, snapshot, userAliveTTL); err != nil {
			return err
		}
	}
	return nil
}

func (s *serverTelemetryService) AliveCounts(ctx context.Context, userIDs []int64) (map[int64]int, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("server telemetry cache unavailable / 节点遥测缓存不可用")
	}
	unique := uniqueInt64(userIDs)
	result := make(map[int64]int, len(unique))
	for _, id := range unique {
		cacheKey := fmt.Sprintf("%s_%d", userAlivePrefix, id)
		snapshot := aliveCache{}
		ok, err := s.cache.GetJSON(ctx, cacheKey, &snapshot)
		if err != nil {
			return nil, err
		}
		if ok {
			result[id] = snapshot.AliveIP
		}
	}
	return result, nil
}

func (s *serverTelemetryService) RecordStatus(ctx context.Context, server *repository.Server, status ServerStatusReport) error {
	if err := ensureServer(server); err != nil {
		return err
	}
	if s.cache == nil {
		return fmt.Errorf("server telemetry cache unavailable / 节点遥测缓存不可用")
	}
	now := time.Now().Unix()
	payload := map[string]any{
		"cpu": status.CPU,
		"mem": map[string]int64{
			"total": status.Mem.Total,
			"used":  status.Mem.Used,
		},
		"swap": map[string]int64{
			"total": status.Swap.Total,
			"used":  status.Swap.Used,
		},
		"disk": map[string]int64{
			"total": status.Disk.Total,
			"used":  status.Disk.Used,
		},
		"updated_at": now,
	}
	cacheTTL := s.statusCacheTTL(ctx)
	statusKey := serverCacheKey(server, "LOAD_STATUS")
	if err := s.cache.SetJSON(ctx, statusKey, payload, cacheTTL); err != nil {
		return err
	}
	lastKey := serverCacheKey(server, "LAST_LOAD_AT")
	if err := s.cache.Set(ctx, lastKey, now, cacheTTL); err != nil {
		return err
	}

	// Persist traffic and resource stats to stat_servers (hourly + daily)
	if s.statServers != nil && (status.TrafficUpload > 0 || status.TrafficDownload > 0) {
		taken := status.Taken
		if taken.IsZero() {
			taken = time.Now()
		}

		// Calculate record_at timestamps for hourly and daily aggregation
		hourlyAt := taken.Truncate(time.Hour).Unix()
		dailyAt := time.Date(taken.Year(), taken.Month(), taken.Day(), 0, 0, 0, 0, taken.Location()).Unix()

		baseRecord := repository.StatServerRecord{
			ServerID:    server.ID,
			Upload:      status.TrafficUpload,
			Download:    status.TrafficDownload,
			CPUAvg:      status.CPU,
			MemUsed:     status.Mem.Used,
			MemTotal:    status.Mem.Total,
			DiskUsed:    status.Disk.Used,
			DiskTotal:   status.Disk.Total,
			OnlineUsers: 0, // Can be populated from cache if needed
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		// Upsert hourly record (RecordType = 0)
		hourlyRecord := baseRecord
		hourlyRecord.RecordAt = hourlyAt
		hourlyRecord.RecordType = 0
		if err := s.statServers.Upsert(ctx, hourlyRecord); err != nil {
			return fmt.Errorf("upsert hourly stat: %w", err)
		}

		// Upsert daily record (RecordType = 1)
		dailyRecord := baseRecord
		dailyRecord.RecordAt = dailyAt
		dailyRecord.RecordType = 1
		if err := s.statServers.Upsert(ctx, dailyRecord); err != nil {
			return fmt.Errorf("upsert daily stat: %w", err)
		}
	}

	return nil
}

func (s *serverTelemetryService) IsNodeOnline(ctx context.Context, server *repository.Server) bool {
	if server == nil {
		return false
	}
	now := time.Now().Unix()
	threshold := int64(300) // 5 minutes

	// Agent 心跳是 self-hosted 节点最可靠的在线信号（agent 持续上报）。
	if server.LastHeartbeatAt > 0 && now-server.LastHeartbeatAt < threshold {
		return true
	}

	if s.cache == nil {
		return server.Status == 1
	}

	pushKey := serverCacheKey(server, "LAST_PUSH_AT")
	if lastPushVal, ok := s.cache.Get(ctx, pushKey); ok {
		if lastPush, ok := toInt64(lastPushVal); ok {
			if now-lastPush < threshold {
				return true
			}
		}
	}
	checkKey := serverCacheKey(server, "LAST_CHECK_AT")
	if lastCheckVal, ok := s.cache.Get(ctx, checkKey); ok {
		if lastCheck, ok := toInt64(lastCheckVal); ok {
			if now-lastCheck < threshold {
				return true
			}
		}
	}
	// Fallback: if no cache data, check database status field
	// This allows nodes manually marked as online to be shown
	return server.Status == 1
}

func (s *serverTelemetryService) RecordHeartbeat(ctx context.Context, server *repository.Server) error {
	if s.servers == nil {
		return fmt.Errorf("server repository unavailable / 节点仓库不可用")
	}
	server.LastHeartbeatAt = time.Now().Unix()
	return s.servers.Update(ctx, server)
}

func (s *serverTelemetryService) deviceLimitMode(ctx context.Context) int {
	now := time.Now().Unix()
	if now < s.deviceModeExpires.Load() {
		return int(s.deviceModeValue.Load())
	}
	mode := s.lookupDeviceLimitMode(ctx)
	s.deviceModeValue.Store(int64(mode))
	s.deviceModeExpires.Store(now + deviceModeTTL)
	return mode
}

func (s *serverTelemetryService) lookupDeviceLimitMode(ctx context.Context) int {
	if s.settings == nil {
		return 0
	}
	setting, err := s.settings.Get(ctx, "device_limit_mode")
	if err != nil || setting == nil {
		return 0
	}
	if value, err := strconv.Atoi(strings.TrimSpace(setting.Value)); err == nil {
		return value
	}
	return 0
}

func (s *serverTelemetryService) statusCacheTTL(ctx context.Context) time.Duration {
	base := s.intervalSetting(ctx, "server_push_interval", 60)
	ttl := base * 3
	if ttl < 300 {
		ttl = 300
	}
	return time.Duration(ttl) * time.Second
}

func (s *serverTelemetryService) intervalSetting(ctx context.Context, key string, fallback int) int {
	if s.settings == nil {
		return fallback
	}
	setting, err := s.settings.Get(ctx, key)
	if err != nil || setting == nil {
		return fallback
	}
	trimmed := strings.TrimSpace(setting.Value)
	if trimmed == "" {
		return fallback
	}
	if value, err := strconv.Atoi(trimmed); err == nil {
		return value
	}
	return fallback
}

func ensureServer(server *repository.Server) error {
	if server == nil {
		return ErrNotFound
	}
	if server.ID <= 0 {
		return fmt.Errorf("invalid server reference / 无效的节点引用")
	}
	return nil
}

func serverCacheKey(server *repository.Server, suffix string) string {
	prefix := strings.ToUpper(strings.TrimSpace(server.Type))
	if prefix == "" {
		prefix = "UNKNOWN"
	}
	return fmt.Sprintf("SERVER_%s_%s_%d", prefix, suffix, server.ID)
}

type aliveCache struct {
	AliveIP int                  `json:"alive_ip"`
	Nodes   map[string]aliveNode `json:"nodes"`
}

type aliveNode struct {
	AliveIPs []string `json:"aliveips"`
	Updated  int64    `json:"lastupdateAt"`
}

func (a *aliveCache) prune(now int64) {
	if a.Nodes == nil {
		a.Nodes = make(map[string]aliveNode)
		return
	}
	for key, entry := range a.Nodes {
		if now-entry.Updated > int64(userAliveExpiry.Seconds()) {
			delete(a.Nodes, key)
		}
	}
}

func (a *aliveCache) count(mode int) int {
	if a.Nodes == nil {
		return 0
	}
	switch mode {
	case 1:
		unique := make(map[string]struct{})
		for _, entry := range a.Nodes {
			for _, val := range entry.AliveIPs {
				ip := strings.SplitN(val, "_", 2)[0]
				if ip == "" {
					continue
				}
				unique[ip] = struct{}{}
			}
		}
		return len(unique)
	default:
		total := 0
		for _, entry := range a.Nodes {
			total += len(entry.AliveIPs)
		}
		return total
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func uniqueInt64(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// MarshalJSON ensures backward compatibility with PHP cache layout.
func (a aliveCache) MarshalJSON() ([]byte, error) {
	raw := make(map[string]any, len(a.Nodes)+1)
	raw["alive_ip"] = a.AliveIP
	for key, entry := range a.Nodes {
		raw[key] = map[string]any{
			"aliveips":     entry.AliveIPs,
			"lastupdateAt": entry.Updated,
		}
	}
	return json.Marshal(raw)
}

// UnmarshalJSON decodes mixed map payloads created by Laravel jobs.
func (a *aliveCache) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	a.Nodes = make(map[string]aliveNode)
	for key, value := range raw {
		if key == "alive_ip" {
			var count int
			if err := json.Unmarshal(value, &count); err == nil {
				a.AliveIP = count
			}
			continue
		}
		var node struct {
			AliveIPs []string `json:"aliveips"`
			Updated  int64    `json:"lastupdateAt"`
		}
		if err := json.Unmarshal(value, &node); err != nil {
			continue
		}
		a.Nodes[key] = aliveNode{AliveIPs: node.AliveIPs, Updated: node.Updated}
	}
	return nil
}

