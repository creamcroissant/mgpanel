// 文件路径: internal/service/plan.go
// 模块说明: 这是 internal 模块里的 plan 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

// PlanService 提供订阅套餐展示与购买校验能力。
type PlanService interface {
	GuestPlans(ctx context.Context) ([]PlanView, error)
	UserPlanDetail(ctx context.Context, userID int64, planID int64) (*PlanView, error)
	AdminPlans(ctx context.Context) ([]AdminPlanView, error)
}

// PlanView 兼容旧版 PlanResource 的字段结构。
type PlanView struct {
	ID                 int64    `json:"id"`
	GroupID            *int64   `json:"group_id"`
	ServerGroupIDs     []int64  `json:"server_group_ids"`
	Name               string   `json:"name"`
	Tags               []string `json:"tags"`
	Content            string   `json:"content"`
	MonthPrice         *int64   `json:"month_price"`
	QuarterPrice       *int64   `json:"quarter_price"`
	HalfYearPrice      *int64   `json:"half_year_price"`
	YearPrice          *int64   `json:"year_price"`
	TwoYearPrice       *int64   `json:"two_year_price"`
	ThreeYearPrice     *int64   `json:"three_year_price"`
	OnetimePrice       *int64   `json:"onetime_price"`
	ResetPrice         *int64   `json:"reset_price"`
	CapacityLimit      any      `json:"capacity_limit"`
	TransferEnable     int64    `json:"transfer_enable"`
	SpeedLimit         *int64   `json:"speed_limit"`
	DeviceLimit        *int64   `json:"device_limit"`
	Show               bool     `json:"show"`
	Sell               bool     `json:"sell"`
	Renew              bool     `json:"renew"`
	ResetTrafficMethod *int64   `json:"reset_traffic_method"`
	Sort               int64    `json:"sort"`
	CreatedAt          int64    `json:"created_at"`
	UpdatedAt          int64    `json:"updated_at"`
}

// PlanGroupView 描述管理端返回的分组信息。
type PlanGroupView struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AdminPlanView 扩展套餐字段，附带管理端统计信息。
type AdminPlanView struct {
	PlanView
	Group            *PlanGroupView `json:"group,omitempty"`
	UsersCount       int64          `json:"users_count"`
	ActiveUsersCount int64          `json:"active_users_count"`
}


const (
	PeriodMonthly      = "monthly"
	PeriodQuarterly    = "quarterly"
	PeriodHalfYearly   = "half_yearly"
	PeriodYearly       = "yearly"
	PeriodTwoYearly    = "two_yearly"
	PeriodThreeYearly  = "three_yearly"
	PeriodOnetime      = "onetime"
	PeriodResetTraffic = "reset_traffic"
)

var legacyPeriodMapping = map[string]string{
	"month_price":      PeriodMonthly,
	"quarter_price":    PeriodQuarterly,
	"half_year_price":  PeriodHalfYearly,
	"year_price":       PeriodYearly,
	"two_year_price":   PeriodTwoYearly,
	"three_year_price": PeriodThreeYearly,
	"onetime_price":    PeriodOnetime,
	"reset_price":      PeriodResetTraffic,
}

var reversedLegacyPeriodMapping = func() map[string]string {
	result := make(map[string]string, len(legacyPeriodMapping))
	for legacy, modern := range legacyPeriodMapping {
		result[modern] = legacy
	}
	return result
}()

type planService struct {
	plans    repository.PlanRepository
	users    repository.UserRepository
	settings repository.SettingRepository
	groups   repository.ServerGroupRepository
	now      func() time.Time
	logger   *slog.Logger
}

// NewPlanService 组装套餐服务依赖。
func NewPlanService(plans repository.PlanRepository, users repository.UserRepository, settings repository.SettingRepository, groups repository.ServerGroupRepository, logger *slog.Logger) PlanService {
	return &planService{
		plans:    plans,
		users:    users,
		settings: settings,
		groups:   groups,
		now:      time.Now,
		logger:   logger,
	}
}

