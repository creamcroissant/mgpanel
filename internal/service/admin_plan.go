// 文件路径: internal/service/admin_plan.go
// 模块说明: 这是 internal 模块里的 admin_plan 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// AdminPlanService exposes management operations for plan catalog.
type AdminPlanService interface {
	Save(ctx context.Context, input AdminPlanSaveInput) error
	Delete(ctx context.Context, id int64) error
	Sort(ctx context.Context, input AdminPlanSortInput) error
	I18n() *i18n.Manager
}

// AdminPlanSaveInput captures fields admins can mutate.
type AdminPlanSaveInput struct {
	ID             int64              `json:"id"`
	Name           *string            `json:"name,omitempty"`
	Sell           *bool              `json:"sell,omitempty"`
	Show           *bool              `json:"show,omitempty"`
	Renew          *bool              `json:"renew,omitempty"`
	TransferEnable *int64             `json:"transfer_enable,omitempty"`
	SpeedLimit     *int64             `json:"speed_limit,omitempty"`
	DeviceLimit    *int64             `json:"device_limit,omitempty"`
	CapacityLimit  *int64             `json:"capacity_limit,omitempty"`
	ResetMethod    *int64             `json:"reset_traffic_method,omitempty"`
	Sort           *int64             `json:"sort,omitempty"`
	Content        *string            `json:"content,omitempty"`
	Prices         map[string]float64 `json:"prices,omitempty"`
	Tags           []string           `json:"tags,omitempty"`
	GroupID        *int64             `json:"group_id,omitempty"`
	ServerGroupIDs []int64            `json:"server_group_ids,omitempty"`
}

// AdminPlanSortInput reorders plan sort values according to provided ids.
type AdminPlanSortInput struct {
	IDs []int64 `json:"ids"`
}

type adminPlanService struct {
	plans repository.PlanRepository
	now   func() time.Time
	i18n  *i18n.Manager
}

// NewAdminPlanService wires admin plan mutations.
func NewAdminPlanService(plans repository.PlanRepository, i18n *i18n.Manager) AdminPlanService {
	return &adminPlanService{plans: plans, now: time.Now, i18n: i18n}
}

func (s *adminPlanService) I18n() *i18n.Manager {
	return s.i18n
}

func (s *adminPlanService) Save(ctx context.Context, input AdminPlanSaveInput) error {
	if s == nil || s.plans == nil {
		return fmt.Errorf("admin plan service not configured / 套餐管理服务未配置")
	}

	// If ID is 0 or negative, create a new plan
	if input.ID <= 0 {
		return s.create(ctx, input)
	}

	plan, err := s.plans.FindByID(ctx, input.ID)
	if err != nil {
		return err
	}
	if input.Name != nil {
		plan.Name = *input.Name
	}
	if input.Sell != nil {
		plan.Sell = *input.Sell
	}
	if input.Show != nil {
		plan.Show = *input.Show
	}
	if input.Renew != nil {
		plan.Renew = *input.Renew
	}
	if input.TransferEnable != nil {
		plan.TransferEnable = max64(*input.TransferEnable, 0)
	}
	if input.SpeedLimit != nil {
		plan.SpeedLimit = optionalPtr(input.SpeedLimit)
	}
	if input.DeviceLimit != nil {
		plan.DeviceLimit = optionalPtr(input.DeviceLimit)
	}
	if input.CapacityLimit != nil {
		plan.CapacityLimit = optionalPtr(input.CapacityLimit)
	}
	if input.ResetMethod != nil {
		plan.ResetTrafficMethod = optionalPtr(input.ResetMethod)
	}
	if input.Sort != nil {
		plan.Sort = *input.Sort
	}
	if input.Content != nil {
		plan.Content = sanitizeHTML(*input.Content)
	}
	if len(input.Prices) > 0 {
		plan.Prices = input.Prices
	}
	if input.Tags != nil {
		plan.Tags = input.Tags
	}
	if input.GroupID != nil {
		plan.GroupID = optionalPtr(input.GroupID)
	}
	plan.UpdatedAt = s.now().Unix()
	if input.ServerGroupIDs != nil {
		return s.plans.UpdateWithGroups(ctx, plan, input.ServerGroupIDs)
	}
	return s.plans.Update(ctx, plan)
}

func (s *adminPlanService) Sort(ctx context.Context, input AdminPlanSortInput) error {
	if s == nil || s.plans == nil {
		return fmt.Errorf("admin plan service not configured / 套餐管理服务未配置")
	}
	if len(input.IDs) == 0 {
		return errors.New("ids cannot be empty / ids 不能为空")
	}
	ids := uniquePositive(input.IDs)
	if len(ids) == 0 {
		return errors.New("ids cannot be empty / ids 不能为空")
	}
	return s.plans.Sort(ctx, ids, s.now().Unix())
}

func (s *adminPlanService) create(ctx context.Context, input AdminPlanSaveInput) error {
	name := ""
	if input.Name != nil {
		name = *input.Name
	}
	if name == "" {
		return errors.New("plan name is required / 套餐名称不能为空")
	}

	now := s.now().Unix()
	plan := &repository.Plan{
		Name:      name,
		Sell:      true,
		Show:      true,
		Renew:     true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if input.Sell != nil {
		plan.Sell = *input.Sell
	}
	if input.Show != nil {
		plan.Show = *input.Show
	}
	if input.Renew != nil {
		plan.Renew = *input.Renew
	}
	if input.TransferEnable != nil {
		plan.TransferEnable = max64(*input.TransferEnable, 0)
	}
	if input.SpeedLimit != nil {
		plan.SpeedLimit = optionalPtr(input.SpeedLimit)
	}
	if input.DeviceLimit != nil {
		plan.DeviceLimit = optionalPtr(input.DeviceLimit)
	}
	if input.CapacityLimit != nil {
		plan.CapacityLimit = optionalPtr(input.CapacityLimit)
	}
	if input.ResetMethod != nil {
		plan.ResetTrafficMethod = optionalPtr(input.ResetMethod)
	}
	if input.Sort != nil {
		plan.Sort = *input.Sort
	}
	if input.Content != nil {
		plan.Content = sanitizeHTML(*input.Content)
	}
	if len(input.Prices) > 0 {
		plan.Prices = input.Prices
	}
	if input.Tags != nil {
		plan.Tags = input.Tags
	}
	if input.GroupID != nil {
		plan.GroupID = optionalPtr(input.GroupID)
	}

	created, err := s.plans.Create(ctx, plan)
	if err != nil {
		return err
	}

	if len(input.ServerGroupIDs) > 0 {
		if err := s.plans.BindGroups(ctx, created.ID, input.ServerGroupIDs); err != nil {
			return err
		}
	}

	return nil
}

func (s *adminPlanService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.plans == nil {
		return fmt.Errorf("admin plan service not configured / 套餐管理服务未配置")
	}
	if id <= 0 {
		return ErrNotFound
	}

	// Check if plan exists
	_, err := s.plans.FindByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return ErrNotFound
		}
		return err
	}

	// Unbind server groups first
	if err := s.plans.ReplaceGroups(ctx, id, nil); err != nil {
		return err
	}

	return s.plans.Delete(ctx, id)
}

func optionalPtr(src *int64) *int64 {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func uniquePositive(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
