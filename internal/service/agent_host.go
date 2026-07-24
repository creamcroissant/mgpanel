package service

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/cache"
	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/template"
)

// AgentHostService manages agent host operations.
type AgentHostService interface {
	// CRUD operations
	Create(ctx context.Context, req CreateAgentHostRequest) (*repository.AgentHost, error)
	RegisterByCommunicationKey(ctx context.Context, req RegisterAgentHostRequest) (*repository.AgentHost, error)
	GetByID(ctx context.Context, id int64) (*repository.AgentHost, error)
	GetByToken(ctx context.Context, token string) (*repository.AgentHost, error)
	Update(ctx context.Context, id int64, req UpdateAgentHostRequest) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]*repository.AgentHost, error)

	// Status updates from agent
	UpdateMetrics(ctx context.Context, token string, metrics AgentHostMetricsReport) error
	UpdateHeartbeat(ctx context.Context, token string) error
	UpdateProtocols(ctx context.Context, token string, protocols []ProtocolInfo) error
	UpdateClientConfigs(ctx context.Context, token string, configs []ClientConfigInfo) error
	UpdateCapabilities(ctx context.Context, token string, coreVersion string, capabilities, buildTags []string) error

	// Template management
	AssignTemplate(ctx context.Context, agentID, templateID int64) error
	CheckTemplateCompatibility(ctx context.Context, agentID, templateID int64) (*TemplateCompatibilityResult, error)

	GenerateConfig(ctx context.Context, agentID int64) ([]byte, error)
	FlushMetrics(ctx context.Context) error

	// GetConfigYAML returns the running configuration YAML for an agent host.
	GetConfigYAML(ctx context.Context, id int64) (string, error)
}

// TemplateCompatibilityResult contains the result of a template compatibility check.
type TemplateCompatibilityResult struct {
	Compatible bool     `json:"compatible"`
	Warnings   []string `json:"warnings,omitempty"`
	Errors     []string `json:"errors,omitempty"`
}

// ProtocolInfo represents a protocol reported by the agent
type ProtocolInfo struct {
	Name    string
	Type    string
	Running bool
	Details []ProtocolDetails // Parsed protocol details
}

// ProtocolDetails contains detailed protocol configuration
type ProtocolDetails struct {
	Protocol  string         `json:"protocol"`
	Tag       string         `json:"tag"`
	Listen    string         `json:"listen"`
	Port      int            `json:"port"`
	Transport *TransportInfo `json:"transport,omitempty"`
	TLS       *TLSInfo       `json:"tls,omitempty"`
	Multiplex *MultiplexInfo `json:"multiplex,omitempty"`
	Users     []UserInfoData `json:"users,omitempty"`
	CoreType  string         `json:"core_type"`
}

// TransportInfo describes transport layer settings
type TransportInfo struct {
	Type        string `json:"type"`
	Path        string `json:"path,omitempty"`
	Host        string `json:"host,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

// TLSInfo describes TLS settings
type TLSInfo struct {
	Enabled    bool         `json:"enabled"`
	ServerName string       `json:"server_name,omitempty"`
	ALPN       []string     `json:"alpn,omitempty"`
	Reality    *RealityInfo `json:"reality,omitempty"`
}

// RealityInfo describes XTLS Reality settings
type RealityInfo struct {
	Enabled       bool     `json:"enabled"`
	ShortIDs      []string `json:"short_ids,omitempty"`
	ServerName    string   `json:"server_name,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	HandshakeAddr string   `json:"handshake_addr,omitempty"`
	HandshakePort int      `json:"handshake_port,omitempty"`
	PublicKey     string   `json:"public_key,omitempty"`
}

// MultiplexInfo describes multiplex (mux) settings
type MultiplexInfo struct {
	Enabled bool        `json:"enabled"`
	Padding bool        `json:"padding,omitempty"`
	Brutal  *BrutalInfo `json:"brutal,omitempty"`
}

// BrutalInfo describes BBR Brutal settings
type BrutalInfo struct {
	Enabled  bool `json:"enabled"`
	UpMbps   int  `json:"up_mbps,omitempty"`
	DownMbps int  `json:"down_mbps,omitempty"`
}

// UserInfoData describes a user configuration in protocol
type UserInfoData struct {
	UUID   string `json:"uuid,omitempty"`
	Flow   string `json:"flow,omitempty"`
	Email  string `json:"email,omitempty"`
	Method string `json:"method,omitempty"`
}

// CreateAgentHostRequest contains data for creating a new agent host.
type CreateAgentHostRequest struct {
	Name string
	Host string
}