func (s *planService) AdminPlans(ctx context.Context) ([]AdminPlanView, error) {
	// 汇总套餐列表、用户数量统计与分组信息。
	if s == nil || s.plans == nil {
		return nil, fmt.Errorf("plan service not configured / 套餐服务未配置")
	}
	plans, err := s.plans.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(plans))
	for _, plan := range plans {
		ids = append(ids, plan.ID)
	}
	countMap := make(map[int64]repository.PlanUserCount, len(plans))
	if len(ids) > 0 && s.users != nil {
		counts, err := s.users.PlanCounts(ctx, ids, s.now().Unix())
		if err != nil {
			return nil, err
		}
		countMap = counts
	}
	groupMap := make(map[int64]*PlanGroupView)
	if s.groups != nil {
		groups, err := s.groups.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, group := range groups {
			if group == nil {
				continue
			}
			groupMap[group.ID] = &PlanGroupView{ID: group.ID, Name: group.Name}
		}
	}
	result := make([]AdminPlanView, 0, len(plans))
	for _, plan := range plans {
		pv, _ := s.buildPlanView(ctx, plan)
		view := AdminPlanView{PlanView: pv}
		if plan.GroupID != nil {
			if group, ok := groupMap[*plan.GroupID]; ok {
				copy := *group
				view.Group = &copy
			}
		}
		if stats, ok := countMap[plan.ID]; ok {
			view.UsersCount = stats.Total
			view.ActiveUsersCount = stats.Active
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *planService) GuestPlans(ctx context.Context) ([]PlanView, error) {
	// 仅返回可展示且仍有容量的套餐。
	rawPlans, err := s.plans.ListVisible(ctx)
	if err != nil {
		return nil, err
	}

	now := s.now().Unix()
	result := make([]PlanView, 0, len(rawPlans))
	for _, plan := range rawPlans {
		ok, err := s.hasCapacity(ctx, plan, now)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		pv, _ := s.buildPlanView(ctx, plan)
		result = append(result, pv)
	}
	return result, nil
}

// UserPlanDetail returns a single plan if the user can purchase or renew it.
func (s *planService) UserPlanDetail(ctx context.Context, userID int64, planID int64) (*PlanView, error) {
	// 校验用户是否可购买/续费指定套餐。
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	plan, err := s.plans.FindByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	now := s.now().Unix()
	ok, err := s.isPlanAvailableForUser(ctx, plan, user, now)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	view, _ := s.buildPlanView(ctx, plan)
	return &view, nil
}


func (s *planService) hasCapacity(ctx context.Context, plan *repository.Plan, now int64) (bool, error) {
	// 判断套餐是否仍有容量可售。
	if plan.CapacityLimit == nil {
		return true, nil
	}
	limit := *plan.CapacityLimit
	if limit <= 0 {
		return false, nil
	}
	total, err := s.users.ActiveCountByPlan(ctx, plan.ID, now)
	if err != nil {
		return false, err
	}
	return (limit - total) > 0, nil
}

func (s *planService) isPlanAvailableForUser(ctx context.Context, plan *repository.Plan, user *repository.User, now int64) (bool, error) {
	// 判断用户是否有权购买/续费套餐。
	if user == nil {
		return false, ErrNotFound
	}
	if user.PlanID == plan.ID {
		return plan.Renew, nil
	}
	if !plan.Show || !plan.Sell {
		return false, nil
	}
	return s.hasCapacity(ctx, plan, now)
}

func (s *planService) ensurePlanAvailableForPurchase(ctx context.Context, plan *repository.Plan, user *repository.User, now int64) error {
	// 校验套餐可购买性并处理售罄逻辑。
	ok, err := s.isPlanAvailableForUser(ctx, plan, user, now)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPlanUnavailable
	}
	if user == nil || user.PlanID != plan.ID {
		capOK, err := s.hasCapacity(ctx, plan, now)
		if err != nil {
			return err
		}
		if !capOK {
			return ErrPlanSoldOut
		}
	}
	return nil
}

func (s *planService) ensureResetTrafficAllowed(user *repository.User, plan *repository.Plan, now int64) error {
	// 校验用户是否允许购买流量重置。
	if user == nil || plan == nil {
		return ErrResetTrafficNotAllowed
	}
	if user.PlanID != plan.ID {
		return ErrResetTrafficNotAllowed
	}
	if !isUserActive(user, now) {
		return ErrResetTrafficNotAllowed
	}
	if !planAllowsReset(plan) {
		return ErrResetTrafficNotAllowed
	}
	return nil
}

func (s *planService) buildPlanView(ctx context.Context, plan *repository.Plan) (PlanView, error) {
	// 组装返回给前端的套餐视图。
	serverGroupIDs, err := s.plans.GetGroups(ctx, plan.ID)
	if err != nil {
		s.logger.Error("plan: get groups failed", "plan_id", plan.ID, "error", err)
		// 保守处理：返回空切片而非 nil
		serverGroupIDs = []int64{}
	}
	view := PlanView{
		ID:                 plan.ID,
		GroupID:            plan.GroupID,
		ServerGroupIDs:     serverGroupIDs,
		Name:               plan.Name,
		Tags:               plan.Tags,
		Content:            s.renderContent(ctx, plan),
		CapacityLimit:      formatCapacity(plan.CapacityLimit),
		TransferEnable:     plan.TransferEnable,
		SpeedLimit:         plan.SpeedLimit,
		DeviceLimit:        plan.DeviceLimit,
		Show:               plan.Show,
		Sell:               plan.Sell,
		Renew:              plan.Renew,
		ResetTrafficMethod: plan.ResetTrafficMethod,
		Sort:               plan.Sort,
		CreatedAt:          plan.CreatedAt,
		UpdatedAt:          plan.UpdatedAt,
	}

	view.MonthPrice = centsFor(plan.Prices, "monthly")
	view.QuarterPrice = centsFor(plan.Prices, "quarterly")
	view.HalfYearPrice = centsFor(plan.Prices, "half_yearly")
	view.YearPrice = centsFor(plan.Prices, "yearly")
	view.TwoYearPrice = centsFor(plan.Prices, "two_yearly")
	view.ThreeYearPrice = centsFor(plan.Prices, "three_yearly")
	view.OnetimePrice = centsFor(plan.Prices, "onetime")
	view.ResetPrice = centsFor(plan.Prices, "reset_traffic")

	return view, nil
}

