package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

var (
	ErrArtifactCompileInvalidRequest   = errors.New("service: invalid artifact compile request / artifact 编译请求无效")
	ErrArtifactCompileUnsupportedField = errors.New("service: unsupported artifact field / artifact 字段不受支持")
)

// 常量：v2ray_api 实验段默认监听地址（与 agent 端 ensureV2RayAPIConfig 兜底默认一致）。
const (
	CoreTypeSingBox       = "sing-box"
	CoreTypeXray          = "xray"
	defaultV2RayAPIListen = "127.0.0.1:19194"
)

// ArtifactCompilerService renders desired artifacts from inbound semantic specs.
type ArtifactCompilerService interface {
	RenderArtifacts(ctx context.Context, req RenderArtifactsRequest) (*RenderArtifactsResult, error)
	RenderCoreConfigs(ctx context.Context, req RenderArtifactsRequest) (*RenderArtifactsResult, error)
	DeleteArtifacts(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64) error
	GetLatestRevision(ctx context.Context, agentHostID int64, coreType string) (int64, error)
}

// RenderArtifactsRequest defines one rendering batch for host/core/revision.
type RenderArtifactsRequest struct {
	AgentHostID     int64
	CoreType        string
	DesiredRevision int64
}

// RenderedArtifactMetadata is trace metadata for one generated artifact.
type RenderedArtifactMetadata struct {
	SpecID      int64  `json:"spec_id"`
	SourceTag   string `json:"source_tag"`
	Filename    string `json:"filename"`
	ContentHash string `json:"content_hash"`
}

// ArtifactRenderWarning is a non-fatal field compatibility warning.
type ArtifactRenderWarning struct {
	CoreType string `json:"core_type"`
	SpecID   int64  `json:"spec_id"`
	Tag      string `json:"tag"`
	Field    string `json:"field"`
	Message  string `json:"message"`
}

// RenderArtifactsResult contains rendered artifact metadata and warnings.
type RenderArtifactsResult struct {
	DesiredRevision int64                      `json:"desired_revision"`
	ArtifactCount   int                        `json:"artifact_count"`
	Artifacts       []RenderedArtifactMetadata `json:"artifacts"`
	Warnings        []ArtifactRenderWarning    `json:"warnings,omitempty"`
}

// ArtifactUnsupportedFieldError indicates explicit renderer incompatibility.
type ArtifactUnsupportedFieldError struct {
	CoreType string `json:"core_type"`
	SpecID   int64  `json:"spec_id"`
	Tag      string `json:"tag"`
	Field    string `json:"field"`
	Message  string `json:"message"`
}

func (e *ArtifactUnsupportedFieldError) Error() string {
	if e == nil {
		return ErrArtifactCompileUnsupportedField.Error()
	}
	return fmt.Sprintf("%s (core=%s spec_id=%d tag=%s field=%s message=%s)",
		ErrArtifactCompileUnsupportedField.Error(),
		e.CoreType,
		e.SpecID,
		e.Tag,
		e.Field,
		e.Message,
	)
}

func (e *ArtifactUnsupportedFieldError) Is(target error) bool {
	return target == ErrArtifactCompileUnsupportedField
}

type artifactRenderer interface {
	CoreType() string
	Render(spec *repository.InboundSpec, semantic *inboundSemanticSpec, semanticObject map[string]any, coreSpecific map[string]any) (*renderedArtifact, []ArtifactRenderWarning, error)
}

type renderedArtifact struct {
	Filename string
	Content  []byte
}

type artifactCompilerService struct {
	specs           repository.InboundSpecRepository
	coreConfigItems repository.CoreConfigItemRepository
	artifacts       repository.DesiredArtifactRepository
	meshPeers       repository.AgentMeshPeerRepository
	exitNodeSets    repository.ExitNodeSetRepository
	routingPolicies repository.RoutingPolicyRepository
	relayPaths      repository.RelayPathRepository
	renderers       map[string]artifactRenderer
}

