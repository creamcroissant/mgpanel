package service

import (
	"context"
	"log/slog"
	"sync/atomic"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/protocol"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

const (
	SubscriptionFilterReasonHidden           = "hidden"
	SubscriptionFilterReasonOffline          = "offline"
	SubscriptionFilterReasonBlocked          = "blocked"
	SubscriptionFilterReasonThresholdReached = "threshold_reached"
	SubscriptionFilterReasonProtocolDisabled = "protocol_disabled"
	SubscriptionFilterReasonGroupDenied      = "group_denied"
	SubscriptionFilterReasonTagMismatch      = "tag_mismatch"
	SubscriptionFilterReasonTypeMismatch     = "type_mismatch"
)

type SubscriptionFilterService interface {
	Filter(ctx context.Context, req SubscriptionFilterRequest) (*SubscriptionFilterResult, error)
	ListFilterReasons(ctx context.Context, req ListSubscriptionFilterReasonsRequest) (*SubscriptionFilterReasonListResult, error)
	GetFilterSummary(ctx context.Context, req SubscriptionFilterSummaryRequest) (*SubscriptionFilterSummary, error)
	// SetPersistReasons 开关过滤原因持久化。订阅热路径固定传 false（写放大
	// 优化）；需要低频重建原因记录时由运维/装配侧显式开启。
	SetPersistReasons(enabled bool)
}

type SubscriptionFilterRequest struct {
	User           *repository.User
	AllowedTypes   map[string]struct{}
	Keywords       []string
	Tags           []string
	PersistReasons bool
}

type ListSubscriptionFilterReasonsRequest struct {
	SourceType    string
	SourceID      *int64
	ServerID      *int64
	Reason        string
	CreatedAfter  *int64
	CreatedBefore *int64
	Limit         int
	Offset        int
	Types         string
	Filter        string
	Tags          string
}

type SubscriptionFilterSummaryRequest struct {
	Types  string
	Filter string
	Tags   string
}

type SubscriptionFilterResult struct {
	Servers       []*repository.Server
	SourceNodes   []protocol.Node
	Reasons       []SubscriptionFilterReasonView
	Available     int
	Filtered      int
	Total         int
	SelfHosted    int
	SourceTotal   int
	SourceEnabled int
}

type SubscriptionFilterSummary struct {
	AvailableNodeCount int            `json:"available_node_count"`
	FilteredNodeCount  int            `json:"filtered_node_count"`
	TotalNodeCount     int            `json:"total_node_count"`
	SelfHostedCount    int            `json:"self_hosted_count"`
	SourceNodeCount    int            `json:"source_node_count"`
	EnabledSourceCount int            `json:"enabled_source_count"`
	ReasonCounts       map[string]int `json:"reason_counts"`
}

type SubscriptionFilterReasonView struct {
	ID         int64  `json:"id"`
	SourceType string `json:"source_type"`
	SourceID   int64  `json:"source_id"`
	ServerID   int64  `json:"server_id"`
	NodeName   string `json:"node_name"`
	Reason     string `json:"reason"`
	Detail     string `json:"detail,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}

type SubscriptionFilterReasonListResult struct {
	Reasons            []SubscriptionFilterReasonView `json:"reasons"`
	Total              int64                          `json:"total"`
	AvailableNodeCount int                            `json:"available_node_count"`
	FilteredNodeCount  int                            `json:"filtered_node_count"`
	TotalNodeCount     int                            `json:"total_node_count"`
	ReasonCounts       map[string]int                 `json:"reason_counts"`
}

type subscriptionFilterService struct {
	servers         repository.ServerRepository
	sources   repository.SubscriptionSourceRepository
	reasons   repository.SubscriptionFilterReasonRepository
	plans     repository.PlanRepository
	selection UserServerSelectionService
	telemetry ServerTelemetryService

	persistReasons atomic.Bool // 低频持久化开关；默认关闭（热路径写放大优化）
}

type subscriptionFilterExternalReason struct {
	reason string
	detail string
}

type subscriptionFilterExternalReasons struct {
	servers map[int64]subscriptionFilterExternalReason
}

type subscriptionSourceNodeKey struct {
	sourceType string
	sourceID   int64
	nodeName   string
}

type subscriptionFilterSourceReasonGroup struct {
	sourceType string
	sourceID   int64
	reasons    []*repository.SubscriptionFilterReason
}

func NewSubscriptionFilterService(servers repository.ServerRepository, sources repository.SubscriptionSourceRepository, reasons repository.SubscriptionFilterReasonRepository, plans repository.PlanRepository, selection UserServerSelectionService, telemetry ServerTelemetryService) SubscriptionFilterService {
	return &subscriptionFilterService{servers: servers, sources: sources, reasons: reasons, plans: plans, selection: selection, telemetry: telemetry}
}

// SetPersistReasons 切换过滤原因持久化模式，结构化日志记录变更。
func (s *subscriptionFilterService) SetPersistReasons(enabled bool) {
	s.persistReasons.Store(enabled)
	slog.Info("subscription filter: reason persistence mode updated", "enabled", enabled)
}

func (s *subscriptionFilterService) Filter(ctx context.Context, req SubscriptionFilterRequest) (*SubscriptionFilterResult, error) {
	if s == nil || s.servers == nil {
		return nil, ErrNotImplemented
	}
	servers, err := s.servers.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	external, err := s.loadExternalFilterReasons(ctx)
	if err != nil {
		return nil, err
	}
	groupIDs, err := s.userGroupIDs(ctx, req.User)
	if err != nil {
		return nil, err
	}
	selectedIDs, selectionActive := s.userSelectedServerIDs(ctx, req.User)

	accepted := make([]*repository.Server, 0, len(servers))
	selfReasons := make([]*repository.SubscriptionFilterReason, 0)
	for _, server := range servers {
		if server == nil {
			continue
		}
		if reason := s.evaluateServer(ctx, server, req, groupIDs, selectedIDs, selectionActive, external); reason != nil {
			selfReasons = append(selfReasons, reason)
			continue
		}
		accepted = append(accepted, server)
	}

	sourceNodes, sourceTotal, sourceEnabled, sourceReasons, err := s.filterSourceNodes(ctx, req, external)
	if err != nil {
		return nil, err
	}
	if (req.PersistReasons || s.persistReasons.Load()) && s.reasons != nil {
		if err := s.persistFilterReasons(ctx, selfReasons, sourceReasons); err != nil {
			return nil, err
		}
	}

	views := subscriptionFilterReasonViews(selfReasons)
	for _, group := range sourceReasons {
		views = append(views, subscriptionFilterReasonViews(group.reasons)...)
	}
	return &SubscriptionFilterResult{
		Servers:       accepted,
		SourceNodes:   sourceNodes,
		Reasons:       views,
		Available:     len(accepted) + len(sourceNodes),
		Filtered:      len(views),
		Total:         len(servers) + sourceTotal,
		SelfHosted:    len(accepted),
		SourceTotal:   sourceTotal,
		SourceEnabled: sourceEnabled,
	}, nil
}

func (s *subscriptionFilterService) ListFilterReasons(ctx context.Context, req ListSubscriptionFilterReasonsRequest) (*SubscriptionFilterReasonListResult, error) {
	if s == nil || s.reasons == nil {
		return nil, ErrNotImplemented
	}
	filter, err := buildSubscriptionFilterReasonFilter(req)
	if err != nil {
		return nil, err
	}
	reasons, err := s.reasons.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	total, err := s.reasons.Count(ctx, filter)
	if err != nil {
		return nil, err
	}
	summary, err := s.GetFilterSummary(ctx, SubscriptionFilterSummaryRequest{Types: req.Types, Filter: req.Filter, Tags: req.Tags})
	if err != nil && err != ErrNotImplemented {
		return nil, err
	}
	result := &SubscriptionFilterReasonListResult{Reasons: subscriptionFilterReasonViews(reasons), Total: total, ReasonCounts: map[string]int{}}
	if summary != nil {
		result.AvailableNodeCount = summary.AvailableNodeCount
		result.FilteredNodeCount = summary.FilteredNodeCount
		result.TotalNodeCount = summary.TotalNodeCount
		result.ReasonCounts = summary.ReasonCounts
	}
	return result, nil
}

func (s *subscriptionFilterService) GetFilterSummary(ctx context.Context, req SubscriptionFilterSummaryRequest) (*SubscriptionFilterSummary, error) {
	allowedTypes := parseRequestedTypes(req.Types)
	keywords := parseFilterKeywords(req.Filter)
	tags := parseTagsFilter(req.Tags)
	result, err := s.Filter(ctx, SubscriptionFilterRequest{AllowedTypes: allowedTypes, Keywords: keywords, Tags: tags})
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, reason := range result.Reasons {
		counts[reason.Reason]++
	}
	return &SubscriptionFilterSummary{
		AvailableNodeCount: result.Available,
		FilteredNodeCount:  result.Filtered,
		TotalNodeCount:     result.Total,
		SelfHostedCount:    result.SelfHosted,
		SourceNodeCount:    len(result.SourceNodes),
		EnabledSourceCount: result.SourceEnabled,
		ReasonCounts:       counts,
	}, nil
}

func (s *subscriptionFilterService) evaluateServer(ctx context.Context, server *repository.Server, req SubscriptionFilterRequest, groupIDs []int64, selectedIDs map[int64]struct{}, selectionActive bool, external subscriptionFilterExternalReasons) *repository.SubscriptionFilterReason {
	if server.Show == 0 {
		return newSubscriptionFilterReason(SubscriptionSourceTypeSelfHosted, 0, server.ID, server.Name, SubscriptionFilterReasonHidden, "server hidden")
	}
	if selectionActive {
		if _, ok := selectedIDs[server.ID]; !ok {
			return newSubscriptionFilterReason(SubscriptionSourceTypeSelfHosted, 0, server.ID, server.Name, SubscriptionFilterReasonGroupDenied, "not in user selection")
		}
	}
	if len(groupIDs) > 0 && !containsGroupID(groupIDs, server.GroupID) {
		return newSubscriptionFilterReason(SubscriptionSourceTypeSelfHosted, 0, server.ID, server.Name, SubscriptionFilterReasonGroupDenied, "server group denied")
	}
	if reason, ok := external.servers[server.ID]; ok {
		return newSubscriptionFilterReason(SubscriptionSourceTypeSelfHosted, 0, server.ID, server.Name, reason.reason, reason.detail)
	}
	if !serverHasKnownProtocol(server) {
		return newSubscriptionFilterReason(SubscriptionSourceTypeSelfHosted, 0, server.ID, server.Name, SubscriptionFilterReasonProtocolDisabled, "protocol disabled")
	}
	if !serverTypeAllowed(server, req.AllowedTypes) {
		return newSubscriptionFilterReason(SubscriptionSourceTypeSelfHosted, 0, server.ID, server.Name, SubscriptionFilterReasonTypeMismatch, "requested type filter")
	}
	if len(req.Tags) > 0 && !matchesAnyTag(decodeStringArray(server.Tags), req.Tags) {
		return newSubscriptionFilterReason(SubscriptionSourceTypeSelfHosted, 0, server.ID, server.Name, SubscriptionFilterReasonTagMismatch, "requested tag filter")
	}
	if len(req.Keywords) > 0 && !matchesKeywords(server, req.Keywords) {
		return newSubscriptionFilterReason(SubscriptionSourceTypeSelfHosted, 0, server.ID, server.Name, SubscriptionFilterReasonBlocked, "requested keyword filter")
	}
	if s.telemetry != nil && !s.telemetry.IsNodeOnline(ctx, server) {
		return newSubscriptionFilterReason(SubscriptionSourceTypeSelfHosted, 0, server.ID, server.Name, SubscriptionFilterReasonOffline, "server offline")
	}
	return nil
}

func (s *subscriptionFilterService) filterSourceNodes(ctx context.Context, req SubscriptionFilterRequest, external subscriptionFilterExternalReasons) ([]protocol.Node, int, int, []subscriptionFilterSourceReasonGroup, error) {
	if s == nil || s.sources == nil {
		return []protocol.Node{}, 0, 0, nil, nil
	}
	sources, err := s.sources.List(ctx, repository.SubscriptionSourceFilter{Limit: 1000})
	if err != nil {
		return nil, 0, 0, nil, err
	}
	nodes := make([]protocol.Node, 0, len(sources))
	reasonGroups := make([]subscriptionFilterSourceReasonGroup, 0, len(sources))
	total := 0
	enabledCount := 0
	for _, source := range sources {
		if source == nil {
			continue
		}
		sourceType := normalizeSubscriptionSourceType(source.Type)
		if sourceType != SubscriptionSourceTypeImported && sourceType != SubscriptionSourceTypeCustom {
			continue
		}
		sourceNodes, parseErr := buildSubscriptionSourceNodes(source)
		if parseErr != nil {
			reasonGroups = append(reasonGroups, subscriptionFilterSourceReasonGroup{sourceType: sourceType, sourceID: source.ID, reasons: []*repository.SubscriptionFilterReason{newSubscriptionFilterReason(sourceType, source.ID, 0, source.Name, SubscriptionFilterReasonBlocked, "source parse failed")}})
			continue
		}
		total += len(sourceNodes)
		group := subscriptionFilterSourceReasonGroup{sourceType: sourceType, sourceID: source.ID}
		if !source.Enabled {
			for _, node := range sourceNodes {
				group.reasons = append(group.reasons, newSubscriptionFilterReason(sourceType, source.ID, node.ID, node.Name, SubscriptionFilterReasonHidden, "source disabled"))
			}
			reasonGroups = append(reasonGroups, group)
			continue
		}
		enabledCount++
		for _, node := range sourceNodes {
			if reason := evaluateSourceNode(sourceType, source.ID, node, req); reason != nil {
				group.reasons = append(group.reasons, reason)
				continue
			}
			nodes = append(nodes, node)
		}
		reasonGroups = append(reasonGroups, group)
	}
	return nodes, total, enabledCount, reasonGroups, nil
}

func evaluateSourceNode(sourceType string, sourceID int64, node protocol.Node, req SubscriptionFilterRequest) *repository.SubscriptionFilterReason {
	// 注：订阅源节点暂无外部屏蔽原因写入方（agent 流量阈值仅记录 server 维度），
	// 原先预留的 external.sources 死路径已移除；后续如需源级阈值屏蔽再恢复。
	if !subscriptionProtocolKnown(node.Type) {
		return newSubscriptionFilterReason(sourceType, sourceID, node.ID, node.Name, SubscriptionFilterReasonProtocolDisabled, "protocol disabled")
	}
	if !typeAllowed(node.Type, req.AllowedTypes) {
		return newSubscriptionFilterReason(sourceType, sourceID, node.ID, node.Name, SubscriptionFilterReasonTypeMismatch, "requested type filter")
	}
	if len(req.Tags) > 0 && !matchesAnyTag(node.Tags, req.Tags) {
		return newSubscriptionFilterReason(sourceType, sourceID, node.ID, node.Name, SubscriptionFilterReasonTagMismatch, "requested tag filter")
	}
	if len(req.Keywords) > 0 && !matchesNodeKeywords(node, req.Keywords) {
		return newSubscriptionFilterReason(sourceType, sourceID, node.ID, node.Name, SubscriptionFilterReasonBlocked, "requested keyword filter")
	}
	return nil
}

func (s *subscriptionFilterService) persistFilterReasons(ctx context.Context, selfReasons []*repository.SubscriptionFilterReason, sourceReasons []subscriptionFilterSourceReasonGroup) error {
	if err := s.reasons.ReplaceForSource(ctx, SubscriptionSourceTypeSelfHosted, 0, selfReasons); err != nil {
		return err
	}
	for _, group := range sourceReasons {
		if err := s.reasons.ReplaceForSource(ctx, group.sourceType, group.sourceID, group.reasons); err != nil {
			return err
		}
	}
	return nil
}

func (s *subscriptionFilterService) loadExternalFilterReasons(ctx context.Context) (subscriptionFilterExternalReasons, error) {
	result := subscriptionFilterExternalReasons{servers: map[int64]subscriptionFilterExternalReason{}}
	if s == nil || s.reasons == nil {
		return result, nil
	}
	reasons, err := s.reasons.List(ctx, repository.SubscriptionFilterReasonFilter{Limit: 10000})
	if err != nil {
		return result, err
	}
	for _, item := range reasons {
		if item == nil {
			continue
		}
		reason := normalizeSubscriptionFilterReason(item.Reason)
		if reason == "" {
			continue
		}
		detail := strings.TrimSpace(item.Detail)
		sourceType := strings.TrimSpace(item.SourceType)
		if sourceType == agentTrafficFilterSourcePolicy && item.ServerID > 0 && reason == SubscriptionFilterReasonThresholdReached {
			result.servers[item.ServerID] = subscriptionFilterExternalReason{reason: reason, detail: detail}
		}
	}
	return result, nil
}

func (s *subscriptionFilterService) userGroupIDs(ctx context.Context, user *repository.User) ([]int64, error) {
	if user == nil {
		return nil, nil
	}
	groupIDs := make([]int64, 0, 4)
	if user.GroupID > 0 {
		groupIDs = append(groupIDs, user.GroupID)
	}
	if user.PlanID > 0 && s.plans != nil {
		planGroups, err := s.plans.GetGroups(ctx, user.PlanID)
		if err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, planGroups...)
	}
	return groupIDs, nil
}

func (s *subscriptionFilterService) userSelectedServerIDs(ctx context.Context, user *repository.User) (map[int64]struct{}, bool) {
	if user == nil || s == nil || s.selection == nil {
		return nil, false
	}
	ids, err := s.selection.GetSelection(ctx, user.ID)
	if err != nil || len(ids) == 0 {
		return nil, false
	}
	selected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			selected[id] = struct{}{}
		}
	}
	return selected, len(selected) > 0
}

func buildSubscriptionFilterReasonFilter(req ListSubscriptionFilterReasonsRequest) (repository.SubscriptionFilterReasonFilter, error) {
	filter := repository.SubscriptionFilterReasonFilter{CreatedAfter: req.CreatedAfter, CreatedBefore: req.CreatedBefore, Limit: req.Limit, Offset: req.Offset}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if strings.TrimSpace(req.SourceType) != "" {
		sourceType := strings.TrimSpace(req.SourceType)
		filter.SourceType = &sourceType
	}
	if req.SourceID != nil {
		filter.SourceID = req.SourceID
	}
	if req.ServerID != nil {
		filter.ServerID = req.ServerID
	}
	if strings.TrimSpace(req.Reason) != "" {
		reason := normalizeSubscriptionFilterReason(req.Reason)
		if reason == "" {
			return filter, ErrBadRequest
		}
		filter.Reason = &reason
	}
	return filter, nil
}

func newSubscriptionFilterReason(sourceType string, sourceID, serverID int64, nodeName, reason, detail string) *repository.SubscriptionFilterReason {
	return &repository.SubscriptionFilterReason{SourceType: strings.TrimSpace(sourceType), SourceID: sourceID, ServerID: serverID, NodeName: strings.TrimSpace(nodeName), Reason: normalizeSubscriptionFilterReason(reason), Detail: strings.TrimSpace(detail)}
}

func subscriptionFilterReasonViews(reasons []*repository.SubscriptionFilterReason) []SubscriptionFilterReasonView {
	views := make([]SubscriptionFilterReasonView, 0, len(reasons))
	for _, reason := range reasons {
		if reason == nil {
			continue
		}
		views = append(views, SubscriptionFilterReasonView{
			ID:         reason.ID,
			SourceType: strings.TrimSpace(reason.SourceType),
			SourceID:   reason.SourceID,
			ServerID:   reason.ServerID,
			NodeName:   strings.TrimSpace(reason.NodeName),
			Reason:     normalizeSubscriptionFilterReason(reason.Reason),
			Detail:     strings.TrimSpace(reason.Detail),
			CreatedAt:  reason.CreatedAt,
		})
	}
	return views
}

func normalizeSubscriptionFilterReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case SubscriptionFilterReasonHidden:
		return SubscriptionFilterReasonHidden
	case SubscriptionFilterReasonOffline:
		return SubscriptionFilterReasonOffline
	case SubscriptionFilterReasonBlocked:
		return SubscriptionFilterReasonBlocked
	case SubscriptionFilterReasonThresholdReached, agentTrafficFilterReasonDisabledServers:
		return SubscriptionFilterReasonThresholdReached
	case SubscriptionFilterReasonProtocolDisabled:
		return SubscriptionFilterReasonProtocolDisabled
	case SubscriptionFilterReasonGroupDenied:
		return SubscriptionFilterReasonGroupDenied
	case SubscriptionFilterReasonTagMismatch:
		return SubscriptionFilterReasonTagMismatch
	case SubscriptionFilterReasonTypeMismatch:
		return SubscriptionFilterReasonTypeMismatch
	default:
		return ""
	}
}

func subscriptionProtocolKnown(protocolType string) bool {
	if normalizeSubscriptionProtocolType(protocolType) != "" {
		return true
	}
	_, ok := validServerTypes[strings.ToLower(strings.TrimSpace(protocolType))]
	return ok
}

// serverProtocols 返回节点配置中包含的协议名列表。
// 节点 type 字段存的是核心类型（sing-box / xray），真正协议在 settings 的 details 里。
func serverProtocols(server *repository.Server) []string {
	if server == nil || len(server.Settings) == 0 {
		return nil
	}
	var details []struct {
		Protocol string `json:"protocol"`
	}
	if err := json.Unmarshal(server.Settings, &details); err != nil {
		return nil
	}
	out := make([]string, 0, len(details))
	for _, d := range details {
		if p := strings.ToLower(strings.TrimSpace(d.Protocol)); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// serverHasKnownProtocol 判断节点至少包含一个已知协议。
func serverHasKnownProtocol(server *repository.Server) bool {
	for _, p := range serverProtocols(server) {
		if subscriptionProtocolKnown(p) {
			return true
		}
	}
	// 兼容旧数据：type 字段直接存协议名时
	return subscriptionProtocolKnown(server.Type)
}

// serverTypeAllowed 判断节点协议是否在允许类型内。
func serverTypeAllowed(server *repository.Server, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, p := range serverProtocols(server) {
		if typeAllowed(p, allowed) {
			return true
		}
	}
	return typeAllowed(server.Type, allowed)
}

func subscriptionFilterSummaryString(result *SubscriptionFilterResult) string {
	if result == nil {
		return ""
	}
	return fmt.Sprintf("available=%d filtered=%d total=%d", result.Available, result.Filtered, result.Total)
}
