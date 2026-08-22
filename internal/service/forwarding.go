package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/creamcroissant/mgpanel/internal/repository"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// 校验规则常量
const (
	// 端口范围
	MinPort = 1
	MaxPort = 65535

	// 协议类型
	ProtocolTCP  = "tcp"
	ProtocolUDP  = "udp"
	ProtocolBoth = "both"

	// 审计日志 Action 类型
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionApply  = "apply"
	ActionFail   = "fail"
)

// ValidProtocols 定义有效的协议类型
var ValidProtocols = []string{ProtocolTCP, ProtocolUDP, ProtocolBoth}

// ForwardingService 管理端口转发规则的业务逻辑
type ForwardingService interface {
	// CRUD 操作
	CreateRule(ctx context.Context, req CreateForwardingRuleRequest) (*repository.ForwardingRule, error)
	UpdateRule(ctx context.Context, id int64, req UpdateForwardingRuleRequest) (*repository.ForwardingRule, error)
	DeleteRule(ctx context.Context, id int64, operatorID *int64) error
	GetRule(ctx context.Context, id int64) (*repository.ForwardingRule, error)

	// 查询操作
	ListByAgent(ctx context.Context, agentHostID int64) ([]*repository.ForwardingRule, error)
	ListEnabledByAgent(ctx context.Context, agentHostID int64) ([]*repository.ForwardingRule, error)

	// 版本管理 (供 gRPC 使用)
	GetVersionForAgent(ctx context.Context, agentHostID int64) (int64, error)

	// Agent 应用结果上报
	LogApplyResult(ctx context.Context, agentHostID int64, ruleID *int64, success bool, detail string) error

	// 审计日志查询
	GetLogs(ctx context.Context, filter repository.ForwardingRuleLogFilter) ([]*repository.ForwardingRuleLog, error)
	CountLogs(ctx context.Context, filter repository.ForwardingRuleLogFilter) (int64, error)
}

// CreateForwardingRuleRequest 创建转发规则请求
type CreateForwardingRuleRequest struct {
	AgentHostID   int64  // 关联的 Agent 主机 ID
	Name          string // 规则名称
	Protocol      string // tcp/udp/both
	ListenPort    int    // 本地监听端口
	TargetAddress string // 目标地址
	TargetPort    int    // 目标端口
	Enabled       bool   // 是否启用
	Priority      int    // 优先级
	Remark        string // 备注
	OperatorID    *int64 // 操作人 ID（管理员）
}

// UpdateForwardingRuleRequest 更新转发规则请求
type UpdateForwardingRuleRequest struct {
	Name          *string // 规则名称
	Protocol      *string // tcp/udp/both
	ListenPort    *int    // 本地监听端口
	TargetAddress *string // 目标地址
	TargetPort    *int    // 目标端口
	Enabled       *bool   // 是否启用
	Priority      *int    // 优先级
	Remark        *string // 备注
	OperatorID    *int64  // 操作人 ID（管理员）
}

// 校验错误
var (
	ErrInvalidProtocol      = errors.New("invalid protocol: must be tcp, udp, or both / 协议无效：仅支持 tcp、udp 或 both")
	ErrInvalidListenPort    = errors.New("invalid listen port: must be between 1 and 65535 / 监听端口无效：必须在 1-65535 之间")
	ErrInvalidTargetPort    = errors.New("invalid target port: must be between 1 and 65535 / 目标端口无效：必须在 1-65535 之间")
	ErrInvalidTargetAddress = errors.New("invalid target address: must be a valid IP / 目标地址无效：需为合法 IP")
	ErrPortConflict         = errors.New("port conflict: another rule is already using this port/protocol combination / 端口冲突：该端口与协议组合已被占用")
	ErrRuleNameRequired     = errors.New("rule name is required / 规则名称不能为空")
	ErrAgentHostRequired    = errors.New("agent host ID is required / 必须指定节点 ID")
)

// forwardingService 实现 ForwardingService 接口
type forwardingService struct {
	rules      repository.ForwardingRuleRepository
	logs       repository.ForwardingRuleLogRepository
	agentHosts repository.AgentHostRepository
	logger     *slog.Logger
}

// NewForwardingService 创建转发规则服务
func NewForwardingService(
	rules repository.ForwardingRuleRepository,
	logs repository.ForwardingRuleLogRepository,
	agentHosts repository.AgentHostRepository,
) ForwardingService {
	return NewForwardingServiceWithLogger(rules, logs, agentHosts, nil)
}

