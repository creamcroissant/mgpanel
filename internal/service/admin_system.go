// 文件路径: internal/service/admin_system.go
// 模块说明: 这是 internal 模块里的 admin_system 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/support/i18n"
)

// AdminSystemService 汇总后台仪表盘需要的系统与队列状态。
type AdminSystemService interface {
	SystemStatus(ctx context.Context) (AdminSystemStatus, error)
	QueueStats(ctx context.Context) (AdminSystemQueueStats, error)
	I18n() *i18n.Manager
}

// NotificationQueueStats 提供通知队列积压指标，避免 async 包循环依赖。
type NotificationQueueStats interface {
	PendingEmails() int
	PendingTelegrams() int
}

// TrafficQueueStats 提供流量队列积压指标。
type TrafficQueueStats interface {
	Pending() int
}

// AdminSystemOptions 注入运行时依赖。
type AdminSystemOptions struct {
	Version           string
	Environment       string
	StartedAt         time.Time
	NotificationQueue NotificationQueueStats
	TrafficQueue      TrafficQueueStats
	Users             repository.UserRepository
	Servers           repository.ServerRepository
	AgentHosts        repository.AgentHostRepository
	Now               func() time.Time
	HostnameResolver  func() (string, error)
	I18n              *i18n.Manager
}

type adminSystemService struct {
	version     string
	environment string
	startedAt   time.Time
	notifier    NotificationQueueStats
	traffic     TrafficQueueStats
	users       repository.UserRepository
	servers     repository.ServerRepository
	agentHosts  repository.AgentHostRepository
	now         func() time.Time
	hostname    func() (string, error)
	i18n        *i18n.Manager
}

// AdminSystemStatus 描述管理后台系统状态返回字段。
type AdminSystemStatus struct {
	Version           string                `json:"version"`
	GoVersion         string                `json:"go_version"`
	Environment       string                `json:"environment"`
	Hostname          string                `json:"hostname"`
	StartedAt         time.Time             `json:"started_at"`
	Uptime            int64                 `json:"uptime"`
	UserCount         int64                 `json:"user_count"`
	ServerCount       int64                 `json:"server_count"`
	AgentCount        int64                 `json:"agent_count"`
	OnlineAgentCount  int64                 `json:"online_agent_count"`
	Logs              AdminSystemLogSummary `json:"logs"`
}

// AdminSystemLogSummary 聚合日志统计，暂无数据时返回零值。
type AdminSystemLogSummary struct {
	Info    int `json:"info"`
	Warning int `json:"warning"`
	Error   int `json:"error"`
	Total   int `json:"total"`
}

// AdminSystemQueueStats 近似 Horizon 风格的队列指标。
type AdminSystemQueueStats struct {
	Status                 bool              `json:"status"`
	Wait                   AdminQueueWait    `json:"wait"`
	RecentJobs             int               `json:"recentJobs"`
	JobsPerMinute          float64           `json:"jobsPerMinute"`
	QueueWithMaxThroughput AdminQueueMetric  `json:"queueWithMaxThroughput"`
	QueueWithMaxRuntime    AdminQueueRuntime `json:"queueWithMaxRuntime"`
	FailedJobs             int               `json:"failedJobs"`
	Periods                AdminQueuePeriods `json:"periods"`
	Processes              int               `json:"processes"`
	PausedMasters          int               `json:"pausedMasters"`
}

// AdminQueueWait 表示队列等待量（此处用任务数近似）。
type AdminQueueWait struct {
	Default int `json:"default"`
}

// AdminQueueMetric 描述队列吞吐量指标。
type AdminQueueMetric struct {
	Name       string `json:"name"`
	Throughput int    `json:"throughput"`
}

// AdminQueueRuntime 表示最大阻塞队列的运行指标。
type AdminQueueRuntime struct {
	Name    string `json:"name"`
	Runtime int    `json:"runtime"`
}

// AdminQueuePeriods 表示统计周期字段。
type AdminQueuePeriods struct {
	RecentJobs int `json:"recentJobs"`
	FailedJobs int `json:"failedJobs"`
}

// NewAdminSystemService 构建系统状态服务。
func NewAdminSystemService(opts AdminSystemOptions) AdminSystemService {
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	hostResolver := opts.HostnameResolver
	if hostResolver == nil {
		hostResolver = os.Hostname
	}
	return &adminSystemService{
		version:     fallbackVersion(opts.Version),
		environment: fallbackEnv(opts.Environment),
		startedAt:   startedAt,
		notifier:    opts.NotificationQueue,
		traffic:     opts.TrafficQueue,
		users:       opts.Users,
		servers:     opts.Servers,
		agentHosts:  opts.AgentHosts,
		now:         nowFn,
		hostname:    hostResolver,
		i18n:        opts.I18n,
	}
}

