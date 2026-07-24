package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/service"
)

type OperationLogsListHandler struct {
	svc service.OperationLogService
}

func NewOperationLogsListHandler(svc service.OperationLogService) *OperationLogsListHandler {
	return &OperationLogsListHandler{svc: svc}
}

func (h *OperationLogsListHandler) Name() string        { return ToolOperationLogsList }
func (h *OperationLogsListHandler) Description() string { return "获取操作日志列表" }
func (h *OperationLogsListHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	req := service.ListOperationLogsRequest{Limit: 50}
	if params != nil {
		if m, ok := params.(map[string]any); ok {
			if scope, ok := m["scope"].(string); ok {
				req.Scope = scope
			}
			if targetID, ok := m["target_id"].(string); ok {
				req.TargetID = targetID
			}
			if limit, ok := m["limit"].(float64); ok {
				req.Limit = int(limit)
			}
		}
	}
	result, err := h.svc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: result}}}, nil
}

type AccessLogsListHandler struct {
	svc service.AccessLogService
}

func NewAccessLogsListHandler(svc service.AccessLogService) *AccessLogsListHandler {
	return &AccessLogsListHandler{svc: svc}
}

func (h *AccessLogsListHandler) Name() string        { return ToolAccessLogsList }
func (h *AccessLogsListHandler) Description() string { return "获取访问日志列表" }
func (h *AccessLogsListHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	filter := repository.AccessLogFilter{Limit: 50}
	if params != nil {
		if m, ok := params.(map[string]any); ok {
			if hostID, ok := m["agent_host_id"].(float64); ok {
				id := int64(hostID)
				filter.AgentHostID = &id
			}
			if limit, ok := m["limit"].(float64); ok {
				filter.Limit = int(limit)
			}
		}
	}
	logs, _, err := h.svc.ListAccessLogs(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: logs}}}, nil
}

type ServerLogHandler struct {
	logDir   string
	maxLines int
}

func NewServerLogHandler(logDir string, maxLines int) *ServerLogHandler {
	return &ServerLogHandler{logDir: logDir, maxLines: maxLines}
}

func (h *ServerLogHandler) Name() string        { return ToolServerLogList }
func (h *ServerLogHandler) Description() string { return "列出服务端日志文件" }
func (h *ServerLogHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	entries, err := os.ReadDir(h.logDir)
	if err != nil {
		return nil, fmt.Errorf("read log dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "xboard-") && strings.HasSuffix(e.Name(), ".log") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: files}}}, nil
}
