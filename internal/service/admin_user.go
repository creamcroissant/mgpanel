// 文件路径: internal/service/admin_user.go
// 模块说明: 这是 internal 模块里的 admin_user 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/support/hash"
	"github.com/creamcroissant/xboard/internal/support/i18n"
	"github.com/google/uuid"
)

// AdminUserService 提供管理员专用的用户管理流程。
type AdminUserService interface {
	Fetch(ctx context.Context, input AdminUserFetchInput) (*AdminUserFetchResult, error)
	GetByID(ctx context.Context, id int64) (*AdminUserView, error)
	Update(ctx context.Context, input AdminUserUpdateInput) (*AdminUserView, error)
	Delete(ctx context.Context, id int64) error
	Generate(ctx context.Context, input AdminUserGenerateInput) (*AdminUserView, error)
	Export(ctx context.Context, input AdminUserFetchInput) ([]byte, error)
	Import(ctx context.Context, data []byte) (*AdminUserImportResult, error)
	I18n() *i18n.Manager
}

// AdminUserImportResult 返回批量导入的结果状态。
type AdminUserImportResult struct {
	SuccessCount int      `json:"success_count"`
	FailureCount int      `json:"failure_count"`
	Errors       []string `json:"errors"`
}

// AdminUserFetchInput 控制列表分页与过滤条件。
type AdminUserFetchInput struct {
	Query  string
	Status *int
	PlanID *int64
	Limit  int
	Offset int
}

// AdminUserFetchResult 包装分页用户列表。
type AdminUserFetchResult struct {
	Users []AdminUserView
	Total int64
}

