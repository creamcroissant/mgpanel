package cdn

import (
	"bytes"
	"fmt"
	"strings"

	corecdn "github.com/creamcroissant/mgpanel/internal/cdn"
)

// CDNSiteConfig represents a CDN site configuration for Caddyfile generation.
type CDNSiteConfig struct {
	ID         int
	Domain     string
	OriginType string // reverse_proxy / static_files / xhttp_l4
	OriginURL  string
	CacheTTL   int
	SSLMode    string // auto_acme / custom / none
}

// CaddyfileBuilder generates Caddyfile configurations for CDN sites.
// It is a pure function receiver — no fields, no state.
type CaddyfileBuilder struct{}

// BuildSites generates a complete Caddyfile from the given site configurations.
func (b *CaddyfileBuilder) BuildSites(sites []*CDNSiteConfig) ([]byte, error) {
	var buf bytes.Buffer

	// Global options block.
	buf.WriteString("{\n")
	buf.WriteString("    admin localhost:2019\n")
	buf.WriteString("    auto_https off\n")

	// Separate L7 and L4 sites.
	var l7Sites []*CDNSiteConfig
	var l7Domains []string
	for _, s := range sites {
		if err := validateCaddySite(s); err != nil {
			return nil, err
		}
		if corecdn.NormalizeCaddyOriginType(s.OriginType) == "xhttp_l4" {
			continue
		}
		l7Sites = append(l7Sites, s)
		l7Domains = append(l7Domains, s.Domain)
	}

	// L4 SNI routing config — present whenever there are any sites.
	if len(sites) > 0 {
		buf.WriteString("    servers {\n")
		buf.WriteString("        metrics\n")
		buf.WriteString("        layer4 {\n")
		buf.WriteString("            @proxy {\n")
		for _, d := range l7Domains {
			buf.WriteString(fmt.Sprintf("                not tls sni %s\n", d))
		}
		buf.WriteString("            }\n")
		buf.WriteString("            handle @proxy {\n")
		buf.WriteString("                proxy localhost:10000 {\n")
		buf.WriteString("                    proxy_protocol v2\n")
		buf.WriteString("                }\n")
		buf.WriteString("            }\n")
		buf.WriteString("        }\n")
		buf.WriteString("    }\n")
	} else {
		buf.WriteString("    servers {\n")
		buf.WriteString("        metrics\n")
		buf.WriteString("    }\n")
	}

	buf.WriteString("}\n\n")

	// Site blocks for L7 sites.
	for _, s := range l7Sites {
		writeSiteBlock(&buf, s)
	}

	return buf.Bytes(), nil
}

// AddSite inserts a site into an existing Caddyfile and returns the updated content.
// If the site is an L4 (xhttp_l4) type the Caddyfile is returned unchanged.
// Returns an error if the site domain already exists.
func (b *CaddyfileBuilder) AddSite(existing []byte, site *CDNSiteConfig) ([]byte, error) {
	if err := validateCaddySite(site); err != nil {
		return nil, err
	}
	if corecdn.NormalizeCaddyOriginType(site.OriginType) == "xhttp_l4" {
		// L4 proxy sites don't need a Caddyfile entry — catch-all L4 already covers them.
		return existing, nil
	}

	if siteDomainExists(existing, site.Domain) {
		return nil, fmt.Errorf("site %s already exists in Caddyfile", site.Domain)
	}

	// Build the site block and append it.
	var siteBlock bytes.Buffer
	writeSiteBlock(&siteBlock, site)

	result := make([]byte, len(existing)+siteBlock.Len())
	copy(result, existing)
	copy(result[len(existing):], siteBlock.Bytes())

	// Add L4 SNI exclusion for non-L4 sites so the @proxy block stays in sync.
	if corecdn.NormalizeCaddyOriginType(site.OriginType) != "xhttp_l4" {
		result = addL4Exclusion(result, site.Domain)
	}

	return result, nil
}

// RemoveSite removes a site block for the given domain from an existing Caddyfile,
// and removes the corresponding L4 exclusion line from the @proxy block.
// If no matching site block is found the content is returned unchanged.
func (b *CaddyfileBuilder) RemoveSite(existing []byte, domain string) ([]byte, error) {
	result := removeSiteBlock(existing, domain)
	result = removeL4Exclusion(result, domain)
	return result, nil
}

// ---------------------------------------------------------------------------
// Helpers: site block generation
// ---------------------------------------------------------------------------

func validateCaddySite(site *CDNSiteConfig) error {
	if site == nil {
		return fmt.Errorf("cdn site is nil")
	}
	return corecdn.ValidateCaddySiteConfig(site.Domain, site.OriginType, site.OriginURL)
}

func writeSiteBlock(buf *bytes.Buffer, site *CDNSiteConfig) {
	buf.WriteString(fmt.Sprintf("%s {\n", site.Domain))
	buf.WriteString(tlsDirective(site))

	switch corecdn.NormalizeCaddyOriginType(site.OriginType) {
	case "static_files":
		rootPath := site.OriginURL
		if rootPath == "" {
			rootPath = "/var/lib/mgpanel/cdn/files"
		}
		buf.WriteString(fmt.Sprintf("    root * %s\n", rootPath))
		buf.WriteString("    file_server\n")
		buf.WriteString("    cache\n")

	case "reverse_proxy":
		originURL := site.OriginURL
		if originURL == "" {
			originURL = "http://localhost:8080"
		}
		buf.WriteString(fmt.Sprintf("    reverse_proxy %s {\n", originURL))
		buf.WriteString("        header_up Host {upstream_hostport}\n")
		if originURL != "" {
			buf.WriteString("        header_up X-Forwarded-For {remote_host}\n")
		}
		if site.SSLMode == "full" {
			buf.WriteString("        transport http {\n")
			buf.WriteString("            tls_insecure_skip_verify\n")
			buf.WriteString("        }\n")
		}
		buf.WriteString("    }\n")
		buf.WriteString(cacheBlock(site.CacheTTL))
		buf.WriteString("    header {\n")
		buf.WriteString("        X-Cache-Status {cache_status}\n")
		buf.WriteString("    }\n")
	}

	buf.WriteString("}\n\n")
}

