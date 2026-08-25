// 文件路径: internal/job/scheduler.go
// 模块说明: 这是 internal 模块里的 scheduler 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package job

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
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

// RegisterOption 配置 RegisterOpts 的行为。
type RegisterOption func(*registerOptions)

type registerOptions struct {
	skipIfBusy bool
	jitterMax  time.Duration
	timeout    time.Duration // 单次执行的上下文超时；零值回落 defaultJobTimeout
}

// SkipIfBusy 让任务在上一次执行未完成时跳过本次触发（防止重叠执行）。
func SkipIfBusy() RegisterOption {
	return func(o *registerOptions) { o.skipIfBusy = true }
}

// WithTimeout 覆盖单次任务执行的上下文超时（默认 defaultJobTimeout）。
// 长耗时任务（如大表清理）应显式设置更长的超时。
func WithTimeout(d time.Duration) RegisterOption {
	return func(o *registerOptions) { o.timeout = d }
}

// WithJitter 让每次触发在执行前随机延迟 [0, jitterMax)，避免多个任务同时爆发。
func WithJitter(max time.Duration) RegisterOption {
	return func(o *registerOptions) { o.jitterMax = max }
}

// Register 绑定 cron 表达式与任务（无选项）。
func (s *Scheduler) Register(spec string, runnable Runnable) (cron.EntryID, error) {
	return s.RegisterOpts(spec, runnable)
}

// RegisterOpts 绑定 cron 表达式与任务，支持并发抑制与错峰选项。
func (s *Scheduler) RegisterOpts(spec string, runnable Runnable, opts ...RegisterOption) (cron.EntryID, error) {
	var cfg registerOptions
	for _, opt := range opts {
		opt(&cfg)
	}
	if runnable == nil {
		return 0, fmt.Errorf("scheduler: runnable is required / runnable 不能为空")
	}
	if spec == "" {
		return 0, fmt.Errorf("scheduler: spec is required / spec 不能为空")
	}
	entryID, err := s.cron.AddFunc(spec, s.wrap(runnable, cfg))
	if err != nil {
		return 0, err
	}
	flags := ""
	if cfg.skipIfBusy {
		flags += " skip_if_busy"
	}
	if cfg.jitterMax > 0 {
		flags += " jitter"
	}
	s.logger.Info("job registered", "job", runnable.Name(), "spec", spec, "options", flags)
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

// wrap 包装任务，提供超时、统一日志、并发抑制与错峰。
func (s *Scheduler) wrap(runnable Runnable, cfg registerOptions) func() {
	var mu sync.Mutex
	busy := false
	return func() {
		if cfg.skipIfBusy {
			mu.Lock()
			if busy {
				mu.Unlock()
				s.logger.Warn("job skipped (previous run still in progress)", "job", runnable.Name())
				return
			}
			busy = true
			mu.Unlock()
			defer func() {
				mu.Lock()
				busy = false
				mu.Unlock()
			}()
		}
		if cfg.jitterMax > 0 {
			delay := time.Duration(rand.Int63n(int64(cfg.jitterMax)))
			if delay > 0 {
				time.Sleep(delay)
			}
		}
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
		timeout := cfg.timeout
		if timeout <= 0 {
			timeout = defaultJobTimeout
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		start := time.Now()
		if err := runnable.Run(ctx); err != nil {
			s.logger.Error("job failed", "job", runnable.Name(), "error", err, "elapsed", time.Since(start))
			return
		}
		s.logger.Debug("job completed", "job", runnable.Name(), "elapsed", time.Since(start))
	}
}
