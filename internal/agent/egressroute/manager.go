// Package egressroute 管理本机在出口集内核分发（B 方案）中的 L3 角色。
//
// 入口角色：点对点隧道 xe<memberID> + `ip rule pref 5500 fwmark 60000+memberID`
// + 表 10000+memberID 的 default 路由（把核心打了 mark 的流量搬进隧道）；
// 出口角色：镜像隧道 xi<entryID> + mgpanel_egress 表内 forward 放行与 MASQUERADE。
// 全部动作幂等全量对账（不再期望的规则/路由/隧道被清理），二进制缺失时 Warn 降级，
// 由 Probe 自检结果驱动调用方（I14：未就绪则不应用新配置）。
// 契约见 docs/plans/20260915-egress-dispatch-l3.md §3.2/§10.4/§10.5。
package egressroute

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/creamcroissant/mgpanel/internal/agent/wgtunnel"
)

const (
	// rulePref 是分发入口 fwmark 规则的 preference（I2；relay 用 5000，互不干扰）。
	rulePref = 5500
	// nftTable 是出口 NAT 表名（I8）；与 relay 的 mgpanel_relay 相互隔离。
	nftTable = "mgpanel_egress"

	roleEntry = "entry"
	roleExit  = "exit"
)

// Assignment 是面板下发的单条分发分配（与 GET /api/v1/agent/egress-routes 的 JSON 同构）。
type Assignment struct {
	Role          string  `json:"role"`            // "entry" | "exit"
	EntryAgentID  int64   `json:"entry_agent_id"`  // 入口主机 ID
	MemberAgentID int64   `json:"member_agent_id"` // 成员（出口）主机 ID；exit 角色为 0
	Mark          int     `json:"mark"`            // 60000 + member_agent_id
	Table         int     `json:"table"`           // 10000 + member_agent_id
	Iface         string  `json:"iface"`           // 入口 xe<memberID> / 出口 xi<entryID>
	ListenPort    int     `json:"listen_port"`     // 21000 + seq
	LocalAddr     string  `json:"local_addr"`      // 隧道网段本端地址（入口 .1 / 出口 .2）
	OwnPrivateKey string  `json:"own_private_key"`
	PeerPublicKey string  `json:"peer_public_key"`
	PeerEndpoint  string  `json:"peer_endpoint"` // 对端 mesh IP:listen_port
	TunnelNet     string  `json:"tunnel_net"`    // 10.220.x.y/30
	MTU           int     `json:"mtu"`           // I7：1400
	Sets          []int64 `json:"sets"`          // 仅供观测，agent 不依赖
}

// MemberState 是单个 assignment 的内核就绪自检结果（供 I14 判定与上报）。
type MemberState struct {
	Role          string `json:"role"`
	EntryAgentID  int64  `json:"entry_agent_id"`
	MemberAgentID int64  `json:"member_agent_id"`
	Iface         string `json:"iface"`
	Mark          int    `json:"mark"`
	Table         int    `json:"table"`
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
}

// Config 可注入的二进制路径与日志（空值用默认；测试指向 stub）。
type Config struct {
	IPBinary       string // 默认 "ip"
	WgBinary       string // 默认 "wg"
	NFTBinary      string // 默认 "nft"
	SysctlBinary   string // 默认 "sysctl"
	SysctlConfPath string // 默认 wgtunnel 的 /etc/sysctl.d/90-mgpanel-egress.conf
	Logger         *slog.Logger

	// AppliedConfigPaths 已应用的核心配置路径（核心实际读取的位置：ManagedDir 及其 merged 输出）。
	// 目录会被整体扫描（sing-box 的产物是多个 fragment）。用于精确判定 mark 是否仍被引用。
	AppliedConfigPaths []string
}

func (c *Config) fill() {
	if c.IPBinary == "" {
		c.IPBinary = "ip"
	}
	if c.WgBinary == "" {
		c.WgBinary = "wg"
	}
	if c.NFTBinary == "" {
		c.NFTBinary = "nft"
	}
	if c.SysctlBinary == "" {
		c.SysctlBinary = "sysctl"
	}
}

