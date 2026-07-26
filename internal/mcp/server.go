package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/creamcroissant/xboard/internal/mcp/tools"
	"github.com/go-chi/chi/v5"
)

// Config holds MCP server configuration.
type Config struct {
	APIKey            string
	LogDir            string
	ServerLogMaxLines int
}

// Server implements the MCP endpoint.
type Server struct {
	config    Config
	registry  *tools.Registry
	validator KeyValidator
	logger    *slog.Logger
	mu        sync.Mutex
	sessions  map[string]*SSESession
}

// NewServer creates an MCP server.
// Pass a KeyValidator to support DB-backed API keys (can be nil).
func NewServer(cfg Config, registry *tools.Registry, logger *slog.Logger, opts ...ServerOption) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		config:    cfg,
		registry:  registry,
		validator: nil,
		logger:    logger.With("component", "mcp"),
		sessions:  make(map[string]*SSESession),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServerOption configures the MCP server.
type ServerOption func(*Server)

// WithKeyValidator sets a KeyValidator for DB-backed API key validation.
func WithKeyValidator(v KeyValidator) ServerOption {
	return func(s *Server) {
		s.validator = v
	}
}

// Mount registers MCP routes on a chi router func. Removed init check so caller controls enablement.
func (s *Server) Mount(r chi.Router) {
	if s == nil {
		return
	}
	r.Route("/mcp", func(mcp chi.Router) {
		mcp.Use(AuthMiddleware(s.config.APIKey, s.validator))
		mcp.Post("/message", s.handleMessage)
		mcp.Get("/events", s.handleEvents)
	})
}

// POST /mcp/message — receive JSON-RPC calls
func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		writeError(w, nil, ErrCodeInvalidReq, "empty request body")
		return
	}
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, nil, ErrCodeParse, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if req.JSONRPC != JSONRPCVersion {
		writeError(w, req.ID, ErrCodeInvalidReq, "invalid jsonrpc version")
		return
	}

	result := s.dispatch(r.Context(), &req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GET /mcp/events — SSE endpoint
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()
	<-r.Context().Done()
}

func (s *Server) dispatch(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	handler, ok := s.registry.Get(req.Method)
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &RPCError{Code: ErrCodeMethodNotFound, Message: fmt.Sprintf("unknown tool: %s", req.Method)},
		}
	}

	result, err := handler.Handle(ctx, req.Params)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &RPCError{Code: ErrCodeInternal, Message: err.Error()},
		}
	}
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}
}

func writeError(w http.ResponseWriter, id any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg},
	}
	json.NewEncoder(w).Encode(resp)
}
