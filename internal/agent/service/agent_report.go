package service

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/creamcroissant/mgpanel/internal/agent/api"
	"github.com/creamcroissant/mgpanel/internal/agent/capability"
	"github.com/creamcroissant/mgpanel/internal/agent/core"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol"
	"github.com/creamcroissant/mgpanel/internal/agent/transport"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

func (a *Agent) report(ctx context.Context) {
	// Collect I/O data outside lock
	var trafficUpload, trafficDownload uint64
	if a.netio != nil {
		delta, err := a.netio.CollectDelta(ctx)
		if err != nil {
			slog.Error("Failed to collect netio traffic", "error", err)
		} else {
			trafficUpload = delta.Upload
			trafficDownload = delta.Download
		}
	}

	// System Status (I/O outside lock)
	stat, err := a.monitor.Collect()
	if err != nil {
		slog.Error("Failed to collect system stats", "error", err)
		return
	}

	// Inject traffic into status payload
	stat.TrafficUpload = trafficUpload
	stat.TrafficDownload = trafficDownload

	// Collect capabilities (may execute external binary, I/O outside lock)
	caps := a.getCapabilities(ctx)

	a.reportMu.Lock()
	a.rollbackExpiredUpdateIfNeeded()
	a.reportMu.Unlock()

	if a.conn != nil {
		state := a.conn.CheckConnection(ctx)
		if state != transport.StateConnected {
			slog.Warn("gRPC connection not ready, skip report", "state", state.String())
			return
		}
	}
	a.reportGRPC(ctx, stat, caps)

	// User-level Traffic (from traffic collector, e.g., xray_api)
	a.reportUserTraffic(ctx)
}