func NewForwardingServiceWithLogger(
	rules repository.ForwardingRuleRepository,
	logs repository.ForwardingRuleLogRepository,
	agentHosts repository.AgentHostRepository,
	logger *slog.Logger,
) ForwardingService {
	if logger == nil {
		logger = slog.Default()
	}
	return &forwardingService{
		rules:      rules,
		logs:       logs,
		agentHosts: agentHosts,
		logger:     logger,
	}
}

// CreateRule 创建转发规则
func (s *forwardingService) CreateRule(ctx context.Context, req CreateForwardingRuleRequest) (*repository.ForwardingRule, error) {
	// 创建流程：校验参数 -> 校验 Agent -> 端口冲突检测 -> 创建 -> 审计日志
	// 校验请求
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// 验证 AgentHost 存在
	if _, err := s.agentHosts.FindByID(ctx, req.AgentHostID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("agent host not found: %d / 节点不存在: %d", req.AgentHostID, req.AgentHostID)
		}
		return nil, fmt.Errorf("check agent host failed: %v / 校验节点失败: %w", err, err)
	}

	// 检测端口冲突
	conflict, err := s.rules.CheckPortConflict(ctx, req.AgentHostID, req.ListenPort, req.Protocol, 0)
	if err != nil {
		return nil, fmt.Errorf("check port conflict failed: %v / 校验端口冲突失败: %w", err, err)
	}
	if conflict {
		return nil, ErrPortConflict
	}

	// 创建规则
	rule := &repository.ForwardingRule{
		AgentHostID:   req.AgentHostID,
		Name:          req.Name,
		Protocol:      req.Protocol,
		ListenPort:    req.ListenPort,
		TargetAddress: req.TargetAddress,
		TargetPort:    req.TargetPort,
		Enabled:       req.Enabled,
		Priority:      req.Priority,
		Remark:        req.Remark,
	}

	if err := s.rules.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("create rule: %w", err)
	}

	// 日志记录失败不影响主流程，仅记录警告
	if err := s.logAction(ctx, rule.ID, rule.AgentHostID, ActionCreate, req.OperatorID, rule); err != nil {
		s.logger.Warn("failed to write forwarding audit log", "error", err, "rule_id", rule.ID, "agent_host_id", rule.AgentHostID, "request_id", chiMiddleware.GetReqID(ctx))
	}

	return rule, nil
}

// UpdateRule 更新转发规则
func (s *forwardingService) UpdateRule(ctx context.Context, id int64, req UpdateForwardingRuleRequest) (*repository.ForwardingRule, error) {
	// 获取现有规则
	rule, err := s.rules.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 记录更新前的状态用于审计
	// 便于后续审计日志对比变更内容
	oldRule := *rule

	// 应用更新
	if req.Name != nil {
		if *req.Name == "" {
			return nil, ErrRuleNameRequired
		}
		rule.Name = *req.Name
	}
	if req.Protocol != nil {
		if !isValidProtocol(*req.Protocol) {
			return nil, ErrInvalidProtocol
		}
		rule.Protocol = *req.Protocol
	}
	if req.ListenPort != nil {
		if !isValidPort(*req.ListenPort) {
			return nil, ErrInvalidListenPort
		}
		rule.ListenPort = *req.ListenPort
	}
	if req.TargetAddress != nil {
		if !isValidAddress(*req.TargetAddress) {
			return nil, ErrInvalidTargetAddress
		}
		rule.TargetAddress = *req.TargetAddress
	}
	if req.TargetPort != nil {
		if !isValidPort(*req.TargetPort) {
			return nil, ErrInvalidTargetPort
		}
		rule.TargetPort = *req.TargetPort
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.Remark != nil {
		rule.Remark = *req.Remark
	}

	// 检测端口冲突（排除当前规则）
	conflict, err := s.rules.CheckPortConflict(ctx, rule.AgentHostID, rule.ListenPort, rule.Protocol, rule.ID)
	if err != nil {
		return nil, fmt.Errorf("check port conflict failed: %v / 校验端口冲突失败: %w", err, err)
	}
	if conflict {
		return nil, ErrPortConflict
	}

	// 保存更新
	if err := s.rules.Update(ctx, rule); err != nil {
		return nil, fmt.Errorf("update rule: %w", err)
	}

	// 记录审计日志（包含变更详情）
	changeDetail := map[string]interface{}{
		"before": oldRule,
		"after":  rule,
	}
	if err := s.logAction(ctx, rule.ID, rule.AgentHostID, ActionUpdate, req.OperatorID, changeDetail); err != nil {
		s.logger.Warn("failed to write forwarding audit log", "error", err, "rule_id", rule.ID, "agent_host_id", rule.AgentHostID, "request_id", chiMiddleware.GetReqID(ctx))
	}

	return rule, nil
}

// DeleteRule 删除转发规则
func (s *forwardingService) DeleteRule(ctx context.Context, id int64, operatorID *int64) error {
	// 删除流程：先记录审计，再删除规则
	// 获取规则信息用于审计
	rule, err := s.rules.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 先记录审计日志（删除后 rule_id 仍可记录）
	if err := s.logAction(ctx, id, rule.AgentHostID, ActionDelete, operatorID, rule); err != nil {
		s.logger.Warn("failed to write forwarding audit log", "error", err, "rule_id", id, "agent_host_id", rule.AgentHostID, "request_id", chiMiddleware.GetReqID(ctx))
	}

	// 删除规则
	if err := s.rules.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}

	return nil
}

