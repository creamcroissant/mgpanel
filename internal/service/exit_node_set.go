package service

import (
	"errors"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ExitNodeSetService 管理出口节点集合（负载均衡/故障转移的成员分组）。
type ExitNodeSetService interface {
	// SetOnChange 注入集合变更联动回调（重渲染编排），回调 error 由本服务记录；nil 表示不联动。
	SetOnChange(fn func(context.Context) error)
	Create(ctx context.Context, req ExitNodeSetCreateRequest) (*repository.ExitNodeSet, error)
	Update(ctx context.Context, req ExitNodeSetUpdateRequest) (*repository.ExitNodeSet, error)
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*ExitNodeSetDetail, error)
	List(ctx context.Context) ([]*ExitNodeSetDetail, error)

	AddMember(ctx context.Context, req ExitNodeSetMemberRequest) error
	RemoveMember(ctx context.Context, setID, agentHostID int64) error
	UpdateMember(ctx context.Context, req ExitNodeSetMemberRequest) error
}

// ExitNodeSetCreateRequest 创建出口集合的请求。
type ExitNodeSetCreateRequest struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Tags        []string                 `json:"tags"`
	Strategy    string                   `json:"strategy"`
	Members     []ExitNodeSetMemberInput `json:"members"`
}

// ExitNodeSetUpdateRequest 更新出口集合的请求。
type ExitNodeSetUpdateRequest struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Strategy    string   `json:"strategy"`
	Enabled     *bool    `json:"enabled"`
}

// ExitNodeSetMemberInput 成员输入（创建/更新集合时）。
type ExitNodeSetMemberInput struct {
	AgentHostID int64 `json:"agent_host_id"`
	Weight      int   `json:"weight"`
}

// ExitNodeSetMemberRequest 成员操作请求。
type ExitNodeSetMemberRequest struct {
	SetID       int64 `json:"set_id"`
	AgentHostID int64 `json:"agent_host_id"`
	Weight      int   `json:"weight"`
	Enabled     *bool `json:"enabled"`
}

// ExitNodeSetDetail 集合详情（含成员）。
type ExitNodeSetDetail struct {
	Set      *repository.ExitNodeSet         `json:"set"`
	Members  []*repository.ExitNodeSetMember `json:"members"`
	HostName map[int64]string                `json:"host_name"` // agent_host_id -> host 显示名
	// UnlockSummary 按 agent_host_id 汇总各成员当前解锁状态（platform -> region/"?"），
	// 供前端出口集合详情展示与分流策略制定参考。
	UnlockSummary map[int64]map[string]string `json:"unlock_summary,omitempty"`
}

type exitNodeSetService struct {
	sets        repository.ExitNodeSetRepository
	agentHosts  repository.AgentHostRepository
	unlockProbe repository.UnlockProbeResultRepository
	logger      *slog.Logger

	// onChange 集合/成员变更后的联动回调（装配层注入，触发受影响 host 重渲染）；nil 安全。
	onChange func(context.Context) error
}

// SetOnChange 注入集合变更联动回调。
func (s *exitNodeSetService) SetOnChange(fn func(context.Context) error) { s.onChange = fn }

// notifyChange 变更成功后通知；回调失败记录但不阻断。
func (s *exitNodeSetService) notifyChange(ctx context.Context) {
	if s == nil || s.onChange == nil {
		return
	}
	if err := s.onChange(ctx); err != nil {
		s.logger.Warn("exit node set change re-render failed", "error", err)
	}
}

// NewExitNodeSetService 创建 ExitNodeSetService。
func NewExitNodeSetService(
	sets repository.ExitNodeSetRepository,
	agentHosts repository.AgentHostRepository,
	unlockProbe repository.UnlockProbeResultRepository,
	logger *slog.Logger,
) ExitNodeSetService {
	return &exitNodeSetService{sets: sets, agentHosts: agentHosts, unlockProbe: unlockProbe, logger: logger}
}

func normalizeExitSetStrategy(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "least_ping":
		return "least_ping"
	case "random":
		return "random"
	case "weighted_random":
		return "weighted_random"
	default:
		return "round_robin"
	}
}

