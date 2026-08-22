package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/repository/sqlite"
)

// ViewType 表示当前视图
type ViewType int

const (
	ViewHostList   ViewType = iota // 服务器列表
	ViewNodeList                   // 服务器下的节点列表
	ViewNodeDetail                 // 节点详情
)

// HostStatus 表示服务器在线状态
type HostStatus string

const (
	StatusOnline  HostStatus = "online"
	StatusWarning HostStatus = "warning"
	StatusOffline HostStatus = "offline"
)

// HostInfo 封装服务器与计算后的状态
type HostInfo struct {
	Host   *repository.AgentHost
	Status HostStatus
	Nodes  []*repository.Server // 该服务器下的节点
}

// NodeInfo 封装节点与计算后的状态
type NodeInfo struct {
	Server *repository.Server
	Status HostStatus
}

// Model 是主 TUI 模型
type Model struct {
	// 数据
	hosts        []HostInfo
	selectedHost int

	// 当前服务器的节点列表（ViewNodeList 时使用）
	nodes        []NodeInfo
	selectedNode int

	// 视图状态
	view         ViewType
	detailNode   *NodeInfo
	currentHost  *HostInfo

	// 存储引用
	store *sqlite.Store

	// 终端尺寸
	width  int
	height int

	// 详情视图的滚动状态
	detailScrollOffset int
	detailContentLines int // 详情内容总行数，用于滚动

	// 状态
	loading bool
	err     error

	// 按键绑定
	keys keyMap
}

// keyMap 定义全部按键绑定
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Quit    key.Binding
	Refresh key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// NewModel 创建新的 TUI 模型
func NewModel(store *sqlite.Store) Model {
	return Model{
		store:        store,
		view:         ViewHostList,
		selectedHost: 0,
		selectedNode: 0,
		keys:         defaultKeyMap(),
		loading:      true,
	}
}

// Init 实现 tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadHosts(),
		tickCmd(),
	)
}

// 消息类型

type hostsLoadedMsg struct {
	hosts []HostInfo
}

type nodesLoadedMsg struct {
	nodes []NodeInfo
}

type errorMsg struct {
	err error
}

type tickMsg time.Time

// 命令

func (m Model) loadHosts() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		agentHosts, err := m.store.AgentHosts().ListAll(ctx)
		if err != nil {
			return errorMsg{err: err}
		}

		hosts := make([]HostInfo, len(agentHosts))
		now := time.Now().Unix()

		for i, host := range agentHosts {
			// 获取该服务器下的所有节点
			nodes, _ := m.store.Servers().FindByAgentHostID(ctx, host.ID)

			hosts[i] = HostInfo{
				Host:   host,
				Status: calcHostStatus(host.LastHeartbeatAt, now),
				Nodes:  nodes,
			}
		}

		return hostsLoadedMsg{hosts: hosts}
	}
}

func (m Model) loadNodesForHost(hostID int64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		servers, err := m.store.Servers().FindByAgentHostID(ctx, hostID)
		if err != nil {
			return errorMsg{err: err}
		}

		nodes := make([]NodeInfo, len(servers))
		now := time.Now().Unix()

		for i, srv := range servers {
			nodes[i] = NodeInfo{
				Server: srv,
				Status: calcNodeStatus(srv.LastHeartbeatAt, now),
			}
		}

		return nodesLoadedMsg{nodes: nodes}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// 辅助函数

func calcHostStatus(lastHeartbeat, now int64) HostStatus {
	if lastHeartbeat == 0 {
		return StatusOffline
	}

	diff := now - lastHeartbeat
	switch {
	case diff <= 120: // 2 分钟
		return StatusOnline
	case diff <= 300: // 5 分钟
		return StatusWarning
	default:
		return StatusOffline
	}
}

func calcNodeStatus(lastHeartbeat, now int64) HostStatus {
	if lastHeartbeat == 0 {
		return StatusOffline
	}

	diff := now - lastHeartbeat
	switch {
	case diff <= 120: // 2 分钟
		return StatusOnline
	case diff <= 300: // 5 分钟
		return StatusWarning
	default:
		return StatusOffline
	}
}
