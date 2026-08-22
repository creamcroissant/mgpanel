package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/mgpanel/internal/async"
	"github.com/creamcroissant/mgpanel/internal/notifier"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

// NodeHeartbeatJob 负责节点心跳检测与离线告警。
type NodeHeartbeatJob struct {
	servers           repository.ServerRepository
	notificationQueue *async.NotificationQueue
	settings          repository.SettingRepository
	logger            *slog.Logger
	timeout           time.Duration
}

// NewNodeHeartbeatJob 构造心跳检测任务。
func NewNodeHeartbeatJob(servers repository.ServerRepository, notificationQueue *async.NotificationQueue, settings repository.SettingRepository, logger *slog.Logger) *NodeHeartbeatJob {
	return &NodeHeartbeatJob{
		servers:           servers,
		notificationQueue: notificationQueue,
		settings:          settings,
		logger:            logger,
		timeout:           5 * time.Minute, // 5 分钟未上报视为离线
	}
}

// Name 返回任务标识。
func (j *NodeHeartbeatJob) Name() string {
	return "node-heartbeat"
}

// Run 扫描节点心跳并更新在线状态。
func (j *NodeHeartbeatJob) Run(ctx context.Context) error {
	j.logger.Debug("Running node heartbeat check")

	servers, err := j.servers.FindAllVisible(ctx)
	if err != nil {
		j.logger.Error("Failed to list servers for heartbeat check", "error", err)
		return err
	}

	now := time.Now().Unix()
	offlineThreshold := now - int64(j.timeout.Seconds())

	for _, server := range servers {
		// 在线但心跳超时，标记离线
		if server.Status > 0 && server.LastHeartbeatAt < offlineThreshold {
			server.Status = 0
			// 更新为离线状态（Update 会刷新 UpdatedAt）
			if err := j.servers.Update(ctx, server); err != nil {
				j.logger.Error("Failed to mark server offline", "server_id", server.ID, "error", err)
			} else {
				j.logger.Info("Marked server offline due to heartbeat timeout", "server_id", server.ID)
				// 发送离线通知
				j.sendOfflineNotification(ctx, server)
			}
		} else if server.Status == 0 && server.LastHeartbeatAt >= offlineThreshold {
			// 离线但心跳恢复，标记在线
			server.Status = 1
			if err := j.servers.Update(ctx, server); err != nil {
				j.logger.Error("Failed to mark server online", "server_id", server.ID, "error", err)
			} else {
				j.logger.Info("Marked server online due to recent heartbeat", "server_id", server.ID)
			}
		}
	}

	return nil
}

// sendOfflineNotification 发送节点离线通知。
func (j *NodeHeartbeatJob) sendOfflineNotification(ctx context.Context, server *repository.Server) {
	if j.notificationQueue == nil || j.settings == nil {
		return
	}

	adminIDSetting, err := j.settings.Get(ctx, "telegram_admin_id")
	if err != nil || adminIDSetting == nil || adminIDSetting.Value == "" {
		// No admin configured
		return
	}

	j.notificationQueue.EnqueueTelegram(notifier.TelegramRequest{
		ChatID:    adminIDSetting.Value,
		Message:   fmt.Sprintf("⚠️ *Node Offline Alert*\n\nNode: %s (ID: %d)\nLast Heartbeat: %s", server.Name, server.ID, time.Unix(server.LastHeartbeatAt, 0).Format(time.RFC1123)),
		ParseMode: "Markdown",
	})
}
