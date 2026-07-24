// 文件路径: internal/job/scheduler.go
// 模块说明: 这是 internal 模块里的 scheduler 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package job

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Runnable 表示由调度器触发的后台任务。
type Runnable interface {
	Name() string
	Run(ctx context.Context) error
}

// Scheduler 封装 cron，并提供日志与优雅停机。
type Scheduler struct {
	cron    *cron.Cron
	logger  *slog.Logger
	mu      sync.Mutex
	started bool
}

const defaultJobTimeout = 2 * time.Minute

// NewScheduler 构建支持秒与自然描述的调度器。
func NewScheduler(logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	c := cron.New(cron.WithParser(parser))
	return &Scheduler{cron: c, logger: logger}
}

// Register 绑定 cron 表达式与任务。
func (s *Scheduler) Register(spec string, runnable Runnable) (cron.EntryID, error) {
	if runnable == nil {
		return 0, fmt.Errorf("scheduler: runnable is required / runnable 不能为空")
	}
	if spec == "" {
		return 0, fmt.Errorf("scheduler: spec is required / spec 不能为空")
	}
	entryID, err := s.cron.AddFunc(spec, s.wrap(runnable))
	if err != nil {
		return 0, err
	}
	s.logger.Info("job registered", "job", runnable.Name(), "spec", spec)
	return entryID, nil
}

// Start 启动调度器并执行任务。
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.cron.Start()
	s.started = true
	s.mu.Unlock()
}

// Stop 停止调度器并等待执行中的任务结束。
func (s *Scheduler) Stop() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return context.Background()
	}
	s.started = false
	return s.cron.Stop()
}

// wrap 包装任务，提供超时与统一日志。
func (s *Scheduler) wrap(runnable Runnable) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("job panicked",
					"job", runnable.Name(),
					"panic", r,
					"stack", string(debug.Stack()))
			}
		}()
		// Use context.Background() as the parent for job execution contexts.
		// robfig/cron's shutdown context only completes after all jobs finish,
		// so using it would prevent jobs from responding to Scheduler.Stop().
		// Individual jobs should implement their own timeout via context.WithTimeout.
		ctx, cancel := context.WithTimeout(context.Background(), defaultJobTimeout)
		defer cancel()
		start := time.Now()
		if err := runnable.Run(ctx); err != nil {
			s.logger.Error("job failed", "job", runnable.Name(), "error", err, "elapsed", time.Since(start))
			return
		}
		s.logger.Debug("job completed", "job", runnable.Name(), "elapsed", time.Since(start))
	}
}
