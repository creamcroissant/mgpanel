// 文件路径: internal/service/auth_password.go
// 模块说明: 这是 internal 模块里的 auth_password 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
)

// compareUserPassword validates legacy and modern password formats.
func compareUserPassword(user *repository.User, password string, hasher hash.Hasher) error {
	if user == nil {
		return hash.ErrPasswordMismatch
	}
	algo := strings.ToLower(strings.TrimSpace(user.PasswordAlgo))
	// fmt.Printf("DEBUG: algo=%s stored=%s input=%s\n", algo, user.Password, password)
	switch algo {
	case "", "bcrypt", "argon2", "argon2id":
		if hasher == nil {
			return hash.ErrPasswordMismatch
		}
		return hasher.Compare(user.Password, password)
	case "md5":
		sum := md5.Sum([]byte(password))
		return compareLegacyHash(hex.EncodeToString(sum[:]), user.Password)
	case "sha256":
		sum := sha256.Sum256([]byte(password))
		return compareLegacyHash(hex.EncodeToString(sum[:]), user.Password)
	case "md5salt":
		salted := password + user.PasswordSalt
		sum := md5.Sum([]byte(salted))
		return compareLegacyHash(hex.EncodeToString(sum[:]), user.Password)
	default:
		if hasher == nil {
			return hash.ErrPasswordMismatch
		}
		return hasher.Compare(user.Password, password)
	}
}

func compareLegacyHash(computed, stored string) error {
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(computed)), []byte(strings.ToLower(stored))) == 1 {
		return nil
	}
	return hash.ErrPasswordMismatch
}
