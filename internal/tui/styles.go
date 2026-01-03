package tui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	text      = lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#FAFAFA"}
	muted     = lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}
)

// Panel styles
var (
	borderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(subtle)

	overviewStyle = borderStyle.
			Padding(0, 1)

	sidebarStyle = borderStyle.
			Padding(0, 1)

	detailsStyle = borderStyle.
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(muted).
			Padding(0, 1)
)

// Text styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text)

	mutedStyle = lipgloss.NewStyle().
			Foreground(muted)
)

// List item styles
var (
	itemStyle = lipgloss.NewStyle().
			Foreground(text)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(highlight).
				Bold(true)

	dimSelectedStyle = lipgloss.NewStyle().
				Foreground(muted)
)

// Help overlay styles
var (
	helpOverlayStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.DoubleBorder()).
				BorderForeground(highlight).
				Padding(1, 2).
				Background(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1A1A1A"})

	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginBottom(1)

	helpHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Underline(true)

	helpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight)

	helpSectionStyle = lipgloss.NewStyle().
				Foreground(text).
				Italic(true)
)

// Status message styles
var (
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Padding(0, 1)

	statusErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6666")).
			Bold(true).
			Padding(0, 1)

	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFCC00")).
			Bold(true).
			Padding(0, 1)
)

// HUD indicator styles
var (
	hudConnectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00"))

	hudRefreshingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFF00"))

	hudErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6666"))

	hudDisconnectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#666666"))
)

// Merge queue status styles
var (
	conflictStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6666")).
			Bold(true)

	rebaseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFCC00"))
)

// Mail status styles
var (
	mailUnreadStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF")).
			Bold(true)

	mailReadStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))
)

// Lifecycle event styles
var (
	spawnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	wakeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#87CEEB"))

	nudgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFCC00"))

	handoffStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF"))

	doneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#32CD32"))

	crashStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6666")).
			Bold(true)

	killStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4500"))
)

// Operational state banner styles
var (
	degradedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Background(lipgloss.Color("#330000")).
			Bold(true).
			Padding(0, 1)

	mutedBannerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFCC00")).
				Bold(true).
				Padding(0, 1)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true)
)