// Manager 幂等地把期望的分配集应用到本机内核状态。
type Manager struct {
	cfg    Config
	logger *slog.Logger

	// mu 串行化 Apply/Probe（同一 Agent 的同步 goroutine 与状态上报可能并发调用）。
	mu sync.Mutex
	// managedIfaces 记录上一轮 Apply 建过的隧道接口，用于本轮不再期望时清理。
	managedIfaces map[string]bool

	// 以下字段实现"延迟拆除"（GAP-2：切回 socks / 成员移除时，核心可能仍在跑含 mark 出站的
	// 旧 revision，若立即拆表会让已打 mark 的流量落回主表从入口直出）：
	// 本轮不再期望的对象先记入 pending*，只有 CommitRemovals（在成功应用新配置之后调用）
	// 才真正拆除；Apply 内部重新校验 lastDesired*，避免误拆刚被重新期望的对象。
	pendingMarks  map[int]int     // mark → table（入口 fwmark 规则 + 策略路由表）
	pendingIfaces map[string]bool // 隧道接口（入口 xe* / 出口 xi*）
	lastMarks     map[int]int     // 上一轮期望的 mark → table
	lastIfaces    map[string]bool
	// ifaceMark 记录接口 → mark（入口指派下发过），用于判断陈旧接口对应的 mark 是否仍被引用。
	ifaceMark map[string]int
	// appliedRevision 最近一次已知的"已应用配置 revision"（由 service 每轮传入）。
	appliedRevision int64
	// pendingSinceRevision 记账时刻的"已应用配置 revision"。
	// 拆除条件是 currentRevision > pendingSinceRevision（自记账以来确实换过配置），
	// 而不是"本轮前进"——生产路径 syncApplyBatch 是异步入队，本轮判断必然不成立。
	pendingSinceRevision int64
}

func NewManager(cfg Config) *Manager {
	cfg.fill()
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		cfg:           cfg,
		logger:        log,
		managedIfaces: map[string]bool{},
		pendingMarks:  map[int]int{},
		pendingIfaces: map[string]bool{},
		lastMarks:     map[int]int{},
		lastIfaces:    map[string]bool{},
		ifaceMark:     map[string]int{},
	}
}

func (m *Manager) bins() wgtunnel.Bins {
	return wgtunnel.Bins{
		IP:             m.cfg.IPBinary,
		Wg:             m.cfg.WgBinary,
		NFT:            m.cfg.NFTBinary,
		Sysctl:         m.cfg.SysctlBinary,
		SysctlConfPath: m.cfg.SysctlConfPath,
		Logger:         m.logger,
	}
}

