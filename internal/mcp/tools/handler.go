package tools

import (
	"context"
)

// ToolCallResult is the structured result returned by a tool handler.
type ToolCallResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"is_error,omitempty"`
}

// ToolContent is a piece of content in a tool result.
type ToolContent struct {
	Type string `json:"type"` // "text" or "json"
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}

// ScopedHandler 可选实现：声明工具所需作用域（read / ops）。
// 未实现者视为 read（只读工具）。
type ScopedHandler interface {
	RequiredScope() string
}

// SchemaHandler 可选实现：声明工具入参 JSON Schema（供 tools/list 输出）。
type SchemaHandler interface {
	InputSchema() map[string]any
}

// InputSchemaOf 返回工具入参 Schema，未声明时回落为空对象。
func InputSchemaOf(h Handler) map[string]any {
	if sh, ok := h.(SchemaHandler); ok {
		if schema := sh.InputSchema(); schema != nil {
			return schema
		}
	}
	return map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}}
}

// RequiredScopeOf 返回处理器所需作用域，默认 read。
func RequiredScopeOf(h Handler) string {
	if sh, ok := h.(ScopedHandler); ok {
		if sc := sh.RequiredScope(); sc != "" {
			return sc
		}
	}
	return ScopeRead
}

// Handler defines a tool call handler.
type Handler interface {
	// Name returns the tool name (JSON-RPC method).
	Name() string
	// Description returns a human-readable description.
	Description() string
	// Handle processes a tool call with the given params.
	Handle(ctx context.Context, params any) (*ToolCallResult, error)
}

// Registry holds all registered tool handlers.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry creates a tool registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register registers a tool handler.
func (r *Registry) Register(h Handler) {
	r.handlers[h.Name()] = h
}

// Get retrieves a handler by name.
func (r *Registry) Get(name string) (Handler, bool) {
	h, ok := r.handlers[name]
	return h, ok
}

// List returns all registered handlers.
func (r *Registry) List() []Handler {
	list := make([]Handler, 0, len(r.handlers))
	for _, h := range r.handlers {
		list = append(list, h)
	}
	return list
}

// Ensure Registry implements Handler compatibility
var _ Handler = (*noopHandler)(nil)

type noopHandler struct{}

func (h *noopHandler) Name() string        { return "noop" }
func (h *noopHandler) Description() string { return "noop" }
func (h *noopHandler) Handle(_ context.Context, _ any) (*ToolCallResult, error) {
	return &ToolCallResult{Content: []ToolContent{{Type: "text", Text: "ok"}}}, nil
}
