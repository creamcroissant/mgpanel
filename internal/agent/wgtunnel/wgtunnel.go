// Package wgtunnel 提供点对点 WireGuard 辅助隧道与隧道内转发/NAT 的幂等内核操作。
//
// relayroute（服务器中继链路）与 egressroute（出口集内核分发）共用同一套动作：
// 隧道只承载标记后的内层流量（allowed-ips 0.0.0.0/0，外层 UDP 走 mesh 骨干），
// 出口侧用 nft forward accept + postrouting masquerade 做纯内核转发。
// 全部命令经可注入的二进制路径执行（测试指向 stub）；二进制缺失时 Warn 并返回
// ErrBinaryUnavailable，由调用方决定降级语义（relay 直接跳过，egress 交由 Probe 报未就绪）。
package wgtunnel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ErrBinaryUnavailable 表示某个必需的外部命令不存在（调用方据此降级跳过而非失败）。
var ErrBinaryUnavailable = errors.New("required binary unavailable")

const (
	// defaultSysctlConfPath 是出口侧常驻 ip_forward 的落盘位置（relayroute 用自己的路径）。
	defaultSysctlConfPath = "/etc/sysctl.d/90-mgpanel-egress.conf"
	ipForwardConfBody     = "net.ipv4.ip_forward = 1\n"
)

// Spec 描述一条点对点辅助隧道。
type Spec struct {
	Iface         string // 接口名（入口 xe<memberID> / 出口 xi<entryID> / 中继 xr<pathID>）
	ListenPort    int    // 隧道监听端口（外层走 mesh 骨干，不新增公网暴露）
	LocalAddr     string // 隧道网段本端地址（含前缀，如 10.220.0.1/30）
	OwnPrivateKey string // base64 私钥；经 0600 临时文件传给 wg（wg 不接受 argv 传私钥）
	PeerPublicKey string
	PeerEndpoint  string // 对端 mesh IP:listen_port
	MTU           int    // >0 时显式 `ip link set <iface> mtu`；0 表示沿用内核默认
}

// Bins 汇总可注入的外部命令路径与运行期文件路径（空值取默认；测试指向 stub）。
type Bins struct {
	IP             string       // 默认 "ip"
	Wg             string       // 默认 "wg"
	NFT            string       // 默认 "nft"
	Sysctl         string       // 默认 "sysctl"
	SysctlConfPath string       // 默认 /etc/sysctl.d/90-mgpanel-egress.conf
	Logger         *slog.Logger // 默认 slog.Default()
}

func (b *Bins) fill() {
	if b.IP == "" {
		b.IP = "ip"
	}
	if b.Wg == "" {
		b.Wg = "wg"
	}
	if b.NFT == "" {
		b.NFT = "nft"
	}
	if b.Sysctl == "" {
		b.Sysctl = "sysctl"
	}
	if b.SysctlConfPath == "" {
		b.SysctlConfPath = defaultSysctlConfPath
	}
	if b.Logger == nil {
		b.Logger = slog.Default()
	}
}

