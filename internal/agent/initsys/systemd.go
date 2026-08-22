package initsys

import (
	"context"
	"fmt"
	"strings"
)

// Systemd implements InitSystem for systemd-based systems.
type Systemd struct{}

func (s *Systemd) Type() string {
	return "systemd"
}

func (s *Systemd) Start(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("systemctl start %s", service))
}

func (s *Systemd) Stop(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("systemctl stop %s", service))
}

func (s *Systemd) Restart(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("systemctl restart %s", service))
}

func (s *Systemd) Reload(ctx context.Context, service string) error {
	// Try reload first, fallback to restart
	err := runCommand(ctx, fmt.Sprintf("systemctl reload %s", service))
	if err != nil {
		return s.Restart(ctx, service)
	}
	return nil
}

func (s *Systemd) Status(ctx context.Context, service string) (bool, error) {
	output, err := runCommandWithOutput(ctx, fmt.Sprintf("systemctl is-active %s", service))
	if err != nil {
		// is-active returns non-zero if not active
		return false, nil
	}
	return strings.TrimSpace(output) == "active", nil
}

func (s *Systemd) Enable(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("systemctl enable %s", service))
}

func (s *Systemd) Disable(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("systemctl disable %s", service))
}