// RegisterAgentHostRequest contains data for zero-touch agent registration.
type RegisterAgentHostRequest struct {
	CommunicationKey string
	Hostname         string
	AdvertiseHost    string
}

// UpdateAgentHostRequest contains data for updating an agent host.
type UpdateAgentHostRequest struct {
	Name *string
	Host *string
}

// AgentHostMetricsReport contains metrics reported by an agent.
type AgentHostMetricsReport struct {
	CPUTotal              float64
	CPUUsed               float64
	MemTotal              int64
	MemUsed               int64
	DiskTotal             int64
	DiskUsed              int64
	UploadTotal           int64
	DownloadTotal         int64
	UploadRateBps         *int64
	DownloadRateBps       *int64
	RawUploadTotalBytes   *int64
	RawDownloadTotalBytes *int64
	BootID                string
	ReportedAt            int64
	AgentVersion          string
	CurrentCoreType       string
}

// ClientConfigInfo represents a client configuration reported by the agent.
type ClientConfigInfo struct {
	Name       string            // Protocol name/tag
	Protocol   string            // vless, hysteria2, etc.
	Port       int               // Main port
	RawConfigs map[string]string // format -> content
}

type AgentHostServiceOptions struct {
	Cache  cache.Store
	Logger *slog.Logger
}

type agentHostService struct {
	agentHosts          repository.AgentHostRepository
	servers             repository.ServerRepository
	serverClientConfigs repository.ServerClientConfigRepository
	configTemplates     repository.ConfigTemplateRepository
	users               repository.UserRepository
	settings            repository.SettingRepository
	metricsBuffer       *agentHostMetricsBuffer
}

func NewAgentHostServiceWithOptions(
	agentHosts repository.AgentHostRepository,
	servers repository.ServerRepository,
	serverClientConfigs repository.ServerClientConfigRepository,
	configTemplates repository.ConfigTemplateRepository,
	users repository.UserRepository,
	settings repository.SettingRepository,
	opts AgentHostServiceOptions,
) AgentHostService {
	return &agentHostService{
		agentHosts:          agentHosts,
		servers:             servers,
		serverClientConfigs: serverClientConfigs,
		configTemplates:     configTemplates,
		users:               users,
		settings:            settings,
		metricsBuffer:       newAgentHostMetricsBuffer(opts.Cache, agentHosts, opts.Logger),
	}
}

// NewAgentHostService creates a new agent host service.
func NewAgentHostService(
	agentHosts repository.AgentHostRepository,
	servers repository.ServerRepository,
	serverClientConfigs repository.ServerClientConfigRepository,
	configTemplates repository.ConfigTemplateRepository,
	users repository.UserRepository,
	settings repository.SettingRepository,
) AgentHostService {
	return NewAgentHostServiceWithOptions(agentHosts, servers, serverClientConfigs, configTemplates, users, settings, AgentHostServiceOptions{})
}

func (s *agentHostService) Create(ctx context.Context, req CreateAgentHostRequest) (*repository.AgentHost, error) {
	token, err := generateAgentHostToken()
	if err != nil {
		return nil, err
	}

	host := &repository.AgentHost{
		Name:            strings.TrimSpace(req.Name),
		Host:            strings.TrimSpace(req.Host),
		Token:           token,
		Status:          0, // Offline initially
		ProvisionStatus: 0, // Active by default for manual creation
	}

	if host.Name == "" || host.Host == "" {
		return nil, fmt.Errorf("name and host are required / 名称和主机地址不能为空")
	}

	if err := s.agentHosts.Create(ctx, host); err != nil {
		return nil, err
	}

	return host, nil
}

func (s *agentHostService) RegisterByCommunicationKey(ctx context.Context, req RegisterAgentHostRequest) (*repository.AgentHost, error) {
	if s == nil || s.settings == nil {
		return nil, fmt.Errorf("agent host service not configured / Agent 服务未配置")
	}

	providedKey := strings.TrimSpace(req.CommunicationKey)
	hostname := strings.TrimSpace(req.Hostname)
	advertiseHost := strings.TrimSpace(req.AdvertiseHost)
	if providedKey == "" || hostname == "" {
		return nil, fmt.Errorf("communication_key and hostname are required / communication_key 与 hostname 不能为空")
	}

	entry, err := s.settings.Get(ctx, communicationKeySettingKey)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, ErrInvalidCommunicationKey
		}
		return nil, err
	}
	expectedKey := ""
	if entry != nil {
		expectedKey = strings.TrimSpace(entry.Value)
	}
	if expectedKey == "" || subtle.ConstantTimeCompare([]byte(expectedKey), []byte(providedKey)) != 1 {
		return nil, ErrInvalidCommunicationKey
	}

	name := hostname
	if name == "" {
		name = buildPendingAgentName()
	}
	hostValue := advertiseHost
	if hostValue == "" {
		hostValue = buildPendingAgentHostPlaceholder()
	}

	token, err := generateAgentHostToken()
	if err != nil {
		return nil, err
	}

	host := &repository.AgentHost{
		Name:            name,
		Host:            hostValue,
		Token:           token,
		Status:          0,
		ProvisionStatus: 1,
	}
	if err := s.agentHosts.Create(ctx, host); err != nil {
		return nil, err
	}
	return host, nil
}