// Ensure 幂等建立/刷新一条点对点 WireGuard 隧道：
//   - 接口不存在: link add(type wireguard) → wg set(listen-port/private-key) → addr add
//     → wg set(peer: endpoint/allowed-ips 0.0.0.0/0/persistent-keepalive 25) → link set up
//   - 接口已存在: 仅刷新 addr（容错已存在）与 peer，并重新下发 listen-port/private-key
//
// MTU > 0 时在 link set up 之前显式设置（隧道套 mesh，内核默认值无余量）。
func Ensure(ctx context.Context, bins Bins, spec Spec) error {
	bins.fill()
	if strings.TrimSpace(spec.Iface) == "" {
		return errors.New("wgtunnel: empty interface name")
	}
	wg, err := bin(bins, bins.Wg, "wireguard tunnel")
	if err != nil {
		return err
	}
	ip, err := bin(bins, bins.IP, "wireguard tunnel")
	if err != nil {
		return err
	}

	exists := true
	if _, err := runCapture(ctx, ip, "link", "show", spec.Iface); err != nil {
		exists = false
	}
	if !exists {
		if err := run(ctx, ip, "link", "add", spec.Iface, "type", "wireguard"); err != nil {
			return fmt.Errorf("link add %s: %w", spec.Iface, err)
		}
	}
	keyFile, err := WriteTempKeyFile(spec.OwnPrivateKey)
	if err != nil {
		return fmt.Errorf("write private key file: %w", err)
	}
	defer func() { _ = os.Remove(keyFile) }()
	if !exists {
		if err := run(ctx, wg, "set", spec.Iface,
			"listen-port", strconv.Itoa(spec.ListenPort),
			"private-key", keyFile); err != nil {
			return fmt.Errorf("wg init %s: %w", spec.Iface, err)
		}
	}
	if err := run(ctx, ip, "addr", "add", spec.LocalAddr, "dev", spec.Iface); err != nil {
		// EEXIST 视为幂等成功；其余错误返回
		if !strings.Contains(err.Error(), "exists") {
			return fmt.Errorf("addr add %s: %w", spec.Iface, err)
		}
	}
	if err := run(ctx, wg, "set", spec.Iface, "peer", spec.PeerPublicKey,
		"endpoint", spec.PeerEndpoint,
		"allowed-ips", "0.0.0.0/0",
		"persistent-keepalive", "25"); err != nil {
		return fmt.Errorf("wg peer %s: %w", spec.Iface, err)
	}
	if exists {
		if err := run(ctx, wg, "set", spec.Iface,
			"listen-port", strconv.Itoa(spec.ListenPort),
			"private-key", keyFile); err != nil {
			return fmt.Errorf("wg refresh %s: %w", spec.Iface, err)
		}
	}
	if spec.MTU > 0 {
		if err := run(ctx, ip, "link", "set", spec.Iface, "mtu", strconv.Itoa(spec.MTU)); err != nil {
			return fmt.Errorf("link set mtu %s: %w", spec.Iface, err)
		}
	}
	if err := run(ctx, ip, "link", "set", spec.Iface, "up"); err != nil {
		return fmt.Errorf("link set up %s: %w", spec.Iface, err)
	}
	bins.Logger.Info("wgtunnel: tunnel ensured",
		slog.String("iface", spec.Iface), slog.Int("port", spec.ListenPort),
		slog.String("local", spec.LocalAddr), slog.Int("mtu", spec.MTU))
	return nil
}

// EnsureForwardNat 幂等确保出口侧的隧道内转发与 NAT：
//   - ip_forward 常驻（sysctl.d 落盘 + sysctl --system，与 mesh/relay 同一开关）
//   - nft 表 table 不存在则建表与 forward-ok / postrouting-nat 两条链
//   - forward-ok: `iifname <iface> accept`；postrouting-nat: `ip saddr <tunnelNet> masquerade`
//
// 两条规则以 comment "<iface>" 打标，表内已存在即跳过（按 iface 幂等）。
func EnsureForwardNat(ctx context.Context, bins Bins, table, iface, tunnelNet string) error {
	bins.fill()
	if strings.TrimSpace(table) == "" || strings.TrimSpace(iface) == "" || strings.TrimSpace(tunnelNet) == "" {
		return errors.New("wgtunnel: EnsureForwardNat requires table, iface and tunnelNet")
	}
	nft, err := bin(bins, bins.NFT, "tunnel forward/nat")
	if err != nil {
		return err
	}
	sysctl, err := bin(bins, bins.Sysctl, "ip_forward")
	if err != nil {
		return err
	}
	if err := os.WriteFile(bins.SysctlConfPath, []byte(ipForwardConfBody), 0o644); err != nil {
		return fmt.Errorf("write sysctl conf: %w", err)
	}
	if err := run(ctx, sysctl, "--system"); err != nil {
		return fmt.Errorf("sysctl --system: %w", err)
	}

	tableOut, err := runCapture(ctx, nft, "list", "table", table)
	if err != nil {
		// 表不存在则建基础结构
		base := [][]string{
			{"add", "table", table},
			{"add", "chain", table, "forward-ok", "{", "type", "filter", "hook", "forward", "priority", "-100", ";", "policy", "accept", ";", "}"},
			{"add", "chain", table, "postrouting-nat", "{", "type", "nat", "hook", "postrouting", "priority", "100", ";", "policy", "accept", ";", "}"},
		}
		for _, args := range base {
			if err := run(ctx, nft, args...); err != nil {
				return fmt.Errorf("nft %v: %w", args, err)
			}
		}
		tableOut = ""
	}

	comment := fmt.Sprintf("comment %q", iface)
	var steps [][]string
	if !strings.Contains(tableOut, `iifname "`+iface+`" accept`) && !strings.Contains(tableOut, "iifname "+iface+" accept") {
		steps = append(steps, []string{"add", "rule", table, "forward-ok",
			"iifname", iface, "accept", comment})
	}
	if !strings.Contains(tableOut, tunnelNet) {
		steps = append(steps, []string{"add", "rule", table, "postrouting-nat",
			"ip", "saddr", tunnelNet, "masquerade", comment})
	}
	for _, args := range steps {
		if err := run(ctx, nft, args...); err != nil {
			return fmt.Errorf("nft %v: %w", args, err)
		}
	}
	// 穿透主机既有防火墙：许多出口机带 iptables/ufw/docker 的 `filter FORWARD` 链，
	// 其 policy 往往是 drop。nftables 同一 hook 上的多个 base chain 都会被评估，
	// 因此仅靠本子系统 priority -100 的 forward-ok 链**不足以**放行（生产实测：
	// Docker 出口机上隧道包被 FORWARD policy drop 丢弃）。
	// 在这些既有链的**链首**插入针对本隧道接口的双向 accept（幂等，按 comment 标记识别）。
	if err := ensureLegacyForwardAccept(ctx, bins, nft, iface); err != nil {
		return err
	}
	if len(steps) > 0 {
		bins.Logger.Info("wgtunnel: tunnel forward/nat rules added",
			slog.String("iface", iface), slog.String("net", tunnelNet), slog.String("table", table))
	}
	return nil
}

