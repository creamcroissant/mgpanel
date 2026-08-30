// Package v2rayapi provides a unified runtime user-management abstraction
// over the v2ray-compatible API exposed by both proxy cores (sing-box and
// xray).
//
// ARCHITECTURE NOTE (verified 2026-08-28 against upstream sources):
//
//   - xray: user management is available via gRPC HandlerService
//     (AlterInbound / GetInboundUsers / GetInboundUsersCount) — see
//     pkg/pb/xray/app/proxyman/command. The xray client implementation
//     (xray_client.go, planned) will speak this gRPC protocol.
//
//   - sing-box: upstream SagerNet/sing-box (main, v1.13.13, v1.13.19) and our
//     fork creamcroissant/sing-box_with_api expose experimental/v2rayapi which
//     registers ONLY StatsService (GetStats / QueryStats / GetSysStats) over
//     gRPC. There is NO user-management RPC (no GetUsers/AddUser/DelUser) and
//     no HTTP/REST surface for user CRUD. sing-box users are statically baked
//     into the rendered config; the only hot-update mechanism is config
//     rewrite + SIGHUP reload. Direction for sing-box dynamic users is still
//     under decision (A: config+SIGHUP reload, B: fork sing-box to add
//     HandlerService, C: other) — see NewManager.
package v2rayapi

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrClientUnavailable is returned when a Manager is constructed for a core
// type whose client is not yet available (e.g. sing-box while the direction is
// undecided).
var ErrClientUnavailable = errors.New("v2rayapi: client not available for core type")

// CoreType values matching the panel's config-center core_type strings.
const (
	CoreTypeSingBox = "sing-box"
	CoreTypeXray    = "xray"
)

// UserCredential is the protocol-agnostic user identity that is injected into
// a running inbound. For vless/vmess the identity is the UUID; for other
// protocols Password carries the credential. Which field applies depends on
// the inbound protocol / core.
type UserCredential struct {
	UUID     string
	Email    string
	Password string
	Flow     string
}

// APIClient is the minimal user-management surface a running core exposes.
// Implementations translate these calls to the core's native API:
//
//   - xray: gRPC HandlerService (AlterInbound with AddUserOperation /
//     RemoveUserOperation, GetInboundUsers for listing).
//   - sing-box: direction TBD (upstream has no user-management RPC; see
//     package doc).
type APIClient interface {
	// ListUsers returns the users currently attached to the inbound identified
	// by inboundTag.
	ListUsers(ctx context.Context, inboundTag string) ([]UserCredential, error)
	// AddUser attaches one user to the inbound identified by inboundTag.
	AddUser(ctx context.Context, inboundTag string, u UserCredential) error
	// RemoveUser detaches the user identified by email from the inbound
	// identified by inboundTag.
	RemoveUser(ctx context.Context, inboundTag, email string) error
}

// Manager is a thread-safe front end that routes user-management calls to the
// appropriate core client based on the configured core type.
type Manager struct {
	coreType string
	endpoint string
	client   APIClient
	mu       sync.Mutex
}

// NewManager constructs a Manager for the given core type and API endpoint.
//
//   - xray: endpoint should be the HandlerService gRPC address, e.g.
//     "127.0.0.1:10085" or "unix:///var/run/xray.sock".
//   - sing-box: client is left nil (ErrClientUnavailable) pending the
//     architecture decision on how sing-box dynamic users will be realized
//     (upstream experimental/v2rayapi has no user-management RPC — see
//     package doc). Once the direction is chosen, the sing-box client is wired
//     here.
func NewManager(coreType, endpoint string) *Manager {
	m := &Manager{coreType: coreType, endpoint: endpoint}
	switch coreType {
	case CoreTypeXray:
		// gRPC HandlerService client (AlterInbound / GetInboundUsers / Count).
		m.client = NewXrayClient(endpoint)
	case CoreTypeSingBox:
		// fork UserService exposing the same HandlerService semantics
		// (unified with xray); see singbox_client.go.
		m.client = NewSingboxClient(endpoint)
	default:
		// Unknown core type; client stays nil (ErrClientUnavailable).
	}
	return m
}

// CoreType returns the configured core type.
func (m *Manager) CoreType() string {
	return m.coreType
}

// Endpoint returns the configured API endpoint.
func (m *Manager) Endpoint() string {
	return m.endpoint
}

// ListUsers lists the users attached to inboundTag on the configured core.
func (m *Manager) ListUsers(ctx context.Context, inboundTag string) ([]UserCredential, error) {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return nil, fmt.Errorf("%w: core_type=%s", ErrClientUnavailable, m.coreType)
	}
	return client.ListUsers(ctx, inboundTag)
}

// AddUser attaches a user to inboundTag on the configured core.
func (m *Manager) AddUser(ctx context.Context, inboundTag string, u UserCredential) error {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("%w: core_type=%s", ErrClientUnavailable, m.coreType)
	}
	return client.AddUser(ctx, inboundTag, u)
}

// RemoveUser detaches the user identified by email from inboundTag.
func (m *Manager) RemoveUser(ctx context.Context, inboundTag, email string) error {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("%w: core_type=%s", ErrClientUnavailable, m.coreType)
	}
	return client.RemoveUser(ctx, inboundTag, email)
}