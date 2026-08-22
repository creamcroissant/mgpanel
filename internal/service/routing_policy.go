package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// RoutingPolicyService 管理路由策略（geosite/domain → 出口集合）。
type RoutingPolicyService interface {
	Create(ctx context.Context, req RoutingPolicyUpsertRequest) (*repository.RoutingPolicy, error)
	Update(ctx context.Context, req RoutingPolicyUpsertRequest) (*repository.RoutingPolicy, error)
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*repository.RoutingPolicy, error)
	List(ctx context.Context, coreType string) ([]*repository.RoutingPolicy, error)
	// ResolveToExitNodeSets 将一条策略解析为实际的出口节点集合引用。
	Resolve(ctx context.Context, id int64) (*repository.RoutingPolicy, error)
}

// RoutingPolicyUpsertRequest 创建/更新路由策略的请求。
type RoutingPolicyUpsertRequest struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	CoreType    string  `json:"core_type"`
	Priority    int     `json:"priority"`
	MatchType   string  `json:"match_type"`
	MatchValue  string  `json:"match_value"`
	Action      string  `json:"action"`
	TargetSetID *int64  `json:"target_set_id"`
	Enabled     *bool   `json:"enabled"`
}

type routingPolicyService struct {
	policies repository.RoutingPolicyRepository
	logger   *slog.Logger
}

// NewRoutingPolicyService 创建 RoutingPolicyService。
func NewRoutingPolicyService(policies repository.RoutingPolicyRepository, logger *slog.Logger) RoutingPolicyService {
	return &routingPolicyService{policies: policies, logger: logger}
}

func normalizeRoutingCoreType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "xray":
		return "xray"
	default:
		return "sing-box"
	}
}

func normalizeRoutingMatchType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "domain":
		return "domain"
	case "ip_cidr":
		return "ip_cidr"
	default:
		return "geosite"
	}
}

func (s *routingPolicyService) Create(ctx context.Context, req RoutingPolicyUpsertRequest) (*repository.RoutingPolicy, error) {
	policy := s.buildPolicy(req)
	if policy.Name == "" {
		return nil, fmt.Errorf("routing policy name is required / 策略名称不能为空")
	}
	if policy.MatchValue == "" {
		return nil, fmt.Errorf("routing policy match value is required / 匹配值不能为空")
	}
	if req.TargetSetID == nil {
		return nil, fmt.Errorf("routing policy needs target set / 策略必须指定出口集合")
	}
	now := time.Now().Unix()
	policy.CreatedAt = now
	policy.UpdatedAt = now
	if err := s.policies.Create(ctx, policy); err != nil {
		return nil, err
	}
	s.logger.Info("routing policy created", "policy_id", policy.ID, "name", policy.Name, "core", policy.CoreType, "match", policy.MatchType+":"+policy.MatchValue, "target_set", *req.TargetSetID)
	return policy, nil
}

func (s *routingPolicyService) Update(ctx context.Context, req RoutingPolicyUpsertRequest) (*repository.RoutingPolicy, error) {
	existing, err := s.policies.FindByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	policy := s.buildPolicy(req)
	// 未提供的字段沿用现有值
	if policy.Name == "" {
		policy.Name = existing.Name
	}
	if policy.CoreType == "" {
		policy.CoreType = existing.CoreType
	}
	if policy.MatchValue == "" {
		policy.MatchValue = existing.MatchValue
	}
	if policy.Action == "" {
		policy.Action = existing.Action
	}
	if policy.TargetSetID == nil {
		policy.TargetSetID = existing.TargetSetID
	}
	if req.Enabled == nil {
		policy.Enabled = existing.Enabled
	}
	policy.ID = existing.ID
	policy.Priority = req.Priority
	policy.UpdatedAt = time.Now().Unix()
	if err := s.policies.Update(ctx, policy); err != nil {
		return nil, err
	}
	s.logger.Info("routing policy updated", "policy_id", policy.ID, "name", policy.Name)
	return policy, nil
}

func (s *routingPolicyService) buildPolicy(req RoutingPolicyUpsertRequest) *repository.RoutingPolicy {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	matchType := normalizeRoutingMatchType(req.MatchType)
	if matchType == "" {
		matchType = "geosite"
	}
	return &repository.RoutingPolicy{
		Name:       strings.TrimSpace(req.Name),
		CoreType:   normalizeRoutingCoreType(req.CoreType),
		Priority:   req.Priority,
		MatchType:  matchType,
		MatchValue: strings.TrimSpace(req.MatchValue),
		Action:     strings.TrimSpace(req.Action),
		TargetSetID: req.TargetSetID,
		Enabled:    enabled,
	}
}

func (s *routingPolicyService) Delete(ctx context.Context, id int64) error {
	if err := s.policies.Delete(ctx, id); err != nil {
		return err
	}
	s.logger.Info("routing policy deleted", "policy_id", id)
	return nil
}

func (s *routingPolicyService) Get(ctx context.Context, id int64) (*repository.RoutingPolicy, error) {
	return s.policies.FindByID(ctx, id)
}

func (s *routingPolicyService) List(ctx context.Context, coreType string) ([]*repository.RoutingPolicy, error) {
	coreType = normalizeRoutingCoreType(coreType)
	return s.policies.ListEnabledByCore(ctx, coreType)
}

func (s *routingPolicyService) Resolve(ctx context.Context, id int64) (*repository.RoutingPolicy, error) {
	return s.policies.FindByID(ctx, id)
}

