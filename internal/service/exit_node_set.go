package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ExitNodeSetService 管理出口节点集合（负载均衡/故障转移的成员分组）。
type ExitNodeSetService interface {
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
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
	Strategy    string                 `json:"strategy"`
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
	Set      *repository.ExitNodeSet     `json:"set"`
	Members  []*repository.ExitNodeSetMember `json:"members"`
	HostName map[int64]string `json:"host_name"` // agent_host_id -> host 显示名
}

type exitNodeSetService struct {
	sets        repository.ExitNodeSetRepository
	agentHosts  repository.AgentHostRepository
	unlockProbe repository.UnlockProbeResultRepository
	logger      *slog.Logger
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
	// 名称唯一
	if _, err := s.sets.FindByName(ctx, set.Name); err == nil {
		return nil, fmt.Errorf("exit node set name already exists / 出口集合名称已存在")
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
	return set, nil
}

func (s *exitNodeSetService) Delete(ctx context.Context, id int64) error {
	if err := s.sets.Delete(ctx, id); err != nil {
		return err
	}
	s.logger.Info("exit node set deleted", "set_id", id)
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
		Set:      set,
		Members:  members,
		HostName: hostNames,
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
	return nil
}

func (s *exitNodeSetService) RemoveMember(ctx context.Context, setID, agentHostID int64) error {
	if err := s.sets.RemoveMember(ctx, setID, agentHostID); err != nil {
		return err
	}
	s.logger.Info("exit node set member removed", "set_id", setID, "agent_host_id", agentHostID)
	return nil
}

func (s *exitNodeSetService) UpdateMember(ctx context.Context, req ExitNodeSetMemberRequest) error {
	now := time.Now().Unix()
	if err := s.sets.UpdateMember(ctx, &repository.ExitNodeSetMember{
		SetID: req.SetID, AgentHostID: req.AgentHostID, Weight: req.Weight, Enabled: req.Enabled != nil && *req.Enabled,
		UpdatedAt: now,
	}); err != nil {
		return err
	}
	s.logger.Info("exit node set member updated", "set_id", req.SetID, "agent_host_id", req.AgentHostID)
	return nil
}