// AdminUserUpdateInput 描述可更新的用户字段。
type AdminUserUpdateInput struct {
	ID             int64   `json:"id"`
	Email          *string `json:"email,omitempty"`
	PlanID         *int64  `json:"plan_id,omitempty"`
	GroupID        *int64  `json:"group_id,omitempty"`
	ExpiredAt      *int64  `json:"expired_at,omitempty"`
	TransferEnable *int64  `json:"transfer_enable,omitempty"`
	Status         *int    `json:"status,omitempty"`
	Banned         *bool   `json:"banned,omitempty"`
	Password       *string `json:"password,omitempty"`
	Remarks        *string `json:"remarks,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

// AdminUserGenerateInput 用于创建新用户。
type AdminUserGenerateInput struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	PlanID         *int64 `json:"plan_id,omitempty"`
	GroupID        *int64 `json:"group_id,omitempty"`
	ExpiredAt      *int64 `json:"expired_at,omitempty"`
	TransferEnable *int64 `json:"transfer_enable,omitempty"`
}

// AdminUserView 对齐 Admin API 返回的用户结构。
type AdminUserView struct {
	ID                int64                   `json:"id"`
	Email             string                  `json:"email"`
	UUID              string                  `json:"uuid"`
	Token             string                  `json:"token"`
	PlanID            int64                   `json:"plan_id"`
	GroupID           int64                   `json:"group_id"`
	Plan              *AdminUserPlanSummary   `json:"plan"`
	Group             *AdminUserGroupSummary  `json:"group"`
	Status            int                     `json:"status"`
	Banned            bool                    `json:"banned"`
	IsAdmin           bool                    `json:"is_admin"`
	IsStaff           bool                    `json:"is_staff"`
	Remarks           string                  `json:"remarks"`
	TransferEnable    int64                   `json:"transfer_enable"`
	TotalUsed         int64                   `json:"total_used"`
	Upload            int64                   `json:"u"`
	Download          int64                   `json:"d"`
	Balance           float64                 `json:"balance"`
	Discount          float64                 `json:"discount"`
	SpeedLimit        *int64                  `json:"speed_limit"`
	DeviceLimit       *int64                  `json:"device_limit"`
	ExpiredAt         int64                   `json:"expired_at"`
	CreatedAt         int64                   `json:"created_at"`
	UpdatedAt         int64                   `json:"updated_at"`
	LastLoginAt       int64                   `json:"last_login_at"`
	LastLoginIP       string                  `json:"last_login_ip"`
	LastOnlineAt      int64                   `json:"last_online_at"`
	T                 int64                   `json:"t"`
	OnlineCount       int                     `json:"online_count"`
	SubscribeURL      string                  `json:"subscribe_url"`
}

// AdminUserPlanSummary 提供管理端所需的最小套餐信息。
type AdminUserPlanSummary struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AdminUserGroupSummary 提供管理端所需的最小分组信息。
type AdminUserGroupSummary struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type adminUserService struct {
	users     repository.UserRepository
	plans     repository.PlanRepository
	groups    repository.ServerGroupRepository
	settings  repository.SettingRepository
	telemetry ServerTelemetryService
	hasher    hash.Hasher
	i18n      *i18n.Manager
}

// NewAdminUserService 组装管理员用户流程所需仓储。
func NewAdminUserService(
	users repository.UserRepository,
	plans repository.PlanRepository,
	groups repository.ServerGroupRepository,
	settings repository.SettingRepository,
	telemetry ServerTelemetryService,
	hasher hash.Hasher,
	i18n *i18n.Manager,
) AdminUserService {
	return &adminUserService{
		users:     users,
		plans:     plans,
		groups:    groups,
		settings:  settings,
		telemetry: telemetry,
		hasher:    hasher,
		i18n:      i18n,
	}
}

func (s *adminUserService) I18n() *i18n.Manager {
	return s.i18n
}

func (s *adminUserService) Fetch(ctx context.Context, input AdminUserFetchInput) (*AdminUserFetchResult, error) {
	if s == nil || s.users == nil {
		return nil, fmt.Errorf("admin user service not configured / 管理用户服务未配置")
	}
	filter := repository.UserSearchFilter{
		Keyword: strings.TrimSpace(input.Query),
		Status:  input.Status,
		PlanID:  input.PlanID,
		Limit:   input.Limit,
		Offset:  input.Offset,
	}
	users, err := s.users.Search(ctx, filter)
	if err != nil {
		return nil, err
	}
	total, err := s.users.CountFiltered(ctx, filter)
	if err != nil {
		return nil, err
	}
	planMap := s.planLookup(ctx)
	groupMap := s.groupLookup(ctx)
	counts := s.aliveCounts(ctx, users)
	subscribeBase := s.subscribeBase(ctx)
	views := make([]AdminUserView, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		meta := adminUserViewMeta{
			plan:          planMap[user.PlanID],
			group:         groupMap[user.GroupID],
			onlineCount:   counts[user.ID],
			subscribeBase: subscribeBase,
		}
		views = append(views, s.buildView(user, meta))
	}
	return &AdminUserFetchResult{Users: views, Total: total}, nil
}

func (s *adminUserService) GetByID(ctx context.Context, id int64) (*AdminUserView, error) {
	if s == nil || s.users == nil {
		return nil, fmt.Errorf("admin user service not configured / 管理用户服务未配置")
	}
	if id <= 0 {
		return nil, ErrNotFound
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	view := s.buildView(user, adminUserViewMeta{
		plan:          s.planByID(ctx, user.PlanID),
		group:         s.groupByID(ctx, user.GroupID),
		subscribeBase: s.subscribeBase(ctx),
	})
	return &view, nil
}

func (s *adminUserService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.users == nil {
		return fmt.Errorf("admin user service not configured / 管理用户服务未配置")
	}
	if id <= 0 {
		return ErrNotFound
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			return ErrNotFound
		}
		return err
	}
	return s.users.Delete(ctx, user.ID)
}

func (s *adminUserService) Update(ctx context.Context, input AdminUserUpdateInput) (*AdminUserView, error) {
	if s == nil || s.users == nil {
		return nil, fmt.Errorf("admin user service not configured / 管理用户服务未配置")
	}
	if input.ID <= 0 {
		return nil, fmt.Errorf("user id is required / 需要用户 id")
	}
	user, err := s.users.FindByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	if input.Email != nil {
		email := normalizeEmail(*input.Email)
		if email == "" {
			return nil, ErrInvalidEmail
		}
		user.Email = email
	}
	var planUpdated bool
	if input.PlanID != nil {
		user.PlanID = *input.PlanID
		planUpdated = true
	}
	if input.GroupID != nil {
		user.GroupID = *input.GroupID
	}
	if input.ExpiredAt != nil {
		user.ExpiredAt = *input.ExpiredAt
	}
	if input.TransferEnable != nil {
		user.TransferEnable = max64(*input.TransferEnable, 0)
	}
	if input.Status != nil {
		user.Status = *input.Status
	}
	if input.Banned != nil {
		user.Banned = *input.Banned
	}
	if input.Password != nil {
		password := strings.TrimSpace(*input.Password)
		if len(password) < 8 || !hasLetterAndNumber(password) {
			return nil, ErrInvalidPassword
		}
		if s.hasher == nil {
			return nil, fmt.Errorf("password hasher unavailable / 密码哈希器不可用")
		}
		hashValue, err := s.hasher.Hash(password)
		if err != nil {
			return nil, err
		}
		user.Password = hashValue
	}
	if input.Remarks != nil {
		user.Remarks = *input.Remarks
	}
	if input.Tags != nil {
		user.Tags = input.Tags
	}
	if planUpdated && s.plans != nil {
		plan, err := s.plans.FindByID(ctx, user.PlanID)
		if err != nil {
			return nil, err
		}
		if input.GroupID == nil {
			if plan.GroupID != nil {
				user.GroupID = *plan.GroupID
			} else {
				user.GroupID = 0
			}
		}
		if input.TransferEnable == nil {
			user.TransferEnable = plan.TransferEnable
		}
	}
	user.UpdatedAt = time.Now().Unix()
	if err := s.users.Save(ctx, user); err != nil {
		return nil, err
	}
	view := s.buildView(user, adminUserViewMeta{
		plan:          s.planByID(ctx, user.PlanID),
		group:         s.groupByID(ctx, user.GroupID),
		subscribeBase: s.subscribeBase(ctx),
	})
	return &view, nil
}

func (s *adminUserService) Generate(ctx context.Context, input AdminUserGenerateInput) (*AdminUserView, error) {
	if s == nil || s.users == nil || s.hasher == nil {
		return nil, fmt.Errorf("admin user service not configured / 管理用户服务未配置")
	}
	email := normalizeEmail(input.Email)
	if email == "" {
		return nil, ErrInvalidEmail
	}
	password := strings.TrimSpace(input.Password)
	if len(password) < 8 || !hasLetterAndNumber(password) {
		return nil, ErrInvalidPassword
	}
	if existing, err := s.users.FindByEmail(ctx, email); err == nil && existing != nil {
		return nil, ErrEmailExists
	} else if err != nil && err != repository.ErrNotFound {
		return nil, err
	}
	var (
		plan           *repository.Plan
		planID         int64
		groupID        int64
		transferEnable int64
	)
	if input.PlanID != nil && *input.PlanID > 0 {
		var err error
		plan, err = s.plans.FindByID(ctx, *input.PlanID)
		if err != nil {
			return nil, err
		}
		planID = plan.ID
		if plan.GroupID != nil {
			groupID = *plan.GroupID
		}
		transferEnable = plan.TransferEnable
	}
	if input.GroupID != nil {
		groupID = *input.GroupID
	}
	if input.TransferEnable != nil {
		transferEnable = max64(*input.TransferEnable, 0)
	}
	hashed, err := s.hasher.Hash(password)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	user := &repository.User{
		UUID:              makeUUID(),
		Token:             makeUUID(),
		Email:             email,
		Password:          hashed,
		PlanID:            planID,
		GroupID:           groupID,
		ExpiredAt:         valueOrZero(input.ExpiredAt),
		TransferEnable:    transferEnable,
		Status:            1,
		Banned:            false,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	created, err := s.users.Create(ctx, user)
	if err != nil {
		return nil, err
	}
	view := s.buildView(created, adminUserViewMeta{
		plan:          plan,
		group:         s.groupByID(ctx, created.GroupID),
		subscribeBase: s.subscribeBase(ctx),
	})
	return &view, nil
}

func (s *adminUserService) Export(ctx context.Context, input AdminUserFetchInput) ([]byte, error) {
	if s == nil || s.users == nil {
		return nil, fmt.Errorf("admin user service not configured / 管理用户服务未配置")
	}
	// 导出时不限制数量
	input.Limit = 0
	input.Offset = 0
	
	filter := repository.UserSearchFilter{
		Keyword: strings.TrimSpace(input.Query),
		Status:  input.Status,
		PlanID:  input.PlanID,
	}
	// 获取符合筛选条件的全部用户
	users, err := s.users.Search(ctx, filter)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	// CSV Header

	for _, u := range users {
		if u == nil {
			continue
		}
		line := fmt.Sprintf("%s,%.2f,%d,%d,%d,%d\n",
			csvEscape(u.Email),
			currencyFromCents(u.BalanceCents),
			u.TransferEnable,
			u.Status,
			u.CreatedAt,
			u.ExpiredAt,
		)
		sb.WriteString(line)
	}

	return []byte(sb.String()), nil
}

func csvEscape(value string) string {
	if value == "" {
		return ""
	}
	trimmed := strings.TrimLeft(value, " \t")
	if trimmed != "" {
		switch trimmed[0] {
		case '=', '+', '-', '@':
			value = "'" + value
		}
	}
	if strings.ContainsAny(value, ",\"\n\r") {
		value = strings.ReplaceAll(value, "\"", "\"\"")
		return "\"" + value + "\""
	}
	return value
}

func parseCSVLine(line string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(line))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	return reader.Read()
}

func (s *adminUserService) Import(ctx context.Context, data []byte) (*AdminUserImportResult, error) {
	if s == nil || s.users == nil {
		return nil, fmt.Errorf("admin user service not configured / 管理用户服务未配置")
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return &AdminUserImportResult{}, nil
	}

	result := &AdminUserImportResult{
		Errors: []string{},
	}

	// Simple CSV parsing: expected format "email,password" (minimal) or more fields
	// Let's assume a simple format for import: email,password
	// Skip header if present (heuristic: check if first line contains "email")
	startIndex := 0
	if len(lines) > 0 && strings.Contains(strings.ToLower(lines[0]), "email") {
		startIndex = 1
	}

	now := time.Now().Unix()

	for i := startIndex; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		record, err := parseCSVLine(line)
		if err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Line %d: %v", i+1, err))
			continue
		}
		if len(record) < 2 {
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Line %d: invalid format (expected email,password,...)", i+1))
			continue
		}

		email := normalizeEmail(record[0])
		if email == "" {
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Line %d: invalid email", i+1))
			continue
		}
		password := strings.TrimSpace(record[1])
		if len(password) < 8 || !hasLetterAndNumber(password) {
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Line %d: invalid password", i+1))
			continue
		}

		// Check existence
		if existing, err := s.users.FindByEmail(ctx, email); err == nil && existing != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Line %d: email %s already exists", i+1, email))
			continue
		}

		hashed, err := s.hasher.Hash(password)
		if err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Line %d: hashing error", i+1))
			continue
		}

		user := &repository.User{
			UUID:           makeUUID(),
			Token:          makeUUID(),
			Email:          email,
			Password:       hashed,
			Status:         1, // Active by default
			CreatedAt:      now,
			UpdatedAt:      now,
			TransferEnable: 0,
		}
	
		if _, err := s.users.Create(ctx, user); err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Line %d: db error: %v", i+1, err))
		} else {
			result.SuccessCount++
		}
	}

	return result, nil
}

type adminUserViewMeta struct {
	plan          *repository.Plan
	group         *repository.ServerGroup
	onlineCount   int
	subscribeBase string
}

func (s *adminUserService) buildView(user *repository.User, meta adminUserViewMeta) AdminUserView {
	if user == nil {
		return AdminUserView{}
	}
	view := AdminUserView{
		ID:                user.ID,
		Email:             user.Email,
		UUID:              strings.TrimSpace(user.UUID),
		Token:             strings.TrimSpace(user.Token),
		PlanID:            user.PlanID,
		GroupID:           user.GroupID,
		Status:            user.Status,
		Banned:            user.Banned,
		IsAdmin:           user.IsAdmin,
		IsStaff:           false,
		Remarks:           user.Remarks,
		TransferEnable:    user.TransferEnable,
		TotalUsed:         user.U + user.D,
		Upload:            user.U,
		Download:          user.D,
		Balance:           currencyFromCents(user.BalanceCents),
		Discount:          0,
		SpeedLimit:        user.SpeedLimit,
		DeviceLimit:       user.DeviceLimit,
		ExpiredAt:         user.ExpiredAt,
		CreatedAt:         user.CreatedAt,
		UpdatedAt:         user.UpdatedAt,
		LastLoginAt:       user.LastLoginAt,
		LastLoginIP:       "",
		LastOnlineAt:      user.LastLoginAt,
		T:                 user.LastLoginAt,
		OnlineCount:       meta.onlineCount,
		SubscribeURL:      buildSubscribeURL(meta.subscribeBase, user.Token),
	}
	if meta.plan != nil {
		view.Plan = &AdminUserPlanSummary{ID: meta.plan.ID, Name: meta.plan.Name}
	}
	if meta.group != nil {
		view.Group = &AdminUserGroupSummary{ID: meta.group.ID, Name: meta.group.Name}
	}
	return view
}

func (s *adminUserService) planLookup(ctx context.Context) map[int64]*repository.Plan {
	if s == nil || s.plans == nil {
		return nil
	}
	plans, err := s.plans.ListAll(ctx)
	if err != nil {
		return nil
	}
	result := make(map[int64]*repository.Plan, len(plans))
	for _, plan := range plans {
		if plan == nil {
			continue
		}
		result[plan.ID] = plan
	}
	return result
}

func (s *adminUserService) planByID(ctx context.Context, id int64) *repository.Plan {
	if s == nil || s.plans == nil || id <= 0 {
		return nil
	}
	plan, err := s.plans.FindByID(ctx, id)
	if err != nil {
		return nil
	}
	return plan
}

func (s *adminUserService) groupLookup(ctx context.Context) map[int64]*repository.ServerGroup {
	if s == nil || s.groups == nil {
		return nil
	}
	groups, err := s.groups.List(ctx)
	if err != nil {
		return nil
	}
	result := make(map[int64]*repository.ServerGroup, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		result[group.ID] = group
	}
	return result
}

func (s *adminUserService) groupByID(ctx context.Context, id int64) *repository.ServerGroup {
	if id <= 0 {
		return nil
	}
	lookup := s.groupLookup(ctx)
	if len(lookup) == 0 {
		return nil
	}
	return lookup[id]
}

func (s *adminUserService) aliveCounts(ctx context.Context, users []*repository.User) map[int64]int {
	if s == nil || s.telemetry == nil {
		return map[int64]int{}
	}
	ids := make([]int64, 0, len(users))
	seen := make(map[int64]struct{}, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		seen[user.ID] = struct{}{}
		ids = append(ids, user.ID)
	}
	if len(ids) == 0 {
		return map[int64]int{}
	}
	counts, err := s.telemetry.AliveCounts(ctx, ids)
	if err != nil {
		return map[int64]int{}
	}
	return counts
}

func (s *adminUserService) subscribeBase(ctx context.Context) string {
	base := strings.TrimSpace(s.settingString(ctx, "subscribe_url"))
	if base != "" {
		return base
	}
	return strings.TrimSpace(s.settingString(ctx, "app_url"))
}

func (s *adminUserService) settingString(ctx context.Context, key string) string {
	if s == nil || s.settings == nil {
		return ""
	}
	entry, err := s.settings.Get(ctx, key)
	if err != nil || entry == nil {
		return ""
	}
	return strings.TrimSpace(entry.Value)
}

func buildSubscribeURL(base, token string) string {
	path := "/api/v1/client/subscribe"
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken != "" {
		path += "?token=" + url.QueryEscape(trimmedToken)
	}
	trimmedBase := strings.TrimSpace(base)
	if trimmedBase == "" {
		return path
	}
	return strings.TrimRight(trimmedBase, "/") + path
}

func currencyFromCents(cents int64) float64 {
	return float64(cents) / 100
}

func makeUUID() string {
	return strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))
}

func valueOrZero(ptr *int64) int64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