func (s *exitNodeSetService) Create(ctx context.Context, req ExitNodeSetCreateRequest) (*repository.ExitNodeSet, error) {
	now := time.Now().Unix()
	set := &repository.ExitNodeSet{
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Tags:        strings.Join(req.Tags, ","),
		Strategy:    normalizeExitSetStrategy(req.Strategy),
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if set.Name == "" {
		return nil, fmt.Errorf("exit node set name is required / 出口集合名称不能为空")
	}
	// 名称唯一（非 NotFound 的 DB 错误必须显式失败，避免误判为“名称可用”）
	if _, err := s.sets.FindByName(ctx, set.Name); err == nil {
		return nil, fmt.Errorf("exit node set name already exists / 出口集合名称已存在")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check exit node set name: %w", err)
	}
	if err := s.sets.Create(ctx, set); err != nil {
		return nil, err
	}
	// 添加成员
	for _, m := range req.Members {
		if m.AgentHostID <= 0 {
			continue
		}
		weight := m.Weight
		if weight <= 0 {
			weight = 1
		}
		if err := s.sets.AddMember(ctx, &repository.ExitNodeSetMember{
			SetID: set.ID, AgentHostID: m.AgentHostID, Weight: weight, Enabled: true,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			s.logger.Warn("add exit node set member failed", "set_id", set.ID, "agent_host_id", m.AgentHostID, "error", err)
		}
	}
	s.logger.Info("exit node set created", "set_id", set.ID, "name", set.Name, "strategy", set.Strategy, "members", len(req.Members))
	s.notifyChange(ctx)
	return set, nil
}

func (s *exitNodeSetService) Update(ctx context.Context, req ExitNodeSetUpdateRequest) (*repository.ExitNodeSet, error) {
	set, err := s.sets.FindByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		set.Name = strings.TrimSpace(req.Name)
	}
	if req.Description != "" {
		set.Description = req.Description
	}
	if req.Tags != nil {
		set.Tags = strings.Join(req.Tags, ",")
	}
	if req.Strategy != "" {
		set.Strategy = normalizeExitSetStrategy(req.Strategy)
	}
	if req.Enabled != nil {
		set.Enabled = *req.Enabled
	}
	set.UpdatedAt = time.Now().Unix()
	if err := s.sets.Update(ctx, set); err != nil {
		return nil, err
	}
	s.logger.Info("exit node set updated", "set_id", set.ID, "name", set.Name)
	s.notifyChange(ctx)
	return set, nil
}

func (s *exitNodeSetService) Delete(ctx context.Context, id int64) error {
	if err := s.sets.Delete(ctx, id); err != nil {
		return err
	}
	s.logger.Info("exit node set deleted", "set_id", id)
	s.notifyChange(ctx)
	return nil
}

func (s *exitNodeSetService) Get(ctx context.Context, id int64) (*ExitNodeSetDetail, error) {
	set, err := s.sets.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	members, err := s.sets.ListMembers(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.buildDetail(ctx, set, members)
}

func (s *exitNodeSetService) List(ctx context.Context) ([]*ExitNodeSetDetail, error) {
	sets, err := s.sets.List(ctx)
	if err != nil {
		return nil, err
	}
	details := make([]*ExitNodeSetDetail, 0, len(sets))
	for _, set := range sets {
		members, err := s.sets.ListMembers(ctx, set.ID)
		if err != nil {
			s.logger.Warn("list set members failed", "set_id", set.ID, "error", err)
			members = nil
		}
		d, err := s.buildDetail(ctx, set, members)
		if err != nil {
			s.logger.Warn("build set detail failed", "set_id", set.ID, "error", err)
			continue
		}
		details = append(details, d)
	}
	return details, nil
}

// buildDetail 组装集合详情，附加 host 名与各成员的解锁状态摘要。
func (s *exitNodeSetService) buildDetail(ctx context.Context, set *repository.ExitNodeSet, members []*repository.ExitNodeSetMember) (*ExitNodeSetDetail, error) {
	hostNames := map[int64]string{}
	unlockSummary := map[int64]map[string]string{}
	for _, m := range members {
		host, err := s.agentHosts.FindByID(ctx, m.AgentHostID)
		if err == nil && host != nil {
			hostNames[m.AgentHostID] = host.Host
		} else {
			hostNames[m.AgentHostID] = fmt.Sprintf("agent-%d", m.AgentHostID)
		}
		// 解锁状态摘要
		if s.unlockProbe != nil {
			if results, err := s.unlockProbe.ListByAgentHost(ctx, m.AgentHostID); err == nil {
				sum := map[string]string{}
				for _, r := range results {
					if r.Status == "unlocked" {
						region := r.Region
						if region == "" {
							region = "?"
						}
						sum[r.Service] = region
					}
				}
				unlockSummary[m.AgentHostID] = sum
			}
		}
	}
	return &ExitNodeSetDetail{
		Set:           set,
		Members:       members,
		HostName:      hostNames,
		UnlockSummary: unlockSummary,
	}, nil
}

func (s *exitNodeSetService) AddMember(ctx context.Context, req ExitNodeSetMemberRequest) error {
	weight := req.Weight
	if weight <= 0 {
		weight = 1
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	now := time.Now().Unix()
	if err := s.sets.AddMember(ctx, &repository.ExitNodeSetMember{
		SetID: req.SetID, AgentHostID: req.AgentHostID, Weight: weight, Enabled: enabled,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return err
	}
	s.logger.Info("exit node set member added", "set_id", req.SetID, "agent_host_id", req.AgentHostID, "weight", weight)
	s.notifyChange(ctx)
	return nil
}

func (s *exitNodeSetService) RemoveMember(ctx context.Context, setID, agentHostID int64) error {
	if err := s.sets.RemoveMember(ctx, setID, agentHostID); err != nil {
		return err
	}
	s.logger.Info("exit node set member removed", "set_id", setID, "agent_host_id", agentHostID)
	s.notifyChange(ctx)
	return nil
}

func (s *exitNodeSetService) UpdateMember(ctx context.Context, req ExitNodeSetMemberRequest) error {
	now := time.Now().Unix()

	// 缺省保留现值：Weight<=0 视为未提供；Enabled 为 nil 视为未提供。
	weight := 0
	enabled := false
	if members, err := s.sets.ListMembers(ctx, req.SetID); err == nil {
		for _, mcur := range members {
			if mcur != nil && mcur.AgentHostID == req.AgentHostID {
				weight = mcur.Weight
				enabled = mcur.Enabled
				break
			}
		}
	} else {
		s.logger.Warn("read current exit set members failed; falling back to request-only update",
			"set_id", req.SetID, "error", err)
	}
	if req.Weight > 0 {
		weight = req.Weight
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	if err := s.sets.UpdateMember(ctx, &repository.ExitNodeSetMember{
		SetID: req.SetID, AgentHostID: req.AgentHostID, Weight: weight, Enabled: enabled,
		UpdatedAt: now,
	}); err != nil {
		return err
	}
	s.logger.Info("exit node set member updated",
		"set_id", req.SetID, "agent_host_id", req.AgentHostID,
		"weight", weight, "enabled", enabled)

	s.notifyChange(ctx)
	return nil
}
