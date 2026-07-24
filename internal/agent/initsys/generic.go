package initsys

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Generic implements InitSystem using direct process management.
// This is a fallback for systems without a recognized init system (e.g., containers).
type Generic struct {
	// BinaryPath is the path to the service binary
	BinaryPath string

	// PidFile is the path to the PID file
	PidFile string

	// Args are the command-line arguments for the service
	Args []string
}

func (g *Generic) Type() string {
	return "generic"
}

func (g *Generic) Start(ctx context.Context, service string) error {
	if g.BinaryPath == "" {
		return fmt.Errorf("generic init system requires BinaryPath to be set")
	}

	startCtx := context.WithoutCancel(ctx)
	cmd := exec.CommandContext(startCtx, g.BinaryPath, g.Args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	configureGenericCommand(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Reap child process to avoid zombie
	go func() {
		if err := cmd.Wait(); err != nil {
			slog.Warn("process exited", "error", err)
		}
	}()

	// Write PID file if configured
	if g.PidFile != "" {
		if err := os.WriteFile(g.PidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
			return fmt.Errorf("failed to write PID file: %w", err)
		}
	}

	return nil
}

func (g *Generic) Stop(ctx context.Context, service string) error {
	pid, err := g.readPid()
	if err != nil {
		return err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Remove PID file
	if g.PidFile != "" {
		os.Remove(g.PidFile)
	}

	return nil
}

func (g *Generic) Restart(ctx context.Context, service string) error {
	stopErr := g.Stop(ctx, service)
	if stopErr != nil {
		slog.Warn("service stop failed during restart", "service", service, "error", stopErr)
	}
	startErr := g.Start(ctx, service)
	if startErr != nil && stopErr != nil {
		return fmt.Errorf("restart failed: stop: %v, start: %w", stopErr, startErr)
	}
	return startErr
}

func (g *Generic) Reload(ctx context.Context, service string) error {
	pid, err := g.readPid()
	if err != nil {
		return g.Restart(ctx, service)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return g.Restart(ctx, service)
	}

	// Send SIGHUP for reload
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return g.Restart(ctx, service)
	}

	return nil
}

func (g *Generic) Status(ctx context.Context, service string) (bool, error) {
	pid, err := g.readPid()
	if err != nil {
		return false, nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}

	// Check if process is alive by sending signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil, nil
}

func (g *Generic) Enable(ctx context.Context, service string) error {
	// Not supported in generic mode
	return nil
}

func (g *Generic) Disable(ctx context.Context, service string) error {
	// Not supported in generic mode
	return nil
}

func (g *Generic) readPid() (int, error) {
	if g.PidFile == "" {
		return 0, fmt.Errorf("PID file not configured")
	}

	data, err := os.ReadFile(g.PidFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID: %w", err)
	}

	return pid, nil
}
