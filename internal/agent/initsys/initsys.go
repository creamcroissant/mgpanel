// Package initsys provides an abstraction layer for different init systems
// (systemd, OpenRC, runit, etc.) to control services in a cross-platform manner.
package initsys

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// InitSystem defines the interface for service management across different init systems.
type InitSystem interface {
	// Type returns the init system type identifier
	Type() string

	// Start starts the service
	Start(ctx context.Context, service string) error

	// Stop stops the service
	Stop(ctx context.Context, service string) error

	// Restart restarts the service
	Restart(ctx context.Context, service string) error

	// Reload reloads the service configuration (if supported)
	Reload(ctx context.Context, service string) error

	// Status checks if the service is running
	Status(ctx context.Context, service string) (running bool, err error)

	// Enable enables the service to start on boot
	Enable(ctx context.Context, service string) error

	// Disable disables the service from starting on boot
	Disable(ctx context.Context, service string) error
}

// Config holds the configuration for init system detection and custom commands.
type Config struct {
	// Type specifies the init system type: auto, systemd, openrc, runit, custom
	Type string `yaml:"type"`

	// ServiceName is the name of the service to manage
	ServiceName string `yaml:"service_name"`

	// Custom commands for when Type is "custom"
	Custom CustomCommands `yaml:"custom"`
}

// CustomCommands defines custom shell commands for service control.
type CustomCommands struct {
	Start   string `yaml:"start"`
	Stop    string `yaml:"stop"`
	Restart string `yaml:"restart"`
	Reload  string `yaml:"reload"`
	Status  string `yaml:"status"`
	Enable  string `yaml:"enable"`
	Disable string `yaml:"disable"`
}

// New creates an InitSystem based on the provided configuration.
// If config.Type is "auto", it will detect the init system automatically.
func New(cfg Config) (InitSystem, error) {
	switch strings.ToLower(cfg.Type) {
	case "systemd":
		return &Systemd{}, nil
	case "openrc":
		return &OpenRC{}, nil
	case "runit":
		return &Runit{}, nil
	case "custom":
		if cfg.Custom.Start == "" || cfg.Custom.Stop == "" {
			return nil, fmt.Errorf("custom init system requires at least start and stop commands")
		}
		return &Custom{commands: cfg.Custom}, nil
	case "auto", "":
		return Detect(), nil
	default:
		return nil, fmt.Errorf("unknown init system type: %s", cfg.Type)
	}
}

// Detect automatically detects the init system based on the environment.
func Detect() InitSystem {
	// 1. Check for systemd
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return &Systemd{}
	}

	// 2. Check for OpenRC (Alpine, Gentoo)
	if _, err := os.Stat("/sbin/rc-service"); err == nil {
		return &OpenRC{}
	}
	if _, err := os.Stat("/sbin/openrc"); err == nil {
		return &OpenRC{}
	}

	// 3. Check for runit (Void Linux, some containers)
	if _, err := os.Stat("/run/runit"); err == nil {
		return &Runit{}
	}

	// 4. Fallback to generic (direct process management)
	return &Generic{}
}

// runCommand executes a command with timeout.
func runCommand(ctx context.Context, command string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	name, args, err := splitCommand(command)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %s, output: %s", err, string(output))
	}
	return nil
}

// runCommandWithOutput executes a command and returns its output.
func runCommandWithOutput(ctx context.Context, command string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	name, args, err := splitCommand(command)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func splitCommand(command string) (string, []string, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "", nil, fmt.Errorf("command is required")
	}

	parts := make([]string, 0, 4)
	var buf strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for _, r := range trimmed {
		switch {
		case escaped:
			buf.WriteRune(r)
			escaped = false
		case r == '\\' && !inSingle:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case !inSingle && !inDouble && (r == ' ' || r == '\t' || r == '\n'):
			if buf.Len() > 0 {
				parts = append(parts, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}

	if escaped || inSingle || inDouble {
		return "", nil, fmt.Errorf("invalid command: unclosed quote or escape")
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("command is required")
	}
	return parts[0], parts[1:], nil
}
