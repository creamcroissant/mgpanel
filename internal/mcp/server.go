package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/creamcroissant/mgpanel/internal/mcp/tools"
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
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		// fallback: direct tool lookup (custom clients)
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
}

// MCPInitializeResult is the response to an initialize request.
type MCPInitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      MCPImplementation `json:"serverInfo"`
	Capabilities    MCPCapabilities    `json:"capabilities"`
}

// MCPImplementation identifies the server implementation.
type MCPImplementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPCapabilities declares what the server supports.
type MCPCapabilities struct {
	Tools MCPToolCapabilities `json:"tools"`
}

// MCPToolCapabilities indicates tool support.
type MCPToolCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result: MCPInitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: MCPImplementation{
				Name:    "mgpanel",
				Version: "0.1.0",
			},
			Capabilities: MCPCapabilities{
				Tools: MCPToolCapabilities{ListChanged: false},
			},
		},
	}
}

// MCPToolDefinition describes a tool in tools/list responses.
type MCPToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	handlers := s.registry.List()
	toolsList := make([]MCPToolDefinition, 0, len(handlers))
	for _, h := range handlers {
		toolsList = append(toolsList, MCPToolDefinition{
			Name:        h.Name(),
			Description: h.Description(),
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}},
		})
	}
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result: map[string]any{
			"tools": toolsList,
		},
	}
}

// MCPToolCallParams is the params for a tools/call request.
type MCPToolCallParams struct {
	Name      string `json:"name"`
	Arguments any    `json:"arguments,omitempty"`
}

func (s *Server) handleToolsCall(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	var params MCPToolCallParams
	switch p := req.Params.(type) {
	case map[string]any:
		name, _ := p["name"].(string)
		params.Name = name
		params.Arguments = p["arguments"]
	default:
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &RPCError{Code: ErrCodeInvalidParams, Message: "invalid params"},
		}
	}

	handler, ok := s.registry.Get(params.Name)
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &RPCError{Code: ErrCodeMethodNotFound, Message: fmt.Sprintf("unknown tool: %s", params.Name)},
		}
	}

	result, err := handler.Handle(ctx, params.Arguments)
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
