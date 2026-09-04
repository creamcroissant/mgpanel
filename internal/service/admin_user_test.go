// 文件路径: internal/service/admin_user_test.go
// 模块说明: 这是 internal 模块里的 admin_user_test 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
)

func TestAdminUserServiceFetch(t *testing.T) {
	repo := newAdminUserRepoStub()
	repo.searchResults = []*repository.User{
		{ID: 1, Email: "demo@example.com", GroupID: 3, Status: 1, TransferEnable: 1024},
	}
	repo.countFilteredResult = 50
	svc := NewAdminUserService(
		repo,
		&adminUserGroupRepoStub{},
		&adminUserSettingRepoStub{},
		&adminUserTelemetryStub{},
		hash.MustBcryptHasher(4),
		nil,
		nil,
		nil,
	)

	result, err := svc.Fetch(context.Background(), AdminUserFetchInput{Query: " demo ", Limit: 20, Offset: 5})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result == nil || len(result.Users) != 1 || result.Users[0].Email != "demo@example.com" {
		t.Fatalf("unexpected fetch result: %#v", result)
	}
	if result.Total != 50 {
		t.Fatalf("expected total 50, got %d", result.Total)
	}
	if repo.lastSearch.Keyword != "demo" {
		t.Fatalf("expected trimmed query, got %q", repo.lastSearch.Keyword)
	}
	if repo.lastSearch.Limit != 20 || repo.lastSearch.Offset != 5 {
		t.Fatalf("unexpected pagination: %+v", repo.lastSearch)
	}
	if repo.lastCountFilter.Keyword != repo.lastSearch.Keyword {
		t.Fatalf("expected count filter to align with search filter")
	}
}

func TestAdminUserServiceUpdate(t *testing.T) {
	repo := newAdminUserRepoStub()
	repo.users[10] = &repository.User{ID: 10, Email: "Old@example.com", Status: 1}
	svc := NewAdminUserService(
		repo,
		&adminUserGroupRepoStub{},
		&adminUserSettingRepoStub{},
		&adminUserTelemetryStub{},
		hash.MustBcryptHasher(4),
		nil,
		nil,
		nil,
	)
	newEmail := "New@Example.com"
	password := "Secret123"
	_, err := svc.Update(context.Background(), AdminUserUpdateInput{ID: 10, Email: &newEmail, Password: &password})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if repo.savedUser == nil || repo.savedUser.Email != "new@example.com" {
		t.Fatalf("expected normalized email, got %#v", repo.savedUser)
	}
	if repo.savedUser.Password == "" {
		t.Fatalf("expected password to be hashed")
	}
}

