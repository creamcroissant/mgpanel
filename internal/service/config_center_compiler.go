package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/template"
)

var (
	ErrArtifactCompileInvalidRequest   = errors.New("service: invalid artifact compile request / artifact 编译请求无效")
	ErrArtifactCompileUnsupportedField = errors.New("service: unsupported artifact field / artifact 字段不受支持")
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
	renderers       map[string]artifactRenderer
}

// NewArtifactCompilerService creates ArtifactCompilerService.
func NewArtifactCompilerService(
	specs repository.InboundSpecRepository,
	artifacts repository.DesiredArtifactRepository,
	coreConfigItems repository.CoreConfigItemRepository,
	meshPeers repository.AgentMeshPeerRepository,
	exitNodeSets repository.ExitNodeSetRepository,
	routingPolicies repository.RoutingPolicyRepository,
) ArtifactCompilerService {
	service := &artifactCompilerService{
		specs:           specs,
		artifacts:       artifacts,
		coreConfigItems: coreConfigItems,
		meshPeers:       meshPeers,
		exitNodeSets:    exitNodeSets,
		routingPolicies: routingPolicies,
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
	if len(enabledSpecs) == 0 {
		return nil, fmt.Errorf("%w (no enabled inbound specs found / 未找到启用的入站配置)", ErrArtifactCompileInvalidRequest)
	}

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
	meshExitArtifacts, err := s.buildMeshExitArtifacts(ctx, req, renderer, enabledSpecs)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, meshExitArtifacts...)

	if err := s.artifacts.DeleteByHostCoreRevision(ctx, req.AgentHostID, coreType, req.DesiredRevision, sourceTags...); err != nil {
		return nil, err
	}
	if err := s.artifacts.CreateBatch(ctx, artifacts); err != nil {
		return nil, err
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

	// Delete old artifacts at same revision then create new ones
	// Collect unique source tags from rendered artifacts to scope deletion
	sourceTags := make([]string, 0, len(metadata))
	seenTag := make(map[string]struct{}, len(metadata))
	for _, m := range metadata {
		if _, ok := seenTag[m.SourceTag]; !ok {
			seenTag[m.SourceTag] = struct{}{}
			sourceTags = append(sourceTags, m.SourceTag)
		}
	}
	if err := s.artifacts.DeleteByHostCoreRevision(ctx, req.AgentHostID, coreType, revision, sourceTags...); err != nil {
		return nil, fmt.Errorf("delete existing artifacts: %w", err)
	}
	if err := s.artifacts.CreateBatch(ctx, artifacts); err != nil {
		return nil, fmt.Errorf("create artifacts: %w", err)
	}

	return &RenderArtifactsResult{
		DesiredRevision: revision,
		ArtifactCount:   len(artifacts),
		Artifacts:       metadata,
	}, nil
}

// buildMeshExitArtifacts 为 mesh 网络中的 agent 生成 socks inbound/outbound + routing rule 等附加 artifacts。
func (s *artifactCompilerService) buildMeshExitArtifacts(
	ctx context.Context,
	req RenderArtifactsRequest,
	renderer artifactRenderer,
	enabledSpecs []*repository.InboundSpec,
) ([]*repository.DesiredArtifact, error) {
	if s.meshPeers == nil {
		return nil, nil
	}

	// 查询本 agent 的 mesh peer 信息（用于本机 WG IP）
	ownPeer, err := s.meshPeers.FindByAgentHostID(ctx, req.AgentHostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil // 本 agent 不在 mesh 中，跳过
		}
		return nil, fmt.Errorf("mesh peer lookup: %w", err)
	}

	// 查询所有 mesh peer，构建 agentHostID → WG IP 映射
	allPeers, err := s.meshPeers.ListByNetworkID(ctx, "default")
	if err != nil {
		return nil, fmt.Errorf("list mesh peers: %w", err)
	}
	peerWGIP := make(map[int64]string, len(allPeers))
	for _, p := range allPeers {
		peerWGIP[p.AgentHostID] = p.WGIP
	}

	coreType := normalizeCoreType(req.CoreType)
	var artifacts []*repository.DesiredArtifact

	// direct outbound：供 selector 兜底 + remote rule_set 下载直连使用，统一生成一次
	directContent := renderMeshDirectOutbound(coreType)
	artifacts = append(artifacts, &repository.DesiredArtifact{
		AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
		Filename: "mesh-direct.json", SourceTag: "direct",
		Content: directContent, ContentHash: artifactHash(directContent),
	})

	// 1. socks inbound：本 agent 监听 WG IP:1080，作为 mesh 出口服务
	//    所有 mesh agent 都生成，确保能作为出口。
	inboundTag := "mesh-inbound"
	inboundContent := renderMeshSocksInbound(coreType, ownPeer.WGIP, 1080, inboundTag)
	artifacts = append(artifacts, &repository.DesiredArtifact{
		AgentHostID:     req.AgentHostID,
		CoreType:        coreType,
		DesiredRevision: req.DesiredRevision,
		Filename:        "mesh-inbound.json",
		SourceTag:       inboundTag,
		Content:         inboundContent,
		ContentHash:     artifactHash(inboundContent),
	})

	// 2. 对每个 spec 生成出口 outbound + routing rule
	//    优先 exit_node_set_id（出口集合），其次 exit_agent_host_id（固定出口）
	setMemberOutboundGen := map[int64]struct{}{} // 避免重复生成成员 socks outbound

	for _, spec := range enabledSpecs {
		specTag := normalizeTag(spec.Tag)

		// 2a. exit_node_set_id：出口集合（负载均衡+故障转移）
		if spec.ExitNodeSetID != nil && *spec.ExitNodeSetID > 0 {
			var err error
			artifacts, err = s.buildMeshExitSetArtifacts(ctx, req, coreType, specTag, *spec.ExitNodeSetID, peerWGIP, setMemberOutboundGen, artifacts)
			if err != nil {
				return nil, fmt.Errorf("build exit set artifacts for spec %s: %w", specTag, err)
			}
			continue
		}

		// 2b. exit_agent_host_id：固定出口（原有逻辑）
		if spec.ExitAgentHostID == nil || *spec.ExitAgentHostID <= 0 {
			continue
		}
		exitID := *spec.ExitAgentHostID
		exitWGIP, ok := peerWGIP[exitID]
		if !ok {
			continue // 出口 agent 不在 mesh 中，跳过
		}
		if exitID == req.AgentHostID {
			continue // 出口节点就是本机，无需隧道
		}

		exitTag := fmt.Sprintf("mesh-exit-%d", exitID)

		// socks outbound：指向出口 agent 的 WG IP:1080
		outboundContent := renderMeshSocksOutbound(coreType, exitWGIP, 1080, exitTag)
		outboundFilename := fmt.Sprintf("mesh-%s-exit.json", specTag)
		artifacts = append(artifacts, &repository.DesiredArtifact{
			AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
			Filename: outboundFilename, SourceTag: specTag + "-exit",
			Content: outboundContent, ContentHash: artifactHash(outboundContent),
		})

		// routing rule：该 inbound 的流量 → 出口 outbound
		routingContent := renderMeshRoutingRule(coreType, specTag, exitTag)
		routingFilename := fmt.Sprintf("mesh-%s-routing.json", specTag)
		artifacts = append(artifacts, &repository.DesiredArtifact{
			AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
			Filename: routingFilename, SourceTag: specTag + "-routing",
			Content: routingContent, ContentHash: artifactHash(routingContent),
		})
	}

	// 3. 应用 routing policies（geosite/domain → 出口集合的自动分流）
	//    路由策略不依赖具体 spec 的出口设置：按 core_type 查询全局启用的策略，
	//    每个策略生成一条 route rule，把匹配流量路由到对应出口集合的 selector outbound。
	if s.routingPolicies != nil {
		policies, err := s.routingPolicies.ListEnabledByCore(ctx, coreType)
		if err != nil {
			return nil, fmt.Errorf("list routing policies: %w", err)
		}
		// 为 geosite 策略生成 rule_set 定义（sing-box）
		if ruleSetContent := buildMeshRoutingPolicyRuleSet(coreType, policies); len(ruleSetContent) > 0 {
			artifacts = append(artifacts, &repository.DesiredArtifact{
				AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
				Filename: "mesh-rule-sets.json", SourceTag: "rule-sets",
				Content: ruleSetContent, ContentHash: artifactHash(ruleSetContent),
			})
		}
		for _, p := range policies {
			if p.TargetSetID == nil || *p.TargetSetID <= 0 {
				continue
			}
			ruleContent := renderRoutingPolicyRule(coreType, p)
			if len(ruleContent) == 0 {
				continue
			}
			filename := fmt.Sprintf("mesh-policy-%d.json", p.ID)
			artifacts = append(artifacts, &repository.DesiredArtifact{
				AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
				Filename: filename, SourceTag: fmt.Sprintf("policy-%d", p.ID),
				Content: ruleContent, ContentHash: artifactHash(ruleContent),
			})
		}
	}

	return artifacts, nil
}