func (s *agentHostService) GetByID(ctx context.Context, id int64) (*repository.AgentHost, error) {
	return s.agentHosts.FindByID(ctx, id)
}

func (s *agentHostService) GetByToken(ctx context.Context, token string) (*repository.AgentHost, error) {
	return s.agentHosts.FindByToken(ctx, token)
}

func (s *agentHostService) Update(ctx context.Context, id int64, req UpdateAgentHostRequest) error {
	host, err := s.agentHosts.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if req.Name != nil {
		host.Name = strings.TrimSpace(*req.Name)
	}
	if req.Host != nil {
		host.Host = strings.TrimSpace(*req.Host)
	}
	if host.Name == "" || host.Host == "" {
		return fmt.Errorf("name and host are required / 名称和主机地址不能为空")
	}
	if host.ProvisionStatus != 0 {
		host.ProvisionStatus = 0
	}

	return s.agentHosts.Update(ctx, host)
}

func (s *agentHostService) Delete(ctx context.Context, id int64) error {
	return s.agentHosts.Delete(ctx, id)
}

func (s *agentHostService) List(ctx context.Context) ([]*repository.AgentHost, error) {
	return s.agentHosts.ListAll(ctx)
}

func (s *agentHostService) FlushMetrics(ctx context.Context) error {
	if s == nil || s.metricsBuffer == nil {
		return nil
	}
	return s.metricsBuffer.Flush(ctx)
}

func (s *agentHostService) UpdateMetrics(ctx context.Context, token string, metrics AgentHostMetricsReport) error {
	host, err := s.agentHosts.FindByToken(ctx, token)
	if err != nil {
		return err
	}

	if s.metricsBuffer != nil {
		if latest, ok, latestErr := s.metricsBuffer.Latest(ctx, host.ID); latestErr != nil {
			return latestErr
		} else if ok {
			host.UploadRateBps = latest.UploadRateBps
			host.DownloadRateBps = latest.DownloadRateBps
			host.RawUploadTotalBytes = latest.RawUploadTotalBytes
			host.RawDownloadTotalBytes = latest.RawDownloadTotalBytes
			host.BootID = latest.BootID
			host.LastRealtimeReportAt = latest.LastRealtimeReportAt
			host.LastRestartAt = latest.LastRestartAt
			host.AgentVersion = latest.AgentVersion
			host.CurrentCoreType = latest.CurrentCoreType
		}
	}

	reportAt := metrics.ReportedAt
	if reportAt == 0 {
		reportAt = time.Now().Unix()
	}
	repoMetrics := repository.AgentHostMetrics{
		CPUTotal:              metrics.CPUTotal,
		CPUUsed:               metrics.CPUUsed,
		MemTotal:              metrics.MemTotal,
		MemUsed:               metrics.MemUsed,
		DiskTotal:             metrics.DiskTotal,
		DiskUsed:              metrics.DiskUsed,
		UploadTotal:           metrics.UploadTotal,
		DownloadTotal:         metrics.DownloadTotal,
		UploadRateBps:         host.UploadRateBps,
		DownloadRateBps:       host.DownloadRateBps,
		RawUploadTotalBytes:   host.RawUploadTotalBytes,
		RawDownloadTotalBytes: host.RawDownloadTotalBytes,
		BootID:                host.BootID,
		LastRealtimeReportAt:  host.LastRealtimeReportAt,
		LastRestartAt:         host.LastRestartAt,
		AgentVersion:          host.AgentVersion,
		CurrentCoreType:       host.CurrentCoreType,
	}
	if metrics.UploadRateBps != nil {
		repoMetrics.UploadRateBps = *metrics.UploadRateBps
		repoMetrics.LastRealtimeReportAt = reportAt
	}
	if metrics.DownloadRateBps != nil {
		repoMetrics.DownloadRateBps = *metrics.DownloadRateBps
		repoMetrics.LastRealtimeReportAt = reportAt
	}
	if metrics.RawUploadTotalBytes != nil {
		repoMetrics.RawUploadTotalBytes = *metrics.RawUploadTotalBytes
		repoMetrics.LastRealtimeReportAt = reportAt
	}
	if metrics.RawDownloadTotalBytes != nil {
		repoMetrics.RawDownloadTotalBytes = *metrics.RawDownloadTotalBytes
		repoMetrics.LastRealtimeReportAt = reportAt
	}
	if metrics.BootID != "" {
		if host.BootID != "" && host.BootID != metrics.BootID {
			repoMetrics.LastRestartAt = reportAt
		}
		repoMetrics.BootID = metrics.BootID
		repoMetrics.LastRealtimeReportAt = reportAt
	}
	if metrics.AgentVersion != "" {
		repoMetrics.AgentVersion = metrics.AgentVersion
		repoMetrics.LastRealtimeReportAt = reportAt
	}
	if metrics.CurrentCoreType != "" {
		repoMetrics.CurrentCoreType = metrics.CurrentCoreType
		repoMetrics.LastRealtimeReportAt = reportAt
	}

	if s.metricsBuffer != nil {
		return s.metricsBuffer.Enqueue(ctx, host.ID, repoMetrics)
	}
	return s.agentHosts.UpdateMetrics(ctx, host.ID, repoMetrics)
}

