package cdn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// CaddyProcess manages a Caddy reverse-proxy child process.
type CaddyProcess struct {
	binPath   string
	configDir string
	adminAddr string

	cmd     *exec.Cmd
	mu      sync.Mutex
	running bool
}

// NewCaddyProcess creates a new CaddyProcess.
func NewCaddyProcess(binPath, configDir string) *CaddyProcess {
	return &CaddyProcess{
		binPath:   binPath,
		configDir: configDir,
		adminAddr: "localhost:2019",
	}
}

// Start launches the caddy run daemon and waits for the admin API to be ready.
// It sets CADDY_ADMIN so the admin endpoint matches p.adminAddr.
func (p *CaddyProcess) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("caddy is already running")
	}
	p.mu.Unlock()

	cmd := exec.CommandContext(ctx, p.binPath, "run",
		"--config", p.configDir+"/Caddyfile",
		"--adapter", "caddyfile",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CADDY_ADMIN="+p.adminAddr)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start caddy: %w", err)
	}

	p.mu.Lock()
	p.cmd = cmd
	p.running = true
	p.mu.Unlock()

	// Reap the child process in the background to prevent zombies.
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.running = false
		p.cmd = nil
		p.mu.Unlock()
		if err != nil {
			slog.Warn("caddy process exited", "error", err)
		} else {
			slog.Info("caddy process exited cleanly")
		}
	}()

	// Wait for admin API to become reachable.
	if err := p.waitReady(ctx); err != nil {
		p.stopProcess(syscall.SIGTERM)
		p.mu.Lock()
		p.running = false
		p.cmd = nil
		p.mu.Unlock()
		return fmt.Errorf("caddy admin API not ready: %w", err)
	}

	return nil
}

// Stop gracefully stops caddy via admin POST /stop.  If the admin call fails,
// it falls back to SIGTERM on the process.  It does NOT call cmd.Wait()
// directly — the reap goroutine from Start() handles that.  Instead it polls
// p.running until the reap goroutine clears it, with a timeout that escalates
// to SIGKILL.
func (p *CaddyProcess) Stop(ctx context.Context) error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	proc := p.cmd.Process
	p.mu.Unlock()

	// Try graceful shutdown via admin API.
	if err := p.adminPost(ctx, "/stop", nil); err != nil {
		slog.Warn("caddy admin /stop failed, falling back to SIGTERM", "error", err)
	}

	// Poll p.running — the reap goroutine from Start() will clear it.
	timeout := 15 * time.Second
	deadline := time.Now().Add(timeout)
	pollInterval := 100 * time.Millisecond
	for time.Now().Before(deadline) {
		p.mu.Lock()
		running := p.running
		p.mu.Unlock()
		if !running {
			return nil
		}
		select {
		case <-ctx.Done():
			goto forceKill
		case <-time.After(pollInterval):
		}
	}

	// Timeout — send SIGTERM as last resort, then force kill
	slog.Warn("caddy stop timeout, escalating to SIGTERM")
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		slog.Warn("caddy SIGTERM failed, sending SIGKILL", "error", err)
	}

	select {
	case <-ctx.Done():
		goto forceKill
	case <-time.After(1 * time.Second):
	}
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

forceKill:
	slog.Warn("caddy stop: sending SIGKILL")
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("caddy force kill: %w", err)
	}
	// Give reap goroutine time to process the signal
	select {
	case <-ctx.Done():
		return fmt.Errorf("caddy force kill cancelled: %w", ctx.Err())
	case <-time.After(500 * time.Millisecond):
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		p.running = false
		p.cmd = nil
		return fmt.Errorf("caddy process still running after SIGKILL")
	}
	return nil
}

// Reload performs a hot reload by POSTing the provided Caddyfile to /load.
func (p *CaddyProcess) Reload(ctx context.Context, caddyfile []byte) error {
	if err := p.adminPost(ctx, "/load", bytes.NewReader(caddyfile)); err != nil {
		return fmt.Errorf("caddy reload: %w", err)
	}
	return nil
}

// Health checks whether the Caddy admin API is reachable.
func (p *CaddyProcess) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+p.adminAddr+"/config/", nil)
	if err != nil {
		return fmt.Errorf("health request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("health check: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Running reports whether the process is currently tracked as running.
func (p *CaddyProcess) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// waitReady polls the admin API until it responds or the context expires.
func (p *CaddyProcess) waitReady(ctx context.Context) error {
	const (
		interval = 200 * time.Millisecond
		timeout  = 10 * time.Second
	)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := p.Health(ctx); err == nil {
			return nil
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// adminPost sends an HTTP POST to the admin API endpoint.
func (p *CaddyProcess) adminPost(ctx context.Context, path string, body io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://"+p.adminAddr+path, body)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("admin API %s responded %d", path, resp.StatusCode)
	}
	return nil
}

// stopProcess sends the given signal to the child process.
func (p *CaddyProcess) stopProcess(sig os.Signal) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(sig)
	}
}
