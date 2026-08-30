package forwarding

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sort"
	"strings"

	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

const defaultNFTBin = "/usr/sbin/nft"

const defaultNFTTableName = "mgpanel_forwarding"

// NFTablesExecutor 使用 nftables 应用转发规则。
type NFTablesExecutor struct {
	tableName string
	nftBin    string
}

// NewNFTablesExecutor 创建执行器，默认表名为内置值。
func NewNFTablesExecutor(tableName, nftBin string) *NFTablesExecutor {
	if strings.TrimSpace(tableName) == "" {
		tableName = defaultNFTTableName
	}
	nftBin = strings.TrimSpace(nftBin)
	if nftBin == "" {
		nftBin = defaultNFTBin
	}
	return &NFTablesExecutor{tableName: tableName, nftBin: nftBin}
}

// CheckAvailability 检查 nftables 是否可用。
func (e *NFTablesExecutor) CheckAvailability(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, e.nftBin, "--version")
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("nftables not available: %w", err)
	}
	return nil
}

// Apply 原子化应用转发规则（通过 nftables）。
func (e *NFTablesExecutor) Apply(ctx context.Context, rules []*agentv1.ForwardingRule) error {
	script, err := e.generateNFTScript(rules)
	if err != nil {
		return err
	}
	if err := e.runNft(ctx, []string{"-c", "-f", "-"}, script); err != nil {
		return fmt.Errorf("nft validate failed: %w", err)
	}
	if err := e.runNft(ctx, []string{"-f", "-"}, script); err != nil {
		return fmt.Errorf("nft apply failed: %w", err)
	}
	return nil
}

// Cleanup 删除转发表（若存在）。
func (e *NFTablesExecutor) Cleanup(ctx context.Context) error {
	script := fmt.Sprintf("table inet %s\n", e.tableName)
	script += fmt.Sprintf("delete table inet %s\n", e.tableName)
	if err := e.runNft(ctx, []string{"-f", "-"}, script); err != nil {
		if isNFTNoSuchTable(err) {
			return nil
		}
		return fmt.Errorf("nft cleanup failed: %w", err)
	}
	return nil
}

func (e *NFTablesExecutor) runNft(ctx context.Context, args []string, script string) error {
	cmd := exec.CommandContext(ctx, e.nftBin, args...)
	cmd.Stdin = strings.NewReader(script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
}

func (e *NFTablesExecutor) generateNFTScript(rules []*agentv1.ForwardingRule) (string, error) {
	if e.tableName == "" {
		return "", errors.New("nftables table name is empty")
	}

	sorted := make([]*agentv1.ForwardingRule, 0, len(rules))
	for _, rule := range rules {
		if rule == nil || !rule.Enabled {
			continue
		}
		sorted = append(sorted, rule)
	}

	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority == sorted[j].Priority {
			return sorted[i].Id < sorted[j].Id
		}
		return sorted[i].Priority < sorted[j].Priority
	})

	var b strings.Builder
	b.WriteString("table inet ")
	b.WriteString(e.tableName)
	b.WriteString("\n")
	b.WriteString("delete table inet ")
	b.WriteString(e.tableName)
	b.WriteString("\n\n")
	b.WriteString("table inet ")
	b.WriteString(e.tableName)
	b.WriteString(" {\n")
	b.WriteString("    chain prerouting {\n")
	b.WriteString("        type nat hook prerouting priority dstnat; policy accept;\n")

	for _, rule := range sorted {
		lines, err := buildPreroutingRule(rule)
		if err != nil {
			return "", err
		}
		for _, line := range lines {
			b.WriteString("        ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("    }\n\n")
	b.WriteString("    chain postrouting {\n")
	b.WriteString("        type nat hook postrouting priority srcnat; policy accept;\n")

	for _, rule := range sorted {
		lines, err := buildPostroutingRule(rule)
		if err != nil {
			return "", err
		}
		for _, line := range lines {
			b.WriteString("        ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("    }\n")
	b.WriteString("}\n")
	return b.String(), nil
}

func buildPreroutingRule(rule *agentv1.ForwardingRule) ([]string, error) {
	addr := strings.TrimSpace(rule.TargetAddress)
	if addr == "" {
		return nil, errors.New("target address is empty")
	}
	port := rule.TargetPort
	if port <= 0 {
		return nil, errors.New("target port is invalid")
	}
	listen := rule.ListenPort
	if listen <= 0 {
		return nil, errors.New("listen port is invalid")
	}

	switch strings.ToLower(strings.TrimSpace(rule.Protocol)) {
	case "tcp":
		return []string{fmt.Sprintf("%stcp dport %d dnat to %s:%d", nfprotoPrefix(addr), listen, addr, port)}, nil
	case "udp":
		return []string{fmt.Sprintf("%sudp dport %d dnat to %s:%d", nfprotoPrefix(addr), listen, addr, port)}, nil
	case "both":
		return []string{
			fmt.Sprintf("%stcp dport %d dnat to %s:%d", nfprotoPrefix(addr), listen, addr, port),
			fmt.Sprintf("%sudp dport %d dnat to %s:%d", nfprotoPrefix(addr), listen, addr, port),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", rule.Protocol)
	}
}

func buildPostroutingRule(rule *agentv1.ForwardingRule) ([]string, error) {
	addr := strings.TrimSpace(rule.TargetAddress)
	if addr == "" {
		return nil, errors.New("target address is empty")
	}
	port := rule.TargetPort
	if port <= 0 {
		return nil, errors.New("target port is invalid")
	}

	switch strings.ToLower(strings.TrimSpace(rule.Protocol)) {
	case "tcp":
		return []string{fmt.Sprintf("%s%stcp dport %d masquerade", nfprotoPrefix(addr), nfprotoAddrLabel(addr), port)}, nil
	case "udp":
		return []string{fmt.Sprintf("%s%sudp dport %d masquerade", nfprotoPrefix(addr), nfprotoAddrLabel(addr), port)}, nil
	case "both":
		return []string{
			fmt.Sprintf("%s%stcp dport %d masquerade", nfprotoPrefix(addr), nfprotoAddrLabel(addr), port),
			fmt.Sprintf("%s%sudp dport %d masquerade", nfprotoPrefix(addr), nfprotoAddrLabel(addr), port),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", rule.Protocol)
	}
}

func nfprotoPrefix(addr string) string {
	if isIPv6(addr) {
		return "meta nfproto ipv6 "
	}
	return "meta nfproto ipv4 "
}

func nfprotoAddrLabel(addr string) string {
	if isIPv6(addr) {
		return "ip6 daddr " + addr + " "
	}
	return "ip daddr " + addr + " "
}

func isIPv6(addr string) bool {
	parsed := net.ParseIP(strings.TrimSpace(addr))
	return parsed != nil && parsed.To4() == nil
}

func isNFTNoSuchTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "not found")
}
