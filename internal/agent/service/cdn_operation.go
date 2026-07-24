package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/creamcroissant/xboard/internal/agent/cdn"
	"github.com/creamcroissant/xboard/internal/agent/command"
)

const (
	// OperationTypeDeployCDN deploys a new CDN site to Caddy.
	OperationTypeDeployCDN = "deploy_cdn_site"

	// OperationTypeRemoveCDN removes a CDN site from Caddy.
	OperationTypeRemoveCDN = "remove_cdn_site"

	// OperationTypeReloadCDN reloads the Caddy configuration.
	OperationTypeReloadCDN = "reload_cdn"

	// OperationTypeInstallCaddy installs the Caddy binary.
	OperationTypeInstallCaddy = "install_caddy"
)

// installCaddyPayload is the JSON payload for install_caddy operations.
type installCaddyPayload struct {
	DownloadURL string `json:"download_url,omitempty"`
}

// deployCDNSitePayload is the JSON payload for deploy_cdn_site operations.
type deployCDNSitePayload struct {
	SiteID     int64  `json:"site_id"`
	Domain     string `json:"domain"`
	OriginType string `json:"origin_type"`
	OriginURL  string `json:"origin_url,omitempty"`
	CacheTTL   int    `json:"cache_ttl"`
	SSLMode    string `json:"ssl_mode"`
}

// removeCDNSitePayload is the JSON payload for remove_cdn_site operations.
type removeCDNSitePayload struct {
	Domain string `json:"domain"`
}

// registerCDNHandlers registers CDN command handlers with the command queue.
func (a *Agent) registerCDNHandlers() error {
	if a == nil || a.commandQueue == nil || a.cdnManager == nil {
		return nil
	}
	handlers := map[string]command.Handler{
		OperationTypeInstallCaddy: a.handleInstallCaddy,
		OperationTypeDeployCDN:    a.handleDeployCDN,
		OperationTypeRemoveCDN:    a.handleRemoveCDN,
		OperationTypeReloadCDN:    a.handleReloadCDN,
	}
	for opType, handler := range handlers {
		if err := a.commandQueue.Register(opType, handler); err != nil {
			return fmt.Errorf("register CDN handler %s: %w", opType, err)
		}
		slog.Debug("registered CDN command handler", "operation_type", opType)
	}
	return nil
}

// handleInstallCaddy handles the install_caddy operation.
func (a *Agent) handleInstallCaddy(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	slog.Info("handling install caddy command", "command_id", task.ID)

	var payload installCaddyPayload
	if len(task.RequestPayload) > 0 {
		if err := json.Unmarshal(task.RequestPayload, &payload); err != nil {
			return command.Result{
				Status:       command.StatusFailed,
				Phase:        "invalid_payload",
				Level:        command.LevelError,
				Message:      "invalid install_caddy payload",
				ErrorMessage: err.Error(),
			}
		}
	}

	_ = reporter.Report(ctx, command.Event{
		EventType: command.EventTypeProgress,
		Status:    command.StatusInProgress,
		Phase:     "installing",
		Level:     command.LevelInfo,
		Message:   "installing caddy binary",
	})

	if err := a.cdnManager.Install(ctx, payload.DownloadURL); err != nil {
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "installing",
			Level:        command.LevelError,
			Message:      "caddy install failed",
			ErrorMessage: err.Error(),
		}
	}

	return command.Result{
		Status:  command.StatusSuccess,
		Phase:   "installed",
		Level:   command.LevelInfo,
		Message: "caddy installed successfully",
	}
}

// handleDeployCDN handles the deploy_cdn_site operation.
func (a *Agent) handleDeployCDN(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	slog.Info("handling deploy cdn site command", "command_id", task.ID)

	var payload deployCDNSitePayload
	if err := json.Unmarshal(task.RequestPayload, &payload); err != nil {
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "invalid_payload",
			Level:        command.LevelError,
			Message:      "invalid deploy_cdn_site payload",
			ErrorMessage: err.Error(),
		}
	}
	if payload.Domain == "" {
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "invalid_payload",
			Level:        command.LevelError,
			Message:      "deploy_cdn_site payload requires domain",
			ErrorMessage: "domain is empty",
		}
	}

	_ = reporter.Report(ctx, command.Event{
		EventType: command.EventTypeProgress,
		Status:    command.StatusInProgress,
		Phase:     "deploying",
		Level:     command.LevelInfo,
		Message:   fmt.Sprintf("deploying cdn site %s", payload.Domain),
	})

	site := &cdn.CDNSiteConfig{
		ID:         int(payload.SiteID),
		Domain:     payload.Domain,
		OriginType: payload.OriginType,
		OriginURL:  payload.OriginURL,
		CacheTTL:   payload.CacheTTL,
		SSLMode:    payload.SSLMode,
	}
	if err := a.cdnManager.DeploySite(ctx, site); err != nil {
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "deploying",
			Level:        command.LevelError,
			Message:      fmt.Sprintf("deploy cdn site %s failed", payload.Domain),
			ErrorMessage: err.Error(),
		}
	}

	return command.Result{
		Status:  command.StatusSuccess,
		Phase:   "deployed",
		Level:   command.LevelInfo,
		Message: fmt.Sprintf("cdn site %s deployed", payload.Domain),
	}
}

// handleRemoveCDN handles the remove_cdn_site operation.
func (a *Agent) handleRemoveCDN(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	slog.Info("handling remove cdn site command", "command_id", task.ID)

	var payload removeCDNSitePayload
	if err := json.Unmarshal(task.RequestPayload, &payload); err != nil {
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "invalid_payload",
			Level:        command.LevelError,
			Message:      "invalid remove_cdn_site payload",
			ErrorMessage: err.Error(),
		}
	}
	if payload.Domain == "" {
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "invalid_payload",
			Level:        command.LevelError,
			Message:      "remove_cdn_site payload requires domain",
			ErrorMessage: "domain is empty",
		}
	}

	_ = reporter.Report(ctx, command.Event{
		EventType: command.EventTypeProgress,
		Status:    command.StatusInProgress,
		Phase:     "removing",
		Level:     command.LevelInfo,
		Message:   fmt.Sprintf("removing cdn site %s", payload.Domain),
	})

	if err := a.cdnManager.RemoveSite(ctx, payload.Domain); err != nil {
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "removing",
			Level:        command.LevelError,
			Message:      fmt.Sprintf("remove cdn site %s failed", payload.Domain),
			ErrorMessage: err.Error(),
		}
	}

	return command.Result{
		Status:  command.StatusSuccess,
		Phase:   "removed",
		Level:   command.LevelInfo,
		Message: fmt.Sprintf("cdn site %s removed", payload.Domain),
	}
}

// handleReloadCDN handles the reload_cdn operation.
func (a *Agent) handleReloadCDN(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	slog.Info("handling reload cdn command", "command_id", task.ID)

	_ = reporter.Report(ctx, command.Event{
		EventType: command.EventTypeProgress,
		Status:    command.StatusInProgress,
		Phase:     "reloading",
		Level:     command.LevelInfo,
		Message:   "reloading caddy configuration",
	})

	if err := a.cdnManager.Reload(ctx); err != nil {
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "reloading",
			Level:        command.LevelError,
			Message:      "reload caddy failed",
			ErrorMessage: err.Error(),
		}
	}

	return command.Result{
		Status:  command.StatusSuccess,
		Phase:   "reloaded",
		Level:   command.LevelInfo,
		Message: "caddy reloaded successfully",
	}
}
