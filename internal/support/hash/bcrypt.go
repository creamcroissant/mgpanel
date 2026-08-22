// 文件路径: internal/support/hash/bcrypt.go
// 模块说明: 这是 internal 模块里的 bcrypt 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package hash

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Hasher 抽象密码哈希能力，供需要校验用户密钥的服务使用。
type Hasher interface {
	Hash(password string) (string, error)
	Compare(hashed, password string) error
	NeedsRehash(hashed string) bool
}

// BcryptHasher 使用 golang.org/x/crypto/bcrypt 实现 Hasher。
type BcryptHasher struct {
	cost int
}

// ErrPasswordMismatch 表示密码与哈希不匹配。
var ErrPasswordMismatch = errors.New("password mismatch / 密码不匹配")

// NewBcryptHasher 校验 cost 并返回基于 bcrypt 的哈希器。
func NewBcryptHasher(cost int) (*BcryptHasher, error) {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("bcrypt cost must be between %d and %d / bcrypt cost 必须在 %d 到 %d 之间", bcrypt.MinCost, bcrypt.MaxCost, bcrypt.MinCost, bcrypt.MaxCost)
	}
	return &BcryptHasher{cost: cost}, nil
}

// MustBcryptHasher 在参数非法时 panic，仅用于启动期配置。
func MustBcryptHasher(cost int) *BcryptHasher {
	h, err := NewBcryptHasher(cost)
	if err != nil {
		panic(err)
	}
	return h
}

// Hash 生成密码的 bcrypt 哈希。
func (h *BcryptHasher) Hash(password string) (string, error) {
	if h == nil {
		return "", fmt.Errorf("bcrypt hasher is required / bcrypt hasher 不能为空")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("password hash failed: %v / 密码哈希失败: %w", err, err)
	}
	return string(hashed), nil
}

// Compare 校验明文密码与哈希是否匹配。
func (h *BcryptHasher) Compare(hashed, password string) error {
	if h == nil {
		return fmt.Errorf("bcrypt hasher 不能为空")
	}
	normalized := normalizeLaravelHash(hashed)
	if err := bcrypt.CompareHashAndPassword(normalized, []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrPasswordMismatch
		}
		return fmt.Errorf("hash comparison failed: %v / 校验哈希失败: %w", err, err)
	}
	return nil
}

// NeedsRehash 判断哈希是否需要提升 cost。
func (h *BcryptHasher) NeedsRehash(hashed string) bool {
	if h == nil {
		return false
	}
	normalized := normalizeLaravelHash(hashed)
	cost, err := bcrypt.Cost(normalized)
	if err != nil {
		return true
	}
	return cost != h.cost
}

func normalizeLaravelHash(hashed string) []byte {
	buf := []byte(hashed)
	if len(buf) > 2 && buf[0] == '$' && buf[1] == '2' && buf[2] == 'y' {
		buf[2] = 'a'
	}
	return buf
}
