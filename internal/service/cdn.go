package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	corecdn "github.com/creamcroissant/xboard/internal/cdn"
	"github.com/creamcroissant/xboard/internal/cdn/cloudflare"
	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/support/security"
)

// CDNService 管理 CDN 加速域名的业务逻辑
type CDNService interface {
	// Site CRUD
	CreateSite(ctx context.Context, req CreateCDNSiteRequest) (*repository.CDNSite, error)
	UpdateSite(ctx context.Context, id int64, req UpdateCDNSiteRequest) (*repository.CDNSite, error)
	DeleteSite(ctx context.Context, id int64) error
	GetSite(ctx context.Context, id int64) (*repository.CDNSite, error)
	ListSites(ctx context.Context, filter repository.CDNSiteFilter) ([]*repository.CDNSite, int64, error)

	// Edge 管理
	AssignEdge(ctx context.Context, siteID, agentHostID int64, weight int) (*repository.CDNEdge, error)
	RemoveEdge(ctx context.Context, id int64) error
	ListEdges(ctx context.Context, siteID int64) ([]*repository.CDNEdge, error)

	// Cache Rule 管理
	CreateCacheRule(ctx context.Context, req CreateCDNCacheRuleRequest) (*repository.CDNCacheRule, error)
	DeleteCacheRule(ctx context.Context, id int64) error
	ListCacheRules(ctx context.Context, siteID int64) ([]*repository.CDNCacheRule, error)

	// 部署操作（stub，暂不实现 command queue）
	DeploySite(ctx context.Context, siteID int64) error
	UndeploySite(ctx context.Context, siteID int64) error

	// Cloudflare 集成
	SaveCloudflareAPIToken(ctx context.Context, apiToken string) error
	GetCloudflareAPIToken(ctx context.Context) (string, error)
	ListCloudflareZones(ctx context.Context) ([]*repository.CloudflareZone, error)
	AddCloudflareZone(ctx context.Context, name, zoneID string) (*repository.CloudflareZone, error)
	RemoveCloudflareZone(ctx context.Context, id int64) error
	ListCloudflareDNSRecords(ctx context.Context, zoneID int64) ([]cloudflare.CFDNSRecord, error)

	// CloudFront 集成
	SetCloudFrontCredentials(ctx context.Context, accessKeyID, secretAccessKey string) error
	GetCloudFrontCredentials(ctx context.Context) (*CloudFrontCredentials, error)
	ListCloudFrontDistributions(ctx context.Context) ([]*repository.CloudFrontDistribution, error)

	// Provider 通用操作
	SyncToProvider(ctx context.Context, siteID int64) error
	InvalidateCache(ctx context.Context, siteID int64) error
	GetProviderStatus(ctx context.Context, siteID int64) (*ProviderStatusResult, error)

	// 加速配置管理（复用 cdn_sites / cdn_edges）
	CreateAcceleration(ctx context.Context, specID int64, provider, domain, originPath string) (*CDNAccelerationConfig, error)
	UpdateAcceleration(ctx context.Context, id int64, req UpdateCDNAccelerationRequest) error
	DeleteAcceleration(ctx context.Context, id int64) error
	GetAccelerationByInboundSpec(ctx context.Context, specID int64) (*CDNAccelerationConfig, error)
	ListAccelerationEdges(ctx context.Context, configID int64) ([]*repository.CDNEdge, error)
	AssignAccelerationEdge(ctx context.Context, configID, agentHostID int64, weight int) (*repository.CDNEdge, error)
	RemoveAccelerationEdge(ctx context.Context, edgeID int64) error

	// 一键部署/卸载加速（stub，Provider 调用暂不集成）
	DeployAcceleration(ctx context.Context, accelerationID int64) error
	UndoDeployAcceleration(ctx context.Context, accelerationID int64) error

	// Origin Latency 监控
	ReportOriginLatency(ctx context.Context, siteID int64, stack string, domain string, latencyMs int64) error
	GetOriginLatencies(ctx context.Context) ([]*repository.CDNOriginLatency, error)
	GetAllOriginLatencies(ctx context.Context) ([]*repository.CDNOriginLatency, error)
}

// CreateCDNSiteRequest 创建 CDN 站点请求
type CreateCDNSiteRequest struct {
	Name             string
	Description      string
	Domain           string
	OriginType       string
	OriginURL        string
	CacheTTL         int
	SSLMode          string
	AccelerationMode string
	Enabled          bool
}

// UpdateCDNSiteRequest 更新 CDN 站点请求
type UpdateCDNSiteRequest struct {
	Name             *string
	Description      *string
	Domain           *string
	OriginType       *string
	OriginURL        *string
	CacheTTL         *int
	SSLMode          *string
	AccelerationMode *string
	Enabled          *bool
}