func (s *agentHostService) UpdateHeartbeat(ctx context.Context, token string) error {
	host, err := s.agentHosts.FindByToken(ctx, token)
	if err != nil {
		return err
	}

	return s.agentHosts.UpdateStatus(ctx, host.ID, 1, time.Now().Unix())
}

func (s *agentHostService) UpdateCapabilities(ctx context.Context, token string, coreVersion string, capabilities, buildTags []string) error {
	host, err := s.agentHosts.FindByToken(ctx, token)
	if err != nil {
		return err
	}

	// Only update if there's actual capability data
	if coreVersion == "" && len(capabilities) == 0 && len(buildTags) == 0 {
		return nil
	}

	return s.agentHosts.UpdateCapabilities(ctx, host.ID, coreVersion, capabilities, buildTags)
}

func (s *agentHostService) UpdateProtocols(ctx context.Context, token string, protocols []ProtocolInfo) error {
	host, err := s.agentHosts.FindByToken(ctx, token)
	if err != nil {
		return err
	}

	servers, err := s.servers.FindByAgentHostID(ctx, host.ID)
	if err != nil {
		return err
	}

	serverMap := make(map[string]*repository.Server)
	for _, srv := range servers {
		serverMap[srv.Name] = srv
	}

	now := time.Now().Unix()

	for _, p := range protocols {
		// Skip non-protocol configs (no parsed protocol details)
		// Only process files that contain actual inbound protocols
		if len(p.Details) == 0 {
			continue
		}

		// Serialize protocol details to JSON for storage in Settings field
		var settingsJSON json.RawMessage = json.RawMessage("{}")
		if len(p.Details) > 0 {
			if jsonBytes, err := json.Marshal(p.Details); err == nil {
				settingsJSON = json.RawMessage(jsonBytes)
			}
		}

		// Determine port from first protocol detail
		var port int
		if len(p.Details) > 0 && p.Details[0].Port > 0 {
			port = p.Details[0].Port
		}

		if srv, exists := serverMap[p.Name]; exists {
			if p.Running {
				srv.LastHeartbeatAt = now
			}
			if p.Type != "" {
				srv.Type = p.Type
			}
			// Update settings with protocol details
			srv.Settings = settingsJSON
			if port > 0 {
				srv.Port = port
			}
			if err := s.servers.Update(ctx, srv); err != nil {
				return fmt.Errorf("update server %s: %v / 更新节点失败: %w", p.Name, err, err)
			}
		} else {
			// Create new server node
			newServer := &repository.Server{
				AgentHostID:     host.ID,
				Name:            p.Name,
				Type:            p.Type,
				LastHeartbeatAt: 0,
				CreatedAt:       now,
				UpdatedAt:       now,
				Host:            host.Host,
				Show:            1,
				// Required defaults
				Port:         port,
				ServerPort:   0,
				Tags:         json.RawMessage("[]"),
				Settings:     settingsJSON,
				ObfsSettings: json.RawMessage("{}"),
			}
			if p.Running {
				newServer.LastHeartbeatAt = now
			}
			if err := s.servers.Create(ctx, newServer); err != nil {
				return fmt.Errorf("create server %s: %v / 创建节点失败: %w", p.Name, err, err)
			}
		}
	}
	return nil
}

