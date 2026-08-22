package transport

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc/connectivity"
	"log/slog"
)

// ConnectionState represents gRPC connection state.
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// ConnectionManager tracks gRPC connection health and error states.
type ConnectionManager struct {
	mu sync.Mutex

	client       *GRPCClient
	state        ConnectionState
	lastLogAt    time.Time
	logInterval  time.Duration
	consecutive  int
	onStateChange func(ConnectionState)
	logger       *slog.Logger
}

// NewConnectionManager creates a new ConnectionManager.
func NewConnectionManager(client *GRPCClient, logger *slog.Logger) *ConnectionManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &ConnectionManager{
		client:      client,
		state:       StateDisconnected,
		logInterval: 30 * time.Second,
		logger:      logger,
	}
}

// SetOnStateChange sets a callback for state transitions.
func (m *ConnectionManager) SetOnStateChange(fn func(ConnectionState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStateChange = fn
}

// IsHealthy returns true if the connection is ready.
func (m *ConnectionManager) IsHealthy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state == StateConnected
}

// CheckConnection updates the current state from the gRPC connection.
func (m *ConnectionManager) CheckConnection(ctx context.Context) ConnectionState {
	state := StateDisconnected
	if m.client == nil || m.client.conn == nil {
		m.updateState(state)
		return state
	}

	switch m.client.conn.GetState() {
	case connectivity.Ready:
		state = StateConnected
	case connectivity.Idle:
		m.client.conn.Connect()
		state = StateConnecting
	case connectivity.Connecting:
		state = StateConnecting
	case connectivity.TransientFailure:
		state = StateReconnecting
	default:
		state = StateDisconnected
	}

	m.updateState(state)
	return state
}

// RecordSuccess marks a successful request and resets error counters.
func (m *ConnectionManager) RecordSuccess() {
	m.mu.Lock()
	m.consecutive = 0
	m.mu.Unlock()
	m.updateState(StateConnected)
}

// RecordError records an error and throttles logs on repeated failures.
func (m *ConnectionManager) RecordError(err error) {
	m.mu.Lock()
	m.consecutive++
	shouldLog := time.Since(m.lastLogAt) >= m.logInterval
	if shouldLog {
		m.lastLogAt = time.Now()
	}
	m.mu.Unlock()

	if shouldLog && err != nil {
		m.logger.Warn("grpc call failed", "error", err)
	}

	if IsRetryable(err) {
		m.updateState(StateReconnecting)
	} else {
		m.updateState(StateDisconnected)
	}
}

func (m *ConnectionManager) updateState(state ConnectionState) {
	m.mu.Lock()
	prev := m.state
	m.state = state
	cb := m.onStateChange
	m.mu.Unlock()

	if state != prev && cb != nil {
		cb(state)
	}
}
