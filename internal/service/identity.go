package service

import "regexp"

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

func isValidUsername(username string) bool {
	normalized := normalizeUsername(username)
	if normalized == "" {
		return false
	}
	return usernamePattern.MatchString(normalized)
}