// bin 返回二进制绝对路径；缺失时返回 error 并记 Warn（调用方降级跳过该步）。
func (m *Manager) bin(name, purpose string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		m.logger.Warn("egress-route: binary unavailable, skipping step",
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

func (m *Manager) runCapture(ctx context.Context, bin string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	return string(out), err // show/list 类命令非零属正常（如规则/表为空）
}

// Apply 全量对账：desired 为本机应生效的全部分配，新增的补齐、不再期望的清理。
// 二进制缺失时对应步骤整体跳过（Warn 降级），就绪与否交由 Probe/调用方判定。
func (m *Manager) Apply(ctx context.Context, desired []Assignment, appliedRevision int64) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appliedRevision = appliedRevision
	if err := m.applyEntries(ctx, desired); err != nil {
		return err
	}
	if err := m.applyExits(ctx, desired); err != nil {
		return err
	}
	m.reapStaleTunnels(ctx, desired)
	return nil
}

// applyEntries 对账入口角色的 fwmark 规则与策略路由表。
func (m *Manager) applyEntries(ctx context.Context, desired []Assignment) error {
	ip, err := m.bin(m.cfg.IPBinary, "egress policy routing")
	if err != nil {
		return nil // 降级：无 ip 二进制不 panic（Probe 会报 not-ready）
	}
	existing, _ := m.runCapture(ctx, ip, "rule", "show")
	desiredMarks := map[int]int{} // mark -> table
	for _, a := range desired {
		if a.Role != roleEntry || a.Iface == "" {
			continue
		}
		if a.Mark <= 0 || a.Table <= 0 {
			m.logger.Warn("egress-route: entry assignment missing mark/table, skipped",
				slog.Int64("entry_agent_id", a.EntryAgentID), slog.Int64("member_agent_id", a.MemberAgentID))
			continue
		}
		if err := m.ensureTunnel(ctx, a); err != nil {
			return err
		}
		desiredMarks[a.Mark] = a.Table
		m.ifaceMark[a.Iface] = a.Mark
		if !strings.Contains(existing, ruleSpec(a.Mark, a.Table)) {
			if err := m.run(ctx, ip, "rule", "add", "pref", strconv.Itoa(rulePref),
				"fwmark", strconv.Itoa(a.Mark), "lookup", strconv.Itoa(a.Table)); err != nil {
				return fmt.Errorf("add egress mark rule %d: %w", a.Mark, err)
			}
			m.logger.Info("egress-route: fwmark rule added",
				slog.Int("mark", a.Mark), slog.Int("table", a.Table))
		}
		// 内层 default 直接走辅助隧道接口（0.0.0.0/0 归属该 peer）
		if err := m.run(ctx, ip, "route", "replace", "default",
			"dev", a.Iface, "table", strconv.Itoa(a.Table)); err != nil {
			return fmt.Errorf("replace route table %d: %w", a.Table, err)
		}
		m.logger.Info("egress-route: policy route applied",
			slog.Int("table", a.Table), slog.String("dev", a.Iface))
	}
	// 不再期望的 pref 5500 规则/表：先记账，等 CommitRemovals（已应用配置不再引用该 mark）再拆（GAP-2）。
	// 反向清理：仍被期望的 mark 必须从待拆账目里移除——否则账目永不为空，
	// pendingSinceRevision 无法在下一轮重新记账时刷新，回退判定会被永久放宽。
	for mark := range desiredMarks {
		delete(m.pendingMarks, mark)
	}
	m.lastMarks = desiredMarks
	for _, line := range strings.Split(existing, "\n") {
		mark, table, ok := parseEgressRuleLine(line)
		if !ok {
			continue
		}
		if _, want := desiredMarks[mark]; want {
			continue
		}
		if len(m.pendingMarks) == 0 && len(m.pendingIfaces) == 0 {
			m.pendingSinceRevision = m.appliedRevision
		}
		m.pendingMarks[mark] = table
	}
	return nil
}

// CommitRemovals 拆除记账的陈旧内核对象。currentRevision 为调用方已知的"已应用配置
// revision"；只有当它严格大于记账时刻的 revision（说明自记账以来确实成功换过配置、
// 核心已不再跑含 mark 的旧配置）才真正拆除，否则保留账目（GAP-2）。
//
// 注意：调用方应**每轮**调用（而不是只在"本轮 revision 前进"时调用）——生产路径
// syncApplyBatch 是异步入队，本轮判断永远来不及；"自记账以来前进"才能收敛。
func (m *Manager) CommitRemovals(ctx context.Context, currentRevision int64) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pendingMarks) == 0 && len(m.pendingIfaces) == 0 {
		return nil
	}
	// 精确信号：已应用配置里还引用的 mark 一律不拆（覆盖 agent 重启 / apply 滞后等窗口）。
	referenced, referencedKnown := marksReferencedByConfigPaths(m.cfg.AppliedConfigPaths)
	revAdvanced := currentRevision > m.pendingSinceRevision

	markRemovable := func(mark int) bool {
		if _, stillWanted := m.lastMarks[mark]; stillWanted {
			return false // 已被重新期望 → 不拆
		}
		if referencedKnown {
			return !referenced[mark] // 已应用配置不再引用 → 可拆
		}
		return revAdvanced // 无法判定时退回保守的 revision 代理条件
	}
	ifaceRemovable := func(iface string) bool {
		if m.lastIfaces[iface] {
			return false
		}
		// 入口隧道可由 mark 精确判定；出口镜像隧道无法由 mark 表达（其流量由入口侧决定），
		// 故退回 revision 代理条件（拆早只会丢包，不会泄漏）。
		if mark, ok := m.ifaceMark[iface]; ok && referencedKnown {
			return !referenced[mark]
		}
		return revAdvanced
	}
	ip, err := m.bin(m.cfg.IPBinary, "egress stale cleanup")
	if err != nil {
		return nil // 降级：无 ip 二进制则保留记账，下轮重试
	}
	for mark, table := range m.pendingMarks {
		if !markRemovable(mark) {
			continue
		}
		_ = m.run(ctx, ip, "rule", "del", "pref", strconv.Itoa(rulePref),
			"fwmark", strconv.Itoa(mark), "lookup", strconv.Itoa(table))
		_ = m.run(ctx, ip, "route", "flush", "table", strconv.Itoa(table))
		delete(m.pendingMarks, mark)
		m.logger.Info("egress-route: stale egress routing removed",
			slog.Int("mark", mark), slog.Int("table", table))
	}
	for iface := range m.pendingIfaces {
		if !ifaceRemovable(iface) {
			continue
		}
		delete(m.pendingIfaces, iface)
		if _, err := m.runCapture(ctx, ip, "link", "show", iface); err != nil {
			continue // 接口已不存在
		}
		if err := m.run(ctx, ip, "link", "del", iface); err != nil {
			m.logger.Warn("egress-route: stale tunnel removal failed",
				slog.String("iface", iface), slog.String("err", err.Error()))
			continue
		}
		delete(m.managedIfaces, iface)
		m.logger.Info("egress-route: stale tunnel removed", slog.String("iface", iface))
	}
	return nil
}