func (s *agentHostService) UpdateClientConfigs(ctx context.Context, token string, configs []ClientConfigInfo) error {
	host, err := s.agentHosts.FindByToken(ctx, token)
	if err != nil {
		return err
	}

	// Get all servers for this agent host
	servers, err := s.servers.FindByAgentHostID(ctx, host.ID)
	if err != nil {
		return err
	}

	// Build map of server name -> server ID
	serverIDMap := make(map[string]int64)
	for _, srv := range servers {
		serverIDMap[srv.Name] = srv.ID
	}

	// Also map by tag from Settings if available
	for _, srv := range servers {
		if srv.Settings != nil && len(srv.Settings) > 0 {
			var details []ProtocolDetails
			if err := json.Unmarshal(srv.Settings, &details); err == nil && len(details) > 0 {
				if details[0].Tag != "" {
					serverIDMap[details[0].Tag] = srv.ID
				}
			}
		}
	}

	// Update client configs for each protocol
	for _, cfg := range configs {
		// Find corresponding server by name or tag
		serverID, found := serverIDMap[cfg.Name]
		if !found {
			// Try matching by port
			for _, srv := range servers {
				if srv.Port == cfg.Port && cfg.Port > 0 {
					serverID = srv.ID
					found = true
					break
				}
			}
		}

		if !found {
			// No matching server found, skip
			continue
		}

		// Upsert each format
		for format, content := range cfg.RawConfigs {
			if content == "" {
				continue
			}

			clientCfg := &repository.ServerClientConfig{
				ServerID:    serverID,
				Format:      format,
				Content:     content,
				ContentHash: hashContent(content),
			}

			if err := s.serverClientConfigs.Upsert(ctx, clientCfg); err != nil {
				return fmt.Errorf("upsert client config for server %d format %s: %v / 保存客户端配置失败: %w", serverID, format, err, err)
			}
		}
	}

	return nil
}

// GenerateConfig generates the final configuration for an agent based on its assigned template and servers.
func (s *agentHostService) GenerateConfig(ctx context.Context, agentID int64) ([]byte, error) {
	host, err := s.agentHosts.FindByID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to find agent host: %v / 获取探针节点失败: %w", err, err)
	}

	if host.TemplateID == 0 {
		return nil, nil // No template assigned, return nil config (agent keeps using local config)
	}

	tpl, err := s.configTemplates.FindByID(ctx, host.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to find config template: %v / 获取配置模板失败: %w", err, err)
	}

	// Build template context (hybrid mode: template defines structure, system injects users and inbounds)
	templateCtx, err := s.buildTemplateContext(ctx, host, tpl)
	if err != nil {
		return nil, fmt.Errorf("failed to build template context: %v / 构建模板上下文失败: %w", err, err)
	}

	// Parse agent capabilities and filter context
	agentCaps := s.parseAgentCapabilities(host)
	filter := template.NewCapabilityFilter(agentCaps)
	filteredCtx, warnings := filter.FilterContext(templateCtx)

	// Log warnings
	for _, w := range warnings {
		slog.Warn("Config generation warning", "agent_id", agentID, "warning", w)
	}

	// Check template compatibility
	if tpl.MinVersion != "" {
		compatible, compatWarnings := filter.CheckTemplateCompatibility(tpl.MinVersion, tpl.Capabilities)
		if !compatible {
			return nil, fmt.Errorf("template incompatible with agent: %v / 模板与探针节点不兼容: %v", compatWarnings, compatWarnings)
		}
		for _, w := range compatWarnings {
			slog.Warn("Template compatibility warning", "agent_id", agentID, "warning", w)
		}
	}

	// Render template
	engine := template.NewEngine()
	configJSON, err := engine.Render(tpl.Content, filteredCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %v / 渲染模板失败: %w", err, err)
	}

	// Validate final config
	validator := template.NewValidator()
	result := validator.ValidateFinalConfig(configJSON, tpl.Type)
	if !result.Valid {
		return nil, fmt.Errorf("generated config validation failed: %v / 生成配置校验失败: %v", result.Errors, result.Errors)
	}

	// Log any warnings from validation
	for _, w := range result.Warnings {
		slog.Warn("Config validation warning", "agent_id", agentID, "warning", w)
	}

	return configJSON, nil
}

