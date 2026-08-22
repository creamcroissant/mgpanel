package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
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
		if !e.IsDir() && strings.HasPrefix(e.Name(), "mgpanel-") && strings.HasSuffix(e.Name(), ".log") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: files}}}, nil
}

// ServerLogTailHandler 查看服务端日志文件内容
type ServerLogTailHandler struct {
	logDir   string
	maxLines int
}

func NewServerLogTailHandler(logDir string, maxLines int) *ServerLogTailHandler {
	return &ServerLogTailHandler{logDir: logDir, maxLines: maxLines}
}

func (h *ServerLogTailHandler) Name() string        { return ToolServerLogTail }
func (h *ServerLogTailHandler) Description() string { return "查看服务端日志文件内容" }
func (h *ServerLogTailHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	if params == nil {
		return nil, fmt.Errorf("filename is required")
	}
	m, ok := params.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid params")
	}
	filename, _ := m["filename"].(string)
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	// 安全校验：只允许读取日志目录下的 .log 文件
	absPath, err := filepath.Abs(filepath.Join(h.logDir, filename))
	if err != nil {
		return nil, fmt.Errorf("invalid filename: %w", err)
	}
	logDirAbs, err := filepath.Abs(h.logDir)
	if err != nil {
		return nil, fmt.Errorf("log dir: %w", err)
	}
	if !strings.HasPrefix(absPath, logDirAbs) {
		return nil, fmt.Errorf("access denied")
	}
	if !strings.HasSuffix(filename, ".log") {
		return nil, fmt.Errorf("only .log files are allowed")
	}

	lines := 50
	if l, ok := m["lines"].(float64); ok && l > 0 {
		lines = int(l)
		if lines > h.maxLines {
			lines = h.maxLines
		}
	}
	grepStr, _ := m["grep"].(string)

	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	// 读取所有行，支持 grep 过滤
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var allLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if grepStr != "" && !strings.Contains(line, grepStr) {
			continue
		}
		allLines = append(allLines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}

	// 取最后 N 行
	start := 0
	if len(allLines) > lines {
		start = len(allLines) - lines
	}
	tail := allLines[start:]
	content := strings.Join(tail, "\n")

	return &ToolCallResult{Content: []ToolContent{{Type: "text", Text: content}}}, nil
}