func TestAdminUserServiceGenerate(t *testing.T) {
	repo := newAdminUserRepoStub()
	svc := NewAdminUserService(
		repo,
		&adminUserGroupRepoStub{},
		&adminUserSettingRepoStub{},
		&adminUserTelemetryStub{},
		hash.MustBcryptHasher(4),
		nil,
		nil,
		nil,
	)
	transfer := int64(4096)
	groupID := int64(9)
	user, err := svc.Generate(context.Background(), AdminUserGenerateInput{
		Email:          "Gen@Example.com",
		Password:       "Secret123",
		GroupID:        &groupID,
		TransferEnable: &transfer,
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if user == nil || user.GroupID != 9 {
		t.Fatalf("expected group assignment, got %+v", user)
	}
	if repo.createdUser == nil || repo.createdUser.TransferEnable != 4096 {
		t.Fatalf("expected transfer override to persist, got %+v", repo.createdUser)
	}
}

type adminUserRepoStub struct {
	users               map[int64]*repository.User
	searchResults       []*repository.User
	lastSearch          repository.UserSearchFilter
	lastCountFilter     repository.UserSearchFilter
	countFilteredResult int64
	savedUser           *repository.User
	createdUser         *repository.User
}

func newAdminUserRepoStub() *adminUserRepoStub {
	return &adminUserRepoStub{users: make(map[int64]*repository.User)}
}

func (r *adminUserRepoStub) FindByID(_ context.Context, id int64) (*repository.User, error) {
	if user, ok := r.users[id]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

func (r *adminUserRepoStub) FindByEmail(_ context.Context, email string) (*repository.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			clone := *user
			return &clone, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *adminUserRepoStub) FindByUsername(_ context.Context, username string) (*repository.User, error) {
	for _, user := range r.users {
		if strings.EqualFold(user.Username, username) {
			clone := *user
			return &clone, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *adminUserRepoStub) FindByToken(context.Context, string) (*repository.User, error) {
	return nil, repository.ErrNotFound
}

func (r *adminUserRepoStub) Save(_ context.Context, user *repository.User) error {
	clone := *user
	r.users[user.ID] = &clone
	r.savedUser = &clone
	return nil
}

func (r *adminUserRepoStub) Create(_ context.Context, user *repository.User) (*repository.User, error) {
	if user.ID == 0 {
		user.ID = int64(len(r.users) + 1)
	}
	clone := *user
	r.users[user.ID] = &clone
	r.createdUser = &clone
	return &clone, nil
}
func (r *adminUserRepoStub) HasAdmin(context.Context) (bool, error) {
	return len(r.users) > 0, nil
}

func (r *adminUserRepoStub) ActiveCountByPlan(context.Context, int64, int64) (int64, error) {
	return 0, nil
}

func (r *adminUserRepoStub) AdjustBalance(context.Context, int64, int64) (bool, error) {
	return false, errors.New("not implemented")
}

func (r *adminUserRepoStub) IncrementTraffic(context.Context, int64, int64, int64) error {
	return nil
}

func (r *adminUserRepoStub) ListActiveForGroups(context.Context, []int64, int64) ([]*repository.NodeUser, error) {
	return nil, nil
}

func (r *adminUserRepoStub) Search(_ context.Context, filter repository.UserSearchFilter) ([]*repository.User, error) {
	r.lastSearch = filter
	if r.searchResults != nil {
		return r.searchResults, nil
	}
	var result []*repository.User
	for _, user := range r.users {
		result = append(result, user)
	}
	return result, nil
}

func (r *adminUserRepoStub) CountFiltered(_ context.Context, filter repository.UserSearchFilter) (int64, error) {
	r.lastCountFilter = filter
	if r.countFilteredResult > 0 {
		return r.countFilteredResult, nil
	}
	return int64(len(r.users)), nil
}

func (r *adminUserRepoStub) Increment(context.Context, int64) error {
	return nil
}

func (r *adminUserRepoStub) Decrement(context.Context, int64) error {
	return nil
}

func (r *adminUserRepoStub) Count(context.Context) (int64, error) {
	return int64(len(r.users)), nil
}

func (r *adminUserRepoStub) CountActive(context.Context, int64) (int64, error) {
	return int64(len(r.users)), nil
}

func (r *adminUserRepoStub) CountCreatedBetween(context.Context, int64, int64) (int64, error) {
	return int64(len(r.users)), nil
}

func (r *adminUserRepoStub) SetTrafficExceeded(context.Context, int64, bool) error {
	return nil
}

func (r *adminUserRepoStub) GetExceededUserIDs(context.Context) ([]int64, error) {
	return nil, nil
}

func (r *adminUserRepoStub) Delete(_ context.Context, id int64) error {
	delete(r.users, id)
	return nil
}

type adminUserGroupRepoStub struct{}

func (g *adminUserGroupRepoStub) List(context.Context) ([]*repository.ServerGroup, error) {
	return nil, nil
}

type adminUserSettingRepoStub struct{}

func (s *adminUserSettingRepoStub) Get(context.Context, string) (*repository.Setting, error) {
	return nil, repository.ErrNotFound
}

func (s *adminUserSettingRepoStub) Upsert(context.Context, *repository.Setting) error {
	return nil
}

func (s *adminUserSettingRepoStub) List(context.Context) ([]repository.Setting, error) {
	return nil, nil
}

func (s *adminUserSettingRepoStub) ListByCategory(context.Context, string) ([]repository.Setting, error) {
	return nil, nil
}

type adminUserTelemetryStub struct{}

func (adminUserTelemetryStub) TrackUserPull(context.Context, *repository.Server, int) error {
	return nil
}
func (adminUserTelemetryStub) RecordPush(context.Context, *repository.Server, []UniProxyPushSample) error {
	return nil
}
func (adminUserTelemetryStub) RecordAlive(context.Context, *repository.Server, map[int64][]string) error {
	return nil
}
func (adminUserTelemetryStub) AliveCounts(context.Context, []int64) (map[int64]int, error) {
	return map[int64]int{}, nil
}
func (adminUserTelemetryStub) RecordStatus(context.Context, *repository.Server, ServerStatusReport) error {
	return nil
}

func (adminUserTelemetryStub) IsNodeOnline(context.Context, *repository.Server) bool {
	return true
}

func (adminUserTelemetryStub) RecordHeartbeat(context.Context, *repository.Server) error {
	return nil
}

func ptrInt64(v int64) *int64 {
	return &v
}
