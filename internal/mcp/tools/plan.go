package tools

import (
	"context"

	"github.com/creamcroissant/mgpanel/internal/service"
)

type PlanListHandler struct {
	svc service.PlanService
}

func NewPlanListHandler(svc service.PlanService) *PlanListHandler {
	return &PlanListHandler{svc: svc}
}

func (h *PlanListHandler) Name() string        { return ToolPlanList }
func (h *PlanListHandler) Description() string { return "获取套餐列表" }
func (h *PlanListHandler) Handle(ctx context.Context, _ any) (*ToolCallResult, error) {
	plans, err := h.svc.AdminPlans(ctx)
	if err != nil {
		return nil, err
	}
	return &ToolCallResult{Content: []ToolContent{{Type: "json", Data: plans}}}, nil
}