// applyExits 对账出口角色的镜像隧道与隧道内转发/NAT。
func (m *Manager) applyExits(ctx context.Context, desired []Assignment) error {
	for _, a := range desired {
		if a.Role != roleExit || a.Iface == "" {
			continue
		}
		if err := m.ensureTunnel(ctx, a); err != nil {
			return err
		}
		if err := m.ensureExitForwardNat(ctx, a); err != nil {
			return err
		}
		m.logger.Info("egress-route: exit tunnel ready",
			slog.String("iface", a.Iface), slog.String("tunnel_net", a.TunnelNet))
	}
	return nil
}

// ensureExitForwardNat 出口侧转发/NAT（实现见 wgtunnel.EnsureForwardNat）。
func (m *Manager) ensureExitForwardNat(ctx context.Context, a Assignment) error {
	err := wgtunnel.EnsureForwardNat(ctx, m.bins(), nftTable, a.Iface, a.TunnelNet)
	if errors.Is(err, wgtunnel.ErrBinaryUnavailable) {
		return nil // 降级：无 nft/sysctl 时跳过（Probe 会报 not-ready）
	}
	if err != nil {
		return fmt.Errorf("exit forward/nat %s: %w", a.Iface, err)
	}
	return nil
}

// ensureTunnel 幂等建立/刷新该 assignment 的点对点隧道（实现见 wgtunnel.Ensure）。
func (m *Manager) ensureTunnel(ctx context.Context, a Assignment) error {
	err := wgtunnel.Ensure(ctx, m.bins(), wgtunnel.Spec{
		Iface:         a.Iface,
		ListenPort:    a.ListenPort,
		LocalAddr:     a.LocalAddr,
		OwnPrivateKey: a.OwnPrivateKey,
		PeerPublicKey: a.PeerPublicKey,
		PeerEndpoint:  a.PeerEndpoint,
		MTU:           a.MTU,
	})
	if errors.Is(err, wgtunnel.ErrBinaryUnavailable) {
		return nil // 降级：无 ip/wg 时跳过（Probe 会报 not-ready）
	}
	if err != nil {
		return fmt.Errorf("ensure tunnel %s: %w", a.Iface, err)
	}
	m.managedIfaces[a.Iface] = true
	return nil
}

