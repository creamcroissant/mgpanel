package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

// View 实现 tea.Model
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var content string

	switch m.view {
	case ViewHostList:
		content = m.renderHostListView()
	case ViewNodeList:
		content = m.renderNodeListView()
	case ViewNodeDetail:
		content = m.renderNodeDetailView()
	}

	return content
}

func (m Model) renderHostListView() string {
	var b strings.Builder

	// 头部
	header := styleHeader.Width(m.width).Render("  MGPanel Server Monitor")
	b.WriteString(header)
	b.WriteString("\n\n")

	// 错误提示
	if m.err != nil {
		b.WriteString(styleOffline.Render(fmt.Sprintf("  Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// 加载提示
	if m.loading {
		b.WriteString(styleMuted().Render("  Loading..."))
		b.WriteString("\n\n")
	}

	// 表头
	tableHeader := fmt.Sprintf(
		"  %-4s │ %-16s │ %-18s │ %-8s │ %-8s │ %-8s │ %-12s │ %s",
		"ID", "Name", "Host", "CPU", "Memory", "Disk", "Traffic", "Nodes",
	)
	b.WriteString(styleTableHeader.Width(m.width).Render(tableHeader))
	b.WriteString("\n")

	// 分隔线
	b.WriteString(styleMuted().Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// 表格行
	if len(m.hosts) == 0 {
		b.WriteString(styleMuted().Render("  No servers found. Add agent hosts first."))
		b.WriteString("\n")
	} else {
		// 按终端高度计算可见行数
		visibleRows := m.height - 12
		if visibleRows < 5 {
			visibleRows = 5
		}

		startIdx := 0
		if m.selectedHost >= visibleRows {
			startIdx = m.selectedHost - visibleRows + 1
		}

		endIdx := startIdx + visibleRows
		if endIdx > len(m.hosts) {
			endIdx = len(m.hosts)
		}

		for i := startIdx; i < endIdx; i++ {
			host := m.hosts[i]
			row := m.renderHostTableRow(host, i == m.selectedHost)
			b.WriteString(row)
			b.WriteString("\n")
		}

		// 滚动提示
		if len(m.hosts) > visibleRows {
			scrollInfo := fmt.Sprintf("  Showing %d-%d of %d servers", startIdx+1, endIdx, len(m.hosts))
			b.WriteString(styleMuted().Render(scrollInfo))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// 状态汇总
	summary := m.renderHostStatusSummary()
	b.WriteString(summary)
	b.WriteString("\n\n")

	// 帮助提示
	help := styleHelp.Render("  [↑/↓] Navigate  [Enter] View Nodes  [r] Refresh  [q] Quit")
	b.WriteString(help)

	return b.String()
}

func (m Model) renderHostTableRow(host HostInfo, selected bool) string {
	status := HostStatusIcon(string(host.Status))

	name := host.Host.Name
	if len(name) > 16 {
		name = name[:13] + "..."
	}

	hostAddr := host.Host.Host
	if len(hostAddr) > 18 {
		hostAddr = hostAddr[:15] + "..."
	}

	// CPU 使用率格式化
	cpuStr := fmt.Sprintf("%.0f%%", host.Host.CPUUsed)

	// 内存使用格式化
	memStr := formatBytes(host.Host.MemUsed) + "/" + formatBytes(host.Host.MemTotal)
	if len(memStr) > 8 {
		memStr = fmt.Sprintf("%.0f%%", float64(host.Host.MemUsed)/float64(host.Host.MemTotal)*100)
	}

	// 磁盘使用格式化
	diskStr := formatBytes(host.Host.DiskUsed) + "/" + formatBytes(host.Host.DiskTotal)
	if len(diskStr) > 8 {
		diskStr = fmt.Sprintf("%.0f%%", float64(host.Host.DiskUsed)/float64(host.Host.DiskTotal)*100)
	}

	// 流量格式化
	traffic := formatBytes(host.Host.UploadTotal + host.Host.DownloadTotal)

	// 协议类型计数
	nodeTypes := countNodeTypes(host.Nodes)

	row := fmt.Sprintf(
		"  %-4d │ %s %-14s │ %-18s │ %-8s │ %-8s │ %-8s │ %-12s │ %s",
		host.Host.ID,
		status,
		name,
		hostAddr,
		cpuStr,
		memStr,
		diskStr,
		traffic,
		nodeTypes,
	)

	if selected {
		return styleTableRowSelected.Width(m.width).Render("▶" + row[1:])
	}
	return styleTableRow.Render(row)
}

func (m Model) renderHostStatusSummary() string {
	online, warning, offline := 0, 0, 0

	for _, h := range m.hosts {
		switch h.Status {
		case StatusOnline:
			online++
		case StatusWarning:
			warning++
		case StatusOffline:
			offline++
		}
	}

	return fmt.Sprintf(
		"  %s %d Online  %s %d Warning  %s %d Offline  │  Total: %d servers",
		styleOnline.Render("●"),
		online,
		styleWarning.Render("◐"),
		warning,
		styleOffline.Render("○"),
		offline,
		len(m.hosts),
	)
}

func (m Model) renderNodeListView() string {
	var b strings.Builder

	// 当前服务器头部
	hostName := "Unknown"
	if m.currentHost != nil {
		hostName = m.currentHost.Host.Name
	}
	header := styleHeader.Width(m.width).Render(fmt.Sprintf("  Nodes on: %s", hostName))
	b.WriteString(header)
	b.WriteString("\n\n")

	// 错误提示
	if m.err != nil {
		b.WriteString(styleOffline.Render(fmt.Sprintf("  Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// 加载提示
	if m.loading {
		b.WriteString(styleMuted().Render("  Loading..."))
		b.WriteString("\n\n")
	}

	// 表头
	tableHeader := fmt.Sprintf(
		"  %-4s │ %-20s │ %-12s │ %-12s │ %s",
		"ID", "Name", "Type", "Status", "Last Seen",
	)
	b.WriteString(styleTableHeader.Width(m.width).Render(tableHeader))
	b.WriteString("\n")

	// 分隔线
	b.WriteString(styleMuted().Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// 表格行
	if len(m.nodes) == 0 {
		b.WriteString(styleMuted().Render("  No nodes deployed on this server"))
		b.WriteString("\n")
	} else {
		// 按终端高度计算可见行数
		visibleRows := m.height - 10
		if visibleRows < 5 {
			visibleRows = 5
		}

		startIdx := 0
		if m.selectedNode >= visibleRows {
			startIdx = m.selectedNode - visibleRows + 1
		}

		endIdx := startIdx + visibleRows
		if endIdx > len(m.nodes) {
			endIdx = len(m.nodes)
		}

		for i := startIdx; i < endIdx; i++ {
			node := m.nodes[i]
			row := m.renderNodeTableRow(node, i == m.selectedNode)
			b.WriteString(row)
			b.WriteString("\n")
		}

		// 滚动提示
		if len(m.nodes) > visibleRows {
			scrollInfo := fmt.Sprintf("  Showing %d-%d of %d nodes", startIdx+1, endIdx, len(m.nodes))
			b.WriteString(styleMuted().Render(scrollInfo))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// 状态汇总
	summary := m.renderNodeStatusSummary()
	b.WriteString(summary)
	b.WriteString("\n\n")

	// 帮助提示
	help := styleHelp.Render("  [↑/↓] Navigate  [Enter] Details  [Esc] Back  [r] Refresh  [q] Quit")
	b.WriteString(help)

	return b.String()
}

func (m Model) renderNodeTableRow(node NodeInfo, selected bool) string {
	status := HostStatusIcon(string(node.Status))
	lastSeen := formatLastSeen(node.Server.LastHeartbeatAt)

	// 使用解析后的协议名，而非文件名
	name := getProtocolDisplayName(node.Server)
	if len(name) > 20 {
		name = name[:17] + "..."
	}

	// 使用解析后的协议类型
	nodeType := getProtocolType(node.Server)
	if len(nodeType) > 12 {
		nodeType = nodeType[:9] + "..."
	}

	row := fmt.Sprintf(
		"  %-4d │ %-20s │ %-12s │ %-12s │ %s",
		node.Server.ID,
		name,
		nodeType,
		status,
		lastSeen,
	)

	if selected {
		return styleTableRowSelected.Width(m.width).Render("▶" + row[1:])
	}
	return styleTableRow.Render(row)
}

func (m Model) renderNodeStatusSummary() string {
	online, warning, offline := 0, 0, 0

	for _, n := range m.nodes {
		switch n.Status {
		case StatusOnline:
			online++
		case StatusWarning:
			warning++
		case StatusOffline:
			offline++
		}
	}

	return fmt.Sprintf(
		"  %s %d  %s %d  %s %d  │  Total: %d nodes",
		styleOnline.Render("●"),
		online,
		styleWarning.Render("◐"),
		warning,
		styleOffline.Render("○"),
		offline,
		len(m.nodes),
	)
}

func (m Model) renderNodeDetailView() string {
	if m.detailNode == nil {
		return "No node selected"
	}

	node := m.detailNode
	srv := node.Server

	// 构建所有内容行
	var contentLines []string

	// 头部：使用协议展示名
	displayName := getProtocolDisplayName(srv)
	title := fmt.Sprintf("  Protocol: %s (#%d)", displayName, srv.ID)
	header := styleHeader.Width(m.width).Render(title)
	contentLines = append(contentLines, header)
	contentLines = append(contentLines, "")

	// 基础信息框
	basicInfo := m.renderBasicInfo(node)
	basicBox := styleDetailBox.Width(m.width - 4).Render(basicInfo)
	contentLines = append(contentLines, strings.Split(basicBox, "\n")...)
	contentLines = append(contentLines, "")

	// 连接信息
	connInfo := m.renderConnectionInfo(srv)
	connBox := styleBox.Width(m.width - 4).Render(connInfo)
	contentLines = append(contentLines, strings.Split(connBox, "\n")...)
	contentLines = append(contentLines, "")

	// 协议详情（从 Settings JSON 解析）
	protoInfo := m.renderProtocolDetails(srv)
	if protoInfo != "" {
		protoBox := styleBox.Width(m.width - 4).Render(protoInfo)
		contentLines = append(contentLines, strings.Split(protoBox, "\n")...)
		contentLines = append(contentLines, "")
	}

	totalLines := len(contentLines)

	// 计算可视区域
	viewportHeight := m.height - 4 // 为帮助提示与边框预留空间
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	// 计算最大滚动偏移
	maxScroll := totalLines - viewportHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	// 将滚动偏移限制在有效范围
	scrollOffset := m.detailScrollOffset
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// 应用滚动偏移
	startIdx := scrollOffset
	endIdx := startIdx + viewportHeight
	if endIdx > len(contentLines) {
		endIdx = len(contentLines)
	}

	// 构建可见内容
	var b strings.Builder
	for i := startIdx; i < endIdx; i++ {
		b.WriteString(contentLines[i])
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	// 可滚动时显示滚动提示
	if totalLines > viewportHeight {
		scrollPos := scrollOffset + 1
		scrollMax := maxScroll + 1
		if scrollMax < 1 {
			scrollMax = 1
		}
		scrollIndicator := styleMuted().Render(fmt.Sprintf(" [%d/%d]", scrollPos, scrollMax))
		b.WriteString("\n")
		b.WriteString(scrollIndicator)
	}

	b.WriteString("\n")

	// 帮助提示（始终在底部）
	help := styleHelp.Render("  [↑/↓] Scroll  [Esc] Back  [r] Refresh  [q] Quit")
	b.WriteString(help)

	return b.String()
}

func (m Model) renderBasicInfo(node *NodeInfo) string {
	srv := node.Server

	var lines []string

	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
		styleLabel.Render("Status:"),
		HostStatusIcon(string(node.Status)),
	))

	// 展示核心类型（sing-box/xray），而非通用类型
	coreType := srv.Type
	if srv.Settings != nil && len(srv.Settings) > 0 && string(srv.Settings) != "{}" {
		var details []ProtocolDetail
		if err := json.Unmarshal(srv.Settings, &details); err == nil && len(details) > 0 {
			if details[0].CoreType != "" {
				coreType = details[0].CoreType
			}
		}
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
		styleLabel.Render("Core:"),
		styleValue.Render(coreType),
	))

	// 展示源文件
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
		styleLabel.Render("Config File:"),
		styleValue.Render(srv.Name),
	))

	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
		styleLabel.Render("Last Heartbeat:"),
		styleValue.Render(formatLastSeen(srv.LastHeartbeatAt)),
	))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderConnectionInfo(srv *repository.Server) string {
	var lines []string

	// 连接信息
	lines = append(lines, styleTitle.Render("Connection Info"))
	lines = append(lines, "")

	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
		styleLabel.Render("Host:"),
		styleValue.Render(srv.Host),
	))

	// 优先使用解析后的协议端口
	port := srv.Port
	if srv.Settings != nil && len(srv.Settings) > 0 && string(srv.Settings) != "{}" {
		var details []ProtocolDetail
		if err := json.Unmarshal(srv.Settings, &details); err == nil && len(details) > 0 {
			if details[0].Port > 0 {
				port = details[0].Port
			}
		}
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
		styleLabel.Render("Port:"),
		styleValue.Render(fmt.Sprintf("%d", port)),
	))

	visible := "No"
	if srv.Show == 1 {
		visible = "Yes"
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
		styleLabel.Render("Visible:"),
		styleValue.Render(visible),
	))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func formatLastSeen(ts int64) string {
	if ts == 0 {
		return "Never"
	}

	t := time.Unix(ts, 0)
	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return t.Format("2006-01-02")
	}
}

func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func countNodeTypes(nodes []*repository.Server) string {
	if len(nodes) == 0 {
		return "No nodes"
	}

	types := make(map[string]int)
	for _, n := range nodes {
		types[n.Type]++
	}

	var parts []string
	for t, count := range types {
		parts = append(parts, fmt.Sprintf("%s×%d", t, count))
	}

	result := strings.Join(parts, ", ")
	if len(result) > 20 {
		return fmt.Sprintf("%d nodes", len(nodes))
	}
	return result
}

// getProtocolDisplayName extracts the protocol display name from Settings JSON
// Returns protocol name (e.g., "VLESS") if parsed successfully, or original name as fallback
func getProtocolDisplayName(srv *repository.Server) string {
	if srv.Settings == nil || len(srv.Settings) == 0 || string(srv.Settings) == "{}" {
		return srv.Name
	}

	var details []ProtocolDetail
	if err := json.Unmarshal(srv.Settings, &details); err != nil {
		return srv.Name
	}

	if len(details) == 0 {
		return srv.Name
	}

	// Build display name from protocol info
	d := details[0]
	if d.Protocol == "" {
		return srv.Name
	}

	// Format: PROTOCOL (tag) or PROTOCOL:port
	name := strings.ToUpper(d.Protocol)
	if d.Tag != "" {
		name = fmt.Sprintf("%s (%s)", name, d.Tag)
	} else if d.Port > 0 {
		name = fmt.Sprintf("%s:%d", name, d.Port)
	}

	return name
}

// getProtocolType returns the protocol type from Settings (e.g., "vless", "vmess")
func getProtocolType(srv *repository.Server) string {
	if srv.Settings == nil || len(srv.Settings) == 0 || string(srv.Settings) == "{}" {
		return srv.Type
	}

	var details []ProtocolDetail
	if err := json.Unmarshal(srv.Settings, &details); err != nil {
		return srv.Type
	}

	if len(details) == 0 || details[0].Protocol == "" {
		return srv.Type
	}

	return strings.ToUpper(details[0].Protocol)
}

// ProtocolDetail represents parsed protocol details from Settings JSON
type ProtocolDetail struct {
	Protocol  string          `json:"protocol"`
	Tag       string          `json:"tag"`
	Listen    string          `json:"listen"`
	Port      int             `json:"port"`
	Transport *TransportInfo  `json:"transport,omitempty"`
	TLS       *TLSInfo        `json:"tls,omitempty"`
	Multiplex *MultiplexInfo  `json:"multiplex,omitempty"`
	Users     []UserInfo      `json:"users,omitempty"`
	Options   map[string]any  `json:"options,omitempty"` // Protocol-specific options
	CoreType  string          `json:"core_type"`
}

type TransportInfo struct {
	Type        string `json:"type"`
	Path        string `json:"path,omitempty"`
	Host        string `json:"host,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

type TLSInfo struct {
	Enabled    bool        `json:"enabled"`
	ServerName string      `json:"server_name,omitempty"`
	ALPN       []string    `json:"alpn,omitempty"`
	Reality    *RealityInfo `json:"reality,omitempty"`
}

type RealityInfo struct {
	Enabled       bool     `json:"enabled"`
	ShortIDs      []string `json:"short_ids,omitempty"`
	ServerName    string   `json:"server_name,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	HandshakeAddr string   `json:"handshake_addr,omitempty"`
	HandshakePort int      `json:"handshake_port,omitempty"`
	PublicKey     string   `json:"public_key,omitempty"`
}

type MultiplexInfo struct {
	Enabled bool        `json:"enabled"`
	Padding bool        `json:"padding,omitempty"`
	Brutal  *BrutalInfo `json:"brutal,omitempty"`
}

type BrutalInfo struct {
	Enabled  bool `json:"enabled"`
	UpMbps   int  `json:"up_mbps,omitempty"`
	DownMbps int  `json:"down_mbps,omitempty"`
}

type UserInfo struct {
	UUID   string `json:"uuid,omitempty"`
	Flow   string `json:"flow,omitempty"`
	Email  string `json:"email,omitempty"`
	Method string `json:"method,omitempty"`
}

func (m Model) renderProtocolDetails(srv *repository.Server) string {
	if srv.Settings == nil || len(srv.Settings) == 0 || string(srv.Settings) == "{}" {
		return ""
	}

	// 将 Settings 解析为 ProtocolDetail 列表
	var details []ProtocolDetail
	if err := json.Unmarshal(srv.Settings, &details); err != nil {
		return ""
	}

	if len(details) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, styleTitle.Render("Protocol Configuration"))
	lines = append(lines, "")

	// 展示首条协议详情（多数配置每个文件只有一个 inbound）
	detail := details[0]

	// 协议类型
	if detail.Protocol != "" {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
			styleLabel.Render("Protocol:"),
			styleValue.Render(strings.ToUpper(detail.Protocol)),
		))
	}

	// 标签
	if detail.Tag != "" {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
			styleLabel.Render("Tag:"),
			styleValue.Render(detail.Tag),
		))
	}

	// 监听地址
	if detail.Listen != "" {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
			styleLabel.Render("Listen:"),
			styleValue.Render(detail.Listen),
		))
	}

	// 端口（以 detail.Port 为准，srv.Port 可能为 0）
	if detail.Port > 0 {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
			styleLabel.Render("Port:"),
			styleValue.Render(fmt.Sprintf("%d", detail.Port)),
		))
	}

	// 传输层
	if detail.Transport != nil && detail.Transport.Type != "" {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
			styleLabel.Render("Transport:"),
			styleValue.Render(strings.ToUpper(detail.Transport.Type)),
		))
		if detail.Transport.Path != "" {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("Path:"),
				styleValue.Render(detail.Transport.Path),
			))
		}
		if detail.Transport.ServiceName != "" {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("Service:"),
				styleValue.Render(detail.Transport.ServiceName),
			))
		}
	}

	// TLS 配置
	if detail.TLS != nil {
		lines = append(lines, "")
		lines = append(lines, styleTitle.Render("TLS Settings"))
		lines = append(lines, "")

		if detail.TLS.Enabled {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("TLS:"),
				styleOnline.Render("Enabled"),
			))
		} else {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("TLS:"),
				styleMuted().Render("Disabled"),
			))
		}

		// ALPN 列表
		if len(detail.TLS.ALPN) > 0 {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("ALPN:"),
				styleValue.Render(strings.Join(detail.TLS.ALPN, ", ")),
			))
		}

		if detail.TLS.Reality != nil && detail.TLS.Reality.Enabled {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("Reality:"),
				styleOnline.Render("Enabled"),
			))
			if detail.TLS.Reality.ServerName != "" {
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleLabel.Render("SNI:"),
					styleValue.Render(detail.TLS.Reality.ServerName),
				))
			}
			// 握手目标
			if detail.TLS.Reality.HandshakeAddr != "" {
				handshakeStr := detail.TLS.Reality.HandshakeAddr
				if detail.TLS.Reality.HandshakePort > 0 {
					handshakeStr = fmt.Sprintf("%s:%d", detail.TLS.Reality.HandshakeAddr, detail.TLS.Reality.HandshakePort)
				}
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleLabel.Render("Handshake:"),
					styleValue.Render(handshakeStr),
				))
			}
			// 指纹
			if detail.TLS.Reality.Fingerprint != "" {
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleLabel.Render("Fingerprint:"),
					styleValue.Render(detail.TLS.Reality.Fingerprint),
				))
			}
			// Short IDs
			if len(detail.TLS.Reality.ShortIDs) > 0 {
				shortIDsStr := formatShortIDs(detail.TLS.Reality.ShortIDs)
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleLabel.Render("Short IDs:"),
					styleValue.Render(shortIDsStr),
				))
			}
		} else if detail.TLS.ServerName != "" {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("SNI:"),
				styleValue.Render(detail.TLS.ServerName),
			))
		}
	}

	// 复用配置
	if detail.Multiplex != nil {
		lines = append(lines, "")
		lines = append(lines, styleTitle.Render("Multiplex Settings"))
		lines = append(lines, "")

		if detail.Multiplex.Enabled {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("Multiplex:"),
				styleOnline.Render("Enabled"),
			))
		} else {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("Multiplex:"),
				styleMuted().Render("Disabled"),
			))
		}

		if detail.Multiplex.Padding {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("Padding:"),
				styleOnline.Render("Enabled"),
			))
		} else {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render("Padding:"),
				styleMuted().Render("Disabled"),
			))
		}

		// Brutal (BBR) 配置
		if detail.Multiplex.Brutal != nil {
			if detail.Multiplex.Brutal.Enabled {
				brutalStr := fmt.Sprintf("Enabled (Up: %d Mbps, Down: %d Mbps)",
					detail.Multiplex.Brutal.UpMbps, detail.Multiplex.Brutal.DownMbps)
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleLabel.Render("Brutal:"),
					styleOnline.Render(brutalStr),
				))
			} else {
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleLabel.Render("Brutal:"),
					styleMuted().Render("Disabled"),
				))
			}
		}
	}

	// 用户列表
	if len(detail.Users) > 0 {
		lines = append(lines, "")
		lines = append(lines, styleTitle.Render(fmt.Sprintf("Users (%d total)", len(detail.Users))))
		lines = append(lines, "")

		// 逐个展示用户（UUID 已脱敏）
		maxUsersToShow := 5 // 限制展示数量，避免过长
		for i, user := range detail.Users {
			if i >= maxUsersToShow {
				lines = append(lines, styleMuted().Render(fmt.Sprintf("  ... and %d more users", len(detail.Users)-maxUsersToShow)))
				break
			}

			userNum := fmt.Sprintf("#%d", i+1)
			lines = append(lines, styleLabel.Render(userNum))

			// UUID（脱敏）
			if user.UUID != "" {
				maskedUUID := maskUUID(user.UUID)
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleMuted().Render("    UUID: "),
					styleValue.Render(maskedUUID),
				))
			}

			// Flow
			if user.Flow != "" {
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleMuted().Render("    Flow: "),
					styleValue.Render(user.Flow),
				))
			}

			// Email
			if user.Email != "" {
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleMuted().Render("    Email: "),
					styleValue.Render(user.Email),
				))
			}

			// Cipher method (for shadowsocks)
			if user.Method != "" {
				lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
					styleMuted().Render("    Cipher: "),
					styleValue.Render(user.Method),
				))
			}
		}
	}

	// Protocol-specific options section
	if len(detail.Options) > 0 {
		lines = append(lines, "")
		lines = append(lines, styleTitle.Render("Protocol Options"))
		lines = append(lines, "")

		for key, value := range detail.Options {
			valueStr := fmt.Sprintf("%v", value)
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
				styleLabel.Render(formatOptionKey(key)+":"),
				styleValue.Render(valueStr),
			))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// formatOptionKey 将 snake_case 转为标题显示格式
func formatOptionKey(key string) string {
	// 简单转换：下划线替换为空格并首字母大写
	words := strings.Split(key, "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// maskUUID 对 UUID 进行脱敏，仅显示前 8 位与后 4 位
// 例如: "d151646a-1234-5678-9abc-2e54d83c1b9c" -> "d151646a-...-1b9c"
func maskUUID(uuid string) string {
	if len(uuid) < 16 {
		return uuid
	}
	// 仅展示前 8 位与后 4 位
	return uuid[:8] + "-...-" + uuid[len(uuid)-4:]
}

// formatShortIDs 格式化 short ID 列表用于展示
func formatShortIDs(ids []string) string {
	if len(ids) == 0 {
		return "(none)"
	}

	var formatted []string
	for _, id := range ids {
		if id == "" {
			formatted = append(formatted, "(default)")
		} else {
			formatted = append(formatted, id)
		}
	}

	result := strings.Join(formatted, ", ")
	if len(result) > 40 {
		return fmt.Sprintf("%d short IDs", len(ids))
	}
	return result
}