func (s *adminSystemService) I18n() *i18n.Manager {
	return s.i18n
}

// SystemStatus 汇总系统状态（版本、环境、计数、运行时信息）。
func (s *adminSystemService) SystemStatus(ctx context.Context) (AdminSystemStatus, error) {
	host, err := s.hostname()
	if err != nil {
		host = "unknown"
	}
	now := s.now().UTC()
	uptime := now.Unix() - s.startedAt.Unix()
	if uptime < 0 {
		uptime = 0
	}

	userCount := int64(0)
	if s.users != nil {
		count, err := s.users.Count(ctx)
		if err != nil {
			return AdminSystemStatus{}, err
		}
		userCount = count
	}

	serverCount := int64(0)
	if s.servers != nil {
		count, err := s.servers.Count(ctx)
		if err != nil {
			return AdminSystemStatus{}, err
		}
		serverCount = count
	}

	agentCount := int64(0)
	onlineAgentCount := int64(0)
	if s.agentHosts != nil {
		count, err := s.agentHosts.Count(ctx)
		if err != nil {
			return AdminSystemStatus{}, err
		}
		agentCount = count

		onlineCount, err := s.agentHosts.CountOnline(ctx)
		if err != nil {
			return AdminSystemStatus{}, err
		}
		onlineAgentCount = onlineCount
	}

	return AdminSystemStatus{
		Version:          s.version,
		GoVersion:        runtime.Version(),
		Environment:      s.environment,
		Hostname:         host,
		StartedAt:        s.startedAt,
		Uptime:           uptime,
		UserCount:        userCount,
		ServerCount:      serverCount,
		AgentCount:       agentCount,
		OnlineAgentCount: onlineAgentCount,
		Logs: AdminSystemLogSummary{
			Info:    0,
			Warning: 0,
			Error:   0,
			Total:   0,
		},
	}, nil
}

// QueueStats 汇总队列堆积与吞吐指标。
func (s *adminSystemService) QueueStats(ctx context.Context) (AdminSystemQueueStats, error) {
	pendingTraffic := 0
	if s.traffic != nil {
		pendingTraffic = s.traffic.Pending()
	}
	pendingEmails := 0
	pendingTelegrams := 0
	if s.notifier != nil {
		pendingEmails = s.notifier.PendingEmails()
		pendingTelegrams = s.notifier.PendingTelegrams()
	}
	workloads := []AdminQueueMetric{
		{Name: "traffic", Throughput: pendingTraffic},
		{Name: "email", Throughput: pendingEmails},
		{Name: "telegram", Throughput: pendingTelegrams},
	}
	maxThroughput := selectMaxThroughput(workloads)
	recentJobs := pendingTraffic + pendingEmails + pendingTelegrams
	queueRuntime := AdminQueueRuntime{Name: maxThroughput.Name, Runtime: maxThroughput.Throughput}
	return AdminSystemQueueStats{
		Status:                 true,
		Wait:                   AdminQueueWait{Default: recentJobs},
		RecentJobs:             recentJobs,
		JobsPerMinute:          float64(recentJobs),
		QueueWithMaxThroughput: maxThroughput,
		QueueWithMaxRuntime:    queueRuntime,
		FailedJobs:             0,
		Periods: AdminQueuePeriods{
			RecentJobs: 1,
			FailedJobs: 1,
		},
		Processes:     1,
		PausedMasters: 0,
	}, nil
}

// selectMaxThroughput 选择吞吐量最高的队列。
func selectMaxThroughput(metrics []AdminQueueMetric) AdminQueueMetric {
	max := AdminQueueMetric{Name: "traffic", Throughput: 0}
	for _, metric := range metrics {
		if metric.Throughput > max.Throughput {
			max = metric
		}
	}
	return max
}

// fallbackVersion 为空时回退到开发版本标识。
func fallbackVersion(version string) string {
	if strings.TrimSpace(version) == "" {
		return "go-dev"
	}
	return version
}

// fallbackEnv 为空时回退到 development。
func fallbackEnv(env string) string {
	if strings.TrimSpace(env) == "" {
		return "development"
	}
	return env
}
