package mcp

import (
	"fmt"
	"io"
	"net/http"
	"sync"
)

// SSESession represents a single SSE connection.
type SSESession struct {
	ID      string
	flusher http.Flusher
	writer  io.Writer
	mu      sync.Mutex
	closed  bool
}

// Send writes an SSE event.
func (s *SSESession) Send(event string, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("session closed")
	}
	_, err := fmt.Fprintf(s.writer, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return err
	}
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

// Close marks the session as closed.
func (s *SSESession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}