// CreateCDNCacheRuleRequest 创建缓存规则请求
type CreateCDNCacheRuleRequest struct {
	SiteID     int64
	MatchType  string
	MatchValue string
	CacheTTL   int
	Bypass     bool
	Priority   int
}

// CloudFrontCredentials 表示 CloudFront 的 AWS 凭证
type CloudFrontCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

// ProviderStatusResult 表示 provider 同步状态
type ProviderStatusResult struct {
	Provider   string `json:"provider"`
	Status     string `json:"status"`
	DomainName string `json:"domain_name"`
	Enabled    bool   `json:"enabled"`
}

// CDNAccelerationConfig 表示一条加速配置（对应 cdn_sites 记录，acceleration_mode=xhttp）
type CDNAccelerationConfig struct {
	ID             int64  `json:"id"`
	InboundSpecID  int64  `json:"inbound_spec_id"`
	CDNSiteID      int64  `json:"cdn_site_id"`
	Provider       string `json:"provider"`
	Domain         string `json:"domain"`
	OriginPath     string `json:"origin_path"`
	OriginProtocol string `json:"origin_protocol"`
	Enabled        bool   `json:"enabled"`
	DeployStatus   string `json:"deploy_status"`
	LastDeployedAt *int64 `json:"last_deployed_at"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

// UpdateCDNAccelerationRequest 更新加速配置的请求（指针字段表示可选更新）
type UpdateCDNAccelerationRequest struct {
	Provider       *string `json:"provider,omitempty"`
	Domain         *string `json:"domain,omitempty"`
	OriginPath     *string `json:"origin_path,omitempty"`
	OriginProtocol *string `json:"origin_protocol,omitempty"`
	Enabled        *bool   `json:"enabled,omitempty"`
}

var ErrCDNSiteInvalid = errors.New("service: invalid cdn site / CDN 站点配置无效")

// cdnService 实现 CDNService 接口
type cdnService struct {
	sites          repository.CDNSiteRepository
	edges          repository.CDNEdgeRepository
	cacheRules     repository.CDNCacheRuleRepository
	originLatency  repository.CDNOriginLatencyRepository
	cfZones        repository.CloudflareZoneRepository
	cfDNS        repository.CloudflareDNSRecordRepository
	cfDists      repository.CloudFrontDistributionRepository
	settings     repository.SettingRepository
	agentHosts   repository.AgentHostRepository
	lifecycleOps AgentLifecycleOperationService
	encKey       string
	logger       *slog.Logger
}

// NewCDNService 创建 CDN 服务
func NewCDNService(
	sites repository.CDNSiteRepository,
	edges repository.CDNEdgeRepository,
	cacheRules repository.CDNCacheRuleRepository,
	settings repository.SettingRepository,
	cfZones repository.CloudflareZoneRepository,
	cfDNS repository.CloudflareDNSRecordRepository,
	cfDists repository.CloudFrontDistributionRepository,
	encryptionKey string,
	extra ...any,
) CDNService {
	return NewCDNServiceWithLogger(sites, edges, cacheRules, settings, cfZones, cfDNS, cfDists, encryptionKey, nil, extra...)
}

func NewCDNServiceWithLogger(
	sites repository.CDNSiteRepository,
	edges repository.CDNEdgeRepository,
	cacheRules repository.CDNCacheRuleRepository,
	settings repository.SettingRepository,
	cfZones repository.CloudflareZoneRepository,
	cfDNS repository.CloudflareDNSRecordRepository,
	cfDists repository.CloudFrontDistributionRepository,
	encryptionKey string,
	logger *slog.Logger,
	extra ...any,
) CDNService {
	if logger == nil {
		logger = slog.Default()
	}
	svc := &cdnService{
		sites:      sites,
		edges:      edges,
		cacheRules: cacheRules,
		cfZones:    cfZones,
		cfDNS:      cfDNS,
		cfDists:    cfDists,
		settings:   settings,
		encKey:     encryptionKey,
		logger:     logger,
	}
	// 从 extra 中提取可选依赖
	for _, dep := range extra {
		switch v := dep.(type) {
		case repository.AgentHostRepository:
			svc.agentHosts = v
		case AgentLifecycleOperationService:
			svc.lifecycleOps = v
		case repository.CDNOriginLatencyRepository:
			svc.originLatency = v
		}
	}
	return svc
}

func validateCDNSiteForCaddy(domain, originType, originURL string) error {
	if err := corecdn.ValidateCaddySiteConfig(domain, originType, originURL); err != nil {
		return fmt.Errorf("%w: %v", ErrCDNSiteInvalid, err)
	}
	return nil
}

// CreateSite 创建 CDN 站点
func (s *cdnService) CreateSite(ctx context.Context, req CreateCDNSiteRequest) (*repository.CDNSite, error) {
	if err := validateCDNSiteForCaddy(req.Domain, req.OriginType, req.OriginURL); err != nil {
		return nil, err
	}

	site := &repository.CDNSite{
		Name:             req.Name,
		Description:      req.Description,
		Domain:           req.Domain,
		OriginType:       req.OriginType,
		OriginURL:        req.OriginURL,
		CacheTTL:         req.CacheTTL,
		SSLMode:          req.SSLMode,
		AccelerationMode: req.AccelerationMode,
		Enabled:          req.Enabled,
		Status:           "active",
	}

	if err := s.sites.Create(ctx, site); err != nil {
		return nil, fmt.Errorf("create cdn site: %w", err)
	}

	return site, nil
}

// UpdateSite 更新 CDN 站点
func (s *cdnService) UpdateSite(ctx context.Context, id int64, req UpdateCDNSiteRequest) (*repository.CDNSite, error) {
	site, err := s.sites.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		site.Name = *req.Name
	}
	if req.Description != nil {
		site.Description = *req.Description
	}
	if req.Domain != nil {
		// 检查域名唯一性（仅当域名变更时）
		if *req.Domain != site.Domain {
			existing, err := s.sites.FindByDomain(ctx, *req.Domain)
			if err != nil && !errors.Is(err, repository.ErrNotFound) {
				return nil, err
			}
			if existing != nil && existing.ID != id {
				return nil, fmt.Errorf("domain %s already in use by site %d", *req.Domain, existing.ID)
			}
		}
		site.Domain = *req.Domain
	}
	if req.OriginType != nil {
		site.OriginType = *req.OriginType
	}
	if req.OriginURL != nil {
		site.OriginURL = *req.OriginURL
	}
	if req.CacheTTL != nil {
		site.CacheTTL = *req.CacheTTL
	}
	if req.SSLMode != nil {
		site.SSLMode = *req.SSLMode
	}
	if req.AccelerationMode != nil {
		site.AccelerationMode = *req.AccelerationMode
	}
	if req.Enabled != nil {
		site.Enabled = *req.Enabled
	}

	if err := validateCDNSiteForCaddy(site.Domain, site.OriginType, site.OriginURL); err != nil {
		return nil, err
	}

	if err := s.sites.Update(ctx, site); err != nil {
		return nil, fmt.Errorf("update cdn site: %w", err)
	}

	return site, nil
}

// DeleteSite 删除 CDN 站点
func (s *cdnService) DeleteSite(ctx context.Context, id int64) error {
	if err := s.sites.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete cdn site: %w", err)
	}
	return nil
}

// GetSite 获取 CDN 站点详情
func (s *cdnService) GetSite(ctx context.Context, id int64) (*repository.CDNSite, error) {
	return s.sites.FindByID(ctx, id)
}

// ListSites 列出 CDN 站点（分页）
func (s *cdnService) ListSites(ctx context.Context, filter repository.CDNSiteFilter) ([]*repository.CDNSite, int64, error) {
	total, err := s.sites.Count(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count cdn sites: %w", err)
	}

	sites, err := s.sites.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list cdn sites: %w", err)
	}

	return sites, total, nil
}

// AssignEdge 为站点分配边缘节点
func (s *cdnService) AssignEdge(ctx context.Context, siteID, agentHostID int64, weight int) (*repository.CDNEdge, error) {
	edge := &repository.CDNEdge{
		SiteID:      siteID,
		AgentHostID: agentHostID,
		Weight:      weight,
		Enabled:     true,
		Status:      "pending",
	}

	if err := s.edges.Create(ctx, edge); err != nil {
		return nil, fmt.Errorf("assign edge: %w", err)
	}

	return edge, nil
}

// RemoveEdge 移除边缘节点分配
func (s *cdnService) RemoveEdge(ctx context.Context, id int64) error {
	if err := s.edges.Delete(ctx, id); err != nil {
		return fmt.Errorf("remove edge: %w", err)
	}
	return nil
}

// ListEdges 列出站点的所有边缘节点
func (s *cdnService) ListEdges(ctx context.Context, siteID int64) ([]*repository.CDNEdge, error) {
	return s.edges.ListBySiteID(ctx, siteID)
}

// CreateCacheRule 创建缓存规则
func (s *cdnService) CreateCacheRule(ctx context.Context, req CreateCDNCacheRuleRequest) (*repository.CDNCacheRule, error) {
	rule := &repository.CDNCacheRule{
		SiteID:     req.SiteID,
		MatchType:  req.MatchType,
		MatchValue: req.MatchValue,
		CacheTTL:   req.CacheTTL,
		Bypass:     req.Bypass,
		Priority:   req.Priority,
	}

	if err := s.cacheRules.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("create cache rule: %w", err)
	}

	return rule, nil
}

// DeleteCacheRule 删除缓存规则
func (s *cdnService) DeleteCacheRule(ctx context.Context, id int64) error {
	if err := s.cacheRules.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete cache rule: %w", err)
	}
	return nil
}

// ListCacheRules 列出站点的所有缓存规则
func (s *cdnService) ListCacheRules(ctx context.Context, siteID int64) ([]*repository.CDNCacheRule, error) {
	return s.cacheRules.ListBySiteID(ctx, siteID)
}

// DeploySite 部署 CDN 站点到其关联的边缘节点
func (s *cdnService) DeploySite(ctx context.Context, siteID int64) error {
	s.logger.Info("deploy site", "site_id", siteID)

	site, err := s.sites.FindByID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("deploy site: find site %d: %w", siteID, err)
	}
	if site == nil {
		return fmt.Errorf("site not found: %d", siteID)
	}

	edges, err := s.edges.ListBySiteID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("deploy site: list edges: %w", err)
	}

	s.logger.Info("deploying site",
		"site_id", siteID,
		"domain", site.Domain,
		"edge_count", len(edges),
	)

	if s.lifecycleOps != nil {
		for _, edge := range edges {
			payload, err := json.Marshal(map[string]any{
				"action":      AgentLifecycleOperationTypeCDNDeploySite,
				"site_id":     siteID,
				"domain":      site.Domain,
				"origin_type": site.OriginType,
				"origin_url":  site.OriginURL,
				"cache_ttl":   site.CacheTTL,
				"ssl_mode":    site.SSLMode,
				"edge_id":     edge.ID,
				"agent_id":    edge.AgentHostID,
			})
			if err != nil {
				s.logger.Error("marshal cdn lifecycle payload", "error", err, "action", "cdn_deploy")
				payload = json.RawMessage("{}")
			}
			_, err = s.lifecycleOps.Create(ctx, CreateAgentLifecycleOperationRequest{
				AgentHostID:    edge.AgentHostID,
				OperationType:  AgentLifecycleOperationTypeCDNDeploySite,
				RequestPayload: payload,
				Source:         "system",
			})
			if err != nil {
				s.logger.Error("deploy site: create lifecycle operation",
					"edge_id", edge.ID,
					"agent_host_id", edge.AgentHostID,
					"error", err,
				)
				continue
			}
			s.logger.Info("deploy site: created lifecycle operation",
				"edge_id", edge.ID,
				"agent_host_id", edge.AgentHostID,
			)
		}
	} else {
		s.logger.Warn("deploy site: lifecycleOps not configured, skipping command queue")
	}

	site.Status = "deploying"
	if err := s.sites.Update(ctx, site); err != nil {
		return fmt.Errorf("deploy site: update status: %w", err)
	}

	s.logger.Info("deploy site complete", "site_id", siteID)
	return nil
}

// UndeploySite 从边缘节点卸载 CDN 站点
func (s *cdnService) UndeploySite(ctx context.Context, siteID int64) error {
	s.logger.Info("undeploy site", "site_id", siteID)

	site, err := s.sites.FindByID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("undeploy site: find site %d: %w", siteID, err)
	}
	if site == nil {
		return fmt.Errorf("site not found: %d", siteID)
	}

	edges, err := s.edges.ListBySiteID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("undeploy site: list edges: %w", err)
	}

	s.logger.Info("undeploying site",
		"site_id", siteID,
		"domain", site.Domain,
		"edge_count", len(edges),
	)

	if s.lifecycleOps != nil {
		for _, edge := range edges {
			payload, err := json.Marshal(map[string]any{
				"action":   AgentLifecycleOperationTypeCDNRemoveSite,
				"site_id":  siteID,
				"domain":   site.Domain,
				"edge_id":  edge.ID,
				"agent_id": edge.AgentHostID,
			})
			if err != nil {
				s.logger.Error("marshal cdn lifecycle payload", "error", err, "action", "cdn_deploy")
				payload = json.RawMessage("{}")
			}
			_, err = s.lifecycleOps.Create(ctx, CreateAgentLifecycleOperationRequest{
				AgentHostID:    edge.AgentHostID,
				OperationType:  AgentLifecycleOperationTypeCDNRemoveSite,
				RequestPayload: payload,
				Source:         "system",
			})
			if err != nil {
				s.logger.Error("undeploy site: create lifecycle operation",
					"edge_id", edge.ID,
					"agent_host_id", edge.AgentHostID,
					"error", err,
				)
				continue
			}
			s.logger.Info("undeploy site: created lifecycle operation",
				"edge_id", edge.ID,
				"agent_host_id", edge.AgentHostID,
			)
		}
	} else {
		s.logger.Warn("undeploy site: lifecycleOps not configured, skipping command queue")
	}

	site.Status = "active"
	if err := s.sites.Update(ctx, site); err != nil {
		return fmt.Errorf("undeploy site: update status: %w", err)
	}

	s.logger.Info("undeploy site complete", "site_id", siteID)
	return nil
}

// ---------------------------------------------------------------------------
// Cloudflare 集成
// ---------------------------------------------------------------------------

const (
	cdnCFAPITokenKey = "cdn_cloudflare_api_token"
	cdnCFAccessKey   = "cdn_cloudfront_access_key"
	cdnCFSecretKey   = "cdn_cloudfront_secret_key"
)

// SaveCloudflareAPIToken 保存（加密）Cloudflare API Token
func (s *cdnService) SaveCloudflareAPIToken(ctx context.Context, apiToken string) error {
	encrypted, err := security.Encrypt([]byte(apiToken), []byte(s.encKey))
	if err != nil {
		return fmt.Errorf("encrypt cloudflare api token: %w", err)
	}
	return s.settings.Upsert(ctx, &repository.Setting{
		Key:      cdnCFAPITokenKey,
		Value:    encrypted,
		Category: "cdn",
	})
}

// GetCloudflareAPIToken 获取（解密）Cloudflare API Token
func (s *cdnService) GetCloudflareAPIToken(ctx context.Context) (string, error) {
	setting, err := s.settings.Get(ctx, cdnCFAPITokenKey)
	if err != nil {
		return "", fmt.Errorf("get cloudflare api token: %w", err)
	}
	decrypted, err := security.Decrypt(setting.Value, []byte(s.encKey))
	if err != nil {
		return "", fmt.Errorf("decrypt cloudflare api token: %w", err)
	}
	return string(decrypted), nil
}

// ListCloudflareZones 列出已添加的 Cloudflare 区域
func (s *cdnService) ListCloudflareZones(ctx context.Context) ([]*repository.CloudflareZone, error) {
	return s.cfZones.List(ctx)
}

// AddCloudflareZone 添加 Cloudflare 区域到管理
func (s *cdnService) AddCloudflareZone(ctx context.Context, name, zoneID string) (*repository.CloudflareZone, error) {
	zone := &repository.CloudflareZone{
		ZoneName: name,
		ZoneID:   zoneID,
		Status:   "active",
		Enabled:  true,
	}
	if err := s.cfZones.Create(ctx, zone); err != nil {
		return nil, fmt.Errorf("add cloudflare zone: %w", err)
	}
	return zone, nil
}

// RemoveCloudflareZone 从管理中移除 Cloudflare 区域
func (s *cdnService) RemoveCloudflareZone(ctx context.Context, id int64) error {
	if err := s.cfZones.Delete(ctx, id); err != nil {
		return fmt.Errorf("remove cloudflare zone: %w", err)
	}
	return nil
}

// ListCloudflareDNSRecords 从 Cloudflare API 获取 DNS 记录列表
func (s *cdnService) ListCloudflareDNSRecords(ctx context.Context, zoneID int64) ([]cloudflare.CFDNSRecord, error) {
	apiToken, err := s.GetCloudflareAPIToken(ctx)
	if err != nil {
		return nil, err
	}
	client := cloudflare.NewClient(apiToken)
	// 获取内部 zone 记录对应的 Cloudflare Zone ID
	zone, err := s.cfZones.FindByID(ctx, zoneID)
	if err != nil {
		return nil, fmt.Errorf("find cloudflare zone: %w", err)
	}
	records, err := client.ListDNSRecords(ctx, zone.ZoneID)
	if err != nil {
		return nil, fmt.Errorf("list cloudflare dns records: %w", err)
	}
	return records, nil
}

// ---------------------------------------------------------------------------
// CloudFront 集成
// ---------------------------------------------------------------------------

// SetCloudFrontCredentials 保存（加密）CloudFront AWS 凭证
func (s *cdnService) SetCloudFrontCredentials(ctx context.Context, accessKeyID, secretAccessKey string) error {
	cred := CloudFrontCredentials{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	}
	plaintext, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("marshal cloudfront credentials: %w", err)
	}
	encrypted, err := security.Encrypt(plaintext, []byte(s.encKey))
	if err != nil {
		return fmt.Errorf("encrypt cloudfront credentials: %w", err)
	}
	return s.settings.Upsert(ctx, &repository.Setting{
		Key:      cdnCFAccessKey,
		Value:    encrypted,
		Category: "cdn",
	})
}

// GetCloudFrontCredentials 获取（解密）CloudFront AWS 凭证
func (s *cdnService) GetCloudFrontCredentials(ctx context.Context) (*CloudFrontCredentials, error) {
	setting, err := s.settings.Get(ctx, cdnCFAccessKey)
	if err != nil {
		return nil, fmt.Errorf("get cloudfront credentials: %w", err)
	}
	decrypted, err := security.Decrypt(setting.Value, []byte(s.encKey))
	if err != nil {
		return nil, fmt.Errorf("decrypt cloudfront credentials: %w", err)
	}
	var cred CloudFrontCredentials
	if err := json.Unmarshal(decrypted, &cred); err != nil {
		return nil, fmt.Errorf("unmarshal cloudfront credentials: %w", err)
	}
	return &cred, nil
}

// ListCloudFrontDistributions 列出已添加的 CloudFront Distribution
func (s *cdnService) ListCloudFrontDistributions(ctx context.Context) ([]*repository.CloudFrontDistribution, error) {
	return s.cfDists.List(ctx)
}

// ---------------------------------------------------------------------------
// Provider 通用操作
// ---------------------------------------------------------------------------

// SyncToProvider 将站点同步到 CDN Provider
func (s *cdnService) SyncToProvider(ctx context.Context, siteID int64) error {
	s.logger.Info("sync cdn site to provider (stub)", "site_id", siteID)
	return nil
}

// InvalidateCache 刷新 CDN 缓存
func (s *cdnService) InvalidateCache(ctx context.Context, siteID int64) error {
	s.logger.Info("invalidate cdn cache (stub)", "site_id", siteID)
	return nil
}

// GetProviderStatus 获取 CDN Provider 状态
func (s *cdnService) GetProviderStatus(ctx context.Context, siteID int64) (*ProviderStatusResult, error) {
	s.logger.Info("get provider status (stub)", "site_id", siteID)
	return &ProviderStatusResult{
		Provider: "unknown",
		Status:   "active",
		Enabled:  true,
	}, nil
}

// ---------------------------------------------------------------------------
// 加速配置管理（复用 cdn_sites / cdn_edges）
// ---------------------------------------------------------------------------

// toAccelerationConfig 将 repository.CDNSite 转换为 CDNAccelerationConfig
func toAccelerationConfig(site *repository.CDNSite) *CDNAccelerationConfig {
	cfg := &CDNAccelerationConfig{
		ID:             site.ID,
		CDNSiteID:      site.ID,
		Provider:       site.Provider,
		Domain:         site.Domain,
		OriginPath:     site.OriginPath,
		OriginProtocol: site.OriginProtocol,
		Enabled:        site.Enabled,
		DeployStatus:   site.Status,
		LastDeployedAt: site.LastDeployedAt,
		CreatedAt:      site.CreatedAt,
		UpdatedAt:      site.UpdatedAt,
	}
	if site.InboundSpecID != nil {
		cfg.InboundSpecID = *site.InboundSpecID
	}
	return cfg
}

// CreateAcceleration 创建加速配置（写一条 cdn_site，acceleration_mode=xhttp）
func (s *cdnService) CreateAcceleration(ctx context.Context, specID int64, provider, domain, originPath string) (*CDNAccelerationConfig, error) {
	site := &repository.CDNSite{
		Name:             domain,
		Description:      "xhttp acceleration",
		Domain:           domain,
		OriginType:       "reverse_proxy",
		OriginURL:        originPath,
		CacheTTL:         0,
		SSLMode:          "auto_acme",
		AccelerationMode: "xhttp",
		InboundSpecID:    &specID,
		Provider:         provider,
		OriginPath:       originPath,
		OriginProtocol:   "https",
		Enabled:          true,
		Status:           "active",
	}

	if err := s.sites.Create(ctx, site); err != nil {
		return nil, fmt.Errorf("create acceleration: %w", err)
	}

	return toAccelerationConfig(site), nil
}

// UpdateAcceleration 更新加速配置
func (s *cdnService) UpdateAcceleration(ctx context.Context, id int64, req UpdateCDNAccelerationRequest) error {
	site, err := s.sites.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find acceleration site: %w", err)
	}

	if req.Provider != nil {
		site.Provider = *req.Provider
	}
	if req.Domain != nil {
		site.Domain = *req.Domain
		site.Name = *req.Domain
	}
	if req.OriginPath != nil {
		site.OriginPath = *req.OriginPath
		site.OriginURL = *req.OriginPath
	}
	if req.OriginProtocol != nil {
		site.OriginProtocol = *req.OriginProtocol
	}
	if req.Enabled != nil {
		site.Enabled = *req.Enabled
	}

	if err := s.sites.Update(ctx, site); err != nil {
		return fmt.Errorf("update acceleration: %w", err)
	}
	return nil
}

// DeleteAcceleration 删除加速配置及关联的边缘节点
func (s *cdnService) DeleteAcceleration(ctx context.Context, id int64) error {
	edges, err := s.edges.ListBySiteID(ctx, id)
	if err != nil {
		return fmt.Errorf("list acceleration edges before delete: %w", err)
	}
	// Pre-validate all edges are deletable
	for _, e := range edges {
		if _, err := s.edges.FindByID(ctx, e.ID); err != nil {
			return fmt.Errorf("pre-check edge %d: %w", e.ID, err)
		}
	}
	// Execute deletions
	for _, e := range edges {
		if err := s.edges.Delete(ctx, e.ID); err != nil {
			return fmt.Errorf("delete edge %d: %w", e.ID, err)
		}
	}
	if err := s.sites.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete acceleration site: %w", err)
	}
	return nil
}

// GetAccelerationByInboundSpec 通过 inbound_spec_id 查询加速配置
func (s *cdnService) GetAccelerationByInboundSpec(ctx context.Context, specID int64) (*CDNAccelerationConfig, error) {
	site, err := s.sites.FindByInboundSpecID(ctx, specID)
	if err != nil {
		return nil, fmt.Errorf("get acceleration by inbound spec: %w", err)
	}
	return toAccelerationConfig(site), nil
}

// ListAccelerationEdges 列出加速配置的所有边缘节点
func (s *cdnService) ListAccelerationEdges(ctx context.Context, configID int64) ([]*repository.CDNEdge, error) {
	edges, err := s.edges.ListBySiteID(ctx, configID)
	if err != nil {
		return nil, fmt.Errorf("list acceleration edges: %w", err)
	}
	return edges, nil
}

// AssignAccelerationEdge 为加速配置分配一个 Agent 边缘节点
func (s *cdnService) AssignAccelerationEdge(ctx context.Context, configID, agentHostID int64, weight int) (*repository.CDNEdge, error) {
	edge := &repository.CDNEdge{
		SiteID:      configID,
		AgentHostID: agentHostID,
		Weight:      weight,
		Enabled:     true,
		Status:      "pending",
	}
	if err := s.edges.Create(ctx, edge); err != nil {
		return nil, fmt.Errorf("assign acceleration edge: %w", err)
	}
	return edge, nil
}

// RemoveAccelerationEdge 移除加速配置上的一个 Agent 边缘节点
func (s *cdnService) RemoveAccelerationEdge(ctx context.Context, edgeID int64) error {
	if err := s.edges.Delete(ctx, edgeID); err != nil {
		return fmt.Errorf("remove acceleration edge: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 一键部署/卸载加速（stub，Provider 调用暂不集成）
// ---------------------------------------------------------------------------

// DeployAcceleration 部署加速配置：
//  1. 获取加速配置和关联的 edge agent 列表
//  2. 调用 Provider 接口（stub，暂不实际调用）
//  3. 通过 command queue 向每个 Agent 下发 deploy_cdn_site
//  4. 更新部署状态
func (s *cdnService) DeployAcceleration(ctx context.Context, accelerationID int64) error {
	// 1. 获取加速配置
	site, err := s.sites.FindByID(ctx, accelerationID)
	if err != nil {
		return fmt.Errorf("deploy acceleration: find site %d: %w", accelerationID, err)
	}
	// 2. 获取关联的 edge agent 列表
	edges, err := s.edges.ListBySiteID(ctx, accelerationID)
	if err != nil {
		return fmt.Errorf("deploy acceleration: list edges: %w", err)
	}

	s.logger.Info("deploying acceleration",
		"acceleration_id", accelerationID,
		"domain", site.Domain,
		"provider", site.Provider,
		"edge_count", len(edges),
	)

	// (stub) Provider 调用：CloudFront.SyncDistribution / Cloudflare.SyncDNS
	s.logger.Info("deploy acceleration: provider sync (stub)", "provider", site.Provider)

	// 3. 通过 command queue 向每个边缘 Agent 下发 deploy_cdn_site
	if s.lifecycleOps != nil {
		for _, edge := range edges {
			payload, err := json.Marshal(map[string]any{
				"action":      AgentLifecycleOperationTypeCDNDeploySite,
				"site_id":     accelerationID,
				"domain":      site.Domain,
				"origin_type": site.OriginType,
				"origin_url":  site.OriginURL,
				"cache_ttl":   site.CacheTTL,
				"ssl_mode":    site.SSLMode,
				"edge_id":     edge.ID,
				"agent_id":    edge.AgentHostID,
			})
			if err != nil {
				s.logger.Error("marshal cdn lifecycle payload", "error", err, "action", "cdn_deploy")
				payload = json.RawMessage("{}")
			}
			_, err = s.lifecycleOps.Create(ctx, CreateAgentLifecycleOperationRequest{
				AgentHostID:    edge.AgentHostID,
				OperationType:  AgentLifecycleOperationTypeCDNDeploySite,
				RequestPayload: payload,
				Source:         "system",
			})
			if err != nil {
				s.logger.Error("deploy acceleration: create lifecycle operation",
					"edge_id", edge.ID,
					"agent_host_id", edge.AgentHostID,
					"error", err,
				)
				continue
			}
			s.logger.Info("deploy acceleration: created lifecycle operation",
				"edge_id", edge.ID,
				"agent_host_id", edge.AgentHostID,
			)
		}
	} else {
		s.logger.Warn("deploy acceleration: lifecycleOps not configured, skipping command queue")
	}

	// 4. 更新部署状态
	site.Status = "deploying"
	if err := s.sites.Update(ctx, site); err != nil {
		return fmt.Errorf("deploy acceleration: update status: %w", err)
	}

	s.logger.Info("deploy acceleration complete", "acceleration_id", accelerationID)
	return nil
}

// UndoDeployAcceleration 卸载加速配置：
//  1. 从 Provider 删除 distribution/DNS 记录（stub，暂不实际调用）
//  2. 向 Agent 下发 remove_cdn_site
//  3. 更新状态
func (s *cdnService) UndoDeployAcceleration(ctx context.Context, accelerationID int64) error {
	site, err := s.sites.FindByID(ctx, accelerationID)
	if err != nil {
		return fmt.Errorf("undo deploy acceleration: find site %d: %w", accelerationID, err)
	}
	edges, err := s.edges.ListBySiteID(ctx, accelerationID)
	if err != nil {
		return fmt.Errorf("undo deploy acceleration: list edges: %w", err)
	}

	s.logger.Info("undeploying acceleration",
		"acceleration_id", accelerationID,
		"domain", site.Domain,
		"edge_count", len(edges),
	)

	// (stub) Provider 调用：CloudFront.DeleteDistribution / Cloudflare DNS 删除
	s.logger.Info("undo deploy acceleration: provider delete (stub)", "provider", site.Provider)

	// 2. 向每个 Agent 下发 remove_cdn_site
	if s.lifecycleOps != nil {
		for _, edge := range edges {
			payload, err := json.Marshal(map[string]any{
				"action":   AgentLifecycleOperationTypeCDNRemoveSite,
				"site_id":  accelerationID,
				"domain":   site.Domain,
				"edge_id":  edge.ID,
				"agent_id": edge.AgentHostID,
			})
			if err != nil {
				s.logger.Error("marshal cdn lifecycle payload", "error", err, "action", "cdn_deploy")
				payload = json.RawMessage("{}")
			}
			_, err = s.lifecycleOps.Create(ctx, CreateAgentLifecycleOperationRequest{
				AgentHostID:    edge.AgentHostID,
				OperationType:  AgentLifecycleOperationTypeCDNRemoveSite,
				RequestPayload: payload,
				Source:         "system",
			})
			if err != nil {
				s.logger.Error("undo deploy acceleration: create lifecycle operation",
					"edge_id", edge.ID,
					"agent_host_id", edge.AgentHostID,
					"error", err,
				)
				continue
			}
			s.logger.Info("undo deploy acceleration: created lifecycle operation",
				"edge_id", edge.ID,
				"agent_host_id", edge.AgentHostID,
			)
		}
	} else {
		s.logger.Warn("undo deploy acceleration: lifecycleOps not configured, skipping command queue")
	}

	// 3. 更新状态
	site.Status = "active"
	if err := s.sites.Update(ctx, site); err != nil {
		return fmt.Errorf("undo deploy acceleration: update status: %w", err)
	}

	s.logger.Info("undo deploy acceleration complete", "acceleration_id", accelerationID)
	return nil
}

// ReportOriginLatency 上报源站延迟数据
func (s *cdnService) ReportOriginLatency(ctx context.Context, siteID int64, stack string, domain string, latencyMs int64) error {
	if s.originLatency == nil {
		s.logger.Warn("originLatency repo not configured, skipping ReportOriginLatency")
		return nil
	}
	return s.originLatency.Upsert(ctx, siteID, stack, latencyMs)
}

// GetOriginLatencies 获取所有源站延迟数据
func (s *cdnService) GetOriginLatencies(ctx context.Context) ([]*repository.CDNOriginLatency, error) {
	if s.originLatency == nil {
		s.logger.Warn("originLatency repo not configured, returning empty")
		return []*repository.CDNOriginLatency{}, nil
	}
	return s.originLatency.ListAll(ctx)
}

// GetAllOriginLatencies 获取所有源站延迟数据（别名，语义与 GetOriginLatencies 相同）
func (s *cdnService) GetAllOriginLatencies(ctx context.Context) ([]*repository.CDNOriginLatency, error) {
	return s.GetOriginLatencies(ctx)
}
