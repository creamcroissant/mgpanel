// 文件路径: internal/notifier/notifier.go
// 模块说明: 这是 internal 模块里的 notifier 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package notifier

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// EmailRequest 描述邮件通知请求。
type EmailRequest struct {
	To        string
	Subject   string
	Template  string
	Body      string
	Variables map[string]any
}

// TelegramRequest 描述通过机器人发送的 Telegram 消息。
type TelegramRequest struct {
	ChatID    string
	Message   string
	ParseMode string
	Variables map[string]any
}

// Service 提供认证流程使用的统一通知能力。
type Service interface {
	SendEmail(ctx context.Context, req EmailRequest) error
	SendTelegram(ctx context.Context, req TelegramRequest) error
}

// ErrNotImplemented 表示未配置真实通知通道。
var ErrNotImplemented = errors.New("notifier: not implemented")

// LoggerService 将通知意图写入日志，适用于测试或引导期。
type LoggerService struct {
	logger *slog.Logger
}

// NewLoggerService 创建仅记录日志的通知服务。
func NewLoggerService(logger *slog.Logger) *LoggerService {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &LoggerService{logger: logger}
}

// SendEmail 记录邮件通知请求。
func (s *LoggerService) SendEmail(ctx context.Context, req EmailRequest) error {
	if strings.TrimSpace(req.To) == "" {
		return fmt.Errorf("recipient is required / 收件人不能为空")
	}
	s.logger.InfoContext(ctx, "email notification", "to", req.To, "subject", req.Subject, "template", req.Template)
	return ErrNotImplemented
}

// SendTelegram 记录 Telegram 通知请求。
func (s *LoggerService) SendTelegram(ctx context.Context, req TelegramRequest) error {
	if strings.TrimSpace(req.ChatID) == "" {
		return fmt.Errorf("telegram chat_id is required / Telegram chat_id 不能为空")
	}
	s.logger.InfoContext(ctx, "telegram notification", "chat_id", req.ChatID, "parse_mode", req.ParseMode)
	return ErrNotImplemented
}
