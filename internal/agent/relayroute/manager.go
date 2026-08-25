// Package relayroute 管理本机在服务器中继链路中的内核路由角色。
//
// 三层路由模型中 L3（物理路径）下沉内核：入口角色打 fwmark 并用策略路由把
// 匹配流量导向下一跳 mesh IP；中间角色开启 ip_forward 并放行 wgmesh0 转发；
// 出口角色无需动作。全部命令经可注入二进制执行（测试 stub），缺失时降级 Warn。
package relayroute

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Role 描述本机在某条链路中的角色（与面板 GET /api/v1/agent/relay-routes 契约一致）。
type Role struct {
	PathID        int64  `json:"path_id"`
	Mark          int    `json:"mark"`             // 40000 + path_id
	Table         int    `json:"table"`            // 100 + path_id
	Role          string `json:"role"`             // "entry" | "mid"
	NextHopMeshIP string `json:"next_hop_mesh_ip"` // entry 必填：下一跳 mesh WG IP
}

const (
	prefMarkRule = 5000
	defaultIface = "wgmesh0"

	nftTable         = "mgpanel_relay"
	sysctlConfPath   = "/etc/sysctl.d/90-mgpanel-relay.conf"
	sysctlConfBody   = "net.ipv4.ip_forward = 1\n"
	ruleMarkerPrefix = "# mgpanel-relay:"
)

// Config 可注入的二进制路径与常量覆盖（空值用默认；测试指向 stub）。
type Config struct {
	IPBinary       string // 默认 "ip"
	NFTBinary      string // 默认 "nft"
	SysctlBinary   string // 默认 "sysctl"
	WgBinary       string // 默认 "wg"；用于入口角色扩展出口 peer AllowedIPs
	InterfaceName  string // 默认 wgmesh0
	SysctlConfPath string // 默认 /etc/sysctl.d/90-mgpanel-relay.conf
	Logger         *slog.Logger
}

func (c *Config) fill() {
	if c.IPBinary == "" {
		c.IPBinary = "ip"
	}
	if c.NFTBinary == "" {
		c.NFTBinary = "nft"
	}
	if c.SysctlBinary == "" {
		c.SysctlBinary = "sysctl"
	}
	if c.InterfaceName == "" {
		c.InterfaceName = defaultIface
	}
	if c.SysctlConfPath == "" {
		c.SysctlConfPath = sysctlConfPath
	}
	if c.WgBinary == "" {
		c.WgBinary = "wg"
	}
}

// Manager 幂等地把期望角色集应用到本机内核状态。
type Manager struct {
	cfg    Config
	logger *slog.Logger
}

func NewManager(cfg Config) *Manager {
	cfg.fill()
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Manager{cfg: cfg, logger: log}
}

// bin 返回二进制绝对路径；缺失时返回空串并记 Warn（调用方跳过该步不报错）。
func (m *Manager) bin(name, purpose string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		m.logger.Warn("relay-route: binary unavailable, skipping step",
			slog.String("binary", name), slog.String("purpose", purpose), slog.String("err", err.Error()))
		return "", fmt.Errorf("%s unavailable: %w", name, err)
	}
	return path, nil
}

