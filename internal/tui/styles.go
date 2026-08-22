package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7C3AED")
	colorSecondary = lipgloss.Color("#A78BFA")
	colorSuccess   = lipgloss.Color("#22C55E")
	colorWarning   = lipgloss.Color("#F59E0B")
	colorDanger    = lipgloss.Color("#EF4444")
	colorMuted     = lipgloss.Color("#6B7280")
	colorBorder    = lipgloss.Color("#374151")

	// Base styles
	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 1)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	// Status indicators
	styleOnline = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	styleWarning = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	styleOffline = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)

	// Table styles
	styleTableHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorPrimary).
				Padding(0, 1)

	styleTableRow = lipgloss.NewStyle().
			Padding(0, 1)

	styleTableRowSelected = lipgloss.NewStyle().
				Background(lipgloss.Color("#1F2937")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1)

	// Box styles
	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	styleDetailBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)

	// Label styles
	styleLabel = lipgloss.NewStyle().
			Foreground(colorMuted).
			Width(16)

	styleValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	// Progress bar styles
	styleProgressFilled = lipgloss.NewStyle().
				Foreground(colorSuccess)

	styleProgressEmpty = lipgloss.NewStyle().
				Foreground(colorMuted)
)

// StatusIcon returns a colored status indicator
func StatusIcon(status string) string {
	switch status {
	case "online":
		return styleOnline.Render("● Online")
	case "warning":
		return styleWarning.Render("◐ Warning")
	case "offline":
		return styleOffline.Render("○ Offline")
	default:
		return styleMuted().Render("? Unknown")
	}
}

// HostStatusIcon returns a colored status indicator for hosts
func HostStatusIcon(status string) string {
	switch status {
	case "online":
		return styleOnline.Render("●")
	case "warning":
		return styleWarning.Render("◐")
	case "offline":
		return styleOffline.Render("○")
	default:
		return styleMuted().Render("?")
	}
}

func styleMuted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorMuted)
}

// ProgressBar renders a simple progress bar
func ProgressBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int(float64(width) * percent / 100)
	empty := width - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += styleProgressFilled.Render("█")
	}
	for i := 0; i < empty; i++ {
		bar += styleProgressEmpty.Render("░")
	}

	return bar
}