// NewArtifactCompilerService creates ArtifactCompilerService.
// relayPaths 允许为 nil（向后兼容旧调用点），nil 时跳过中继链路产物渲染。
func NewArtifactCompilerService(
	specs repository.InboundSpecRepository,
	artifacts repository.DesiredArtifactRepository,
	coreConfigItems repository.CoreConfigItemRepository,
	meshPeers repository.AgentMeshPeerRepository,
	exitNodeSets repository.ExitNodeSetRepository,
	routingPolicies repository.RoutingPolicyRepository,
	relayPaths repository.RelayPathRepository,
) ArtifactCompilerService {
	service := &artifactCompilerService{
		specs:           specs,
		artifacts:       artifacts,
		coreConfigItems: coreConfigItems,
		meshPeers:       meshPeers,
		exitNodeSets:    exitNodeSets,
		routingPolicies: routingPolicies,
		relayPaths:      relayPaths,
		renderers:       map[string]artifactRenderer{},
	}

	singBoxRenderer := newSingBoxArtifactRenderer()
	xrayRenderer := newXrayArtifactRenderer()
	service.renderers[singBoxRenderer.CoreType()] = singBoxRenderer
	service.renderers[xrayRenderer.CoreType()] = xrayRenderer

	return service
}

func (s *artifactCompilerService) RenderArtifacts(ctx context.Context, req RenderArtifactsRequest) (*RenderArtifactsResult, error) {
	if s == nil || s.specs == nil || s.artifacts == nil {
		return nil, fmt.Errorf("artifact compiler service not configured / artifact 编译服务未配置")
	}
	if req.AgentHostID <= 0 {
		return nil, fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrArtifactCompileInvalidRequest)
	}
	if req.DesiredRevision <= 0 {
		return nil, fmt.Errorf("%w (desired_revision must be greater than 0 / 必须大于 0)", ErrArtifactCompileInvalidRequest)
	}

	coreType := normalizeCoreType(req.CoreType)
	if coreType == "" {
		return nil, fmt.Errorf("%w (core_type must be sing-box or xray / 必须是 sing-box 或 xray)", ErrArtifactCompileInvalidRequest)
	}
	renderer := s.renderers[coreType]
	if renderer == nil {
		return nil, fmt.Errorf("%w (renderer not found for core_type=%s / 未找到该核心渲染器)", ErrArtifactCompileInvalidRequest, coreType)
	}

	specs, err := s.listSpecsByHostAndCore(ctx, req.AgentHostID, coreType)
	if err != nil {
		return nil, err
	}
	enabledSpecs := make([]*repository.InboundSpec, 0, len(specs))
	for _, item := range specs {
		if item == nil || !item.Enabled {
			continue
		}
		enabledSpecs = append(enabledSpecs, item)
	}
	// 注意：不因 enabledSpecs 为空而提前返回。
	// mesh 基础产物（mesh-direct/mesh-inbound/rule-sets/routing-policy）不依赖 spec，
	// 只要是 mesh 成员（存在 WG IP）就应生成——无 spec 绑定的 mesh 节点（如仅作出口/
	// 中继的 agent）也能拿到可下发的配置。若该 host 既无 spec 也不是 mesh 成员，
	// 产物为空，由上层（PrepareApplyRun）拒绝为无可交付配置（422）。

	sort.Slice(enabledSpecs, func(i, j int) bool {
		leftTag := normalizeTag(enabledSpecs[i].Tag)
		rightTag := normalizeTag(enabledSpecs[j].Tag)
		if leftTag == rightTag {
			return enabledSpecs[i].ID < enabledSpecs[j].ID
		}
		return leftTag < rightTag
	})

	artifacts := make([]*repository.DesiredArtifact, 0, len(enabledSpecs))
	metadata := make([]RenderedArtifactMetadata, 0, len(enabledSpecs))
	warnings := make([]ArtifactRenderWarning, 0)
	filenameSet := make(map[string]struct{}, len(enabledSpecs))

	for _, spec := range enabledSpecs {
		normalizedSemantic, normalizedCoreSpecific, semantic, err := validateSpecInput(coreType, spec.Tag, spec.SemanticSpec, spec.CoreSpecific)
		if err != nil {
			return nil, fmt.Errorf("validate inbound spec for render (spec_id=%d tag=%s): %w", spec.ID, spec.Tag, err)
		}

		semanticObject, err := artifactDecodeJSONObject(normalizedSemantic)
		if err != nil {
			return nil, fmt.Errorf("decode semantic_spec (spec_id=%d tag=%s): %w", spec.ID, spec.Tag, err)
		}
		coreSpecificObject, err := artifactDecodeJSONObject(normalizedCoreSpecific)
		if err != nil {
			return nil, fmt.Errorf("decode core_specific (spec_id=%d tag=%s): %w", spec.ID, spec.Tag, err)
		}

		rendered, renderedWarnings, err := renderer.Render(spec, semantic, semanticObject, coreSpecificObject)
		if err != nil {
			return nil, err
		}
		if rendered == nil {
			return nil, fmt.Errorf("renderer returned nil artifact (spec_id=%d tag=%s)", spec.ID, spec.Tag)
		}
		if strings.TrimSpace(rendered.Filename) == "" {
			return nil, fmt.Errorf("renderer returned empty filename (spec_id=%d tag=%s)", spec.ID, spec.Tag)
		}
		if !artifactValidFilename(rendered.Filename) {
			return nil, &ArtifactUnsupportedFieldError{
				CoreType: coreType,
				SpecID:   spec.ID,
				Tag:      spec.Tag,
				Field:    "filename",
				Message:  "must be a safe .json filename / 必须是安全的 .json 文件名",
			}
		}
		if _, exists := filenameSet[rendered.Filename]; exists {
			return nil, &ArtifactUnsupportedFieldError{
				CoreType: coreType,
				SpecID:   spec.ID,
				Tag:      spec.Tag,
				Field:    "filename",
				Message:  fmt.Sprintf("duplicate artifact filename in one batch: %s / 同批次文件名冲突", rendered.Filename),
			}
		}
		filenameSet[rendered.Filename] = struct{}{}

		hash := md5.Sum(rendered.Content)
		contentHash := hex.EncodeToString(hash[:])

		artifact := &repository.DesiredArtifact{
			AgentHostID:     req.AgentHostID,
			CoreType:        coreType,
			DesiredRevision: req.DesiredRevision,
			Filename:        rendered.Filename,
			SourceTag:       normalizeTag(spec.Tag),
			Content:         rendered.Content,
			ContentHash:     contentHash,
		}
		artifacts = append(artifacts, artifact)
		metadata = append(metadata, RenderedArtifactMetadata{
			SpecID:      spec.ID,
			SourceTag:   artifact.SourceTag,
			Filename:    artifact.Filename,
			ContentHash: artifact.ContentHash,
		})
		warnings = append(warnings, renderedWarnings...)
	}

	// Collect unique source tags from rendered artifacts to scope deletion
	sourceTags := make([]string, 0, len(metadata))
	seenTag := make(map[string]struct{}, len(metadata))
	for _, m := range metadata {
		if _, ok := seenTag[m.SourceTag]; !ok {
			seenTag[m.SourceTag] = struct{}{}
			sourceTags = append(sourceTags, m.SourceTag)
		}
	}

	// — 生成 mesh 出口相关的额外 artifacts（socks inbound/outbound + routing rules）—
	// 含 route.rules 的产物统一收集后按 I1 命名（文件名字典序 == 求值序）。
	meshExitArtifacts, meshRules, err := s.buildMeshExitArtifacts(ctx, req, renderer, enabledSpecs)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, meshExitArtifacts...)
	artifacts = append(artifacts, ruleArtifactsFrom(meshRules, req.AgentHostID, coreType, req.DesiredRevision)...)

	// v2ray_api 实验段：spec.core_specific.sing-box.v2ray_api.enabled=true 时生成
	// experimental-v2ray-api.json 独立 fragment（与 mesh-*.json 同级）。
	// 该 fragment 由编译器产物为准；agent 端 ensureV2RayAPIConfig 兜底（listen 默认一致）
	// 不会重复，因为 fragment 文件名不同（experimental-v2ray-api.json vs experimental_v2ray.json）。
	v2rayArtifacts, v2rayErr := buildV2RayAPIFragmentArtifacts(ctx, req, coreType, enabledSpecs)
	if v2rayErr != nil {
		return nil, v2rayErr
	}
	artifacts = append(artifacts, v2rayArtifacts...)

	// 中继链路产物已改为 mark 化：入口出站+路由规则由 buildRelaySpecRouting 在 spec 绑定时生成，
	// 物理路径由 agent 侧 RelayRouteManager 下发的 nftables 策略路由实现。

	if _, err := s.artifacts.ReplaceRevision(ctx, req.AgentHostID, coreType, req.DesiredRevision, artifacts, sourceTags...); err != nil {
		return nil, fmt.Errorf("replace artifacts: %w", err)
	}
	// 回收孤儿产物：产物替换是按 source_tag/filename 局部进行的，删除策略 / 出口集 / 规格后
	// 其规则与出站产物不会被任何替换覆盖，会继续留在该修订号内并下发到节点生效
	// （生产实测：已删除的 policy-1 仍在 hy-e5 上把 netflix 分流到旧 socks 出口）。
	if orphans := s.orphanCompilerArtifacts(ctx, req.AgentHostID, coreType, req.DesiredRevision, artifacts); len(orphans) > 0 {
		deleted, derr := s.artifacts.DeleteByFilenames(ctx, req.AgentHostID, coreType, req.DesiredRevision, orphans...)
		if derr != nil {
			return nil, fmt.Errorf("delete orphan artifacts: %w", derr)
		}
		slog.Info("compiler: orphan artifacts reclaimed",
			"agent_host_id", req.AgentHostID, "core_type", coreType, "revision", req.DesiredRevision,
			"deleted", deleted, "orphans", strings.Join(orphans, ","))
	}

	return &RenderArtifactsResult{
		DesiredRevision: req.DesiredRevision,
		ArtifactCount:   len(artifacts),
		Artifacts:       metadata,
		Warnings:        warnings,
	}, nil
}

