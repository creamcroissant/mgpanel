package service

import (
	"net/url"
	"strings"
	"sync"
	"unicode"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
)

func newUserUUID() string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	return strings.ToLower(raw)
}

func newUserToken() string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	return strings.ToLower(raw)
}

func hasLetterAndNumber(password string) bool {
	hasLetter := false
	hasNumber := false
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasNumber = true
		}
		if hasLetter && hasNumber {
			return true
		}
	}
	return false
}

func sanitizeHTML(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	return defaultHTMLSanitizer().Sanitize(trimmed)
}

var defaultHTMLSanitizer = sync.OnceValue(func() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("target", "rel").OnElements("a")
	policy.RequireNoFollowOnLinks(true)
	policy.RequireNoReferrerOnLinks(true)
	policy.AllowURLSchemes("http", "https")
	policy.AllowRelativeURLs(true)
	policy.AllowElements("img")
	policy.AllowAttrs("src", "alt", "title", "width", "height", "loading").OnElements("img")
	policy.AddTargetBlankToFullyQualifiedLinks(true)
	policy.AddSpaceWhenStrippingTag(true)
	policy.AllowDataURIImages()
	return policy
})

func sanitizeRedirectPath(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if strings.ContainsAny(trimmed, "\r\n\t") {
		return ""
	}
	if strings.HasPrefix(trimmed, "//") {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "#/") {
		return trimmed
	}
	return ""
}

func SanitizeRedirectPath(input string) string {
	return sanitizeRedirectPath(input)
}
