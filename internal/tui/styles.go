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
