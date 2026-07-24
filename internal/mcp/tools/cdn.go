package tools

import (
	"context"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/service"
)

type CDNSiteListHandler struct {
	svc service.CDNService
}

func NewCDNSiteListHandler(svc service.CDNService) *CDNSiteListHandler {
	return &CDNSiteListHandler{svc: svc}
}

func (h *CDNSiteListHandler) Name() string        { return ToolCDNSiteList }
func (h *CDNSiteListHandler) Description() string { return "获取CDN站点列表" }
func (h *CDNSiteListHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	sites, _, err := h.svc.ListSites(ctx, repository.CDNSiteFilter{})
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: sites}}}, nil
}