func (m *Manager) run(ctx context.Context, bin string, args ...string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", bin, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Apply 全量对账：desired 为本机应生效的全部角色，缺失的清理、新增的补齐。
// 任一二进制缺失时对应类别整体跳过（Warn），其余类别照常应用。
func (m *Manager) Apply(ctx context.Context, desired []Role) error {
	if err := m.applyEntries(ctx, desired); err != nil {
		return err
	}
	return m.applyMidInfra(ctx, desired)
}

// applyEntries 对账 fwmark 规则与策略路由表（仅 entry 角色）。
func (m *Manager) applyEntries(ctx context.Context, desired []Role) error {
	ip, err := m.bin(m.cfg.IPBinary, "policy routing")
	if err != nil {
		return nil // 降级：无 ip 不 panic
	}
	existing, _ := m.runCapture(ctx, ip, "rule", "show")
	desiredMarks := map[int]bool{}
	for _, r := range desired {
		if r.Role != "entry" || r.NextHopMeshIP == "" {
			continue
		}
		desiredMarks[r.Mark] = true
		spec := ruleSpec(r.Mark, r.Table)
		if !strings.Contains(existing, spec) {
			if err := m.run(ctx, ip, "rule", "add", "pref", strconv.Itoa(prefMarkRule),
				"fwmark", strconv.Itoa(r.Mark), "lookup", strconv.Itoa(r.Table)); err != nil {
				return fmt.Errorf("add mark rule: %w", err)
			}
			m.logger.Info("relay-route: fwmark rule added", slog.Int("mark", r.Mark), slog.Int("table", r.Table))
		}
		if err := m.run(ctx, ip, "route", "replace", "default", "via", r.NextHopMeshIP,
			"dev", m.cfg.InterfaceName, "table", strconv.Itoa(r.Table)); err != nil {
			return fmt.Errorf("replace route table %d: %w", r.Table, err)
		}
		m.logger.Info("relay-route: policy route applied",
				slog.Int("table", r.Table), slog.String("via", r.NextHopMeshIP))
		m.maybeExpandWGAllowedIPs(ctx, r.NextHopMeshIP)
	}
	// 清理不再期望的 mark 规则/路由
	for _, line := range strings.Split(existing, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "fwmark") || !strings.Contains(line, "lookup") {
			continue
		}
		mark, table, ok := parseRuleLine(line)
		if !ok || desiredMarks[mark] {
			continue
		}
		_ = m.run(ctx, ip, "rule", "del", "pref", strconv.Itoa(prefMarkRule),
			"fwmark", strconv.Itoa(mark), "lookup", strconv.Itoa(table))
		_ = m.run(ctx, ip, "route", "flush", "table", strconv.Itoa(table))
		m.logger.Info("relay-route: stale relay routing removed", slog.Int("mark", mark), slog.Int("table", table))
	}
	return nil
}

// applyMidInfra 中间/出口跳基础设施：ip_forward + nftables forward 放行。
// 存在任一 mid 或 exit 角色即确保就位；全量对账时不主动拆除。
func (m *Manager) applyMidInfra(ctx context.Context, desired []Role) error {
	hasMid := false
	for _, r := range desired {
		if r.Role == "mid" || r.Role == "exit" {
			hasMid = true
			break
		}
	}
	if !hasMid {
		return nil
	}
	sysctl, err := m.bin(m.cfg.SysctlBinary, "ip_forward")
	if err != nil {
		return nil
	}
	if err := os.WriteFile(m.cfg.SysctlConfPath, []byte(sysctlConfBody), 0o644); err != nil {
		return fmt.Errorf("write sysctl conf: %w", err)
	}
	if err := m.run(ctx, sysctl, "--system"); err != nil {
		return fmt.Errorf("sysctl --system: %w", err)
	}
	nft, err := m.bin(m.cfg.NFTBinary, "forward allow")
	if err != nil {
		return nil
	}
	if _, err := m.runCapture(ctx, nft, "list", "table", nftTable); err != nil {
		// 表不存在则逐条创建（argv 形式，便于审计与测试观测）；含 forward 放行与出口 MASQUERADE
		meshCidr := m.meshCIDR()
		steps := [][]string{
			{"add", "table", nftTable},
			{"add", "chain", nftTable, "forward-ok", "{", "type", "filter", "hook", "forward", "priority", "-100", ";", "policy", "accept", ";", "}"},
			{"add", "rule", nftTable, "forward-ok", "iifname", m.cfg.InterfaceName, "oifname", m.cfg.InterfaceName, "accept"},
			{"add", "chain", nftTable, "postrouting-nat", "{", "type", "nat", "hook", "postrouting", "priority", "100", ";", "policy", "accept", ";", "}"},
			{"add", "rule", nftTable, "postrouting-nat", "ip", "saddr", meshCidr, "oifname", "!=", m.cfg.InterfaceName, "masquerade"},
		}
		for _, args := range steps {
			if err := m.run(ctx, nft, args...); err != nil {
				return fmt.Errorf("nft %v: %w", args, err)
			}
		}
		m.logger.Info("relay-route: nftables forward+nat infra created", slog.String("iface", m.cfg.InterfaceName))
	}
	return nil
}

