package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// NodeNamer generates auto-names for server nodes based on a configurable
// template (e.g. "{flag}{region}_{agent_name}{serial}") and manages per-agent
// serial counting so that the first inbound on an agent gets no serial suffix
// and subsequent inbounds get -02, -03, etc.
type NodeNamer struct {
	settings AdminSystemSettingsService
	mu       sync.Mutex
	serials  map[int64]int // agentHostID -> next serial (1-indexed, 1 = first = no suffix)
}

// NewNodeNamer creates a NodeNamer backed by the given settings service.
func NewNodeNamer(settings AdminSystemSettingsService) *NodeNamer {
	return &NodeNamer{
		settings: settings,
		serials:  make(map[int64]int),
	}
}

// FlagEmoji converts a 2-letter ISO 3166-1 alpha-2 country code to its
// corresponding regional indicator flag emoji using standard Unicode offset calculation.
// Returns the input code as fallback if it is not a valid 2-letter uppercase ASCII code.
func FlagEmoji(countryCode string) string {
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if len(code) != 2 {
		return countryCode
	}
	r1, r2 := rune(code[0]), rune(code[1])
	if r1 >= 'A' && r1 <= 'Z' && r2 >= 'A' && r2 <= 'Z' {
		// Regional Indicator Symbol Letter A is U+1F1E6 (127462)
		const base = 0x1F1E6
		return string([]rune{base + (r1 - 'A'), base + (r2 - 'A')})
	}
	return countryCode
}

// IsNamingEnabled reads the node_naming_enabled setting (category "naming").
// Returns true when the setting is absent, empty, or "1".
func (n *NodeNamer) IsNamingEnabled(ctx context.Context) bool {
	v, err := n.settings.Get(ctx, "node_naming_enabled")
	if err != nil || v == "" {
		return true // default enabled
	}
	return v == "1"
}

// namingTemplate reads the node_naming_template setting (category "naming").
func (n *NodeNamer) namingTemplate(ctx context.Context) string {
	v, err := n.settings.Get(ctx, "node_naming_template")
	if err != nil || v == "" {
		return "{flag}{region}_{agent_name}{serial}"
	}
	return v
}

// BuildName computes a server node name from the template and the given
// inputs without mutating the receiver's serial state.
func (n *NodeNamer) BuildName(ctx context.Context, host *repository.AgentHost, srv *repository.Server, serial int) string {
	if !n.IsNamingEnabled(ctx) {
		return srv.Name
	}
	tpl := n.namingTemplate(ctx)
	if tpl == "" {
		return srv.Name
	}

	result := tpl

	flag := FlagEmoji(host.Country)
	result = strings.ReplaceAll(result, "{flag}", flag)

	region := host.Country
	result = strings.ReplaceAll(result, "{region}", region)

	agentName := host.Name
	result = strings.ReplaceAll(result, "{agent_name}", agentName)

	if serial > 0 {
		result = strings.ReplaceAll(result, "{serial}", fmt.Sprintf("-%02d", serial))
	} else {
		// serial == 0 means first server, no serial suffix
		result = strings.ReplaceAll(result, "{serial}", "")
		// Also clean up any trailing separator that would end up doubled
		result = cleanTrailingSeparator(result)
	}

	srvType := srv.Type
	result = strings.ReplaceAll(result, "{type}", srvType)

	return result
}

// cleanTrailingSeparator removes a trailing dash, underscore, space, or
// hyphen sequence that would remain after an empty {serial} substitution.
func cleanTrailingSeparator(s string) string {
	// Common separators that may trail an empty serial
	for strings.HasSuffix(s, "-") || strings.HasSuffix(s, "_") || strings.HasSuffix(s, " ") {
		s = strings.TrimSuffix(s, "-")
		s = strings.TrimSuffix(s, "_")
		s = strings.TrimSuffix(s, " ")
	}
	return s
}

// ShouldRename returns true only when the server's current name equals the
// original protocol tag, indicating it has not been manually renamed by an
// admin. Manually-named servers are preserved.
func (n *NodeNamer) ShouldRename(srv *repository.Server, originalTag string) bool {
	return srv.Name == originalTag
}

// Apply computes a new name for the server, increments the per-agent serial
// counter, and returns the new name. The caller is responsible for persisting
// the name to the database.
//
// Apply does NOT check ShouldRename; callers should gate on it themselves
// so they control when the rename is skipped.
func (n *NodeNamer) Apply(ctx context.Context, host *repository.AgentHost, srv *repository.Server) (string, error) {
	if !n.IsNamingEnabled(ctx) {
		return srv.Name, nil
	}

	n.mu.Lock()
	n.serials[host.ID]++
	counter := n.serials[host.ID]
	n.mu.Unlock()

	// counter == 1 => first server on this agent => no serial suffix displayed
	// counter >= 2 => displayed as -02, -03, ...
	var displaySerial int
	if counter > 1 {
		displaySerial = counter
	}
	newName := n.BuildName(ctx, host, srv, displaySerial)

	return newName, nil
}