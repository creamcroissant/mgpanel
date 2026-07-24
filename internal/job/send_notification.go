// 文件路径: internal/job/send_notification.go
// 模块说明: 这是 internal 模块里的 send_notification 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/creamcroissant/xboard/internal/async"
	"github.com/creamcroissant/xboard/internal/notifier"
)

// SendEmailJob 处理邮件通知队列。
type SendEmailJob struct {
	Queue    *async.NotificationQueue
	Notifier notifier.Service
	Logger   *slog.Logger
}

// NewSendEmailJob 构造邮件通知任务。
func NewSendEmailJob(queue *async.NotificationQueue, notifier notifier.Service, logger *slog.Logger) *SendEmailJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &SendEmailJob{Queue: queue, Notifier: notifier, Logger: logger}
}

// Name 返回任务标识。
func (j *SendEmailJob) Name() string { return "notify.email" }

// Run 发送邮件通知。
func (j *SendEmailJob) Run(ctx context.Context) error {
	if j == nil || j.Queue == nil || j.Notifier == nil {
		return fmt.Errorf("email notification job dependencies not configured / 邮件通知任务依赖未配置")
	}
	emails := j.Queue.DrainEmails()
	if len(emails) == 0 {
		return nil
	}
	for i, req := range emails {
		if err := j.Notifier.SendEmail(ctx, req); err != nil {
			if errors.Is(err, notifier.ErrNotImplemented) {
				j.Logger.Warn("notification email not delivered", "reason", err)
				continue
			}
			j.Queue.RequeueEmail(req)
			// Requeue remaining items to prevent data loss
			for _, remaining := range emails[i+1:] {
				j.Queue.RequeueEmail(remaining)
			}
			return err
		}
	}
	j.Logger.Debug("email notifications sent", "count", len(emails))
	return nil
}

// SendTelegramJob 处理 Telegram 通知队列。
type SendTelegramJob struct {
	Queue    *async.NotificationQueue
	Notifier notifier.Service
	Logger   *slog.Logger
}

// NewSendTelegramJob 构造 Telegram 通知任务。
func NewSendTelegramJob(queue *async.NotificationQueue, notifier notifier.Service, logger *slog.Logger) *SendTelegramJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &SendTelegramJob{Queue: queue, Notifier: notifier, Logger: logger}
}

// Name 返回任务标识。
func (j *SendTelegramJob) Name() string { return "notify.telegram" }

// Run 发送 Telegram 通知。
func (j *SendTelegramJob) Run(ctx context.Context) error {
	if j == nil || j.Queue == nil || j.Notifier == nil {
		return fmt.Errorf("telegram notification job dependencies not configured / Telegram 通知任务依赖未配置")
	}
	msgs := j.Queue.DrainTelegrams()
	if len(msgs) == 0 {
		return nil
	}
	for i, req := range msgs {
		if err := j.Notifier.SendTelegram(ctx, req); err != nil {
			if errors.Is(err, notifier.ErrNotImplemented) {
				j.Logger.Warn("telegram notification not delivered", "reason", err)
				continue
			}
			j.Queue.RequeueTelegram(req)
			// Requeue remaining items to prevent data loss
			for _, remaining := range msgs[i+1:] {
				j.Queue.RequeueTelegram(remaining)
			}
			return err
		}
	}
	j.Logger.Debug("telegram notifications sent", "count", len(msgs))
	return nil
}