// buildMeshExitSetArtifacts 为出口集合（ExitNodeSetID）生成 
// selector outbound + 成员 socks outbound + routing rule。
func (s *artifactCompilerService) buildMeshExitSetArtifacts(
	ctx context.Context,
	req RenderArtifactsRequest,
	coreType, specTag string,
	setID int64,
	peerWGIP map[int64]string,
	setMemberOutboundGen map[int64]struct{},
	artifacts []*repository.DesiredArtifact,
) ([]*repository.DesiredArtifact, error) {
	if s.exitNodeSets == nil {
		return artifacts, nil
	}
	members, err := s.exitNodeSets.ListMembers(ctx, setID)
	if err != nil {
		return artifacts, err
	}
	if len(members) == 0 {
		return artifacts, nil
	}

	var memberOutboundTags []string
	for _, m := range members {
		if !m.Enabled {
			continue
		}
		wgIP, ok := peerWGIP[m.AgentHostID]
		if !ok {
			continue // 该成员不在 mesh 中，跳过
		}
		if m.AgentHostID == req.AgentHostID {
			continue // 跳过自身
		}
		exitTag := fmt.Sprintf("mesh-exit-%d", m.AgentHostID)
		memberOutboundTags = append(memberOutboundTags, exitTag)

		// 每个成员只生成一次 socks outbound（可被多个集共享）
		if _, gen := setMemberOutboundGen[m.AgentHostID]; gen {
			continue
		}
		setMemberOutboundGen[m.AgentHostID] = struct{}{}

		outboundContent := renderMeshSocksOutbound(coreType, wgIP, 1080, exitTag)
		artifacts = append(artifacts, &repository.DesiredArtifact{
			AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
			Filename: fmt.Sprintf("mesh-exit-%d.json", m.AgentHostID), SourceTag: fmt.Sprintf("exit-%d", m.AgentHostID),
			Content: outboundContent, ContentHash: artifactHash(outboundContent),
		})
	}

	if len(memberOutboundTags) == 0 {
		return artifacts, nil
	}

	// 将本地 direct 作为最后兜底（mesh 出口全部不可达时自动回落本机直连）
	selectorMembers := append([]string{}, memberOutboundTags...)
	selectorMembers = append(selectorMembers, "direct")

	// 生成 selector outbound（sing-box 的 selector 类型）
	setTag := fmt.Sprintf("mesh-exit-set-%d", setID)
	selectorContent := renderMeshSelectorOutbound(coreType, setTag, selectorMembers)
	artifacts = append(artifacts, &repository.DesiredArtifact{
		AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
		Filename: fmt.Sprintf("mesh-%s-set.json", specTag), SourceTag: specTag + "-set",
		Content: selectorContent, ContentHash: artifactHash(selectorContent),
	})

	// 生成 routing rule → selector outbound
	routingContent := renderMeshRoutingRule(coreType, specTag, setTag)
	artifacts = append(artifacts, &repository.DesiredArtifact{
		AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
		Filename: fmt.Sprintf("mesh-%s-routing.json", specTag), SourceTag: specTag + "-routing",
		Content: routingContent, ContentHash: artifactHash(routingContent),
	})

	return artifacts, nil
}

