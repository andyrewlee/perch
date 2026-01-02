package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// HelpOverlay renders a help screen with all keybindings
type HelpOverlay struct {
	keyMap KeyMap
}

// NewHelpOverlay creates a new help overlay
func NewHelpOverlay(km KeyMap) *HelpOverlay {
	return &HelpOverlay{keyMap: km}
}

// Render generates the help overlay content
func (h *HelpOverlay) Render(width, height int) string {
	// Build help content
	content := h.buildContent()

	// Calculate box dimensions
	contentLines := strings.Split(content, "\n")
	boxWidth := 50
	if boxWidth > width-4 {
		boxWidth = width - 4
	}
	boxHeight := len(contentLines) + 4
	if boxHeight > height-4 {
		boxHeight = height - 4
	}

	// Style the box
	box := helpBoxStyle.
		Width(boxWidth).
		Render(content)

	// Center the box
	return centerOverlay(box, width, height)
}

// buildContent creates the formatted help text
func (h *HelpOverlay) buildContent() string {
	var sb strings.Builder

	// Title
	sb.WriteString(helpTitleStyle.Render("Keyboard Shortcuts"))
	sb.WriteString("\n\n")

	// Navigation section
	sb.WriteString(helpSectionStyle.Render("Navigation"))
	sb.WriteString("\n")
	sb.WriteString(h.formatBinding(h.keyMap.Up))
	sb.WriteString(h.formatBinding(h.keyMap.Down))
	sb.WriteString(h.formatBinding(h.keyMap.Left))
	sb.WriteString(h.formatBinding(h.keyMap.Right))
	sb.WriteString(h.formatBinding(h.keyMap.NextPanel))
	sb.WriteString(h.formatBinding(h.keyMap.PrevPanel))
	sb.WriteString("\n")

	// Actions section
	sb.WriteString(helpSectionStyle.Render("Actions"))
	sb.WriteString("\n")
	sb.WriteString(h.formatBinding(h.keyMap.Select))
	sb.WriteString(h.formatBinding(h.keyMap.Refresh))
	sb.WriteString("\n")

	// General section
	sb.WriteString(helpSectionStyle.Render("General"))
	sb.WriteString("\n")
	sb.WriteString(h.formatBinding(h.keyMap.Help))
	sb.WriteString(h.formatBinding(h.keyMap.Quit))

	// Footer
	sb.WriteString("\n\n")
	sb.WriteString(helpFooterStyle.Render("Press ? or Esc to close"))

	return sb.String()
}

// formatBinding formats a single keybinding line
func (h *HelpOverlay) formatBinding(b key.Binding) string {
	help := b.Help()
	keyStr := helpKeyStyle.Render(padRight(help.Key, 12))
	descStr := helpDescStyle.Render(help.Desc)
	return keyStr + descStr + "\n"
}

// centerOverlay centers content within the given dimensions
func centerOverlay(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	contentHeight := len(lines)
	contentWidth := maxLineWidth(lines)

	// Vertical padding
	topPad := (height - contentHeight) / 2
	if topPad < 0 {
		topPad = 0
	}

	// Horizontal padding
	leftPad := (width - contentWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	// Build centered content
	var sb strings.Builder
	for i := 0; i < topPad; i++ {
		sb.WriteString("\n")
	}
	for _, line := range lines {
		sb.WriteString(strings.Repeat(" ", leftPad))
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

// maxLineWidth returns the width of the longest line
func maxLineWidth(lines []string) int {
	max := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > max {
			max = w
		}
	}
	return max
}

// padRight pads a string to the specified width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// Help overlay styles (styles shared with styles.go are defined there)
var (
	helpBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(highlight).
			Padding(1, 2).
			Background(lipgloss.Color("#1a1a1a"))

	helpDescStyle = lipgloss.NewStyle().
			Foreground(muted)

	helpFooterStyle = lipgloss.NewStyle().
			Foreground(muted).
			Italic(true)
)
