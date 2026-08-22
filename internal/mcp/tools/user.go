package tools

import (
	"context"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/service"
)

type UserListHandler struct {
	svc service.AdminUserService
}

func NewUserListHandler(svc service.AdminUserService) *UserListHandler {
	return &UserListHandler{svc: svc}
}

func (h *UserListHandler) Name() string        { return ToolUserList }
func (h *UserListHandler) Description() string { return "获取用户列表" }
func (h *UserListHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	filter := service.AdminUserFetchInput{Limit: 50}
	if params != nil {
		if m, ok := params.(map[string]any); ok {
			if offset, ok := m["offset"].(float64); ok {
				filter.Offset = int(offset)
			}
			if limit, ok := m["limit"].(float64); ok {
				filter.Limit = int(limit)
			}
		}
	}
	result, err := h.svc.Fetch(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: result}}}, nil
}

type UserDetailHandler struct {
	svc service.AdminUserService
}

func NewUserDetailHandler(svc service.AdminUserService) *UserDetailHandler {
	return &UserDetailHandler{svc: svc}
}

func (h *UserDetailHandler) Name() string        { return ToolUserDetail }
func (h *UserDetailHandler) Description() string { return "获取用户详情" }
func (h *UserDetailHandler) Handle(ctx context.Context, params any) (*ToolCallResult, error) {
	id, err := parseID(params)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	user, err := h.svc.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: user}}}, nil
}