func (s *artifactCompilerService) DeleteArtifacts(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64) error {
	if s == nil || s.artifacts == nil {
		return fmt.Errorf("artifact compiler service not configured / artifact 编译服务未配置")
	}
	normalizedCore := normalizeCoreType(coreType)
	if normalizedCore == "" {
		return fmt.Errorf("%w (core_type must be sing-box or xray / 必须是 sing-box 或 xray)", ErrArtifactCompileInvalidRequest)
	}
	if agentHostID <= 0 {
		return fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrArtifactCompileInvalidRequest)
	}
	if desiredRevision <= 0 {
		return fmt.Errorf("%w (desired_revision must be greater than 0 / 必须大于 0)", ErrArtifactCompileInvalidRequest)
	}
	return s.artifacts.DeleteByHostCoreRevision(ctx, agentHostID, normalizedCore, desiredRevision)
}

// RenderCoreConfigs renders core config items (outbound, routing, dns, core_settings) into artifacts.
func (s *artifactCompilerService) RenderCoreConfigs(ctx context.Context, req RenderArtifactsRequest) (*RenderArtifactsResult, error) {
	if s == nil || s.coreConfigItems == nil || s.artifacts == nil {
		return nil, fmt.Errorf("artifact compiler service not configured / artifact 编译服务未配置")
	}
	if req.AgentHostID <= 0 {
		return nil, fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrArtifactCompileInvalidRequest)
	}

	coreType := normalizeCoreType(req.CoreType)
	if coreType == "" {
		return nil, fmt.Errorf("%w (core_type must be sing-box or xray / 必须是 sing-box 或 xray)", ErrArtifactCompileInvalidRequest)
	}

	// Use the requested revision if provided, otherwise compute one
	revision := req.DesiredRevision
	if revision <= 0 {
		latest, err := s.artifacts.GetLatestRevision(ctx, req.AgentHostID, coreType)
		if err != nil {
			return nil, fmt.Errorf("get latest revision: %w", err)
		}
		if latest <= 0 {
			revision = 1
		} else {
			revision = latest + 1
		}
	}

	// List all enabled core config items for this host
	filter := repository.CoreConfigItemFilter{
		AgentHostID: &req.AgentHostID,
		CoreType:    &coreType,
		Enabled:     boolPtr(true),
		Limit:       1000,
		Offset:      0,
	}
	items, err := s.coreConfigItems.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list core config items: %w", err)
	}

	artifacts := make([]*repository.DesiredArtifact, 0, len(items))
	metadata := make([]RenderedArtifactMetadata, 0, len(items))

	for _, item := range items {
		if item == nil {
			continue
		}

		// Serialize config_data to JSON bytes
		configDataBytes, err := json.Marshal(item.ConfigData)
		if err != nil {
			return nil, fmt.Errorf("marshal core config item (id=%d tag=%s): %w", item.ID, item.Tag, err)
		}

		// Build filename
		filename := fmt.Sprintf("core-%s-%s.json", item.ConfigType, item.Tag)

		// Compute content hash
		hash := md5.Sum(configDataBytes)

		artifact := &repository.DesiredArtifact{
			AgentHostID:     req.AgentHostID,
			CoreType:        coreType,
			DesiredRevision: revision,
			Filename:        filename,
			SourceTag:       item.Tag,
			Content:         configDataBytes,
			ContentHash:     hex.EncodeToString(hash[:]),
		}

		artifacts = append(artifacts, artifact)
		metadata = append(metadata, RenderedArtifactMetadata{
			SpecID:      item.ID,
			SourceTag:   item.Tag,
			Filename:    filename,
			ContentHash: hex.EncodeToString(hash[:]),
		})
	}

	if len(artifacts) == 0 {
		return &RenderArtifactsResult{
			DesiredRevision: revision,
			ArtifactCount:   0,
			Artifacts:       nil,
			Warnings:        nil,
		}, nil
	}

	// Atomically replace artifacts at same revision (single transaction)
	// Collect unique source tags from rendered artifacts to scope deletion
	sourceTags := make([]string, 0, len(metadata))
	seenTag := make(map[string]struct{}, len(metadata))
	for _, m := range metadata {
		if _, ok := seenTag[m.SourceTag]; !ok {
			seenTag[m.SourceTag] = struct{}{}
			sourceTags = append(sourceTags, m.SourceTag)
		}
	}
	if _, err := s.artifacts.ReplaceRevision(ctx, req.AgentHostID, coreType, revision, artifacts, sourceTags...); err != nil {
		return nil, fmt.Errorf("replace existing artifacts: %w", err)
	}

	return &RenderArtifactsResult{
		DesiredRevision: revision,
		ArtifactCount:   len(artifacts),
		Artifacts:       metadata,
	}, nil
}