func (m *Manager) runCapture(ctx context.Context, bin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	return string(out), err // show/list 类命令非零属正常（如规则为空）
}

// ruleSpec 与 `ip rule show` 输出片段匹配用的规格串（iproute2 输出形如
// "5000:\tfrom all fwmark 0x9d41 lookup 101"）。
func ruleSpec(mark, table int) string {
	return fmt.Sprintf("fwmark 0x%x lookup %d", mark, table)
}

// parseRuleLine 从 ip rule show 行提取 fwmark(十进制)与 table 号。
func parseRuleLine(line string) (mark, table int, ok bool) {
	fields := strings.Fields(line)
	for i, f := range fields {
		if f == "fwmark" && i+1 < len(fields) {
			v, err := strconv.ParseInt(strings.TrimPrefix(fields[i+1], "0x"), 16, 32)
			if err != nil {
				return 0, 0, false
			}
			mark = int(v)
		}
		if f == "lookup" && i+1 < len(fields) {
			v, err := strconv.Atoi(fields[i+1])
			if err != nil {
				return 0, 0, false
			}
			table = v
			ok = true
		}
	}
	if mark == 0 {
		return 0, 0, false
	}
	return mark, table, ok
}

// maybeExpandWGAllowedIPs 为入口角色的下一跳 mesh IP 对应的 WG peer 追加 0.0.0.0/0。
// 这样 marked 流量才能通过该 peer 的隧道出口出网（默认每 peer 只有 /32 授权）。
func (m *Manager) maybeExpandWGAllowedIPs(ctx context.Context, nextHopMeshIP string) {
	wg, err := m.bin(m.cfg.WgBinary, "wg set")
	if err != nil {
		return
	}
	// 解析 wg show 输出找到该 IP 对应的 peer 公钥
	out, err := m.runCapture(ctx, wg, "show", m.cfg.InterfaceName, "allowed-ips")
	if err != nil {
		m.logger.Warn("relay-route: wg show allowed-ips failed", slog.String("err", err.Error()))
		return
	}
	pubKey := ""
	var currentIPs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		for _, aip := range fields[1:] {
			if strings.HasPrefix(aip, nextHopMeshIP) {
				pubKey = fields[0]
				currentIPs = fields[1:]
				break
			}
		}
		if pubKey != "" {
			break
		}
	}
	if pubKey == "" {
		return
	}
	// 检查是否已含 0.0.0.0/0
	for _, aip := range currentIPs {
		if aip == "0.0.0.0/0" {
			return // 已存在，无需重复添加
		}
	}
	newIPs := fmt.Sprintf("%s,0.0.0.0/0", strings.Join(currentIPs, ","))
	if err := m.run(ctx, wg, "set", m.cfg.InterfaceName, "peer", pubKey, "allowed-ips", newIPs); err != nil {
		m.logger.Warn("relay-route: wg set allowed-ips failed", slog.String("err", err.Error()))
		return
	}
	m.logger.Info("relay-route: allowed-ips expanded to include 0.0.0.0/0",
		slog.String("peer", pubKey[:16]+"..."), slog.String("next_hop", nextHopMeshIP))
}

// meshCIDR 返回 mesh 组网网段（与面板 Mesh 网络 NetworkCIDR 10.144.0.0/24 一致）。
// 出口 masquerade 需将发自本网段的包源地址改写为公网。
func (m *Manager) meshCIDR() string {
	return "10.144.0.0/24"
}