// GetRule 获取单条规则
func (s *forwardingService) GetRule(ctx context.Context, id int64) (*repository.ForwardingRule, error) {
	return s.rules.FindByID(ctx, id)
}

// ListByAgent 列出指定 Agent 的所有规则
func (s *forwardingService) ListByAgent(ctx context.Context, agentHostID int64) ([]*repository.ForwardingRule, error) {
	// 规则列表用于前端展示与 Agent 同步
	return s.rules.ListByAgentHostID(ctx, agentHostID)
}

// ListEnabledByAgent 列出指定 Agent 的所有启用规则
func (s *forwardingService) ListEnabledByAgent(ctx context.Context, agentHostID int64) ([]*repository.ForwardingRule, error) {
	return s.rules.ListEnabledByAgentHostID(ctx, agentHostID)
}

// GetVersionForAgent 获取 Agent 的规则版本（用于增量同步）
func (s *forwardingService) GetVersionForAgent(ctx context.Context, agentHostID int64) (int64, error) {
	return s.rules.GetMaxVersion(ctx, agentHostID)
}

// LogApplyResult 记录 Agent 应用规则的结果
func (s *forwardingService) LogApplyResult(ctx context.Context, agentHostID int64, ruleID *int64, success bool, detail string) error {
	action := ActionApply
	if !success {
		action = ActionFail
	}

	log := &repository.ForwardingRuleLog{
		RuleID:      ruleID,
		AgentHostID: agentHostID,
		Action:      action,
		OperatorID:  nil, // Agent 上报时无操作人
		Detail:      detail,
	}

	return s.logs.Create(ctx, log)
}

// GetLogs 获取审计日志列表
func (s *forwardingService) GetLogs(ctx context.Context, filter repository.ForwardingRuleLogFilter) ([]*repository.ForwardingRuleLog, error) {
	return s.logs.List(ctx, filter)
}

// CountLogs 获取审计日志总数
func (s *forwardingService) CountLogs(ctx context.Context, filter repository.ForwardingRuleLogFilter) (int64, error) {
	return s.logs.Count(ctx, filter)
}

// validateCreateRequest 校验创建请求
func (s *forwardingService) validateCreateRequest(req CreateForwardingRuleRequest) error {
	if req.AgentHostID == 0 {
		return ErrAgentHostRequired
	}
	if req.Name == "" {
		return ErrRuleNameRequired
	}
	if !isValidProtocol(req.Protocol) {
		return ErrInvalidProtocol
	}
	if !isValidPort(req.ListenPort) {
		return ErrInvalidListenPort
	}
	if !isValidPort(req.TargetPort) {
		return ErrInvalidTargetPort
	}
	if !isValidAddress(req.TargetAddress) {
		return ErrInvalidTargetAddress
	}
	return nil
}

// logAction 记录审计日志
func (s *forwardingService) logAction(ctx context.Context, ruleID, agentHostID int64, action string, operatorID *int64, detail interface{}) error {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		detailJSON = []byte("{}")
	}

	ruleIDPtr := &ruleID
	if action == ActionDelete {
		// 删除操作后规则 ID 可能无效，但仍记录便于追溯
	}

	log := &repository.ForwardingRuleLog{
		RuleID:      ruleIDPtr,
		AgentHostID: agentHostID,
		Action:      action,
		OperatorID:  operatorID,
		Detail:      string(detailJSON),
	}

	return s.logs.Create(ctx, log)
}

// isValidProtocol 校验协议是否有效
func isValidProtocol(protocol string) bool {
	for _, p := range ValidProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}

// isValidPort 校验端口是否有效
func isValidPort(port int) bool {
	return port >= MinPort && port <= MaxPort
}

// isValidAddress 校验目标地址是否有效（仅 IP）
func isValidAddress(addr string) bool {
	if addr == "" {
		return false
	}
	return net.ParseIP(addr) != nil
}
