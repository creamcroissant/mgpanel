package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/agent/command"
	"github.com/creamcroissant/mgpanel/internal/agent/unlock"
)

const agentCommandActionUnlockProbe = "unlock_probe"

// toUnlockServices 将配置中的平台名列表转为 unlock.Service 列表。
// 无法识别的名称被忽略；空输入返回 nil（表示使用默认列表）。
func toUnlockServices(names []string) []unlock.Service {
	var out []unlock.Service
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		svc := unlock.Service(strings.ToLower(n))
		// 仅接受已知平台，避免误配置
		for _, known := range unlock.AllServices() {
			if svc == known {
				out = append(out, svc)
				break
			}
		}
	}
	return out
}

// registerUnlockProbeHandler 注册 unlock_probe 命令，面板可下发手动触发解锁检测。
func (a *Agent) registerUnlockProbeHandler() error {
	if a.commandQueue == nil {
		return nil
	}
	return a.commandQueue.Register(agentCommandActionUnlockProbe, func(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
		if a.unlockMgr == nil {
			return command.Result{
				Status:       command.StatusFailed,
				Level:        command.LevelError,
				Message:      "unlock probe disabled / 解锁检测未启用",
				ErrorMessage: "unlock probe disabled",
				Terminal:     true,
			}
		}
		// 支持可选的 services 参数
		var payload struct {
			Services []string `json:"services,omitempty"`
		}
		_ = json.Unmarshal(task.RequestPayload, &payload)
		if len(payload.Services) > 0 {
			a.unlockMgr.ProbeAndReportWith(ctx, toUnlockServices(payload.Services))
		} else {
			a.unlockMgr.ProbeAndReport(ctx)
		}
		return command.Result{
			Status:   command.StatusSuccess,
			Level:    command.LevelInfo,
			Message:  "unlock probe completed",
			Terminal: true,
		}
	})
}
