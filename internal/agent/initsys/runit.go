package initsys

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Runit implements InitSystem for runit-based systems (Void Linux, some containers).
type Runit struct {
	// ServiceDir is the directory containing service links (default: /var/service)
	ServiceDir string
}

func (r *Runit) serviceDir() string {
	if r.ServiceDir != "" {
		return r.ServiceDir
	}
	return "/var/service"
}

func (r *Runit) Type() string {
	return "runit"
}

func (r *Runit) Start(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("sv start %s", service))
}

func (r *Runit) Stop(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("sv stop %s", service))
}

func (r *Runit) Restart(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("sv restart %s", service))
}

func (r *Runit) Reload(ctx context.Context, service string) error {
	// runit uses HUP signal for reload
	err := runCommand(ctx, fmt.Sprintf("sv hup %s", service))
	if err != nil {
		return r.Restart(ctx, service)
	}
	return nil
}

func (r *Runit) Status(ctx context.Context, service string) (bool, error) {
	output, err := runCommandWithOutput(ctx, fmt.Sprintf("sv status %s", service))
	if err != nil {
		return false, nil
	}
	// runit status starts with "run:" when running
	return len(output) >= 4 && output[:4] == "run:", nil
}

func (r *Runit) Enable(ctx context.Context, service string) error {
	// runit enables services by creating symlinks
	source := filepath.Join("/etc/sv", service)
	target := filepath.Join(r.serviceDir(), service)

	if _, err := os.Stat(target); err == nil {
		return nil // Already enabled
	}

	return os.Symlink(source, target)
}

func (r *Runit) Disable(ctx context.Context, service string) error {
	target := filepath.Join(r.serviceDir(), service)
	if err := r.Stop(ctx, service); err != nil {
		// Ignore stop errors
	}
	return os.Remove(target)
}
