package service

import (
	"context"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// UserServerSelectionService manages user server preferences.
type UserServerSelectionService interface {
	// UpdateSelection updates the server selection for a user.
	UpdateSelection(ctx context.Context, userID int64, serverIDs []int64) error
	// GetSelection returns the list of selected server IDs for a user.
	GetSelection(ctx context.Context, userID int64) ([]int64, error)
	// ClearSelection removes all server selections for a user.
	ClearSelection(ctx context.Context, userID int64) error
}

type userServerSelectionService struct {
	repo repository.UserTrafficRepository
}

// NewUserServerSelectionService creates a new UserServerSelectionService.
func NewUserServerSelectionService(repo repository.UserTrafficRepository) UserServerSelectionService {
	return &userServerSelectionService{repo: repo}
}

// UpdateSelection updates the server selection for a user.
func (s *userServerSelectionService) UpdateSelection(ctx context.Context, userID int64, serverIDs []int64) error {
	return s.repo.ReplaceUserSelections(ctx, userID, serverIDs)
}

// GetSelection returns the list of selected server IDs for a user.
func (s *userServerSelectionService) GetSelection(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.GetUserServerIDs(ctx, userID)
}

// ClearSelection removes all server selections for a user.
func (s *userServerSelectionService) ClearSelection(ctx context.Context, userID int64) error {
	return s.repo.ClearUserSelections(ctx, userID)
}