// reapStaleTunnels 删除上一轮建过、本轮不再期望的隧道接口（幂等：接口已不存在视为清理完成）。
// agent 重启会丢失记录，残留接口此时无规则/无 NAT、不承载流量，仅占一个 wg 监听位。
func (m *Manager) reapStaleTunnels(ctx context.Context, desired []Assignment) {
	ip, err := m.bin(m.cfg.IPBinary, "stale tunnel cleanup")
	if err != nil {
		return
	}
	desiredIfaces := map[string]bool{}
	for _, a := range desired {
		if a.Iface != "" {
			desiredIfaces[a.Iface] = true
		}
	}
	m.lastIfaces = desiredIfaces
	// 已消失的接口直接销账；仍存在但不再期望的接口记账，等 CommitRemovals 拆除（GAP-2）。
	for iface := range m.managedIfaces {
		if desiredIfaces[iface] {
			delete(m.pendingIfaces, iface)
			continue
		}
		if _, err := m.runCapture(ctx, ip, "link", "show", iface); err != nil {
			delete(m.managedIfaces, iface) // 接口已不存在，清理完成
			continue
		}
		if len(m.pendingMarks) == 0 && len(m.pendingIfaces) == 0 {
			m.pendingSinceRevision = m.appliedRevision
		}
		m.pendingIfaces[iface] = true
	}
}

// Probe 自检每个 assignment 的内核状态：入口校验 iface 存在 + fwmark 规则存在 + 表内
// default 路由指向该 iface；出口校验 iface 存在。返回逐成员明细（供 I14 判定）。
// 缺 ip 二进制时所有成员报 OK=false（error 字段说明原因），调用方据此跳过本轮配置应用。
func (m *Manager) Probe(ctx context.Context, desired []Assignment) ([]MemberState, error) {
	if m == nil {
		return nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	states := make([]MemberState, 0, len(desired))
	if len(desired) == 0 {
		return states, nil
	}
	ip, err := m.bin(m.cfg.IPBinary, "readiness probe")
	if err != nil {
		for _, a := range desired {
			st := newMemberState(a)
			st.Error = fmt.Sprintf("%s unavailable", m.cfg.IPBinary)
			states = append(states, st)
		}
		return states, nil
	}
	rules, _ := m.runCapture(ctx, ip, "rule", "show")
	for _, a := range desired {
		st := newMemberState(a)
		switch {
		case a.Iface == "":
			st.Error = "assignment missing iface"
		case !m.ifaceExists(ctx, ip, a.Iface):
			st.Error = fmt.Sprintf("interface %s missing", a.Iface)
		case a.Role != roleEntry:
			// 出口角色只要求镜像隧道存在（转发/NAT 由 nft 异步生效）
		case a.Mark <= 0 || a.Table <= 0:
			st.Error = "assignment missing mark/table"
		case !strings.Contains(rules, ruleSpec(a.Mark, a.Table)):
			st.Error = fmt.Sprintf("ip rule fwmark %d lookup %d missing", a.Mark, a.Table)
		default:
			out, err := m.runCapture(ctx, ip, "route", "show", "default", "table", strconv.Itoa(a.Table))
			if err != nil || !strings.Contains(out, "dev "+a.Iface) {
				st.Error = fmt.Sprintf("default route via %s missing in table %d", a.Iface, a.Table)
			}
		}
		st.OK = st.Error == ""
		states = append(states, st)
	}
	return states, nil
}

func newMemberState(a Assignment) MemberState {
	return MemberState{
		Role:          a.Role,
		EntryAgentID:  a.EntryAgentID,
		MemberAgentID: a.MemberAgentID,
		Iface:         a.Iface,
		Mark:          a.Mark,
		Table:         a.Table,
	}
}

func (m *Manager) ifaceExists(ctx context.Context, ip, iface string) bool {
	_, err := m.runCapture(ctx, ip, "link", "show", iface)
	return err == nil
}

// ruleSpec 与 `ip rule show` 输出片段匹配用的规格串（iproute2 输出形如
// "5500:\tfrom all fwmark 0xea64 lookup 10004"）。
func ruleSpec(mark, table int) string {
	return fmt.Sprintf("fwmark 0x%x lookup %d", mark, table)
}

// parseEgressRuleLine 从 `ip rule show` 行提取 fwmark(十进制)与 table 号。
// 非 pref 5500 的规则（如 relay 的 5000、系统默认规则）返回 ok=false，避免误删他人规则。
func parseEgressRuleLine(line string) (mark, table int, ok bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 || fields[0] != strconv.Itoa(rulePref)+":" {
		return 0, 0, false
	}
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
