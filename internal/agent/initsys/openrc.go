package initsys

import (
	"context"
	"fmt"
	"strings"
)

// OpenRC implements InitSystem for OpenRC-based systems (Alpine, Gentoo).
type OpenRC struct{}

func (o *OpenRC) Type() string {
	return "openrc"
}

func (o *OpenRC) Start(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("rc-service %s start", service))
}

func (o *OpenRC) Stop(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("rc-service %s stop", service))
}

func (o *OpenRC) Restart(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("rc-service %s restart", service))
}

func (o *OpenRC) Reload(ctx context.Context, service string) error {
	// Try reload first, fallback to restart
	err := runCommand(ctx, fmt.Sprintf("rc-service %s reload", service))
	if err != nil {
		return o.Restart(ctx, service)
	}
	return nil
}

func (o *OpenRC) Status(ctx context.Context, service string) (bool, error) {
	output, err := runCommandWithOutput(ctx, fmt.Sprintf("rc-service %s status", service))
	if err != nil {
		return false, nil
	}
	// OpenRC status output contains "started" when running
	return strings.Contains(strings.ToLower(output), "started"), nil
}

func (o *OpenRC) Enable(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("rc-update add %s default", service))
}

func (o *OpenRC) Disable(ctx context.Context, service string) error {
	return runCommand(ctx, fmt.Sprintf("rc-update del %s default", service))
}
