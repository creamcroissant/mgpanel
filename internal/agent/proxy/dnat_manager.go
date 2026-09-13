package proxy

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// DNATRule represents a DNAT rule.
type DNATRule struct {
	ExternalPort int
	InternalPort int
	Protocol     string // "tcp", "udp", "both"
}

// DNATManager manages nftables DNAT rules for zero-downtime switching.
type DNATManager struct {
	nftBin    string
	tableName string
	logger    *slog.Logger
}

// NewDNATManager creates a new DNATManager.
func NewDNATManager(nftBin, tableName string, logger *slog.Logger) *DNATManager {
	if strings.TrimSpace(nftBin) == "" {
		nftBin = "/usr/sbin/nft"
	}
	if strings.TrimSpace(tableName) == "" {
		tableName = "mgpanel_proxy"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &DNATManager{
		nftBin:    nftBin,
		tableName: tableName,
		logger:    logger,
	}
}

// EnsureInfrastructure ensures the table and chains exist.
func (m *DNATManager) EnsureInfrastructure(ctx context.Context) error {
	commands := []string{
		fmt.Sprintf("add table inet %s", m.tableName),
		fmt.Sprintf("add chain inet %s prerouting { type nat hook prerouting priority dstnat; policy accept; }", m.tableName),
		fmt.Sprintf("add chain inet %s output { type nat hook output priority 0; policy accept; }", m.tableName),
	}

	for _, cmd := range commands {
		if err := m.runNft(ctx, cmd); err != nil {
			if isNftAlreadyExists(err) {
				continue
			}
			return err
		}
	}
	return nil
}

// SwitchAtomic applies the given DNAT rules atomically.
// It completely replaces the content of the prerouting chain.
func (m *DNATManager) SwitchAtomic(ctx context.Context, rules []*DNATRule) error {
	script := m.buildRuleset(rules)
	return m.runNft(ctx, script)
}

func (m *DNATManager) buildRuleset(rules []*DNATRule) string {
	var b strings.Builder

	b.WriteString("table inet ")
	b.WriteString(m.tableName)
	b.WriteString("\n")
	b.WriteString("delete table inet ")
	b.WriteString(m.tableName)
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "table inet %s {\n", m.tableName)
	b.WriteString("    chain prerouting {\n")
	b.WriteString("        type nat hook prerouting priority dstnat; policy accept;\n")

	for _, rule := range rules {
		if rule.ExternalPort <= 0 || rule.InternalPort <= 0 {
			continue
		}

		protocols := resolveProtocols(rule.Protocol)
		for _, proto := range protocols {
			fmt.Fprintf(&b, "        meta nfproto ipv4 %s dport %d dnat to 127.0.0.1:%d\n",
				proto, rule.ExternalPort, rule.InternalPort)
			fmt.Fprintf(&b, "        meta nfproto ipv6 %s dport %d dnat to [::1]:%d\n",
				proto, rule.ExternalPort, rule.InternalPort)
		}
	}
	b.WriteString("    }\n")
	b.WriteString("\n")
	b.WriteString("    chain output {\n")
	b.WriteString("        type nat hook output priority 0; policy accept;\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")

	return b.String()
}

// AddRules appends DNAT rules using full atomic swap.
func (m *DNATManager) AddRules(ctx context.Context, rules []*DNATRule) error {
	return m.SwitchAtomic(ctx, rules)
}

// RemoveRules removes DNAT rules using full atomic swap.
func (m *DNATManager) RemoveRules(ctx context.Context, rules []*DNATRule, handles map[string]int) error {
	return m.SwitchAtomic(ctx, rules)
}

// Cleanup removes the nftables table and all its rules.
func (m *DNATManager) Cleanup(ctx context.Context) error {
	script := fmt.Sprintf("delete table inet %s", m.tableName)
	err := m.runNft(ctx, script)
	if err != nil {
		// Ignore error if table does not exist
		if isNftNoSuchTable(err) {
			return nil
		}
		return err
	}
	return nil
}

func (m *DNATManager) runNft(ctx context.Context, script string) error {
	cmd := exec.CommandContext(ctx, m.nftBin, "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("nft execution failed: %w, stderr: %s", err, msg)
		}
		return fmt.Errorf("nft execution failed: %w", err)
	}
	return nil
}

func resolveProtocols(protocol string) []string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "tcp":
		return []string{"tcp"}
	case "udp":
		return []string{"udp"}
	case "both":
		return []string{"tcp", "udp"}
	default:
		return []string{"tcp"}
	}
}

func isNftAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "file exists") ||
		strings.Contains(msg, "already exists")
}

func isNftNoSuchTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "not found")
}
