package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// RoutingPolicyService 管理路由策略（geosite/domain → 出口集合）。
type RoutingPolicyService interface {
	// SetOnChange 注入变更联动回调（重渲染编排），nil 表示不联动。
	SetOnChange(fn func(context.Context) error)
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
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	CoreType    string `json:"core_type"`
	Priority    int    `json:"priority"`
	MatchType   string `json:"match_type"`
	MatchValue  string `json:"match_value"`
	Action      string `json:"action"`
	TargetSetID *int64 `json:"target_set_id"`
	// SpecID 非 nil 表示入站规则（仅对绑定入站生效）；nil 为全局策略。
	// 更新时 nil 沿用现有值（与 TargetSetID 同语义）。
	SpecID  *int64 `json:"spec_id"`
	Enabled *bool  `json:"enabled"`
}

type routingPolicyService struct {
	policies repository.RoutingPolicyRepository
	// specs 用于校验 SpecID 引用存在性；nil 时跳过校验（测试/旧装配点）。
	specs   repository.InboundSpecRepository
	logger  *slog.Logger

	// onChange 策略变更后的联动回调（由装配层注入，用于触发受影响 host 重渲染）；nil 安全。
	onChange func(context.Context) error
}

// SetOnChange 注入策略变更联动回调。
func (s *routingPolicyService) SetOnChange(fn func(context.Context) error) { s.onChange = fn }

// notifyChange 变更成功后通知；回调错误只记录不阻断主流程。
func (s *routingPolicyService) notifyChange(ctx context.Context) {
	if s == nil || s.onChange == nil {
		return
	}
	if err := s.onChange(ctx); err != nil {
		s.logger.Warn("routing policy change re-render failed", "error", err)
	}
}

// NewRoutingPolicyService 创建 RoutingPolicyService。specs 允许为 nil（跳过 spec 引用校验）。
func NewRoutingPolicyService(policies repository.RoutingPolicyRepository, specs repository.InboundSpecRepository, logger *slog.Logger) RoutingPolicyService {
	return &routingPolicyService{policies: policies, specs: specs, logger: logger}
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
	if err := s.validateSpecRef(ctx, policy.SpecID); err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	policy.CreatedAt = now
	policy.UpdatedAt = now
	if err := s.policies.Create(ctx, policy); err != nil {
		return nil, err
	}
	s.logger.Info("routing policy created", "policy_id", policy.ID, "name", policy.Name, "core", policy.CoreType, "match", policy.MatchType+":"+policy.MatchValue, "target_set", *req.TargetSetID)
	s.notifyChange(ctx)
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
	if policy.SpecID == nil {
		policy.SpecID = existing.SpecID
	}
	if err := s.validateSpecRef(ctx, policy.SpecID); err != nil {
		return nil, err
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
	s.notifyChange(ctx)
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
		Name:        strings.TrimSpace(req.Name),
		CoreType:    normalizeRoutingCoreType(req.CoreType),
		Priority:    req.Priority,
		MatchType:   matchType,
		MatchValue:  strings.TrimSpace(req.MatchValue),
		Action:      strings.TrimSpace(req.Action),
		TargetSetID: req.TargetSetID,
		SpecID:      req.SpecID,
		Enabled:     enabled,
	}
}

// validateSpecRef 校验策略绑定的 spec 引用存在；specs 仓库未注入时跳过。
func (s *routingPolicyService) validateSpecRef(ctx context.Context, specID *int64) error {
	if s == nil || s.specs == nil || specID == nil || *specID <= 0 {
		return nil
	}
	if _, err := s.specs.FindByID(ctx, *specID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("routing policy bound spec not found / 绑定的入站不存在")
		}
		return fmt.Errorf("validate routing policy spec: %w", err)
	}
	return nil
}

func (s *routingPolicyService) Delete(ctx context.Context, id int64) error {
	if err := s.policies.Delete(ctx, id); err != nil {
		return err
	}
	s.logger.Info("routing policy deleted", "policy_id", id)
	s.notifyChange(ctx)
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
