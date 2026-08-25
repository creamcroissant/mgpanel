package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// RelayPathService 管理服务器中继链路（多跳流量走向，与入口协议配置正交）。
type RelayPathService interface {
	Create(ctx context.Context, req RelayPathUpsertRequest) (*repository.RelayPath, error)
	Update(ctx context.Context, id int64, req RelayPathUpsertRequest) (*repository.RelayPath, error)
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*repository.RelayPath, error)
	List(ctx context.Context, coreType string) ([]*repository.RelayPath, error)
	// Validate 校验链路合法性：序列长度>=2、agent 存在、无重复 agent（循环）。
	Validate(ctx context.Context, req RelayPathUpsertRequest) error
}

// RelayPathUpsertRequest 创建/更新中继链路的请求（nodes 按数组顺序即 sequence 0..N-1）。
type RelayPathUpsertRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	CoreType    string                `json:"core_type"`
	Enabled     *bool                 `json:"enabled"`
	Nodes       []RelayPathNodeInput  `json:"nodes"`
}

// RelayPathNodeInput 链路节点输入。
type RelayPathNodeInput struct {
	Sequence    int64 `json:"sequence"`
	AgentHostID int64 `json:"agent_host_id"`
}

// ErrRelayPathValidation 中继链路校验失败（错误信息已含中文细节，可直接返回前端）。
var ErrRelayPathValidation = errors.New("relay path validation failed / 中继链路校验失败")

type relayPathService struct {
	repo      repository.RelayPathRepository
	agents    repository.AgentHostRepository
	logger    *slog.Logger
}

// NewRelayPathService 组装中继链路服务。
func NewRelayPathService(repo repository.RelayPathRepository, agents repository.AgentHostRepository, logger *slog.Logger) RelayPathService {
	return &relayPathService{repo: repo, agents: agents, logger: logger}
}

func (s *relayPathService) Create(ctx context.Context, req RelayPathUpsertRequest) (*repository.RelayPath, error) {
	if err := s.Validate(ctx, req); err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	p := req.toRelayPath(now, now)
	id, err := s.repo.Create(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("create relay path: %w", err)
	}
	slog.Info("relay path created", "id", id, "name", p.Name, "hops", len(p.Nodes))
	return s.Get(ctx, id)
}

func (s *relayPathService) Update(ctx context.Context, id int64, req RelayPathUpsertRequest) (*repository.RelayPath, error) {
	if err := s.Validate(ctx, req); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p := req.toRelayPath(existing.CreatedAt, time.Now().Unix())
	p.ID = id
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("update relay path %d: %w", id, err)
	}
	slog.Info("relay path updated", "id", id, "name", p.Name, "hops", len(p.Nodes))
	return s.Get(ctx, id)
}

func (s *relayPathService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete relay path %d: %w", id, err)
	}
	slog.Info("relay path deleted", "id", id)
	return nil
}

func (s *relayPathService) Get(ctx context.Context, id int64) (*repository.RelayPath, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get relay path %d: %w", id, err)
	}
	return p, nil
}

func (s *relayPathService) List(ctx context.Context, coreType string) ([]*repository.RelayPath, error) {
	paths, err := s.repo.List(ctx, coreType)
	if err != nil {
		return nil, fmt.Errorf("list relay paths: %w", err)
	}
	return paths, nil
}

// Validate 链路合法性校验：
//   - 名称必填
//   - 序列长度 >= 2（至少入口+出口）
//   - sequence 必须从 0 连续递增
//   - 同一链路内不允许出现重复 agent（防环）
//   - 每个 agent 必须真实存在
func (s *relayPathService) Validate(ctx context.Context, req RelayPathUpsertRequest) error {
	if req.Name == "" {
		return fmt.Errorf("%w: 链路名称不能为空", ErrRelayPathValidation)
	}
	if len(req.Nodes) < 2 {
		return fmt.Errorf("%w: 链路至少需要 2 个节点（入口+出口）", ErrRelayPathValidation)
	}
	seen := make(map[int64]struct{}, len(req.Nodes))
	for i, n := range req.Nodes {
		// 数组顺序即 sequence 0..N-1（冻结契约）：忽略客户端传入的 Sequence 零值/噪声，
		// 以下标归一化，避免“未显式传序号”被误判为非连续。
		n.Sequence = int64(i)
		req.Nodes[i] = n
		if n.AgentHostID <= 0 {
			return fmt.Errorf("%w: 第 %d 跳缺少有效的 agent_host_id", ErrRelayPathValidation, i)
		}
		if _, dup := seen[n.AgentHostID]; dup {
			return fmt.Errorf("%w: 链路中出现重复节点 agent=%d（禁止环路）", ErrRelayPathValidation, n.AgentHostID)
		}
		seen[n.AgentHostID] = struct{}{}
		if _, err := s.agents.FindByID(ctx, n.AgentHostID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return fmt.Errorf("%w: 第 %d 跳的 agent(%d) 不存在", ErrRelayPathValidation, i, n.AgentHostID)
			}
			return fmt.Errorf("validate relay path agent %d: %w", n.AgentHostID, err)
		}
	}
	return nil
}

// toRelayPath 将请求转换为领域对象（Nodes 按 slice 下标重排 sequence，保证连续）。
func (req RelayPathUpsertRequest) toRelayPath(createdAt, updatedAt int64) *repository.RelayPath {
	coreType := req.CoreType
	if coreType == "" {
		coreType = "sing-box"
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	nodes := make([]repository.RelayPathNode, len(req.Nodes))
	for i, n := range req.Nodes {
		nodes[i] = repository.RelayPathNode{Sequence: i, AgentHostID: n.AgentHostID}
	}
	return &repository.RelayPath{
		Name:        req.Name,
		Description: req.Description,
		CoreType:    coreType,
		Enabled:     enabled,
		Nodes:       nodes,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
