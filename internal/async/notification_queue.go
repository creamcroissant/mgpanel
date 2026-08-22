// 文件路径: internal/async/notification_queue.go
// 模块说明: 这是 internal 模块里的 notification_queue 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package async

import (
	"maps"
	"sync"

	"github.com/creamcroissant/mgpanel/internal/notifier"
)

// NotificationQueue buffers outbound email & telegram tasks for background dispatch.
type NotificationQueue struct {
	mu        sync.Mutex
	emails    []notifier.EmailRequest
	telegrams []notifier.TelegramRequest
}

// NewNotificationQueue returns an empty notification queue instance.
func NewNotificationQueue() *NotificationQueue {
	return &NotificationQueue{
		emails:    make([]notifier.EmailRequest, 0),
		telegrams: make([]notifier.TelegramRequest, 0),
	}
}

// EnqueueEmail appends a pending email request.
func (q *NotificationQueue) EnqueueEmail(req notifier.EmailRequest) {
	if q == nil || req.To == "" {
		return
	}
	q.mu.Lock()
	q.emails = append(q.emails, cloneEmailRequest(req))
	q.mu.Unlock()
}

// EnqueueTelegram appends a pending telegram message.
func (q *NotificationQueue) EnqueueTelegram(req notifier.TelegramRequest) {
	if q == nil || req.ChatID == "" {
		return
	}
	q.mu.Lock()
	q.telegrams = append(q.telegrams, cloneTelegramRequest(req))
	q.mu.Unlock()
}

// DrainEmails returns all pending email requests and clears the buffer.
func (q *NotificationQueue) DrainEmails() []notifier.EmailRequest {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	drained := q.emails
	q.emails = make([]notifier.EmailRequest, 0)
	return drained
}

// DrainTelegrams returns all pending telegram messages and clears the buffer.
func (q *NotificationQueue) DrainTelegrams() []notifier.TelegramRequest {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	drained := q.telegrams
	q.telegrams = make([]notifier.TelegramRequest, 0)
	return drained
}

// PendingEmails reports buffered email tasks.
func (q *NotificationQueue) PendingEmails() int {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.emails)
}

// PendingTelegrams reports buffered telegram tasks.
func (q *NotificationQueue) PendingTelegrams() int {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.telegrams)
}

// InvalidateSettingCache is a no-op placeholder for future setting caches.
func (q *NotificationQueue) InvalidateSettingCache() {
	if q == nil {
		return
	}
}

// RequeueEmail prepends an email task for retry.
func (q *NotificationQueue) RequeueEmail(req notifier.EmailRequest) {
	if q == nil || req.To == "" {
		return
	}
	q.mu.Lock()
	q.emails = append([]notifier.EmailRequest{cloneEmailRequest(req)}, q.emails...)
	q.mu.Unlock()
}

// RequeueTelegram prepends a telegram task for retry.
func (q *NotificationQueue) RequeueTelegram(req notifier.TelegramRequest) {
	if q == nil || req.ChatID == "" {
		return
	}
	q.mu.Lock()
	q.telegrams = append([]notifier.TelegramRequest{cloneTelegramRequest(req)}, q.telegrams...)
	q.mu.Unlock()
}

func cloneEmailRequest(req notifier.EmailRequest) notifier.EmailRequest {
	cloned := req
	if len(req.Variables) > 0 {
		cloned.Variables = maps.Clone(req.Variables)
	}
	return cloned
}

func cloneTelegramRequest(req notifier.TelegramRequest) notifier.TelegramRequest {
	cloned := req
	if len(req.Variables) > 0 {
		cloned.Variables = maps.Clone(req.Variables)
	}
	return cloned
}
