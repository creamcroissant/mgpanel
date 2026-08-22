package cdn

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

var ErrUnsafeCaddyValue = errors.New("cdn: unsafe caddy value")

func ValidateCaddySiteConfig(domain, originType, originURL string) error {
	if err := ValidateCaddySiteDomain(domain); err != nil {
		return err
	}
	switch NormalizeCaddyOriginType(originType) {
	case "xhttp_l4":
		return nil
	case "reverse_proxy":
		return ValidateCaddyReverseProxyOrigin(originURL)
	case "static_files":
		return ValidateCaddyStaticRoot(originURL)
	default:
		return fmt.Errorf("%w: unsupported origin type %q", ErrUnsafeCaddyValue, originType)
	}
}

func NormalizeCaddyOriginType(originType string) string {
	switch originType {
	case "", "ip", "domain", "s3":
		return "reverse_proxy"
	default:
		return originType
	}
}

func ValidateCaddySiteDomain(domain string) error {
	if domain == "" || strings.TrimSpace(domain) != domain {
		return fmt.Errorf("%w: invalid domain", ErrUnsafeCaddyValue)
	}
	if containsCaddySyntax(domain) || strings.ContainsAny(domain, `/\\`) {
		return fmt.Errorf("%w: invalid domain", ErrUnsafeCaddyValue)
	}

	host := domain
	if strings.Count(domain, ":") == 1 {
		var port string
		var err error
		host, port, err = net.SplitHostPort(domain)
		if err != nil {
			parts := strings.Split(domain, ":")
			host = parts[0]
			port = parts[1]
		}
		if port == "" {
			return fmt.Errorf("%w: invalid domain port", ErrUnsafeCaddyValue)
		}
		p, err := strconv.Atoi(port)
		if err != nil || p <= 0 || p > 65535 {
			return fmt.Errorf("%w: invalid domain port", ErrUnsafeCaddyValue)
		}
	}

	if !isValidDNSHost(host) {
		return fmt.Errorf("%w: invalid domain", ErrUnsafeCaddyValue)
	}
	return nil
}

func ValidateCaddySafeArgument(value, field string) error {
	if value == "" {
		return nil
	}
	if strings.TrimSpace(value) != value || containsCaddySyntax(value) || hasWhitespace(value) {
		return fmt.Errorf("%w: invalid %s", ErrUnsafeCaddyValue, field)
	}
	return nil
}

func ValidateCaddyReverseProxyOrigin(originURL string) error {
	if originURL == "" {
		return nil
	}
	if err := ValidateCaddySafeArgument(originURL, "reverse proxy origin"); err != nil {
		return err
	}
	parsed, err := url.Parse(originURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: invalid reverse proxy origin", ErrUnsafeCaddyValue)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: invalid reverse proxy origin scheme", ErrUnsafeCaddyValue)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%w: invalid reverse proxy origin", ErrUnsafeCaddyValue)
	}
	return nil
}

func ValidateCaddyStaticRoot(root string) error {
	if root == "" {
		return nil
	}
	if strings.TrimSpace(root) != root || containsCaddySyntax(root) || hasWhitespace(root) {
		return fmt.Errorf("%w: invalid static root", ErrUnsafeCaddyValue)
	}
	if !filepath.IsAbs(root) {
		return fmt.Errorf("%w: static root must be absolute", ErrUnsafeCaddyValue)
	}
	for _, segment := range strings.Split(root, "/") {
		if segment == ".." {
			return fmt.Errorf("%w: invalid static root", ErrUnsafeCaddyValue)
		}
	}
	return nil
}

func containsCaddySyntax(value string) bool {
	for _, r := range value {
		if r == '{' || r == '}' || r == '\n' || r == '\r' || r == '\t' || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func hasWhitespace(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func isValidDNSHost(host string) bool {
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}
