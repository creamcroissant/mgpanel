// 文件路径: internal/service/server_access.go
// 模块说明: 这是 internal 模块里的 server_access 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

func loadServerUser(ctx context.Context, users repository.UserRepository, rawUserID string) (*repository.User, error) {
	if users == nil {
		return nil, errors.New("user repository unavailable / 用户仓储不可用")
	}
	reference := strings.TrimSpace(rawUserID)
	if reference == "" {
		return nil, errors.New("missing user identifier / 缺少用户标识")
	}
	if uid, err := parseUserID(reference); err == nil {
		user, err := users.FindByID(ctx, uid)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		return user, nil
	}
	user, err := users.FindByToken(ctx, reference)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func isServerAccessAllowed(user *repository.User) bool {
	if user == nil || user.Banned || user.Status != 1 {
		return false
	}
	if user.TransferEnable <= 0 {
		return false
	}
	if user.ExpiredAt == 0 {
		return true
	}
	return user.ExpiredAt >= time.Now().Unix()
}

func queryServersForUser(ctx context.Context, repo repository.ServerRepository, user *repository.User) ([]*repository.Server, error) {
	if repo == nil {
		return nil, errors.New("server repository unavailable / 服务器仓储不可用")
	}
	if user == nil {
		return []*repository.Server{}, nil
	}
	if user.GroupID > 0 {
		return repo.FindByGroupIDs(ctx, []int64{user.GroupID})
	}
	return repo.FindAllVisible(ctx)
}