func tlsDirective(site *CDNSiteConfig) string {
	switch site.SSLMode {
	case "none":
		return ""
	case "off":
		return ""
	case "flexible":
		return "" // flexible encryption = no TLS to origin
	case "full":
		return "    tls\n"
	case "full_strict":
		return "    tls\n"
	case "custom":
		return "    tls\n"
	default: // auto_acme
		return "    tls\n"
	}
}

func cacheBlock(ttl int) string {
	if ttl <= 0 {
		ttl = 3600
	}
	var buf bytes.Buffer
	buf.WriteString("    cache {\n")
	buf.WriteString("        allowed_http_verbs GET HEAD\n")
	buf.WriteString(fmt.Sprintf("        ttl %d\n", ttl))
	buf.WriteString("    }\n")
	return buf.String()
}

// ---------------------------------------------------------------------------
// Helpers: Caddyfile text manipulation
// ---------------------------------------------------------------------------

// siteDomainExists checks whether a site block with the given domain
// already exists in the Caddyfile text.
func siteDomainExists(data []byte, domain string) bool {
	marker := fmt.Sprintf("\n%s {", domain)
	return bytes.Contains(data, []byte(marker))
}

// removeSiteBlock removes the site block whose opening line is "domain {".
// The block is tracked by brace depth so nested braces are handled correctly.
func removeSiteBlock(data []byte, domain string) []byte {
	var out bytes.Buffer
	lines := bytes.Split(data, []byte("\n"))
	skip := false
	depth := 0

	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if !skip && bytes.HasPrefix(trimmed, []byte(domain+" {")) {
			skip = true
			depth = 1
			continue
		}
		if skip {
			for _, ch := range trimmed {
				switch ch {
				case '{':
					depth++
				case '}':
					depth--
				}
			}
			if depth <= 0 {
				skip = false
			}
			continue
		}
		out.Write(line)
		out.Write([]byte("\n"))
	}

	return bytes.TrimSuffix(out.Bytes(), []byte("\n"))
}

// ---------------------------------------------------------------------------
// Helpers: line scanning
// ---------------------------------------------------------------------------

// extractSiteDomains scans the Caddyfile for site blocks (after the global
// options block) and returns their domain names.
func extractSiteDomains(data []byte) []string {
	globalEnd := findGlobalBlockEnd(data)
	if globalEnd < 0 {
		return nil
	}
	rest := data[globalEnd:]

	var domains []string
	lines := bytes.Split(rest, []byte("\n"))
	for _, line := range lines {
		// Site blocks start at column 0. Skip indented lines (directives
		// inside a block, such as "cache {", "header {").
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		trimmed := bytes.TrimSpace(line)
		if idx := bytes.Index(trimmed, []byte(" {")); idx > 0 {
			domain := string(trimmed[:idx])
			if !strings.Contains(domain, " ") && !strings.HasPrefix(domain, "#") {
				domains = append(domains, domain)
			}
		}
	}
	return domains
}

// findGlobalBlockEnd returns the byte offset immediately after the global
// options block's closing "}". Returns -1 if the block cannot be found.
func findGlobalBlockEnd(data []byte) int {
	lines := bytes.Split(data, []byte("\n"))
	depth := 0
	foundOpen := false

	for i, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		if !foundOpen {
			if trimmed[0] == '{' {
				foundOpen = true
				depth++
			}
			continue
		}
		// Count braces within the global block.
		for _, ch := range trimmed {
			switch ch {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		if depth == 0 {
			// Return offset after the closing "}" of this line.
			offset := 0
			for j := 0; j <= i; j++ {
				offset += len(lines[j]) + 1 // +1 for the \n
			}
			return offset
		}
	}
	return -1
}

// removeL4Exclusion removes the "not tls sni <domain>" line from the L4
// @proxy block if one exists.  Returns data unchanged if the exclusion
// line is not found.
func removeL4Exclusion(data []byte, domain string) []byte {
	marker := []byte(fmt.Sprintf("not tls sni %s", domain))
	if !bytes.Contains(data, marker) {
		return data
	}

	var out bytes.Buffer
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if string(trimmed) == string(marker) {
			continue // skip this line
		}
		out.Write(line)
		out.Write([]byte("\n"))
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n"))
}

// addL4Exclusion adds a "not tls sni <domain>" exclusion line to the @proxy
// block.  If the exclusion already exists the data is returned unchanged.
func addL4Exclusion(data []byte, domain string) []byte {
	marker := []byte(fmt.Sprintf("not tls sni %s", domain))
	if bytes.Contains(data, marker) {
		return data
	}

	blockMarker := []byte("            @proxy {")
	idx := bytes.Index(data, blockMarker)
	if idx < 0 {
		return data
	}
	// Find the end of the opening line.
	lineEnd := bytes.IndexByte(data[idx:], '\n')
	if lineEnd < 0 {
		return data
	}
	exclusion := fmt.Sprintf("                not tls sni %s\n", domain)
	insertPos := idx + lineEnd + 1
	result := make([]byte, 0, len(data)+len(exclusion))
	result = append(result, data[:insertPos]...)
	result = append(result, []byte(exclusion)...)
	result = append(result, data[insertPos:]...)
	return result
}
