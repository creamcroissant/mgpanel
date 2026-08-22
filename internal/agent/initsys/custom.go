package initsys

import (
	"context"
	"strings"
)

// Custom implements InitSystem using user-defined shell commands.
type Custom struct {
	commands CustomCommands
}

func (c *Custom) Type() string {
	return "custom"
}

func renderServiceCommand(command, service string) string {
	cmd := strings.ReplaceAll(command, "{{service}}", service)
	return strings.ReplaceAll(cmd, "{service}", service)
}

func (c *Custom) Start(ctx context.Context, service string) error {
	return runCommand(ctx, renderServiceCommand(c.commands.Start, service))
}

func (c *Custom) Stop(ctx context.Context, service string) error {
	return runCommand(ctx, renderServiceCommand(c.commands.Stop, service))
}

func (c *Custom) Restart(ctx context.Context, service string) error {
	if c.commands.Restart != "" {
		return runCommand(ctx, renderServiceCommand(c.commands.Restart, service))
	}
	// Fallback: stop then start
	if err := c.Stop(ctx, service); err != nil {
		// Ignore stop errors
	}
	return c.Start(ctx, service)
}

func (c *Custom) Reload(ctx context.Context, service string) error {
	if c.commands.Reload != "" {
		return runCommand(ctx, renderServiceCommand(c.commands.Reload, service))
	}
	// Fallback to restart
	return c.Restart(ctx, service)
}

func (c *Custom) Status(ctx context.Context, service string) (bool, error) {
	if c.commands.Status == "" {
		return false, nil
	}
	_, err := runCommandWithOutput(ctx, renderServiceCommand(c.commands.Status, service))
	// Status command returns 0 if running
	return err == nil, nil
}

func (c *Custom) Enable(ctx context.Context, service string) error {
	if c.commands.Enable == "" {
		return nil // Not supported
	}
	return runCommand(ctx, renderServiceCommand(c.commands.Enable, service))
}

func (c *Custom) Disable(ctx context.Context, service string) error {
	if c.commands.Disable == "" {
		return nil // Not supported
	}
	return runCommand(ctx, renderServiceCommand(c.commands.Disable, service))
}