// compilerArtifactPrefixes 编译器产物文件名前缀。核心配置项产物由 RenderCoreConfigs 生成，
// 前缀固定为 core-<configType>-<tag>.json，因此按前缀即可安全区分归属，互不误删。
var compilerArtifactPrefixes = []string{"mesh-", "route-", "inbound-", "relay-", "experimental-"}

// orphanCompilerArtifacts 返回该修订号下「属于编译器命名空间、但本次渲染已不再产出」的孤儿文件名。
func (s *artifactCompilerService) orphanCompilerArtifacts(
	ctx context.Context,
	agentHostID int64,
	coreType string,
	revision int64,
	fresh []*repository.DesiredArtifact,
) []string {
	if s == nil || s.artifacts == nil {
		return nil
	}
	existing, err := s.artifacts.List(ctx, repository.DesiredArtifactFilter{
		AgentHostID:     agentHostID,
		CoreType:        &coreType,
		DesiredRevision: &revision,
		Limit:           1000,
	})
	if err != nil {
		slog.Warn("compiler: list artifacts for orphan sweep failed",
			"agent_host_id", agentHostID, "revision", revision, "error", err)
		return nil
	}
	freshNames := make(map[string]struct{}, len(fresh))
	for _, a := range fresh {
		if a != nil && a.Filename != "" {
			freshNames[a.Filename] = struct{}{}
		}
	}
	var orphans []string
	for _, a := range existing {
		if a == nil || a.Filename == "" {
			continue
		}
		if _, ok := freshNames[a.Filename]; ok {
			continue
		}
		if !hasCompilerArtifactPrefix(a.Filename) {
			continue
		}
		orphans = append(orphans, a.Filename)
	}
	return orphans
}