func (a *Agent) reportGRPC(ctx context.Context, stat api.StatusPayload, caps *capability.DetectedCapabilities) {
	// Collect CDN origin latency probe results
	a.cdnProbeResultsMu.Lock()
	originLatencies := make([]*agentv1.OriginLatencyEntry, len(a.cdnProbeResults))
	copy(originLatencies, a.cdnProbeResults)
	a.cdnProbeResults = a.cdnProbeResults[:0] // Clear slice after copying
	a.cdnProbeResultsMu.Unlock()

	// Build protobuf status report
	statusReport := &agentv1.StatusReport{
		Timestamp: time.Now().Unix(),
		System: &agentv1.SystemMetrics{
			CpuUsage:        stat.CPU,
			MemoryUsage:     float64(stat.Mem.Used) / float64(stat.Mem.Total) * 100,
			MemoryTotal:     float64(stat.Mem.Total),
			MemoryUsed:      float64(stat.Mem.Used),
			DiskUsage:       float64(stat.Disk.Used) / float64(stat.Disk.Total) * 100,
			DiskTotal:       float64(stat.Disk.Total),
			DiskUsed:        float64(stat.Disk.Used),
			UptimeSeconds:   int64(stat.Uptime),
			ConnectionCount: 0, // TODO: Get connection count if available
			Load1:           stat.Load1,
			Load5:           stat.Load5,
			Load15:          stat.Load15,
			ProcessCount:    int32(stat.ProcessCount),
			TcpCount:        int32(stat.TcpCount),
			UdpCount:        int32(stat.UdpCount),
			// Core capabilities
			CoreVersion:     caps.CoreVersion,
			Capabilities:    caps.Capabilities,
			BuildTags:       caps.BuildTags,
			AgentVersion:    a.cfg.Update.CurrentVersion,
			CurrentCoreType: caps.CoreType,
			BootId:          readBootID(),
			AllCores:        buildAllCoresProto(caps.AllCores),
		},
		Network: &agentv1.NetworkMetrics{
			UploadBytes:           stat.NetIO.Up,
			DownloadBytes:         stat.NetIO.Down,
			UploadDelta:           stat.TrafficUpload,
			DownloadDelta:         stat.TrafficDownload,
			RawUploadTotalBytes:   a.collectRawUpload(ctx),
			RawDownloadTotalBytes: a.collectRawDownload(ctx),
			RawCountersPresent:    a.netio != nil,
		},
		CommandQueue:      a.commandQueueStatsProto(),
		UpdateStatus:      a.updateStatusProto(),
		OriginLatencies:   originLatencies,
		MeshPeerLatencies: a.meshPeerLatenciesProto(),
		// 运行配置随每次上报携带（~KB级）：panel落库后前端可看、push_config成功判据靠它。
		// 重读失败仅告警不阻断上报（上次缓存值仍有效）。
		AgentConfigYaml: a.currentConfigYAML(),
	}

	// Add core instances
	if a.coreMgr != nil {
		statusReport.Instances = buildCoreInstanceReport(a.coreMgr.ListInstances())
	}

	if a.inventoryScanner != nil {
		inventory, inboundIndex, scanErr := a.inventoryScanner.Scan()
		if scanErr != nil {
			slog.Error("Failed to scan config inventory", "error", scanErr)
		} else {
			statusReport.Inventory = inventory
			statusReport.InboundIndex = inboundIndex
		}
	}

	if configsWithDetails, err := a.protoMgr.ListConfigsWithDetailsBySource("managed"); err == nil {
		// Check global service status
		running, _ := a.protoMgr.ServiceStatus(ctx)

		protocols := make([]*agentv1.ProtocolState, 0, len(configsWithDetails))
		for _, cfg := range configsWithDetails {
			state := &agentv1.ProtocolState{
				Name:        cfg.Filename,
				Type:        "sing-box", // Default, will be overridden if detected
				Running:     running,
				ContentHash: cfg.ContentHash,
			}

			// Convert parsed details to protobuf
			if len(cfg.Protocols) > 0 {
				state.Type = cfg.Protocols[0].CoreType // Use detected core type
				details := make([]*agentv1.ProtocolDetails, 0, len(cfg.Protocols))
				for _, p := range cfg.Protocols {
					detail := &agentv1.ProtocolDetails{
						Protocol:   p.Protocol,
						Tag:        p.Tag,
						Listen:     p.Listen,
						Port:       int32(p.Port),
						SourceFile: p.SourceFile,
						CoreType:   p.CoreType,
					}

					// Transport config
					if p.Transport != nil {
						detail.Transport = &agentv1.TransportConfig{
							Type:        p.Transport.Type,
							Path:        p.Transport.Path,
							Host:        p.Transport.Host,
							ServiceName: p.Transport.ServiceName,
						}
					}

					// TLS config
					if p.TLS != nil {
						detail.Tls = &agentv1.TLSConfig{
							Enabled:    p.TLS.Enabled,
							ServerName: p.TLS.ServerName,
							Alpn:       p.TLS.ALPN,
						}
						if p.TLS.Reality != nil {
							detail.Tls.Reality = &agentv1.RealityConfig{
								Enabled:       p.TLS.Reality.Enabled,
								ShortIds:      p.TLS.Reality.ShortIDs,
								ServerName:    p.TLS.Reality.ServerName,
								Fingerprint:   p.TLS.Reality.Fingerprint,
								HandshakeAddr: p.TLS.Reality.HandshakeAddr,
								HandshakePort: int32(p.TLS.Reality.HandshakePort),
								PublicKey:     p.TLS.Reality.PublicKey,
							}
						}
					}

					// Multiplex config
					if p.Multiplex != nil {
						detail.Multiplex = &agentv1.MultiplexConfig{
							Enabled: p.Multiplex.Enabled,
							Padding: p.Multiplex.Padding,
						}
						if p.Multiplex.Brutal != nil {
							detail.Multiplex.Brutal = &agentv1.BrutalConfig{
								Enabled:  p.Multiplex.Brutal.Enabled,
								UpMbps:   int32(p.Multiplex.Brutal.UpMbps),
								DownMbps: int32(p.Multiplex.Brutal.DownMbps),
							}
						}
					}

					// Users
					for _, u := range p.Users {
						detail.Users = append(detail.Users, &agentv1.ProtocolUserInfo{
							Uuid:   u.UUID,
							Flow:   u.Flow,
							Email:  u.Email,
							Method: u.Method,
						})
					}

					details = append(details, detail)
				}
				state.Details = details
			}

			protocols = append(protocols, state)
		}
		statusReport.Protocols = protocols
	} else {
		slog.Error("Failed to list protocol configs", "error", err)
	}

	// Parse subscribe directory for client configs
	if subData, err := a.subParse.Parse(); err == nil && len(subData.Configs) > 0 {
		clientConfigs := make([]*agentv1.ClientConfig, 0, len(subData.Configs))
		for _, cfg := range subData.Configs {
			clientConfig := &agentv1.ClientConfig{
				Name:     cfg.Name,
				Protocol: cfg.Protocol,
				Server:   cfg.Server,
				Port:     int32(cfg.Port),
				// Authentication
				Uuid:     cfg.UUID,
				Password: cfg.Password,
				// Transport
				Network:     cfg.Network,
				Path:        cfg.Path,
				ServiceName: cfg.ServiceName,
				// TLS
				Tls:         cfg.TLS,
				Sni:         cfg.SNI,
				Alpn:        cfg.ALPN,
				Fingerprint: cfg.Fingerprint,
				Insecure:    cfg.Insecure,
				// Reality
				RealityEnabled:   cfg.RealityEnabled,
				RealityPublicKey: cfg.RealityPublicKey,
				RealityShortId:   cfg.RealityShortID,
				// VLESS
				Flow: cfg.Flow,
				// Hysteria2
				HopPorts:    cfg.HopPorts,
				HopInterval: int32(cfg.HopInterval),
				UpMbps:      int32(cfg.UpMbps),
				DownMbps:    int32(cfg.DownMbps),
				// TUIC
				CongestionControl: cfg.CongestionControl,
				// Shadowsocks
				Cipher: cfg.Cipher,
				// ShadowTLS
				ShadowtlsVersion:  int32(cfg.ShadowTLSVersion),
				ShadowtlsPassword: cfg.ShadowTLSPassword,
				// Multiplex
				MuxEnabled: cfg.MuxEnabled,
				MuxPadding: cfg.MuxPadding,
				// Raw configs
				RawConfigs: cfg.RawConfigs,
			}
			clientConfigs = append(clientConfigs, clientConfig)
		}

		statusReport.ClientConfigs = &agentv1.ClientConfigReport{
			Configs:     clientConfigs,
			ContentHash: subData.ContentHash,
		}
		slog.Debug("Parsed subscribe configs", "count", len(clientConfigs))
	} else if err != nil {
		slog.Warn("Failed to parse subscribe directory", "error", err)
	}

	if resp, err := a.grpc.ReportStatus(ctx, statusReport); err != nil {
		slog.Error("Failed to report status via gRPC", "error", err)
	} else {
		if resp == nil || !resp.GetSuccess() {
			message := "empty status response"
			if resp != nil {
				message = resp.GetMessage()
			}
			slog.Warn("Panel rejected status report", "message", message)
			return
		}
		a.confirmUpdaterHealthy()
		slog.Debug("Reported status via gRPC",
			"traffic_up", stat.TrafficUpload,
			"traffic_down", stat.TrafficDownload,
			"inventory_count", len(statusReport.Inventory),
			"inbound_index_count", len(statusReport.InboundIndex))

		// Check for interval updates
		updated := false
		if resp.SyncIntervalSeconds > 0 {
			if previous := a.currentSyncInterval.Swap(resp.SyncIntervalSeconds); previous != resp.SyncIntervalSeconds {
				updated = true
			}
		}
		if resp.ReportIntervalSeconds > 0 {
			if previous := a.currentReportInterval.Swap(resp.ReportIntervalSeconds); previous != resp.ReportIntervalSeconds {
				updated = true
			}
		}

		if updated {
			select {
			case a.updateTickerCh <- struct{}{}:
			default:
			}
		}
	}
}

