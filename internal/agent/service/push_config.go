package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/command"
	"github.com/creamcroissant/mgpanel/internal/agent/config"
	"github.com/creamcroissant/mgpanel/internal/agent/initsys"
)

// agentCommandActionPushConfig 是 panel → agent 的"下发全量 agent 配置"命令 key。
// 与 report_config/sync_users/geo_refresh 同模式：面板派发 agent_lifecycle_operation
// type=push_config，agent 在 syncAgentCommands 周期拉到本地 queue 并执行。
const agentCommandActionPushConfig = "push_config"

// agentServiceName 是 agent 自身的服务名，与 deploy/agent.service 安装的单元名一致。
// 重启走 initsys 抽象（systemd/openrc/runit 自动适配），禁止硬编码 systemctl。
const agentServiceName = "mgpanel-agent"

// pushConfigPayload 是 push_config 命令的请求体：全量 config.yml 内容。
type pushConfigPayload struct {
	ConfigYAML string `json:"config_yaml"`
}

// registerPushConfigHandler 向 commandQueue 注册 push_config 命令处理器。
func (a *Agent) registerPushConfigHandler() error {
	if a == nil || a.commandQueue == nil {
		return nil
	}
	return a.commandQueue.Register(agentCommandActionPushConfig, a.handlePushConfig)
}

// handlePushConfig 是 push_config 命令的 handler 主体：
// 1) 解析 payload 取 config_yaml（缺失/空 → failed，原文件不动）；
// 2) 备份当前 config.yml 为 config.yml.bak.<时间戳>；
// 3) 写新文件后用 config.Load() 校验，非法则回滚备份并报 failed；
// 4) 合法则刷新本地缓存、先返回成功（由 queue 上报 panel），再经 initsys 异步重启自己生效。
func (a *Agent) handlePushConfig(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	fail := func(message, errMessage string) command.Result {
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "completed",
			Level:        command.LevelError,
			Message:      message,
			ErrorMessage: errMessage,
			Terminal:     true,
		}
	}

	var payload pushConfigPayload
	if err := json.Unmarshal(task.RequestPayload, &payload); err != nil {
		slog.Warn("push_config: invalid payload", "command_id", task.ID, "error", err)
		return fail("push_config payload 无效 / invalid payload", err.Error())
	}
	if strings.TrimSpace(payload.ConfigYAML) == "" {
		slog.Warn("push_config: empty config_yaml", "command_id", task.ID)
		return fail("push_config payload 缺少 config_yaml / missing config_yaml", "config_yaml is empty")
	}

	a.configFileMu.RLock()
	path := a.configFilePath
	a.configFileMu.RUnlock()
	if strings.TrimSpace(path) == "" {
		return fail("push_config 未配置 config 文件路径 / config file path not set", "config file path is empty")
	}

	original, err := os.ReadFile(path)
	if err != nil {
		slog.Error("push_config: read current config failed", "path", path, "error", err)
		return fail("push_config 读取当前配置失败 / read current config failed", err.Error())
	}
	// 沿用原文件权限，避免备份/新文件权限漂移。
	mode := os.FileMode(0o600)
	if fi, statErr := os.Stat(path); statErr == nil {
		mode = fi.Mode().Perm()
	}

	backupPath := fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backupPath, original, mode); err != nil {
		slog.Error("push_config: backup failed", "path", backupPath, "error", err)
		return fail("push_config 备份当前配置失败 / backup failed", err.Error())
	}

	if err := os.WriteFile(path, []byte(payload.ConfigYAML), mode); err != nil {
		slog.Error("push_config: write new config failed", "path", path, "error", err)
		return fail("push_config 写入新配置失败 / write new config failed", err.Error())
	}

	if _, err := config.Load(path); err != nil {
		slog.Error("push_config: new config invalid, rolling back", "path", path, "error", err)
		if rbErr := os.WriteFile(path, original, mode); rbErr != nil {
			slog.Error("push_config: rollback failed", "path", path, "error", rbErr)
			return fail("push_config 新配置非法且回滚失败 / invalid config and rollback failed",
				fmt.Sprintf("validate: %v; rollback: %v", err, rbErr))
		}
		return fail("push_config 新配置非法，已回滚 / invalid config, rolled back", err.Error())
	}

	// 刷新本地缓存，下次 StatusReport 携带最新内容。
	if _, err := a.readConfigFile(); err != nil {
		slog.Warn("push_config: refresh cache failed", "error", err)
	}

	slog.Info("push_config applied, restarting agent service",
		"command_id", task.ID, "backup", backupPath)
	restartAgentServiceAsync()

	return command.Result{
		Status:   command.StatusSuccess,
		Phase:    "completed",
		Level:    command.LevelInfo,
		Message:  "config pushed and validated, agent restarting / 配置已下发校验通过，agent 重启中",
		Terminal: true,
	}
}

// restartAgentServiceAsync 经 initsys 抽象异步重启 agent 自身服务。
// 延迟重启让成功结果先经 commandQueue 上报 panel；测试模式下跳过（禁止操作宿主服务）。
func restartAgentServiceAsync() {
	if testing.Testing() {
		return
	}
	go func() {
		time.Sleep(2 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := initsys.Detect().Restart(ctx, agentServiceName); err != nil {
			slog.Error("push_config: self restart failed", "service", agentServiceName, "error", err)
		}
	}()
}
