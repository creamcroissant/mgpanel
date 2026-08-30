package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// SyncUser describes a runtime user to be injected into an inbound through the
// v2ray-api HandlerService (unified user sync). Fields align with the agent-side
// v2rayapi.UserCredential so the agent can diff/Add/Remove by
// stable identity (Email), protocol credential (UUID for vless/vmess, Password
// for trojan/shadowsocks), and optional vless flow.
type SyncUser struct {
	Email    string `json:"email"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
	Flow     string `json:"flow,omitempty"`
}

// InboundUserTarget is the desired user set for one inbound (keyed by its tag).
type InboundUserTarget struct {
	Tag      string     `json:"tag"`
	CoreType string     `json:"core_type"`
	Users    []SyncUser `json:"users"`
}

// userSyncProtocols is the protocol whitelist for runtime user injection.
// vless/vmess identify users by UUID; trojan/shadowsocks by password.
var userSyncProtocols = map[string]struct{}{
	"vless": {}, "vmess": {}, "trojan": {}, "shadowsocks": {},
}

// userSyncPageLimit is the per-page page size for paged repository reads.
const userSyncPageLimit = 200

// UserSyncService computes the desired user set per inbound for an agent host.
type UserSyncService interface {
	ComputeForHost(ctx context.Context, hostID int64) ([]InboundUserTarget, error)
}

// AgentUserSyncService is the agent-facing service behind GET /api/v1/agent/users-for-sync.
// It resolves the host by token and computes its inbound user targets.
type AgentUserSyncService interface {
	ResolveForToken(ctx context.Context, token string) ([]InboundUserTarget, error)
}

type agentUserSyncService struct {
	syncs UserSyncService
	hosts repository.AgentHostRepository
}

// NewAgentUserSyncService builds the agent-facing service from a user-sync
// calculator and the agent-host repository used for token resolution.
func NewAgentUserSyncService(syncs UserSyncService, hosts repository.AgentHostRepository) AgentUserSyncService {
	return &agentUserSyncService{syncs: syncs, hosts: hosts}
}

// ResolveForToken resolves the host by token and computes its inbound targets.
func (s *agentUserSyncService) ResolveForToken(ctx context.Context, token string) ([]InboundUserTarget, error) {
	return ResolveUsersForHost(ctx, token, s.syncs, s.hosts)
}

type userSyncService struct {
	specs repository.InboundSpecRepository
	users repository.UserRepository
	logger interface {
		Error(msg string, args ...any)
	}
}

// NewUserSyncService builds a UserSyncService over the inbound-spec and user
// repositories. A nil logger falls back to the package default.
func NewUserSyncService(specs repository.InboundSpecRepository, users repository.UserRepository) UserSyncService {
	return &userSyncService{specs: specs, users: users}
}

// ComputeForHost returns the desired v2ray-api user targets for every enabled
// spec bound to hostID whose protocol is in the runtime-user whitelist.
//
// The per-inbound user set is a union of:
//   - global enabled users (users table, status=1) projected onto the inbound's
//     protocol (UUID for vless/vmess, password for trojan/shadowsocks);
//   - static users declared in the spec's semantic_spec.users array (kept for
//     backward compatibility with existing static configs).
func (s *userSyncService) ComputeForHost(ctx context.Context, hostID int64) ([]InboundUserTarget, error) {
	specs, err := s.listEnabledSpecs(ctx, hostID)
	if err != nil {
		return nil, fmt.Errorf("list enabled inbound specs: %w", err)
	}
	activeUsers, err := s.listActiveUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active users: %w", err)
	}

	targets := make([]InboundUserTarget, 0, len(specs))
	for _, spec := range specs {
		if spec == nil || !spec.Enabled {
			continue
		}
		protocol, staticUsers, err := s.parseSpecUsers(spec)
		if err != nil {
			return nil, err
		}
		if !isUserSyncProtocol(protocol) {
			continue
		}
		target := InboundUserTarget{
			Tag:      spec.Tag,
			CoreType: spec.CoreType,
			Users:    mergeSyncUsers(projectUsersByProtocol(activeUsers, protocol), staticUsers),
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// listEnabledSpecs pages through specs bound to hostID (host-specific or a
// template spec bound to the host), retaining only enabled ones.
func (s *userSyncService) listEnabledSpecs(ctx context.Context, hostID int64) ([]*repository.InboundSpec, error) {
	enabled := true
	var all []*repository.InboundSpec
	offset := 0
	for {
		items, err := s.specs.ListByAgentHost(ctx, hostID, repository.InboundSpecFilter{
			Enabled: &enabled,
			Limit:   userSyncPageLimit,
			Offset:  offset,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < userSyncPageLimit {
			break
		}
		offset += len(items)
	}
	return all, nil
}

// listActiveUsers returns all enabled (status=1) users under the panel. It
// stops early on the first page shorter than the page size.
func (s *userSyncService) listActiveUsers(ctx context.Context) ([]*repository.User, error) {
	status := 1
	var all []*repository.User
	offset := 0
	for {
		items, err := s.users.Search(ctx, repository.UserSearchFilter{
			Status: &status,
			Limit:  userSyncPageLimit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < userSyncPageLimit {
			break
		}
		offset += len(items)
	}
	return all, nil
}

// parseSpecUsers extracts the inbound protocol and the inline static users from
// a spec's semantic_spec. Static users reuse the pipeline's unified-user parser.
func (s *userSyncService) parseSpecUsers(spec *repository.InboundSpec) (string, []SyncUser, error) {
	semanticObject, err := decodeSpecSemanticObject(spec.SemanticSpec)
	if err != nil {
		return "", nil, fmt.Errorf("decode semantic_spec (spec_id=%d tag=%s): %w", spec.ID, spec.Tag, err)
	}
	protocol := artifactStringByKeys(semanticObject, "protocol")
	unified := artifactBuildUnifiedUsers(semanticObject)
	static := make([]SyncUser, 0, len(unified))
	for _, u := range unified {
		static = append(static, SyncUser{
			Email:    u.Email,
			UUID:     u.UUID,
			Password: u.Password,
			Flow:     u.Flow,
		})
	}
	return protocol, static, nil
}

// decodeSpecSemanticObject unmarshals a spec semantic_spec JSON into an object.
func decodeSpecSemanticObject(raw json.RawMessage) (map[string]any, error) {
	trimmed := string(raw)
	if len(trimmed) == 0 {
		return map[string]any{}, nil
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(trimmed), &object); err != nil {
		return nil, err
	}
	if object == nil {
		object = map[string]any{}
	}
	return object, nil
}

// isUserSyncProtocol reports whether a protocol participates in runtime user
// injection.
func isUserSyncProtocol(protocol string) bool {
	_, ok := userSyncProtocols[protocol]
	return ok
}

// projectUsersByProtocol projects global enabled users onto the credential
// shape expected by the inbound's protocol.
func projectUsersByProtocol(users []*repository.User, protocol string) []SyncUser {
	out := make([]SyncUser, 0, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		su := SyncUser{Email: u.Email}
		if isUUIDProtocol(protocol) {
			su.UUID = u.UUID
		} else {
			su.Password = u.Password
		}
		out = append(out, su)
	}
	return out
}

// isUUIDProtocol reports whether the protocol authenticates by UUID.
func isUUIDProtocol(protocol string) bool {
	return protocol == "vless" || protocol == "vmess"
}

// mergeSyncUsers unions two user lists keyed by Email, keeping the first
// occurrence (global users take precedence over spec static users).
func mergeSyncUsers(primary, secondary []SyncUser) []SyncUser {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	out := make([]SyncUser, 0, len(primary)+len(secondary))
	for _, list := range [][]SyncUser{primary, secondary} {
		for _, u := range list {
			key := u.Email
			if key == "" {
				key = u.UUID
			}
			if key == "" {
				key = u.Password
			}
			if key == "" {
				continue
			}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, u)
		}
	}
	return out
}
