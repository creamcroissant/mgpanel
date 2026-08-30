package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/command"
	"github.com/creamcroissant/mgpanel/internal/agent/v2rayapi"
)

// agentCommandActionSyncUsers 是 panel → agent 的"同步 v2ray-api 用户"命令 key。
// 与 geo_refresh 同模式：panel 派发 agent_lifecycle_operation type=sync_users，
// agent 在 syncAgentCommands 周期拉到本地 queue 并执行。
const agentCommandActionSyncUsers = "sync_users"

// syncUsersHTTPTimeout 是拉取 users-for-sync 端点时的超时。
const syncUsersHTTPTimeout = 8 * time.Second

// syncUser is the wire shape of one user in the panel's users-for-sync response.
type syncUser struct {
	Email    string `json:"email"`
	UUID     string `json:"uuid"`
	Password string `json:"password"`
	Flow     string `json:"flow"`
}

// syncInbound is the wire shape of one inbound's desired user set.
type syncInbound struct {
	Tag      string      `json:"tag"`
	CoreType string      `json:"core_type"`
	Users    []syncUser  `json:"users"`
}

// syncUsersResponse is the response envelope of GET /api/v1/agent/users-for-sync.
// panel 序列化为 {"data":{"inbounds":[...]}}（data 是对象，inbounds 数组在其内）。
type syncUsersResponse struct {
	Data struct {
		Inbounds []syncInbound `json:"inbounds"`
	} `json:"data"`
}

// registerSyncUsersHandler 向 commandQueue 注册 sync_users 命令处理器。
func (a *Agent) registerSyncUsersHandler() error {
	if a.commandQueue == nil {
		return nil
	}
	return a.commandQueue.Register(agentCommandActionSyncUsers, a.handleSyncUsers)
}

// handleSyncUsers 是 sync_users 命令 handler 主体：
// 1) 校验 v2rayapiMgr 可用（nil → failed）；
// 2) GET {base}/api/v1/agent/users-for-sync?token=<host_token> 拉取本机应注入的用户集；
// 3) 对每个 inbound 做 ListUsers ↔ 目标集 diff，缺失 AddUser、多余 RemoveUser（按 email 对齐）。
func (a *Agent) handleSyncUsers(ctx context.Context, _ command.Task, _ command.Reporter) command.Result {
	if a == nil {
		return command.Result{Status: command.StatusFailed, Level: command.LevelError, Message: "agent is nil"}
	}
	if a.v2rayapiMgr == nil {
		return command.Result{
			Status:       command.StatusFailed,
			Level:        command.LevelError,
			Message:      "v2ray api not configured / v2ray-api 未启用",
			ErrorMessage: "v2ray api not configured",
		}
	}

	hostToken := strings.TrimSpace(a.cfg.Panel.HostToken)
	if hostToken == "" {
		slog.Debug("sync_users: host_token empty, skip", "action", agentCommandActionSyncUsers)
		return command.Result{Status: command.StatusSuccess, Level: command.LevelInfo, Message: "sync_users skipped (no host token)"}
	}

	targets, err := a.fetchUsersForSync(ctx)
	if err != nil {
		slog.Warn("sync_users: fetch users-for-sync failed", "error", err)
		return command.Result{
			Status:       command.StatusFailed,
			Level:        command.LevelError,
			Message:      "fetch users-for-sync failed / 拉取用户同步失败",
			ErrorMessage: "fetch users-for-sync failed",
		}
	}

	coreType := a.v2rayapiMgr.CoreType()
	var handledInbounds, added, removed int
	var firstErr error
	for _, inbound := range targets {
		if inbound.Tag == "" {
			continue
		}
		if inbound.CoreType != "" && inbound.CoreType != coreType {
			slog.Debug("sync_users: skip inbound for other core",
				"inbound", inbound.Tag, "target_core", inbound.CoreType, "agent_core", coreType)
			continue
		}
		diffResult, err := a.syncOneInbound(ctx, inbound)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			slog.Warn("sync_users: inbound reconcile failed",
				"inbound", inbound.Tag, "error", err)
			continue
		}
		handledInbounds++
		added += diffResult.added
		removed += diffResult.removed
	}

	if firstErr != nil {
		return command.Result{
			Status:       command.StatusFailed,
			Level:        command.LevelWarn,
			Message:      fmt.Sprintf("sync_users partial: inbounds=%d added=%d removed=%d / 用户同步部分失败", handledInbounds, added, removed),
			ErrorMessage: firstErr.Error(),
		}
	}
	return command.Result{
		Status:  command.StatusSuccess,
		Level:   command.LevelInfo,
		Message: fmt.Sprintf("sync_users done: inbounds=%d added=%d removed=%d", handledInbounds, added, removed),
	}
}

type syncOneResult struct {
	added   int
	removed int
}

// syncOneInbound reconciles one inbound's runtime users with the target set:
// missing users are added (by email), extra users are removed (by email).
func (a *Agent) syncOneInbound(ctx context.Context, inbound syncInbound) (syncOneResult, error) {
	var res syncOneResult

	current, err := a.v2rayapiMgr.ListUsers(ctx, inbound.Tag)
	if err != nil {
		return res, fmt.Errorf("list users (inbound=%s): %w", inbound.Tag, err)
	}

	currentByEmail := make(map[string]v2rayapi.UserCredential, len(current))
	for _, u := range current {
		if u.Email != "" {
			currentByEmail[u.Email] = u
		}
	}

	targetByEmail := make(map[string]syncUser, len(inbound.Users))
	for _, u := range inbound.Users {
		if u.Email != "" {
			targetByEmail[u.Email] = u
		}
	}

	// Add missing targets.
	for email, target := range targetByEmail {
		if _, exists := currentByEmail[email]; exists {
			continue
		}
		cred := v2rayapi.UserCredential{
			UUID:     target.UUID,
			Email:    target.Email,
			Password: target.Password,
			Flow:     target.Flow,
		}
		if err := a.v2rayapiMgr.AddUser(ctx, inbound.Tag, cred); err != nil {
			return res, fmt.Errorf("add user (inbound=%s email=%s): %w", inbound.Tag, email, err)
		}
		res.added++
	}

	// Remove stale users not in the target set.
	for email := range currentByEmail {
		if _, exists := targetByEmail[email]; exists {
			continue
		}
		if err := a.v2rayapiMgr.RemoveUser(ctx, inbound.Tag, email); err != nil {
			return res, fmt.Errorf("remove user (inbound=%s email=%s): %w", inbound.Tag, email, err)
		}
		res.removed++
	}
	return res, nil
}

// fetchUsersForSync GETs the panel's users-for-sync endpoint and returns the
// desired per-inbound user sets for this agent host.
func (a *Agent) fetchUsersForSync(ctx context.Context) ([]syncInbound, error) {
	hostToken := strings.TrimSpace(a.cfg.Panel.HostToken)
	if hostToken == "" {
		return nil, errors.New("host token empty")
	}
	base := strings.TrimSuffix(resolvePanelHTTPBase(a.cfg), "/")
	if base == "" {
		return nil, errors.New("panel http base empty")
	}
	reqURL := base + "/api/v1/agent/users-for-sync?token=" + url.QueryEscape(hostToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: syncUsersHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var payload syncUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Data.Inbounds == nil {
		payload.Data.Inbounds = []syncInbound{}
	}
	return payload.Data.Inbounds, nil
}