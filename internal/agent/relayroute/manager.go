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
	Role          string `json:"role"`             // "entry" | "mid" | "exit"
	NextHopMeshIP string `json:"next_hop_mesh_ip"` // 兼容字段：entry 时为首跳 mesh IP

	// v2 独立辅助隧道（entry/exit 使用）：
	// 点对点 wg 接口 xr<pathID>，外层 UDP 走 mesh 骨干（endpoint=对端 mesh WG IP），
	// 内层 0.0.0.0/0 无冲突归属本接口，彻底消除单接口多 0/0 互抢。
	IfaceName     string `json:"iface_name"`
	ListenPort    int    `json:"listen_port"`
	LocalAddr     string `json:"local_addr"`
	OwnPrivateKey string `json:"own_private_key"`
	PeerPublicKey string `json:"peer_public_key"`
	PeerEndpoint  string `json:"peer_endpoint"`
	MeshCIDR      string `json:"mesh_cidr"`
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
	if err := m.applyExitTunnels(ctx, desired); err != nil {
		return err
	}
	return m.applyMidInfra(ctx, desired)
}

// applyExitTunnels 出口角色：确保对称辅助隧道 + 隧道内转发/NAT 规则（按 iface 动态、幂等）。
func (m *Manager) applyExitTunnels(ctx context.Context, desired []Role) error {
	for _, r := range desired {
		if r.Role != "exit" || r.IfaceName == "" {
			continue
		}
		if err := m.ensureTunnel(ctx, r); err != nil {
			return fmt.Errorf("ensure tunnel %s: %w", r.IfaceName, err)
		}
		if err := m.ensureTunnelForwardNat(ctx, r); err != nil {
			return fmt.Errorf("tunnel forward/nat %s: %w", r.IfaceName, err)
		}
		m.logger.Info("relay-route: exit tunnel ready",
			slog.String("iface", r.IfaceName), slog.Int("table", r.Table))
	}
	return nil
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
		if r.Role != "entry" || r.IfaceName == "" {
			continue
		}
		if err := m.ensureTunnel(ctx, r); err != nil {
			return fmt.Errorf("ensure tunnel %s: %w", r.IfaceName, err)
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
		// v2: 默认路由直接走辅助隧道接口（内层 0/0 归属本隧道 peer，不再 via wgmesh0）
		if err := m.run(ctx, ip, "route", "replace", "default",
			"dev", r.IfaceName, "table", strconv.Itoa(r.Table)); err != nil {
			return fmt.Errorf("replace route table %d: %w", r.Table, err)
		}
		m.logger.Info("relay-route: policy route applied",
				slog.Int("table", r.Table), slog.String("dev", r.IfaceName))
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

// ensureTunnel 幂等建立/刷新点对点辅助 wg 隧道 xr<pathID>：
//   - 接口不存在: link add(type wireguard) → addr add → wg set(listen-port/private-key/peer) → link set up
//   - 已存在: 仅刷新 addr(容错已存在) 与 peer(endpoint/allowed-ips/keepalive)
//
// 私钥经临时文件传入(wg 不接受 argv 传私钥)，用后即删。
func (m *Manager) ensureTunnel(ctx context.Context, r Role) error {
	wg, err := m.bin(m.cfg.WgBinary, "wireguard tunnel")
	if err != nil {
		return nil // 降级：无 wg 二进制不 panic
	}
	ip, err := m.bin(m.cfg.IPBinary, "wireguard tunnel")
	if err != nil {
		return nil
	}
	exists := true
	if _, err := m.runCapture(ctx, ip, "link", "show", r.IfaceName); err != nil {
		exists = false
	}
	if !exists {
		if err := m.run(ctx, ip, "link", "add", r.IfaceName, "type", "wireguard"); err != nil {
			return fmt.Errorf("link add %s: %w", r.IfaceName, err)
		}
	}
	keyFile, err := writeTempKeyFile(r.OwnPrivateKey)
	if err != nil {
		return fmt.Errorf("write private key file: %w", err)
	}
	defer func() { _ = os.Remove(keyFile) }()
	if !exists {
		if err := m.run(ctx, wg, "set", r.IfaceName,
			"listen-port", strconv.Itoa(r.ListenPort),
			"private-key", keyFile); err != nil {
			return fmt.Errorf("wg init %s: %w", r.IfaceName, err)
		}
	}
	if err := m.run(ctx, ip, "addr", "add", r.LocalAddr, "dev", r.IfaceName); err != nil {
		// EEXIST 视为幂等成功；其余错误返回
		if !strings.Contains(err.Error(), "exists") {
			return fmt.Errorf("addr add %s: %w", r.IfaceName, err)
		}
	}
	if err := m.run(ctx, wg, "set", r.IfaceName, "peer", r.PeerPublicKey,
		"endpoint", r.PeerEndpoint,
		"allowed-ips", "0.0.0.0/0",
		"persistent-keepalive", "25"); err != nil {
		return fmt.Errorf("wg peer %s: %w", r.IfaceName, err)
	}
	if exists {
		if err := m.run(ctx, wg, "set", r.IfaceName,
			"listen-port", strconv.Itoa(r.ListenPort),
			"private-key", keyFile); err != nil {
			return fmt.Errorf("wg refresh %s: %w", r.IfaceName, err)
		}
	}
	if err := m.run(ctx, ip, "link", "set", r.IfaceName, "up"); err != nil {
		return fmt.Errorf("link set up %s: %w", r.IfaceName, err)
	}
	m.logger.Info("relay-route: aux tunnel ensured",
		slog.String("iface", r.IfaceName), slog.Int("port", r.ListenPort), slog.String("local", r.LocalAddr))
	return nil
}

// ensureTunnelForwardNat 出口侧隧道内转发/NAT（按 iface 幂等）：
//   - ip_forward 常驻(sysctl.d，同 mesh mid 基础设施复用)
//   - nft mgpanel_relay 表内: forward-ok 链 `iifname <iface> accept`
//     postrouting-nat 链 `ip saddr <tunnelNet> masquerade`
//
// 规则以 comment "xr<id>" 打标，存在即跳过。
func (m *Manager) ensureTunnelForwardNat(ctx context.Context, r Role) error {
	nft, err := m.bin(m.cfg.NFTBinary, "tunnel forward/nat")
	if err != nil {
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
	tableOut, err := m.runCapture(ctx, nft, "list", "table", nftTable)
	if err != nil {
		// 表不存在则建基础结构
		base := [][]string{
			{"add", "table", nftTable},
			{"add", "chain", nftTable, "forward-ok", "{", "type", "filter", "hook", "forward", "priority", "-100", ";", "policy", "accept", ";", "}"},
			{"add", "chain", nftTable, "postrouting-nat", "{", "type", "nat", "hook", "postrouting", "priority", "100", ";", "policy", "accept", ";", "}"},
		}
		for _, args := range base {
			if err := m.run(ctx, nft, args...); err != nil {
				return fmt.Errorf("nft %v: %w", args, err)
			}
		}
		tableOut = ""
	}
	tunnelNet := fmt.Sprintf("10.200.%d.0/30", r.PathID%250)
	comment := fmt.Sprintf("comment \"xr%d\"", r.PathID)
	var steps [][]string
	if !strings.Contains(tableOut, `iifname "`+r.IfaceName+`" accept`) && !strings.Contains(tableOut, "iifname "+r.IfaceName+" accept") {
		steps = append(steps, []string{"add", "rule", nftTable, "forward-ok",
			"iifname", r.IfaceName, "accept", comment})
	}
	if !strings.Contains(tableOut, tunnelNet) {
		steps = append(steps, []string{"add", "rule", nftTable, "postrouting-nat",
			"ip", "saddr", tunnelNet, "masquerade", comment})
	}
	for _, args := range steps {
		if err := m.run(ctx, nft, args...); err != nil {
			return fmt.Errorf("nft %v: %w", args, err)
		}
	}
	if len(steps) > 0 {
		m.logger.Info("relay-route: tunnel forward/nat rules added",
			slog.String("iface", r.IfaceName), slog.String("net", tunnelNet))
	}
	return nil
}

// writeTempKeyFile 将 base64 私钥落为 0600 临时文件（wg private-key 参数只收文件路径）。
func writeTempKeyFile(privB64 string) (string, error) {
	f, err := os.CreateTemp("", "mgpanel-wg-key-*")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(f.Name(), 0o600); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if _, err := f.WriteString(privB64 + "\n"); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
