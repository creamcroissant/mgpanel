package loguploader

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/xboard/internal/agent/transport"
	agentv1 "github.com/creamcroissant/xboard/pkg/pb/agent/v1"
)

// Uploader periodically reads agent log files and uploads via gRPC.
type Uploader struct {
	client   *transport.GRPCClient
	logDir   string
	enabled  bool
	maxLines int
	interval time.Duration
	source   string
	logger   *slog.Logger

	stopCh   chan struct{}
	stopOnce sync.Once
}

// LogUploadConfig mirrors the config struct for decoupling.
type LogUploadConfig struct {
	Enabled         bool
	MaxLines        int
	IntervalSeconds int
	Source          string
}

// NewUploader creates a log uploader.
func NewUploader(client *transport.GRPCClient, logDir string, cfg LogUploadConfig, logger *slog.Logger) *Uploader {
	if logger == nil {
		logger = slog.Default()
	}
	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	maxLines := cfg.MaxLines
	if maxLines <= 0 {
		maxLines = 50
	}
	return &Uploader{
		client:   client,
		logDir:   logDir,
		enabled:  cfg.Enabled,
		maxLines: maxLines,
		interval: interval,
		source:   strings.TrimSpace(strings.ToLower(cfg.Source)),
		logger:   logger.With("component", "loguploader"),
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic log upload.
func (u *Uploader) Start() {
	if u == nil || !u.enabled || u.client == nil {
		return
	}
	go u.run()
}

// Stop terminates the upload loop.
func (u *Uploader) Stop() {
	if u != nil {
		u.stopOnce.Do(func() { close(u.stopCh) })
	}
}

func (u *Uploader) run() {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	u.upload()

	for {
		select {
		case <-u.stopCh:
			return
		case <-ticker.C:
			u.upload()
		}
	}
}

func (u *Uploader) upload() {
	if u == nil || u.client == nil {
		return
	}
	entries := u.tailLogFiles()
	if len(entries) == 0 {
		return
	}

	batchSize := 100
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		batch := entries[i:end]
		req := &agentv1.ReportAgentLogsRequest{
			Entries:    batch,
			ReportedAt: time.Now().Unix(),
		}
		if _, err := u.client.ReportAgentLogs(context.Background(), req); err != nil {
			u.logger.Warn("failed to upload agent logs", "error", err, "batch_size", len(batch))
		}
	}
}

func (u *Uploader) tailLogFiles() []*agentv1.AgentLogEntry {
	if u.logDir == "" {
		return nil
	}
	var logFiles []string

	switch u.source {
	case "agent", "all":
		agentFiles, _ := filepath.Glob(filepath.Join(u.logDir, "xboard-agent-*.log"))
		logFiles = append(logFiles, agentFiles...)
	}
	if u.source == "core" || u.source == "all" {
		coreFiles, _ := filepath.Glob(filepath.Join(u.logDir, "core-*.log"))
		logFiles = append(logFiles, coreFiles...)
	}

	if len(logFiles) == 0 {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(logFiles)))

	target := logFiles[0]
	return u.readTail(target, u.maxLines)
}

func (u *Uploader) readTail(path string, n int) []*agentv1.AgentLogEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 4096)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
	}
	if err := scanner.Err(); err != nil {
		u.logger.Warn("log file read incomplete", "path", path, "error", err)
	}

	entries := make([]*agentv1.AgentLogEntry, 0, len(lines))
	now := time.Now().Unix()
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		entries = append(entries, &agentv1.AgentLogEntry{
			Timestamp: now,
			Level:     inferLogLevel(line),
			Message:   truncateString(line, 4096),
			Source:    u.resolveSource(path),
		})
	}
	return entries
}

func inferLogLevel(line string) string {
	upper := strings.ToUpper(strings.TrimSpace(line))
	switch {
	case strings.Contains(upper, "ERROR"):
		return "error"
	case strings.Contains(upper, "WARN"):
		return "warn"
	case strings.Contains(upper, "DEBUG"):
		return "debug"
	default:
		return "info"
	}
}

func (u *Uploader) resolveSource(path string) string {
	base := filepath.Base(path)
	if strings.HasPrefix(base, "xboard-agent") {
		return "agent"
	}
	if strings.HasPrefix(base, "core-") {
		return "core"
	}
	return "agent"
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	var truncated strings.Builder
	for i, r := range s {
		if i >= maxLen-3 {
			truncated.WriteString("...")
			break
		}
		truncated.WriteRune(r)
	}
	return truncated.String()
}