func (s *planService) renderContent(ctx context.Context, plan *repository.Plan) string {
	// 替换套餐描述中的占位符。
	if strings.TrimSpace(plan.Content) == "" {
		return ""
	}

	replacements := map[string]string{
		"{{transfer}}":     strconv.FormatInt(plan.TransferEnable, 10),
		"{{speed}}":        limitText(plan.SpeedLimit),
		"{{devices}}":      limitText(plan.DeviceLimit),
		"{{reset_method}}": s.resetMethodText(ctx, plan),
	}

	content := plan.Content
	for placeholder, value := range replacements {
		content = strings.ReplaceAll(content, placeholder, value)
	}
	return content
}

func (s *planService) resetMethodText(ctx context.Context, plan *repository.Plan) string {
	// 输出流量重置策略的文本描述。
	method := plan.ResetTrafficMethod
	if method == nil {
		if v, ok := s.settingInt(ctx, "reset_traffic_method"); ok {
			method = &v
		} else {
			def := planResetMonthly
			method = &def
		}
	}

	switch *method {
	case planResetFirstDayMonth:
		return "First Day of Month"
	case planResetMonthly:
		return "Monthly"
	case planResetNever:
		return "Never"
	case planResetFirstDayYear:
		return "First Day of Year"
	case planResetYearly:
		return "Yearly"
	default:
		return "Monthly"
	}
}

func (s *planService) settingInt(ctx context.Context, key string) (int64, bool) {
	// 读取整型设置值，用于重置策略等配置。
	if s.settings == nil {
		return 0, false
	}
	setting, err := s.settings.Get(ctx, key)
	if err != nil || setting == nil {
		return 0, false
	}
	value := strings.TrimSpace(setting.Value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func centsFor(prices map[string]float64, key string) *int64 {
	// 将浮点价格转换为分（整数）。
	if len(prices) == 0 {
		return nil
	}
	value, ok := prices[key]
	if !ok || value <= 0 {
		return nil
	}
	cents := int64(math.Round(value * 100))
	return &cents
}

func limitText(value *int64) string {
	// 将限速/设备数转为展示文本。
	if value == nil || *value == 0 {
		return "No Limit"
	}
	return strconv.FormatInt(*value, 10)
}

func formatCapacity(limit *int64) any {
	// 容量数值为 0/负数时显示售罄。
	if limit == nil {
		return nil
	}
	if *limit <= 0 {
		return "Sold out"
	}
	return *limit
}

func isUserActive(user *repository.User, now int64) bool {
	// 判断用户是否仍处于有效期内。
	if user == nil {
		return false
	}
	if user.Status != 1 || user.Banned {
		return false
	}
	if user.ExpiredAt == 0 {
		return true
	}
	return user.ExpiredAt >= now
}

func planAllowsReset(plan *repository.Plan) bool {
	// 判断套餐是否允许购买流量重置。
	if plan == nil {
		return false
	}
	if plan.ResetTrafficMethod != nil && *plan.ResetTrafficMethod == planResetNever {
		return false
	}
	price := centsFor(plan.Prices, PeriodResetTraffic)
	return price != nil && *price > 0
}

// NormalizePlanPeriod 兼容旧字段名并输出统一周期标识。
func NormalizePlanPeriod(period string) string {
	normalized := strings.TrimSpace(strings.ToLower(period))
	if normalized == "" {
		return ""
	}
	if mapped, ok := legacyPeriodMapping[normalized]; ok {
		return mapped
	}
	switch normalized {
	case PeriodMonthly,
		PeriodQuarterly,
		PeriodHalfYearly,
		PeriodYearly,
		PeriodTwoYearly,
		PeriodThreeYearly,
		PeriodOnetime,
		PeriodResetTraffic:
		return normalized
	default:
		return ""
	}
}

// LegacyPlanPeriod 将新周期标识转换回历史字段名。
func LegacyPlanPeriod(period string) string {
	normalized := strings.TrimSpace(strings.ToLower(period))
	if legacy, ok := reversedLegacyPeriodMapping[normalized]; ok {
		return legacy
	}
	return normalized
}

const (
	planResetFirstDayMonth = int64(0)
	planResetMonthly       = int64(1)
	planResetNever         = int64(2)
	planResetFirstDayYear  = int64(3)
	planResetYearly        = int64(4)
)
