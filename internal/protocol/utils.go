// 文件路径: internal/protocol/utils.go
// 模块说明: 这是 internal 模块里的 utils 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package protocol

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

func settingValue(settings map[string]any, path string) any {
	if len(settings) == 0 {
		return nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	segments := strings.Split(path, ".")
	var current any = settings
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = obj[segment]
		if !ok {
			return nil
		}
	}
	return current
}

func settingString(settings map[string]any, path string) string {
	val := settingValue(settings, path)
	switch v := val.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func settingBool(settings map[string]any, path string) bool {
	val := settingValue(settings, path)
	switch v := val.(type) {
	case bool:
		return v
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "1" || lower == "true" || lower == "yes"
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

func settingMap(settings map[string]any, path string) map[string]any {
	val := settingValue(settings, path)
	if m, ok := val.(map[string]any); ok {
		return m
	}
	return nil
}

func formatI18n(i18nMgr *i18n.Manager, lang, key string, args ...any) string {
	if i18nMgr == nil {
		return key
	}
	return i18nMgr.Translate(lang, key, args...)
}

func buildUserHeaders(user *repository.User, lang string, i18nMgr *i18n.Manager) map[string]string {
	if user == nil {
		return nil
	}

	headers := map[string]string{
		"subscription-userinfo":   fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", user.U, user.D, user.TransferEnable, user.ExpiredAt),
		"profile-update-interval": "24",
	}

	// 添加用户标识
	if user.Email != "" {
		headers["profile-title"] = user.Email
	}

	// 检查账户状态并添加警告
	now := time.Now().Unix()

	// 检查是否即将到期 (7天内)
	if user.ExpiredAt > 0 {
		daysLeft := (user.ExpiredAt - now) / 86400
		if daysLeft <= 0 {
			headers["x-subscription-status"] = formatI18n(i18nMgr, lang, "subscription.status.expired")
		} else if daysLeft <= 7 {
			headers["x-subscription-status"] = formatI18n(i18nMgr, lang, "subscription.status.expiring_in_days", daysLeft)
		}
	}

	// 检查流量是否不足 (剩余不足10%)
	if user.TransferEnable > 0 {
		used := user.U + user.D
		remaining := user.TransferEnable - used
		if remaining <= 0 {
			headers["x-traffic-status"] = formatI18n(i18nMgr, lang, "subscription.status.exhausted")
		} else if float64(remaining)/float64(user.TransferEnable) < 0.1 {
			headers["x-traffic-status"] = formatI18n(i18nMgr, lang, "subscription.status.low")
		}
	}

	return headers
}
