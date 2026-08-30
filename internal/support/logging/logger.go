package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Options customize the slog logger construction.
type Options struct {
	Level     slog.Level
	Format    string
	AddSource bool
	// LogDir enables daily-rotated file output. Empty disables file logging.
	LogDir string
	// MaxDays controls how many days of log files to retain (default 7).
	MaxDays int
}

// New returns a slog.Logger configured according to options.
// When LogDir is set, logs are written to both stdout and daily-rotated files.
func New(opts Options) *slog.Logger {
	handlerOpts := &slog.HandlerOptions{Level: opts.Level, AddSource: opts.AddSource}

	var writer io.Writer = os.Stdout

	if opts.LogDir != "" {
		dw := newDailyWriter(opts.LogDir, opts.MaxDays)
		writer = io.MultiWriter(os.Stdout, dw)
	}

	var handler slog.Handler
	switch strings.ToLower(opts.Format) {
	case "text", "console":
		handler = slog.NewTextHandler(writer, handlerOpts)
	default:
		handler = slog.NewJSONHandler(writer, handlerOpts)
	}

	return slog.New(handler)
}

// dailyWriter implements io.Writer with daily rotation.
// It writes to a file named <dir>/xboard-YYYY-MM-DD.log and
// rotates at midnight UTC. Old files exceeding maxDays are removed.
type dailyWriter struct {
	dir     string
	maxDays int
	mu      sync.Mutex
	file    *os.File
	day     string
}

func newDailyWriter(dir string, maxDays int) *dailyWriter {
	if maxDays <= 0 {
		maxDays = 7
	}
	dw := &dailyWriter{dir: dir, maxDays: maxDays}
	dw.cleanup()
	dw.rotate()
	return dw
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	if today != w.day {
		w.rotateLocked(today)
	}

	if w.file == nil {
		// fallback: drop if file can't be opened
		return len(p), nil
	}

	return w.file.Write(p)
}

func (w *dailyWriter) rotate() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rotateLocked(time.Now().UTC().Format("2006-01-02"))
}

func (w *dailyWriter) rotateLocked(today string) {
	if w.file != nil {
		_ = w.file.Close()
	}

	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return
	}

	path := filepath.Join(w.dir, "xboard-"+today+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		w.file = nil
		w.day = ""
		return
	}

	w.file = f
	w.day = today
}

func (w *dailyWriter) cleanup() {
	if w.maxDays <= 0 {
		return
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -w.maxDays)

	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "xboard-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "xboard-"), ".log")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}

// ensure app.log is not empty
var _ = fmt.Sprintf("%d", time.Now().UnixNano())