// buildTemplateContext constructs the template context from host, template and servers.
func (s *agentHostService) buildTemplateContext(ctx context.Context, host *repository.AgentHost, tpl *repository.ConfigTemplate) (*template.TemplateContext, error) {
	// Fetch servers for this agent
	servers, err := s.servers.FindByAgentHostID(ctx, host.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch servers: %v / 获取节点列表失败: %w", err, err)
	}

	// Build inbounds from servers
	inbounds := make([]template.InboundConfig, 0, len(servers))
	groupSet := make(map[int64]struct{})

	for _, srv := range servers {
		if srv.Settings == nil || len(srv.Settings) == 0 {
			continue
		}

		// Parse Settings which is stored as []ProtocolDetails JSON
		var details []ProtocolDetails
		if err := json.Unmarshal(srv.Settings, &details); err != nil || len(details) == 0 {
			continue
		}

		// Track group IDs for user fetching
		if srv.GroupID > 0 {
			groupSet[srv.GroupID] = struct{}{}
		}

		// Convert each protocol detail to InboundConfig
		for _, d := range details {
			inbound := s.convertProtocolDetailsToInbound(d)
			inbounds = append(inbounds, inbound)
		}
	}

	// Fetch users for all server groups
	groupIDs := make([]int64, 0, len(groupSet))
	for id := range groupSet {
		groupIDs = append(groupIDs, id)
	}

	var users []template.UserConfig
	if len(groupIDs) > 0 {
		nodeUsers, err := s.users.ListActiveForGroups(ctx, groupIDs, time.Now().Unix())
		if err == nil {
			for _, u := range nodeUsers {
				users = append(users, template.UserConfig{
					ID:      u.ID,
					UUID:    u.UUID,
					Email:   u.Email,
					Enabled: true,
				})
			}
		}
	}

	// Build default outbounds
	outbounds := []template.OutboundConfig{
		{Type: "direct", Tag: "direct"},
		{Type: "block", Tag: "block"},
	}

	return &template.TemplateContext{
		Inbounds:  inbounds,
		Outbounds: outbounds,
		Users:     users,
		Agent: template.AgentInfo{
			ID:           host.ID,
			Name:         host.Name,
			Host:         host.Host,
			CoreType:     tpl.Type,
			CoreVersion:  host.CoreVersion,
			Capabilities: host.Capabilities,
			BuildTags:    host.BuildTags,
		},
		Server: template.ServerInfo{
			LogLevel:     "info",
			ListenAddr:   "::",
			StatsEnabled: true,
		},
	}, nil
}

// parseAgentCapabilities constructs AgentCapabilities from host data.
func (s *agentHostService) parseAgentCapabilities(host *repository.AgentHost) *template.AgentCapabilities {
	caps := &template.AgentCapabilities{
		CoreType:    "sing-box", // Default
		CoreVersion: host.CoreVersion,
		BuildTags:   host.BuildTags,
	}

	// Convert string capabilities to Capability type
	for _, c := range host.Capabilities {
		caps.Capabilities = append(caps.Capabilities, template.Capability(c))
	}

	// If no capabilities reported, derive from version
	if len(caps.Capabilities) == 0 && host.CoreVersion != "" {
		caps.Capabilities = template.DeriveCapabilities(caps.CoreType, host.CoreVersion, host.BuildTags)
	}

	return caps
}

// convertProtocolDetailsToInbound converts service-level ProtocolDetails to template.InboundConfig.
func (s *agentHostService) convertProtocolDetailsToInbound(d ProtocolDetails) template.InboundConfig {
	inbound := template.InboundConfig{
		Type:       d.Protocol,
		Tag:        d.Tag,
		Listen:     d.Listen,
		ListenPort: d.Port,
	}

	// Convert Transport
	if d.Transport != nil {
		inbound.Transport = &template.TransportConfig{
			Type:        d.Transport.Type,
			Path:        d.Transport.Path,
			Host:        d.Transport.Host,
			ServiceName: d.Transport.ServiceName,
		}
	}

	// Convert TLS
	if d.TLS != nil && d.TLS.Enabled {
		inbound.TLS = &template.TLSConfig{
			Enabled:    d.TLS.Enabled,
			ServerName: d.TLS.ServerName,
			ALPN:       d.TLS.ALPN,
		}

		// Convert Reality
		if d.TLS.Reality != nil && d.TLS.Reality.Enabled {
			inbound.TLS.Reality = &template.RealityConfig{
				Enabled:   true,
				ShortIDs:  d.TLS.Reality.ShortIDs,
				PublicKey: d.TLS.Reality.PublicKey,
			}
			if d.TLS.Reality.HandshakeAddr != "" {
				inbound.TLS.Reality.Handshake = &template.HandshakeConfig{
					Server:     d.TLS.Reality.HandshakeAddr,
					ServerPort: d.TLS.Reality.HandshakePort,
				}
			}

			// Mark Reality as required capability
			inbound.RequiredCapabilities = append(inbound.RequiredCapabilities, "reality")
		}
	}

	// Convert Multiplex
	if d.Multiplex != nil && d.Multiplex.Enabled {
		inbound.Multiplex = &template.MultiplexConfig{
			Enabled: d.Multiplex.Enabled,
			Padding: d.Multiplex.Padding,
		}

		// Mark Multiplex as required capability
		inbound.RequiredCapabilities = append(inbound.RequiredCapabilities, "multiplex")

		if d.Multiplex.Brutal != nil && d.Multiplex.Brutal.Enabled {
			inbound.Multiplex.Brutal = &template.BrutalConfig{
				Enabled:  true,
				UpMbps:   d.Multiplex.Brutal.UpMbps,
				DownMbps: d.Multiplex.Brutal.DownMbps,
			}
			// Mark Brutal as required capability
			inbound.RequiredCapabilities = append(inbound.RequiredCapabilities, "brutal")
		}
	}

	return inbound
}

