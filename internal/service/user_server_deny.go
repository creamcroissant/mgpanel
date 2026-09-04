package service

import (
	"context"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// UserServerDenyService manages per-user node denylist (blacklist).
type UserServerDenyService interface {
	// GetUserDeniedServerIDs returns the server IDs denied for a user.
	GetUserDeniedServerIDs(ctx context.Context, userID int64) ([]int64, error)
	// ReplaceUserDenies atomically replaces the denied server set for a user.
	// An empty list clears the denylist.
	ReplaceUserDenies(ctx context.Context, userID int64, serverIDs []int64) error
	// ClearUserDenies removes all denied server entries for a user.
	ClearUserDenies(ctx context.Context, userID int64) error
}

type userServerDenyService struct {
	repo repository.UserTrafficRepository
}

// NewUserServerDenyService creates a new UserServerDenyService.
func NewUserServerDenyService(repo repository.UserTrafficRepository) UserServerDenyService {
	return &userServerDenyService{repo: repo}
}

// GetUserDeniedServerIDs returns the server IDs denied for a user.
func (s *userServerDenyService) GetUserDeniedServerIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.GetUserDeniedServerIDs(ctx, userID)
}

// ReplaceUserDenies atomically replaces the denied server set for a user.
func (s *userServerDenyService) ReplaceUserDenies(ctx context.Context, userID int64, serverIDs []int64) error {
	return s.repo.ReplaceUserDenies(ctx, userID, serverIDs)
}

// ClearUserDenies removes all denied server entries for a user.
func (s *userServerDenyService) ClearUserDenies(ctx context.Context, userID int64) error {
	return s.repo.ClearUserDenies(ctx, userID)
}