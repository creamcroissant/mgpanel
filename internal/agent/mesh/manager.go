package mesh

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PeerStatus describes a single WireGuard peer's live state.
type PeerStatus struct {
	ID              string `json:"id"`
	PublicKey       string `json:"public_key"`
	Endpoint        string `json:"endpoint"`
	AllowedIPs      string `json:"allowed_ips"`
	LatestHandshake int64  `json:"latest_handshake"` // unix timestamp
	TransferRx      int64  `json:"transfer_rx"`
	TransferTx      int64  `json:"transfer_tx"`
	Online          bool   `json:"online"`
}

// LocalStatus describes the local WireGuard interface status.
type LocalStatus struct {
	PublicKey string `json:"public_key"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
}

// Config holds the configuration for the mesh manager.
type Config struct {
	// IPBinary 覆盖 ip(8) 工具路径；空值用 "ip"（测试可指向 stub）。
	IPBinary string
	ConfigDir     string
	InterfaceName string
	ListenPort    int
	WgBinary      string
	NetworkCIDR   string
}

// Manager manages a WireGuard mesh network interface.
type Manager struct {
	cfg    Config
	wg     *WGExecutor
	cm     *ConfigManager
	logger *slog.Logger
	mu     sync.Mutex
}

// NewManager creates a new mesh Manager.
func NewManager(cfg Config, logger *slog.Logger) *Manager {
	return &Manager{
		cfg:    cfg,
		wg:     NewWGExecutor(cfg.WgBinary, cfg.InterfaceName),
		cm:     NewConfigManager(cfg.ConfigDir),
		logger: logger,
	}
}

// InterfaceName returns the WireGuard interface name.
func (m *Manager) InterfaceName() string {
	return m.cfg.InterfaceName
}

// WgBinary returns the path to the wg binary.
func (m *Manager) WgBinary() string {
	return m.cfg.WgBinary
}

// NetworkCIDR returns the configured network CIDR.
func (m *Manager) NetworkCIDR() string {
	return m.cfg.NetworkCIDR
}
// PeerCount returns the number of peers in the config directory.
func (m *Manager) PeerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	peers, err := m.cm.ReadAllPeers()
	if err != nil {
		return 0
	}
	return len(peers)
}

// ReadLocalConfig delegates to ConfigManager.ReadLocalConfig.
func (m *Manager) ReadLocalConfig() (*LocalConfig, error) {
	return m.cm.ReadLocalConfig()
}

// WriteLocalConfig delegates to ConfigManager.WriteLocalConfig.
func (m *Manager) WriteLocalConfig(cfg LocalConfig) error {
	return m.cm.WriteLocalConfig(cfg)
}

// RemoveLocalConfig delegates to ConfigManager.RemoveLocalConfig.
func (m *Manager) RemoveLocalConfig() error {
	return m.cm.RemoveLocalConfig()
}

// BuildWGConfig builds the wg setconf format string from LocalConfig.
func (m *Manager) BuildWGConfig(cfg LocalConfig) string {
	return m.buildWGConfig(cfg)
}

// ApplyWGConfig applies a wg config to the interface.
func (m *Manager) ApplyWGConfig(ctx context.Context, configContent string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.wg.SetConfig(ctx, configContent)
}

// WritePeerConfig delegates to ConfigManager.WritePeerConfig.
func (m *Manager) WritePeerConfig(peer PeerConfig) error {
	return m.cm.WritePeerConfig(peer)
}

// RemovePeerConfig delegates to ConfigManager.RemovePeerConfig.
func (m *Manager) RemovePeerConfig(peerID string) error {
	return m.cm.RemovePeerConfig(peerID)
}

// Start initializes the WireGuard interface and applies configuration.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validateWgBinary(); err != nil {
		return fmt.Errorf("mesh start: %w", err)
	}

	if err := m.cm.EnsureDirs(); err != nil {
		return fmt.Errorf("mesh ensure dirs: %w", err)
	}

	if err := m.createInterface(ctx); err != nil {
		return fmt.Errorf("mesh create interface: %w", err)
	}

	// Read and apply local config if it exists
	localCfg, err := m.cm.ReadLocalConfig()
	if err != nil {
		return fmt.Errorf("mesh read local config: %w", err)
	}
	if localCfg != nil {
		wgConfig := m.buildWGConfig(*localCfg)
		if err := m.wg.SetConfig(ctx, wgConfig); err != nil {
			return fmt.Errorf("mesh set wg config: %w", err)
		}
		if localCfg.Address != "" {
			if err := m.assignIP(ctx, localCfg.Address); err != nil {
				return fmt.Errorf("mesh assign ip: %w", err)
			}
		}
	}

	if err := m.interfaceUp(ctx); err != nil {
		return fmt.Errorf("mesh interface up: %w", err)
	}

	// Sync existing peers
	peers, err := m.cm.ReadAllPeers()
	if err != nil {
		return fmt.Errorf("mesh read peers: %w", err)
	}
	for _, peer := range peers {
		if peer.PublicKey != "" {
			if err := m.wg.AddPeer(ctx, peer.PublicKey, peer.Endpoint, peer.AllowedIPs, peer.Keepalive); err != nil {
				m.logger.Warn("mesh add peer failed", "peer_id", peer.ID, "error", err)
			}
		}
	}

	m.logger.Info("mesh interface started",
		"iface", m.cfg.InterfaceName,
		"port", m.cfg.ListenPort,
		"cidr", m.cfg.NetworkCIDR,
	)
	return nil
}

// Stop tears down the WireGuard interface.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.deleteInterface(ctx); err != nil {
		return fmt.Errorf("mesh delete interface: %w", err)
	}
	m.logger.Info("mesh interface stopped", "iface", m.cfg.InterfaceName)
	return nil
}

// DumpPeers returns live WireGuard peer status by running `wg show <iface> dump`.
func (m *Manager) DumpPeers(ctx context.Context) ([]PeerStatus, error) {
	if err := m.validateWgBinary(); err != nil {
		return nil, fmt.Errorf("mesh dump peers: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := exec.CommandContext(ctx, m.cfg.WgBinary, "show", m.cfg.InterfaceName, "dump")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wg show dump: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return nil, nil // only header line, no peers
	}

	// First line is interface info: private_key, public_key, listen_port, fwmark
	// Following lines are peers: public_key, endpoint, allowed_ips, latest_handshake, transfer_rx, transfer_tx, persistent_keepalive
	var statuses []PeerStatus
	for i, line := range lines {
		if i == 0 {
			continue // skip interface line
		}
		parts := strings.Split(strings.TrimSpace(line), "\t")
		if len(parts) < 6 {
			continue
		}

		hs, _ := strconv.ParseInt(parts[3], 10, 64)
		rx, _ := strconv.ParseInt(parts[4], 10, 64)
		tx, _ := strconv.ParseInt(parts[5], 10, 64)
		now := time.Now().Unix()

		statuses = append(statuses, PeerStatus{
			PublicKey:       parts[0],
			Endpoint:        parts[1],
			AllowedIPs:      parts[2],
			LatestHandshake: hs,
			TransferRx:      rx,
			TransferTx:      tx,
			Online:          hs > 0 && (now-hs) < 180, // online if handshake within 3 min
		})
	}

	return statuses, nil
}

// GetLocalStatus returns the local interface's WG key and assigned IP.
func (m *Manager) GetLocalStatus(ctx context.Context) (*LocalStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	local, err := m.cm.ReadLocalConfig()
	if err != nil || local == nil {
		return nil, err
	}
	pubkey, err := m.getPublicKey(ctx)
	if err != nil {
		return nil, err
	}
	return &LocalStatus{
		PublicKey: pubkey,
		Address:   local.Address,
		Port:      local.ListenPort,
	}, nil
}

func (m *Manager) getPublicKey(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, m.cfg.WgBinary, "show", m.cfg.InterfaceName, "public-key")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("wg show public-key: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// SyncPeers synchronizes the peer list with the provided peers.
func (m *Manager) SyncPeers(ctx context.Context, peers []PeerConfig) error {
	if err := m.validateWgBinary(); err != nil {
		return fmt.Errorf("mesh sync peers: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, err := m.cm.ReadAllPeers()
	if err != nil {
		return fmt.Errorf("sync peers read existing: %w", err)
	}

	existingMap := make(map[string]PeerConfig, len(existing))
	for _, p := range existing {
		existingMap[p.ID] = p
	}

	incomingMap := make(map[string]PeerConfig, len(peers))
	for _, p := range peers {
		incomingMap[p.ID] = p
	}

	// Remove peers that no longer exist
	for id, existingPeer := range existingMap {
		if _, keep := incomingMap[id]; !keep {
			if existingPeer.PublicKey != "" {
				if err := m.wg.RemovePeer(ctx, existingPeer.PublicKey); err != nil {
					m.logger.Warn("sync peers remove failed", "peer_id", id, "error", err)
				}
			}
			if err := m.cm.RemovePeerConfig(id); err != nil {
				m.logger.Warn("sync peers remove config failed", "peer_id", id, "error", err)
			}
		}
	}

	// Add or update peers
	for _, peer := range peers {
		if existingPeer, exists := existingMap[peer.ID]; exists {
			if existingPeer.PublicKey == peer.PublicKey &&
				existingPeer.Endpoint == peer.Endpoint &&
				stringsEqual(existingPeer.AllowedIPs, peer.AllowedIPs) &&
				existingPeer.Keepalive == peer.Keepalive {
				continue // no change
			}
		}

		if err := m.cm.WritePeerConfig(peer); err != nil {
			m.logger.Warn("sync peers write config failed", "peer_id", peer.ID, "error", err)
			continue
		}
		if peer.PublicKey != "" {
			if err := m.wg.AddPeer(ctx, peer.PublicKey, peer.Endpoint, peer.AllowedIPs, peer.Keepalive); err != nil {
				m.logger.Warn("sync peers add peer failed", "peer_id", peer.ID, "error", err)
			}
		}
	}

	return nil
}

func (m *Manager) createInterface(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, m.ipBinary(), "link", "add", m.cfg.InterfaceName, "type", "wireguard")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// 接口已存在视为成功：崩溃重启后 wgmesh0 残留时 Start 可幂等恢复，
		// 后续 SetConfig 会重新下发私钥/端口/地址。
		if isAlreadyExists(out, err) {
			m.logger.Info("mesh interface already present, reusing", "iface", m.cfg.InterfaceName)
			return nil
		}
		return fmt.Errorf("ip link add: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *Manager) deleteInterface(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, m.ipBinary(), "link", "del", m.cfg.InterfaceName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip link del: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *Manager) interfaceUp(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, m.ipBinary(), "link", "set", m.cfg.InterfaceName, "up")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip link set up: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *Manager) assignIP(ctx context.Context, addr string) error {
	cmd := exec.CommandContext(ctx, m.ipBinary(), "addr", "add", addr, "dev", m.cfg.InterfaceName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// 地址已配置同样视为成功（与接口复用同场景的幂等恢复）。
		if isAlreadyExists(out, err) {
			m.logger.Info("mesh address already assigned, keeping", "iface", m.cfg.InterfaceName, "addr", addr)
			return nil
		}
		return fmt.Errorf("ip addr add: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ipBinary 返回 ip 工具路径（默认 "ip"，测试可注入 stub）。
func (m *Manager) ipBinary() string {
	if m.cfg.IPBinary != "" {
		return m.cfg.IPBinary
	}
	return "ip"
}

// validateWgBinary 前置校验 wg 可执行文件可用，避免测试/误配置场景下
// 以真实网络命令触碰系统（fail-fast + 可观测）。
func (m *Manager) validateWgBinary() error {
	if m.cfg.WgBinary == "" {
		return fmt.Errorf("wg binary not configured")
	}
	// 裸命令名（如 "wg"）必须走 $PATH 解析；os.Stat 只查相对/绝对路径，
	// 会让 PATH 下真实存在的 wg 被误判为缺失。
	if _, err := exec.LookPath(m.cfg.WgBinary); err != nil {
		return fmt.Errorf("wg binary unavailable at %s: %w", m.cfg.WgBinary, err)
	}
	return nil
}

// isAlreadyExists 判断 netlink 错误是否为“对象已存在”（幂等恢复判定）。
func isAlreadyExists(out []byte, err error) bool {
	return strings.Contains(string(out), "File exists") || strings.Contains(err.Error(), "File exists")
}

func (m *Manager) buildWGConfig(cfg LocalConfig) string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	if cfg.PrivateKey != "" {
		b.WriteString(fmt.Sprintf("PrivateKey = %s\n", cfg.PrivateKey))
	}
	if cfg.ListenPort > 0 {
		b.WriteString(fmt.Sprintf("ListenPort = %d\n", cfg.ListenPort))
	}
	return b.String()
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