func convertToSingBoxInbound(d ProtocolDetails) (map[string]interface{}, error) {
	inbound := make(map[string]interface{})

	// Basic fields
	inbound["type"] = d.Protocol
	inbound["tag"] = d.Tag

	if d.Listen != "" {
		inbound["listen"] = d.Listen
	}
	if d.Port > 0 {
		inbound["listen_port"] = d.Port
	}

	// Users
	if len(d.Users) > 0 {
		var users []map[string]interface{}
		for _, u := range d.Users {
			user := make(map[string]interface{})
			if u.UUID != "" {
				user["uuid"] = u.UUID
			}
			if u.Email != "" {
				user["name"] = u.Email
			}
			if u.Method != "" {
				user["method"] = u.Method // Shadowsocks
			}
			if u.Flow != "" {
				user["flow"] = u.Flow // VLESS / XTLS
			}
			// Shadowsocks specific
			if d.Protocol == "shadowsocks" {
				if u.Method != "" {
					user["method"] = u.Method
				}
				if u.UUID != "" {
					user["password"] = u.UUID // UUID field reused as password
				}
			}
			users = append(users, user)
		}
		inbound["users"] = users
	}

	// Transport
	if d.Transport != nil {
		transport := make(map[string]interface{})
		transport["type"] = d.Transport.Type

		switch d.Transport.Type {
		case "http":
			if d.Transport.Host != "" {
				transport["host"] = []string{d.Transport.Host}
			}
			if d.Transport.Path != "" {
				transport["path"] = d.Transport.Path
			}
		case "ws":
			if d.Transport.Path != "" {
				transport["path"] = d.Transport.Path
			}
			if d.Transport.Host != "" {
				transport["headers"] = map[string]string{"Host": d.Transport.Host}
			}
		case "grpc":
			if d.Transport.ServiceName != "" {
				transport["service_name"] = d.Transport.ServiceName
			}
		}
		inbound["transport"] = transport
	}

	// TLS
	if d.TLS != nil && d.TLS.Enabled {
		tls := make(map[string]interface{})
		tls["enabled"] = true
		if d.TLS.ServerName != "" {
			tls["server_name"] = d.TLS.ServerName
		}
		if len(d.TLS.ALPN) > 0 {
			tls["alpn"] = d.TLS.ALPN
		}

		// Reality
		if d.TLS.Reality != nil && d.TLS.Reality.Enabled {
			reality := make(map[string]interface{})
			reality["enabled"] = true
			if d.TLS.Reality.PublicKey != "" {
				reality["public_key"] = d.TLS.Reality.PublicKey
			}
			if d.TLS.Reality.ShortIDs != nil {
				reality["short_id"] = d.TLS.Reality.ShortIDs
			}
			if d.TLS.Reality.HandshakeAddr != "" && d.TLS.Reality.HandshakePort > 0 {
				reality["handshake"] = fmt.Sprintf("%s:%d", d.TLS.Reality.HandshakeAddr, d.TLS.Reality.HandshakePort)
			} else if d.TLS.Reality.ServerName != "" {
				// Fallback to server_name:443 if handshake not explicit?
				// Usually handshake is required for Reality.
			}
			tls["reality"] = reality
		}

		inbound["tls"] = tls
	}

	// Multiplex
	if d.Multiplex != nil && d.Multiplex.Enabled {
		mux := make(map[string]interface{})
		mux["enabled"] = true
		if d.Multiplex.Padding {
			mux["padding"] = true
		}
		if d.Multiplex.Brutal != nil && d.Multiplex.Brutal.Enabled {
			brutal := make(map[string]interface{})
			brutal["enabled"] = true
			if d.Multiplex.Brutal.UpMbps > 0 {
				brutal["up_mbps"] = d.Multiplex.Brutal.UpMbps
			}
			if d.Multiplex.Brutal.DownMbps > 0 {
				brutal["down_mbps"] = d.Multiplex.Brutal.DownMbps
			}
			mux["brutal"] = brutal
		}
		inbound["multiplex"] = mux
	}

	return inbound, nil
}

