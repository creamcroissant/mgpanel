// 文件路径: internal/security/audit.go
// 模块说明: 这是 internal 模块里的 audit 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package security

import (
	"context"
	"io"
	"log/slog"
	"time"
)

// Event 表示安全相关的行为（如登录或验证码验证）。
type Event struct {
	Kind      string
	ActorID   string
	IP        string
	UserAgent string
	Metadata  map[string]any
	Occurred  time.Time
}

// Recorder 记录安全事件，供后续分析。
type Recorder interface {
	Record(ctx context.Context, event Event)
}

// LoggerRecorder 将审计事件写入 slog.Logger。
type LoggerRecorder struct {
	logger *slog.Logger
}

// NewLoggerRecorder 返回记录器，写入指定 logger（为空时丢弃）。
func NewLoggerRecorder(logger *slog.Logger) *LoggerRecorder {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &LoggerRecorder{logger: logger}
}

// Record 实现 Recorder 并记录审计事件。
func (r *LoggerRecorder) Record(ctx context.Context, event Event) {
	if r == nil || r.logger == nil {
		return
	}
	if event.Occurred.IsZero() {
		event.Occurred = time.Now().UTC()
	}
	r.logger.InfoContext(ctx, "audit event", "kind", event.Kind, "actor_id", event.ActorID, "ip", event.IP, "ua", event.UserAgent, "metadata", event.Metadata, "occurred", event.Occurred.Format(time.RFC3339Nano))
}