// hasCompilerArtifactPrefix 判断文件名是否属于编译器产物命名空间。
func hasCompilerArtifactPrefix(filename string) bool {
	for _, prefix := range compilerArtifactPrefixes {
		if strings.HasPrefix(filename, prefix) {
			return true
		}
	}
	return false
}

// GetLatestRevision returns the highest desired_revision seen for a host/core_type.
func (s *artifactCompilerService) GetLatestRevision(ctx context.Context, agentHostID int64, coreType string) (int64, error) {
	if s == nil || s.artifacts == nil {
		return 0, fmt.Errorf("artifact compiler service not configured / artifact 编译服务未配置")
	}
	normalizedCore := normalizeCoreType(coreType)
	if normalizedCore == "" {
		return 0, fmt.Errorf("%w (core_type must be sing-box or xray / 必须是 sing-box 或 xray)", ErrArtifactCompileInvalidRequest)
	}
	return s.artifacts.GetLatestRevision(ctx, agentHostID, normalizedCore)
}

func (s *artifactCompilerService) listSpecsByHostAndCore(ctx context.Context, agentHostID int64, coreType string) ([]*repository.InboundSpec, error) {
	limit := 200
	offset := 0
	all := make([]*repository.InboundSpec, 0)

	for {
		items, err := s.specs.ListByAgentHost(ctx, agentHostID, repository.InboundSpecFilter{
			CoreType: &coreType,
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		all = append(all, items...)
		if len(items) < limit {
			break
		}
		offset += limit
	}

	return all, nil
}

var artifactFilenameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