func (a *Agent) reportUserTraffic(ctx context.Context) {
	// Use gRPC for traffic reporting
	samples, err := a.traffic.Collect(ctx)
	if err != nil {
		slog.Error("Failed to collect traffic", "error", err)
		return
	}

	if len(samples) == 0 {
		return
	}

	// Convert to protobuf format
	userTraffic := make([]*agentv1.UserTraffic, 0, len(samples))
	unmapped := 0
	for _, s := range samples {
		userID := s.UserID
		if userID <= 0 {
			if mappedID, ok := a.resolveUserIDByUID(s.UID); ok {
				userID = mappedID
			}
		}
		if userID <= 0 {
			unmapped++
			continue
		}
		if s.Upload == 0 && s.Download == 0 {
			continue
		}

		userTraffic = append(userTraffic, &agentv1.UserTraffic{
			UserId:        userID,
			UploadBytes:   s.Upload,
			DownloadBytes: s.Download,
		})
	}

	if len(userTraffic) == 0 {
		if unmapped > 0 {
			slog.Warn("Skip traffic samples due to unresolved user mapping", "unmapped", unmapped, "samples", len(samples))
		}
		return
	}

	reportID := strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))
	if _, err := a.grpc.ReportTraffic(ctx, userTraffic, reportID); err != nil {
		slog.Error("Failed to push traffic via gRPC", "error", err, "report_id", reportID)
	} else {
		slog.Debug("Pushed traffic samples via gRPC", "count", len(userTraffic), "source_samples", len(samples), "unmapped", unmapped, "report_id", reportID)
	}
}

func normalizeUserEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	return strings.ToLower(email)
}

func (a *Agent) refreshUserEmailMapping(users []*agentv1.UserInfo) {
	mapped := make(map[string]int64, len(users))
	for _, u := range users {
		if u == nil || u.UserId <= 0 {
			continue
		}
		email := normalizeUserEmail(u.Email)
		if email == "" {
			continue
		}
		mapped[email] = u.UserId
	}

	a.userEmailMu.Lock()
	a.userIDByEmail = mapped
	a.userEmailMu.Unlock()
}

func (a *Agent) resolveUserIDByUID(uid string) (int64, bool) {
	email := normalizeUserEmail(uid)
	if email == "" {
		return 0, false
	}

	a.userEmailMu.RLock()
	userID, ok := a.userIDByEmail[email]
	a.userEmailMu.RUnlock()
	if !ok || userID <= 0 {
		return 0, false
	}
	return userID, true
}

