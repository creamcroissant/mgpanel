package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case hostsLoadedMsg:
		m.loading = false
		m.hosts = msg.hosts
		m.err = nil
		return m, nil

	case nodesLoadedMsg:
		m.loading = false
		m.nodes = msg.nodes
		m.err = nil

		// Update detail node if viewing
		if m.view == ViewNodeDetail && m.detailNode != nil {
			for i := range m.nodes {
				if m.nodes[i].Server.ID == m.detailNode.Server.ID {
					m.detailNode = &m.nodes[i]
					break
				}
			}
		}
		return m, nil

	case errorMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tickMsg:
		// Auto refresh based on current view
		switch m.view {
		case ViewHostList:
			return m, tea.Batch(m.loadHosts(), tickCmd())
		case ViewNodeList, ViewNodeDetail:
			if m.currentHost != nil {
				return m, tea.Batch(m.loadNodesForHost(m.currentHost.Host.ID), tickCmd())
			}
			return m, tickCmd()
		}
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Up):
		return m.handleUp()

	case key.Matches(msg, m.keys.Down):
		return m.handleDown()

	case key.Matches(msg, m.keys.Enter):
		return m.handleEnter()

	case key.Matches(msg, m.keys.Back):
		return m.handleBack()

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()
	}

	return m, nil
}

func (m Model) handleUp() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewHostList:
		if len(m.hosts) > 0 {
			m.selectedHost--
			if m.selectedHost < 0 {
				m.selectedHost = len(m.hosts) - 1
			}
		}
	case ViewNodeList:
		if len(m.nodes) > 0 {
			m.selectedNode--
			if m.selectedNode < 0 {
				m.selectedNode = len(m.nodes) - 1
			}
		}
	case ViewNodeDetail:
		// Scroll up in detail view
		if m.detailScrollOffset > 0 {
			m.detailScrollOffset--
		}
	}
	return m, nil
}

func (m Model) handleDown() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewHostList:
		if len(m.hosts) > 0 {
			m.selectedHost++
			if m.selectedHost >= len(m.hosts) {
				m.selectedHost = 0
			}
		}
	case ViewNodeList:
		if len(m.nodes) > 0 {
			m.selectedNode++
			if m.selectedNode >= len(m.nodes) {
				m.selectedNode = 0
			}
		}
	case ViewNodeDetail:
		// Scroll down in detail view
		// Calculate viewport and estimate max scroll
		viewportHeight := m.height - 4
		if viewportHeight < 5 {
			viewportHeight = 5
		}
		// Estimate content lines (will be clamped by render function if too high)
		// Use a generous upper bound to allow scrolling
		estimatedContentLines := 100 // Safe upper bound
		maxScroll := estimatedContentLines - viewportHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.detailScrollOffset < maxScroll {
			m.detailScrollOffset++
		}
	}
	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewHostList:
		if len(m.hosts) > 0 {
			m.currentHost = &m.hosts[m.selectedHost]
			m.view = ViewNodeList
			m.selectedNode = 0
			m.loading = true
			return m, m.loadNodesForHost(m.currentHost.Host.ID)
		}
	case ViewNodeList:
		if len(m.nodes) > 0 {
			m.detailNode = &m.nodes[m.selectedNode]
			m.view = ViewNodeDetail
			m.detailScrollOffset = 0 // Reset scroll position when entering detail view
		}
	}
	return m, nil
}

func (m Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewNodeDetail:
		m.view = ViewNodeList
		m.detailNode = nil
	case ViewNodeList:
		m.view = ViewHostList
		m.currentHost = nil
		m.nodes = nil
		m.selectedNode = 0
	}
	return m, nil
}

func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	m.loading = true
	switch m.view {
	case ViewHostList:
		return m, m.loadHosts()
	case ViewNodeList, ViewNodeDetail:
		if m.currentHost != nil {
			return m, m.loadNodesForHost(m.currentHost.Host.ID)
		}
		return m, m.loadHosts()
	}
	return m, nil
}