// renderMeshSocksInbound 生成 sing-box / Xray 的 socks inbound 配置（监听 WG IP:port）。
func renderMeshSocksInbound(coreType, listenIP string, port int, tag string) []byte {
	if coreType == "xray" {
		data := map[string]any{
			"inbounds": []any{
				map[string]any{
					"protocol": "socks",
					"tag":      tag,
					"listen":   listenIP,
					"port":     port,
					"settings": map[string]any{
						"udp":  false,
						"auth": "noauth",
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	// sing-box
	data := map[string]any{
		"inbounds": []any{
			map[string]any{
				"type":        "socks",
				"tag":         tag,
				"listen":      listenIP,
				"listen_port": port,
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshSocksOutbound 生成 sing-box / Xray 的 socks outbound 配置。
func renderMeshSocksOutbound(coreType, serverIP string, port int, tag string) []byte {
	if coreType == "xray" {
		data := map[string]any{
			"outbounds": []any{
				map[string]any{
					"protocol": "socks",
					"tag":      tag,
					"settings": map[string]any{
						"servers": []any{
							map[string]any{
								"address": serverIP,
								"port":    port,
							},
						},
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	// sing-box
	data := map[string]any{
		"outbounds": []any{
			map[string]any{
				"type":        "socks",
				"tag":         tag,
				"server":      serverIP,
				"server_port": port,
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshRoutingRule 生成 sing-box / Xray 的路由规则，将指定 inbound tag 的流量路由到出口 outbound。
func renderMeshRoutingRule(coreType, inboundTag, outboundTag string) []byte {
	if coreType == "xray" {
		data := map[string]any{
			"routing": map[string]any{
				"rules": []any{
					map[string]any{
						"type":       "field",
						"inboundTag": []string{inboundTag},
						"outboundTag": outboundTag,
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	// sing-box
	data := map[string]any{
		"route": map[string]any{
			"rules": []any{
				map[string]any{
					"inbound":  []string{inboundTag},
					"outbound": outboundTag,
				},
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderRoutingPolicyRule 根据路由策略生成 sing-box / Xray 的 route rule。
// matchType: geosite / domain / ip_cidr，匹配成功 → outbound 到目标出口集合的 selector。
func renderRoutingPolicyRule(coreType string, p *repository.RoutingPolicy) []byte {
	if p == nil || p.TargetSetID == nil {
		return nil
	}
	outboundTag := fmt.Sprintf("mesh-exit-set-%d", *p.TargetSetID)

	if coreType == "xray" {
		// Xray routing rule: type=field + domain 匹配
		var matchField []string
		switch p.MatchType {
		case "domain":
			matchField = []string{"domain:" + p.MatchValue}
		case "ip_cidr":
			matchField = []string{"ip:" + p.MatchValue}
		default: // geosite
			matchField = []string{"geosite:" + p.MatchValue}
		}
		data := map[string]any{
			"routing": map[string]any{
				"rules": []any{
					map[string]any{
						"type":        "field",
						"domain":      matchField,
						"outboundTag": outboundTag,
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}

	// sing-box route rule
	var rule map[string]any
	switch p.MatchType {
	case "domain":
		rule = map[string]any{"domain_suffix": []string{p.MatchValue}, "outbound": outboundTag}
	case "ip_cidr":
		rule = map[string]any{"ip_cidr": []string{p.MatchValue}, "outbound": outboundTag}
	default: // geosite
		// sing-box 的 geosite 需要通过 rule_set 引用。
		// 生成一条 rule_set 引用规则 + 自动注入 rule_set 定义。
		// rule_set 定义在调用方通过 buildMeshRoutingPolicyRuleSet 生成。
		rule = map[string]any{"rule_set": []string{fmt.Sprintf("geosite-%s", p.MatchValue)}, "outbound": outboundTag}
	}
	data := map[string]any{
		"route": map[string]any{
			"rules": []any{rule},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// buildMeshRoutingPolicyRuleSet 为所有 geosite 类型的 routing policy 生成 rule_set 定义。
// 每个 rule_set 指向社区维护的 sing-box 规则集 URL。
func buildMeshRoutingPolicyRuleSet(coreType string, policies []*repository.RoutingPolicy) []byte {
	if coreType == "xray" {
		return nil // Xray 不需要 rule_set
	}
	ruleSets := make([]map[string]any, 0)
	for _, p := range policies {
		if p.MatchType != "geosite" || p.TargetSetID == nil {
			continue
		}
		ruleSets = append(ruleSets, map[string]any{
			"type": "remote",
			"tag":  fmt.Sprintf("geosite-%s", p.MatchValue),
			"url":  fmt.Sprintf("https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-%s.srs", p.MatchValue),
			"download_detour": "direct",
		})
	}
	if len(ruleSets) == 0 {
		return nil
	}
	data := map[string]any{
		"route": map[string]any{
			"rule_set": ruleSets,
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshDirectOutbound 生成 sing-box / Xray 的 direct outbound（用于 rule_set 直连下载）。
func renderMeshDirectOutbound(coreType string) []byte {
	if coreType == "xray" {
		data := map[string]any{
			"outbounds": []any{
				map[string]any{"protocol": "freedom", "tag": "direct"},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	data := map[string]any{
		"outbounds": []any{
			map[string]any{"type": "direct", "tag": "direct"},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshSelectorOutbound 生成 sing-box / Xray 的 selector outbound。
// sing-box: type=selector（自动故障转移+负载均衡）；Xray: balancing（fallback）。
func renderMeshSelectorOutbound(coreType, tag string, memberOutboundTags []string) []byte {	if coreType == "xray" {
		// Xray 使用 balancing 策略组做负载均衡 + balancerSettings 做故障转移
		data := map[string]any{
			"routing": map[string]any{
				"balancers": []any{
					map[string]any{
						"tag":        tag,
						"selector":   memberOutboundTags,
						"strategy":   map[string]any{"type": "roundRobin"},
						"fallbackTag": firstOrEmpty(memberOutboundTags),
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	// sing-box: selector outbound，URL 测试自动选可达节点，天然支持故障转移
	data := map[string]any{
		"outbounds": []any{
			map[string]any{
				"type":        "selector",
				"tag":         tag,
				"outbounds":   memberOutboundTags,
				"default":     firstOrEmpty(memberOutboundTags),
				"interrupt_exist_connections": false,
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// firstOrEmpty 返回切片第一个元素，空则返回空串。
func firstOrEmpty(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func artifactHash(content []byte) string {
	h := md5.Sum(content)
	return hex.EncodeToString(h[:])
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

func buildUnifiedInboundFromSemantic(tag string, semantic *inboundSemanticSpec, semanticObject map[string]any) (template.UnifiedInbound, error) {
	if semantic == nil {
		return template.UnifiedInbound{}, fmt.Errorf("semantic spec is nil")
	}

	inbound := template.UnifiedInbound{
		Tag:      normalizeTag(firstNonEmpty(tag, semantic.Tag)),
		Protocol: strings.TrimSpace(semantic.Protocol),
		Listen:   normalizeListen(semantic.Listen),
		Port:     semantic.Port,
	}

	if artifactRawObjectPresent(semantic.TLS) {
		tlsObject, err := artifactDecodeJSONObject(semantic.TLS)
		if err != nil {
			return template.UnifiedInbound{}, fmt.Errorf("parse semantic_spec.tls: %w", err)
		}
		inbound.TLS = artifactBuildUnifiedTLS(tlsObject)
	}
	if artifactRawObjectPresent(semantic.Transport) {
		transportObject, err := artifactDecodeJSONObject(semantic.Transport)
		if err != nil {
			return template.UnifiedInbound{}, fmt.Errorf("parse semantic_spec.transport: %w", err)
		}
		inbound.Transport = artifactBuildUnifiedTransport(transportObject)
	}
	if artifactRawObjectPresent(semantic.Multiplex) {
		multiplexObject, err := artifactDecodeJSONObject(semantic.Multiplex)
		if err != nil {
			return template.UnifiedInbound{}, fmt.Errorf("parse semantic_spec.multiplex: %w", err)
		}
		inbound.Multiplex = artifactBuildUnifiedMultiplex(multiplexObject)
	}
	if artifactRawObjectPresent(semantic.Sniffing) {
		sniffingObject, err := artifactDecodeJSONObject(semantic.Sniffing)
		if err != nil {
			return template.UnifiedInbound{}, fmt.Errorf("parse semantic_spec.sniffing: %w", err)
		}
		inbound.Sniffing = artifactBuildUnifiedSniffing(sniffingObject)
	}

	// Extract users and options from raw semanticObject (data not in typed struct fields)
	if semanticObject != nil {
		inbound.Users = artifactBuildUnifiedUsers(semanticObject)
		if optionsRaw, ok := semanticObject["options"]; ok {
			if optionsMap, ok := optionsRaw.(map[string]any); ok {
				inbound.Options = optionsMap
			}
		}
	}

	return inbound, nil
}

func artifactBuildUnifiedTLS(raw map[string]any) *template.UnifiedTLS {
	if len(raw) == 0 {
		return nil
	}
	enabledValue, enabledFound := artifactLookupFirst(raw, "enabled")
	enabled := true
	if enabledFound {
		enabled = artifactToBool(enabledValue)
	}

	tls := &template.UnifiedTLS{
		Enabled:    enabled,
		ServerName: artifactStringByKeys(raw, "server_name", "serverName"),
		ALPN:       artifactStringSliceByKeys(raw, "alpn", "ALPN"),
		CertPath:   artifactStringByKeys(raw, "cert_path", "certPath", "certificateFile"),
		KeyPath:    artifactStringByKeys(raw, "key_path", "keyPath", "keyFile"),
	}

	realityRaw, ok := artifactMapByKeys(raw, "reality", "reality_settings", "realitySettings")
	if ok {
		reality := &template.UnifiedReality{
			Enabled:         artifactToBoolWithDefault(artifactLookupFirstValue(realityRaw, "enabled"), true),
			PrivateKey:      artifactStringByKeys(realityRaw, "private_key", "privateKey"),
			PublicKey:       artifactStringByKeys(realityRaw, "public_key", "publicKey"),
			ShortIDs:        artifactStringSliceByKeys(realityRaw, "short_ids", "shortIds", "short_id", "shortId"),
			ServerNames:     artifactStringSliceByKeys(realityRaw, "server_names", "serverNames"),
			HandshakeServer: artifactStringByKeys(realityRaw, "handshake_server", "handshakeServer"),
			HandshakePort:   artifactIntByKeys(realityRaw, "handshake_port", "handshakePort"),
			Fingerprint:     artifactStringByKeys(realityRaw, "fingerprint"),
		}

		if len(reality.ServerNames) == 0 {
			singleServerName := artifactStringByKeys(realityRaw, "server_name", "serverName")
			if singleServerName != "" {
				reality.ServerNames = []string{singleServerName}
			}
		}
		if reality.HandshakeServer == "" {
			if dest := artifactStringByKeys(realityRaw, "dest"); dest != "" {
				host, port := artifactSplitHostPort(dest)
				reality.HandshakeServer = host
				if reality.HandshakePort == 0 {
					reality.HandshakePort = port
				}
			}
		}

		tls.Reality = reality
	}

	if !tls.Enabled && tls.ServerName == "" && len(tls.ALPN) == 0 && tls.CertPath == "" && tls.KeyPath == "" && tls.Reality == nil {
		return nil
	}
	return tls
}

func artifactBuildUnifiedTransport(raw map[string]any) *template.UnifiedTransport {
	if len(raw) == 0 {
		return nil
	}

	transport := &template.UnifiedTransport{
		Type:        strings.TrimSpace(artifactStringByKeys(raw, "type", "network")),
		Path:        artifactStringByKeys(raw, "path"),
		Host:        artifactStringByKeys(raw, "host"),
		ServiceName: artifactStringByKeys(raw, "service_name", "serviceName"),
		Headers:     artifactStringMapByKeys(raw, "headers"),
	}
	if transport.Type == "" {
		transport.Type = "tcp"
	}
	if transport.Host == "" {
		hostCandidates := artifactStringSliceByKeys(raw, "host")
		if len(hostCandidates) > 0 {
			transport.Host = hostCandidates[0]
		}
	}
	if len(transport.Headers) == 0 {
		transport.Headers = nil
	}

	return transport
}

func artifactSingleInboundFromPayload(payload []byte) (map[string]any, error) {
	var document map[string]any
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil, err
	}
	inboundsRaw, ok := document["inbounds"]
	if !ok {
		return nil, fmt.Errorf("render payload missing inbounds")
	}
	inboundsArray, ok := inboundsRaw.([]any)
	if !ok || len(inboundsArray) != 1 {
		return nil, fmt.Errorf("render payload must contain exactly one inbound")
	}
	inbound, ok := artifactToMap(inboundsArray[0])
	if !ok {
		return nil, fmt.Errorf("render payload inbound must be object")
	}
	return inbound, nil
}

func artifactMarshalInbound(inbound map[string]any) ([]byte, error) {
	document := map[string]any{
		"inbounds": []map[string]any{inbound},
	}
	return json.Marshal(document)
}

func artifactDecodeJSONObject(raw []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		object = map[string]any{}
	}
	return object, nil
}

func artifactRawObjectPresent(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	return string(trimmed) != "{}"
}

func artifactSemanticUnknownWarnings(coreType string, spec *repository.InboundSpec, semanticObject map[string]any, supported map[string]struct{}) []ArtifactRenderWarning {
	if len(semanticObject) == 0 {
		return nil
	}
	keys := make([]string, 0)
	for key := range semanticObject {
		if _, ok := supported[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	warnings := make([]ArtifactRenderWarning, 0, len(keys))
	for _, key := range keys {
		warnings = append(warnings, ArtifactRenderWarning{
			CoreType: coreType,
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    fmt.Sprintf("semantic_spec.%s", key),
			Message:  "field is unsupported and ignored / 字段不受支持，已忽略",
		})
	}
	return warnings
}

func artifactExtractCoreSection(spec *repository.InboundSpec, coreType string, coreSpecific map[string]any, coreKeys ...string) (map[string]any, []ArtifactRenderWarning, error) {
	warnings := make([]ArtifactRenderWarning, 0)
	if len(coreSpecific) == 0 {
		return nil, warnings, nil
	}

	knownTop := map[string]struct{}{
		"core_type": {},
	}
	for _, key := range coreKeys {
		knownTop[key] = struct{}{}
	}

	unknownTop := make([]string, 0)
	for key := range coreSpecific {
		if _, ok := knownTop[key]; ok {
			continue
		}
		unknownTop = append(unknownTop, key)
	}
	sort.Strings(unknownTop)
	for _, key := range unknownTop {
		warnings = append(warnings, ArtifactRenderWarning{
			CoreType: coreType,
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    fmt.Sprintf("core_specific.%s", key),
			Message:  "field is unsupported and ignored / 字段不受支持，已忽略",
		})
	}

	foundKeys := make([]string, 0, 1)
	for _, key := range coreKeys {
		if _, ok := coreSpecific[key]; ok {
			foundKeys = append(foundKeys, key)
		}
	}

	if len(foundKeys) == 0 {
		return nil, warnings, nil
	}
	if len(foundKeys) > 1 {
		return nil, warnings, &ArtifactUnsupportedFieldError{
			CoreType: coreType,
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    "core_specific",
			Message:  fmt.Sprintf("ambiguous core section keys: %s / 核心扩展键冲突", strings.Join(foundKeys, ",")),
		}
	}

	sectionRaw := coreSpecific[foundKeys[0]]
	section, ok := artifactToMap(sectionRaw)
	if !ok {
		return nil, warnings, &ArtifactUnsupportedFieldError{
			CoreType: coreType,
			SpecID:   spec.ID,
			Tag:      spec.Tag,
			Field:    fmt.Sprintf("core_specific.%s", foundKeys[0]),
			Message:  "must be a JSON object / 必须是 JSON 对象",
		}
	}

	return section, warnings, nil
}

func artifactApplyCoreSection(
	inbound map[string]any,
	section map[string]any,
	reserved map[string]struct{},
	spec *repository.InboundSpec,
	coreType string,
	sectionField string,
) (string, error) {
	if len(section) == 0 {
		return "", nil
	}
	customFilename := ""

	keys := make([]string, 0, len(section))
	for key := range section {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := section[key]
		if key == "filename" {
			name, ok := value.(string)
			if !ok {
				return "", &ArtifactUnsupportedFieldError{
					CoreType: coreType,
					SpecID:   spec.ID,
					Tag:      spec.Tag,
					Field:    sectionField + ".filename",
					Message:  "must be a string / 必须是字符串",
				}
			}
			customFilename = strings.TrimSpace(name)
			continue
		}
		if _, exists := reserved[key]; exists {
			return "", &ArtifactUnsupportedFieldError{
				CoreType: coreType,
				SpecID:   spec.ID,
				Tag:      spec.Tag,
				Field:    sectionField + "." + key,
				Message:  "cannot override semantic core field / 不能覆盖语义层核心字段",
			}
		}
		inbound[key] = value
	}

	return customFilename, nil
}

var artifactFilenameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func artifactResolveFilename(spec *repository.InboundSpec, customFilename string) (string, error) {
	if strings.TrimSpace(customFilename) != "" {
		if !artifactValidFilename(customFilename) {
			return "", &ArtifactUnsupportedFieldError{
				CoreType: spec.CoreType,
				SpecID:   spec.ID,
				Tag:      spec.Tag,
				Field:    "filename",
				Message:  "must be a safe .json filename / 必须是安全的 .json 文件名",
			}
		}
		return customFilename, nil
	}

	tag := normalizeTag(spec.Tag)
	if tag == "" {
		tag = "inbound"
	}
	safeTag := artifactFilenameSanitizer.ReplaceAllString(tag, "_")
	safeTag = strings.Trim(safeTag, "._-")
	if safeTag == "" {
		safeTag = "inbound"
	}
	return fmt.Sprintf("inbound-%d-%s.json", spec.ID, safeTag), nil
}

func artifactValidFilename(filename string) bool {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, `\\`) {
		return false
	}
	if strings.Contains(trimmed, "..") {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(trimmed), ".json") {
		return false
	}
	return true
}

func artifactLookupFirst(object map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		value, ok := object[key]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func artifactLookupFirstValue(object map[string]any, keys ...string) any {
	value, _ := artifactLookupFirst(object, keys...)
	return value
}

func artifactStringByKeys(object map[string]any, keys ...string) string {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return ""
	}
	return artifactToString(value)
}

func artifactIntByKeys(object map[string]any, keys ...string) int {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return 0
	}
	return artifactToInt(value)
}

func artifactStringSliceByKeys(object map[string]any, keys ...string) []string {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return nil
	}
	return artifactToStringSlice(value)
}

func artifactMapByKeys(object map[string]any, keys ...string) (map[string]any, bool) {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return nil, false
	}
	mapped, ok := artifactToMap(value)
	return mapped, ok
}

func artifactStringMapByKeys(object map[string]any, keys ...string) map[string]string {
	value, ok := artifactLookupFirst(object, keys...)
	if !ok {
		return nil
	}
	mapped, ok := artifactToMap(value)
	if !ok {
		return nil
	}
	result := make(map[string]string, 0)
	for key, item := range mapped {
		text := artifactToString(item)
		if text == "" {
			continue
		}
		result[key] = text
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func artifactToMap(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	mapped, ok := value.(map[string]any)
	if ok {
		return mapped, true
	}
	mappedAny, ok := value.(map[string]interface{})
	if !ok {
		return nil, false
	}
	result := make(map[string]any, len(mappedAny))
	for key, item := range mappedAny {
		result[key] = item
	}
	return result, true
}

func artifactToBool(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return strings.EqualFold(strings.TrimSpace(item), "true")
	case float64:
		return item != 0
	case int:
		return item != 0
	case int64:
		return item != 0
	default:
		return false
	}
}

func artifactToBoolWithDefault(value any, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return artifactToBool(value)
}

func artifactToInt(value any) int {
	switch item := value.(type) {
	case int:
		return item
	case int8:
		return int(item)
	case int16:
		return int(item)
	case int32:
		return int(item)
	case int64:
		return int(item)
	case float64:
		return int(item)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func artifactToString(value any) string {
	switch item := value.(type) {
	case string:
		return strings.TrimSpace(item)
	case json.Number:
		return item.String()
	default:
		return ""
	}
}

func artifactToStringSlice(value any) []string {
	switch item := value.(type) {
	case []string:
		result := make([]string, 0, len(item))
		for _, part := range item {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			result = append(result, trimmed)
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []any:
		result := make([]string, 0, len(item))
		for _, part := range item {
			trimmed := artifactToString(part)
			if trimmed == "" {
				continue
			}
			result = append(result, trimmed)
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case string:
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	default:
		return nil
	}
}

func artifactSplitHostPort(dest string) (string, int) {
	trimmed := strings.TrimSpace(dest)
	if trimmed == "" {
		return "", 0
	}
	index := strings.LastIndex(trimmed, ":")
	if index <= 0 || index >= len(trimmed)-1 {
		return trimmed, 0
	}
	host := strings.TrimSpace(trimmed[:index])
	portText := strings.TrimSpace(trimmed[index+1:])
	port, err := strconv.Atoi(portText)
	if err != nil {
		return trimmed, 0
	}
	return host, port
}

// artifactBuildUnifiedMultiplex builds a UnifiedMultiplex from a decoded JSON object.
func artifactBuildUnifiedMultiplex(raw map[string]any) *template.UnifiedMultiplex {
	if len(raw) == 0 {
		return nil
	}
	multiplex := &template.UnifiedMultiplex{
		Enabled:    artifactToBoolWithDefault(artifactLookupFirstValue(raw, "enabled"), true),
		Protocol:   artifactStringByKeys(raw, "protocol"),
		MaxStreams: artifactIntByKeys(raw, "max_streams", "maxStreams"),
		Padding:    artifactToBoolWithDefault(artifactLookupFirstValue(raw, "padding"), false),
	}
	if brutalRaw, ok := artifactMapByKeys(raw, "brutal"); ok {
		multiplex.Brutal = &template.UnifiedBrutal{
			Enabled:  artifactToBoolWithDefault(artifactLookupFirstValue(brutalRaw, "enabled"), true),
			UpMbps:   artifactIntByKeys(brutalRaw, "up_mbps", "upMbps"),
			DownMbps: artifactIntByKeys(brutalRaw, "down_mbps", "downMbps"),
		}
	}
	return multiplex
}

// artifactBuildUnifiedSniffing builds a UnifiedSniffing from a decoded JSON object.
func artifactBuildUnifiedSniffing(raw map[string]any) *template.UnifiedSniffing {
	if len(raw) == 0 {
		return nil
	}
	return &template.UnifiedSniffing{
		Enabled:         artifactToBoolWithDefault(artifactLookupFirstValue(raw, "enabled"), true),
		DestOverride:    artifactStringSliceByKeys(raw, "dest_override", "destOverride"),
		MetadataOnly:    artifactToBoolWithDefault(artifactLookupFirstValue(raw, "metadata_only", "metadataOnly"), false),
		DomainsExcluded: artifactStringSliceByKeys(raw, "domains_excluded", "domainsExcluded"),
		RouteOnly:       artifactToBoolWithDefault(artifactLookupFirstValue(raw, "route_only", "routeOnly"), false),
	}
}

// artifactBuildUnifiedUsers extracts []UnifiedUser from a parsed semantic spec object.
// It looks for a "users" array where each entry has uuid/id, email/name, password, flow, method.
func artifactBuildUnifiedUsers(semanticObject map[string]any) []template.UnifiedUser {
	if semanticObject == nil {
		return nil
	}
	rawUsers, ok := semanticObject["users"]
	if !ok {
		return nil
	}
	usersArr, ok := rawUsers.([]any)
	if !ok {
		return nil
	}
	result := make([]template.UnifiedUser, 0, len(usersArr))
	for _, raw := range usersArr {
		userMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		user := template.UnifiedUser{
			UUID:     firstNonEmpty(artifactStringByKeys(userMap, "uuid", "id"), ""),
			Email:    artifactStringByKeys(userMap, "email", "name"),
			Password: artifactStringByKeys(userMap, "password", "pass"),
			Flow:     artifactStringByKeys(userMap, "flow"),
			Method:   artifactStringByKeys(userMap, "method"),
		}
		result = append(result, user)
	}
	return result
}

// boolPtr returns a pointer to a bool value.
func boolPtr(v bool) *bool {
	return &v
}