// tunnelAcceptMarker 本隧道接口在既有防火墙链里插入的 accept 规则标记（幂等识别用）。
func tunnelAcceptMarker(iface string) string { return "mgpanel-tunnel:" + iface }

// ensureLegacyForwardAccept 在既有 `filter FORWARD` 链（ip / inet 两族）里保证
// 「iifname <iface> accept」与「oifname <iface> accept」两条规则存在。
//
// 为什么需要：nftables 同一 hook 的多个 base chain 都会被评估，其它链（iptables-nft /
// ufw / docker 留下的 `filter FORWARD`，policy drop）会把隧道转发流量丢掉。
// 在链首插入 accept 即可放行；链不存在则跳过（纯 nft 防火墙场景）。
//
// 只处理**接口维度**（iifname/oifname），不放开其它流量，保持最小授权：
//   - iifname: 从隧道进来的客户端流量
//   - oifname: 回程（互联网 → 隧道 → 入口）
func ensureLegacyForwardAccept(ctx context.Context, bins Bins, nft, iface string) error {
	marker := tunnelAcceptMarker(iface)
	for _, family := range []string{"ip", "inet"} {
		chainOut, err := runCapture(ctx, nft, "list", "chain", family, "filter", "FORWARD")
		if err != nil || strings.TrimSpace(chainOut) == "" {
			continue // 该族无既有 FORWARD 链
		}
		for _, spec := range [][]string{
			{"iifname", iface},
			{"oifname", iface},
		} {
			quoted := spec[0] + ` "` + iface + `"`
			plain := spec[0] + " " + iface
			if strings.Contains(chainOut, marker) && (strings.Contains(chainOut, quoted) || strings.Contains(chainOut, plain)) {
				continue // 已存在（标记 + 同一接口方向）
			}
			args := append([]string{"insert", "rule", family, "filter", "FORWARD"}, spec...)
			// comment 值必须整体作为**一个带引号的 argv 词**传入（nft 解析器要求），
			// 与既有规则同样的写法：comment "xxx"。裸值含 ':' 会报 syntax error。
			args = append(args, "accept", fmt.Sprintf("comment %q", marker))
			if err := run(ctx, nft, args...); err != nil {
				bins.Logger.Warn("wgtunnel: insert legacy forward accept failed",
					slog.String("family", family), slog.String("iface", iface), slog.String("err", err.Error()))
				continue
			}
			bins.Logger.Info("wgtunnel: legacy forward accept inserted",
				slog.String("family", family), slog.String("iface", iface), slog.String("direction", spec[0]))
		}
	}
	return nil
}

// WriteTempKeyFile 将 base64 私钥落为 0600 临时文件（wg private-key 参数只收文件路径）。
func WriteTempKeyFile(privB64 string) (string, error) {
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

// bin 返回二进制绝对路径；缺失时 Warn 并返回 ErrBinaryUnavailable（调用方据此降级）。
func bin(bins Bins, name, purpose string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		bins.Logger.Warn("wgtunnel: binary unavailable, skipping step",
			slog.String("binary", name), slog.String("purpose", purpose), slog.String("err", err.Error()))
		return "", fmt.Errorf("%w: %s: %v", ErrBinaryUnavailable, name, err)
	}
	return path, nil
}

func run(ctx context.Context, bin string, args ...string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", bin, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runCapture(ctx context.Context, bin string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	return string(out), err // show/list 类命令非零属正常（如接口/表不存在）
}
