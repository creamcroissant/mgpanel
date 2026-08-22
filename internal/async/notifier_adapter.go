// 文件路径: internal/async/notifier_adapter.go
// 模块说明: 这是 internal 模块里的 notifier_adapter 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package async

import (
	"context"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/notifier"
)

// QueueNotifier implements notifier.Service by enqueueing requests for background workers.
type QueueNotifier struct {
	queue *NotificationQueue
}

// NewQueueNotifier wraps a notification queue to satisfy notifier.Service for application flows.
func NewQueueNotifier(queue *NotificationQueue) notifier.Service {
	return &QueueNotifier{queue: queue}
}

// SendEmail enqueues the email request for asynchronous delivery.
func (n *QueueNotifier) SendEmail(ctx context.Context, req notifier.EmailRequest) error {
	if n == nil || n.queue == nil {
		return fmt.Errorf("notification queue unavailable / 通知队列不可用")
	}
	n.queue.EnqueueEmail(req)
	return nil
}

// SendTelegram enqueues the telegram request for asynchronous delivery.
func (n *QueueNotifier) SendTelegram(ctx context.Context, req notifier.TelegramRequest) error {
	if n == nil || n.queue == nil {
		return fmt.Errorf("notification queue unavailable / 通知队列不可用")
	}
	n.queue.EnqueueTelegram(req)
	return nil
}