func buildCoreInstanceReport(instances []*core.CoreInstance) []*agentv1.CoreInstance {
	if len(instances) == 0 {
		return nil
	}

	pbInstances := make([]*agentv1.CoreInstance, 0, len(instances))
	for _, inst := range instances {
		pbInstances = append(pbInstances, &agentv1.CoreInstance{
			Id:       inst.ID,
			CoreType: string(inst.CoreType),
			Status:   string(inst.Status),
			ListenPorts: func() []int32 {
				if len(inst.ListenPorts) == 0 {
					return nil
				}
				ports := make([]int32, len(inst.ListenPorts))
				for i, port := range inst.ListenPorts {
					ports[i] = int32(port)
				}
				return ports
			}(),
			ConfigPath: inst.ConfigPath,
			ConfigHash: inst.ConfigHash,
			Pid:        int32(inst.PID),
			StartedAt:  inst.StartedAt,
			Error:      inst.Error,
		})
	}
	return pbInstances
}

// determineListenPort returns the appropriate Xray listen port based on CDN status.
// When CDN is enabled, Xray should listen on internal port 10000 (behind Caddy).
// When CDN is disabled, Xray should listen on external port 443 directly.
func (a *Agent) determineListenPort() []int {
	if a.cfg.CDN.Enabled {
		return []int{10000}
	}
	return []int{443}
}

// getCapabilities returns cached or fresh capabilities
// Capabilities are cached for 1 hour to avoid excessive command executions
func (a *Agent) getCapabilities(ctx context.Context) *capability.DetectedCapabilities {
	a.capsMu.Lock()
	defer a.capsMu.Unlock()

	now := time.Now().Unix()
	cacheExpiry := int64(3600) // 1 hour

	// Return cached if still valid
	if a.cachedCaps != nil && now-a.capsDetectedAt < cacheExpiry {
		return a.cachedCaps
	}

	// Detect fresh capabilities
	caps := a.capDet.Detect(ctx)

	// Cache the result
	a.cachedCaps = caps
	a.capsDetectedAt = now

	slog.Info("Detected core capabilities",
		"core_type", caps.CoreType,
		"version", caps.CoreVersion,
		"capabilities", caps.Capabilities,
		"build_tags", caps.BuildTags)

	return caps
}

// applyUsers converts gRPC UserInfo to protocol.UserConfig and injects them into the config.
func (a *Agent) applyUsers(ctx context.Context, users []*agentv1.UserInfo) error {
	if len(users) == 0 {
		return nil
	}

	// Convert gRPC UserInfo to protocol.UserConfig
	userConfigs := make([]protocol.UserConfig, 0, len(users))
	for _, u := range users {
		userConfigs = append(userConfigs, protocol.UserConfig{
			UUID:    u.Uuid,
			Email:   u.Email,
			Enabled: u.Enabled,
		})
	}

	// Detect core type and use appropriate injection method
	coreType := a.protoMgr.DetectCoreType()
	slog.Debug("Detected core type for user injection", "core_type", coreType)

	switch coreType {
	case "xray":
		return a.protoMgr.InjectUsersXray(ctx, userConfigs)
	case "sing-box":
		return a.protoMgr.InjectUsers(ctx, userConfigs)
	default:
		// Try Sing-box first as it's the default
		if err := a.protoMgr.InjectUsers(ctx, userConfigs); err != nil {
			// If that fails, try Xray
			return a.protoMgr.InjectUsersXray(ctx, userConfigs)
		}
		return nil
	}
}

func readBootID() string {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (a *Agent) collectRawUpload(ctx context.Context) *agentv1.MetricUInt64Value {
	if a.netio == nil {
		return nil
	}
	sent, _, err := a.netio.CollectCumulative(ctx)
	if err != nil {
		return nil
	}
	return &agentv1.MetricUInt64Value{Value: sent}
}

func (a *Agent) collectRawDownload(ctx context.Context) *agentv1.MetricUInt64Value {
	if a.netio == nil {
		return nil
	}
	_, recv, err := a.netio.CollectCumulative(ctx)
	if err != nil {
		return nil
	}
	return &agentv1.MetricUInt64Value{Value: recv}
}

// buildAllCoresProto converts capability.DetectedCoreInfo to agentv1.CoreInfo for gRPC status reporting.
func buildAllCoresProto(cores []capability.DetectedCoreInfo) []*agentv1.CoreInfo {
	if len(cores) == 0 {
		return nil
	}
	out := make([]*agentv1.CoreInfo, 0, len(cores))
	for _, c := range cores {
		out = append(out, &agentv1.CoreInfo{
			Type:         c.Type,
			Version:      c.Version,
			Installed:    c.Version != "",
			Capabilities: c.Capabilities,
		})
	}
	return out
}