func hashContent(content string) string {
	h := md5.Sum([]byte(content))
	return hex.EncodeToString(h[:])
}

// AssignTemplate assigns a template to an agent host after checking compatibility.
func (s *agentHostService) AssignTemplate(ctx context.Context, agentID, templateID int64) error {
	// Get the agent host
	host, err := s.agentHosts.FindByID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("find agent host: %v / 获取探针节点失败: %w", err, err)
	}

	// If templateID is 0, clear the template assignment
	if templateID == 0 {
		host.TemplateID = 0
		return s.agentHosts.Update(ctx, host)
	}

	// Check template existence
	_, err = s.configTemplates.FindByID(ctx, templateID)
	if err != nil {
		return fmt.Errorf("find config template: %v / 获取配置模板失败: %w", err, err)
	}

	// Check compatibility (log warnings but don't block assignment)
	result, err := s.CheckTemplateCompatibility(ctx, agentID, templateID)
	if err != nil {
		slog.Warn("Failed to check template compatibility",
			"agent_id", agentID,
			"template_id", templateID,
			"error", err,
		)
	} else if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			slog.Warn("Template compatibility warning",
				"agent_id", agentID,
				"template_id", templateID,
				"warning", w,
			)
		}
	}

	// Assign the template
	host.TemplateID = templateID
	return s.agentHosts.Update(ctx, host)
}

func generateAgentHostToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate token: %v / 生成 token 失败: %w", err, err)
	}
	return hex.EncodeToString(tokenBytes), nil
}

func (s *agentHostService) GetConfigYAML(ctx context.Context, id int64) (string, error) {
	host, err := s.agentHosts.FindByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("find agent host: %v / 获取探针节点失败: %w", err, err)
	}
	return host.ConfigYAML, nil
}

func buildPendingAgentName() string {
	suffix, err := randomHex(4)
	if err != nil {
		return "Agent-pending"
	}
	return fmt.Sprintf("Agent-%s", suffix)
}

func buildPendingAgentHostPlaceholder() string {
	suffix, err := randomHex(6)
	if err != nil {
		return "pending-host"
	}
	return fmt.Sprintf("pending-%s", suffix)
}

func randomHex(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// CheckTemplateCompatibility checks if a template is compatible with an agent's capabilities.
func (s *agentHostService) CheckTemplateCompatibility(ctx context.Context, agentID, templateID int64) (*TemplateCompatibilityResult, error) {
	result := &TemplateCompatibilityResult{
		Compatible: true,
		Warnings:   []string{},
		Errors:     []string{},
	}

	// Get the agent host
	host, err := s.agentHosts.FindByID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("find agent host: %w", err)
	}

	// Get the template
	tpl, err := s.configTemplates.FindByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("find config template: %w", err)
	}

	// Check if template is valid
	if !tpl.IsValid {
		result.Errors = append(result.Errors, fmt.Sprintf("Template has validation errors: %s / 模板校验失败: %s", tpl.ValidationError, tpl.ValidationError))
		result.Compatible = false
		return result, nil
	}

	// Build agent capabilities
	agentCaps := s.parseAgentCapabilities(host)

	// Check minimum version requirement
	if tpl.MinVersion != "" {
		if host.CoreVersion == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Template requires version %s, but agent has not reported its version yet / 模板要求版本 %s，但探针尚未上报版本",
				tpl.MinVersion, tpl.MinVersion,
			))
		} else {
			filter := template.NewCapabilityFilter(agentCaps)
			if !filter.SupportsVersion(tpl.MinVersion) {
				result.Errors = append(result.Errors, fmt.Sprintf(
					"Template requires version %s, agent has %s / 模板要求版本 %s，探针版本为 %s",
					tpl.MinVersion, host.CoreVersion, tpl.MinVersion, host.CoreVersion,
				))
				result.Compatible = false
			}
		}
	}

	// Check required capabilities
	for _, reqCap := range tpl.Capabilities {
		if !agentCaps.SupportsCapability(template.Capability(reqCap)) {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"Template requires capability '%s' which may not be supported by agent / 模板需要能力 '%s'，探针可能不支持",
				reqCap, reqCap,
			))
		}
	}

	// Additional checks based on template content
	if len(host.Capabilities) == 0 && host.CoreVersion == "" {
		result.Warnings = append(result.Warnings, "Agent has not reported capabilities yet. Compatibility cannot be fully verified. / 探针尚未上报能力，无法完全校验兼容性。")
	}

	return result, nil
}
