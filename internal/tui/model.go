package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Panel represents which panel is currently focused
type Panel int

const (
	PanelOverview Panel = iota
	PanelSidebar
	PanelDetails
)

// Model is the main TUI model
type Model struct {
	width  int
	height int

	// Panel focus
	focus Panel

	// Town data and renderer
	town             Town
	overviewRenderer *OverviewRenderer

	// Placeholder content for panels (will be replaced by data layer)
	sidebarContent string
	detailsContent string

	// Ready indicates the terminal size is known
	ready bool

	// Help overlay state
	showHelp bool
	firstRun bool
}

// New creates a new Model
func New() Model {
	town := MockTown()
	return Model{
		focus:            PanelOverview,
		town:             town,
		overviewRenderer: NewOverviewRenderer(town),
		sidebarContent:   "Sidebar",
		detailsContent:   "Details",
	}
}

// NewFirstRun creates a new Model with help overlay shown (for first-time users)
func NewFirstRun() Model {
	m := New()
	m.firstRun = true
	m.showHelp = true
	return m
}

// NewWithTown creates a new Model with the given town data
func NewWithTown(town Town) Model {
	return Model{
		focus:            PanelOverview,
		town:             town,
		overviewRenderer: NewOverviewRenderer(town),
		sidebarContent:   "Sidebar",
		detailsContent:   "Details",
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If help is showing, any key dismisses it
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = true
		case "tab":
			m.focus = (m.focus + 1) % 3
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	}

	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.showHelp {
		return m.renderHelpOverlay()
	}

	return m.renderLayout()
}

// renderLayout creates the full layout
func (m Model) renderLayout() string {
	// Reserve space for footer
	footerHeight := 1
	availableHeight := m.height - footerHeight

	// Calculate panel dimensions
	overviewHeight := availableHeight * 35 / 100 // 35% for overview
	bodyHeight := availableHeight - overviewHeight

	sidebarWidth := m.width * 25 / 100 // 25% for sidebar
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}
	if sidebarWidth > 40 {
		sidebarWidth = 40
	}
	detailsWidth := m.width - sidebarWidth

	// Render panels
	overview := m.renderOverview(m.width, overviewHeight)
	sidebar := m.renderSidebar(sidebarWidth, bodyHeight)
	details := m.renderDetails(detailsWidth, bodyHeight)
	footer := m.renderFooter()

	// Combine sidebar and details horizontally
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, details)

	// Stack vertically
	return lipgloss.JoinVertical(lipgloss.Left, overview, body, footer)
}

// renderOverview renders the overview panel
func (m Model) renderOverview(width, height int) string {
	// Account for border (2 chars each side)
	innerWidth := width - 4
	innerHeight := height - 2

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	title := titleStyle.Render("Town Overview")

	// Use the overview renderer to generate the town map
	var content string
	if m.overviewRenderer != nil {
		// Reserve space for title
		mapHeight := innerHeight - 2
		if mapHeight < 1 {
			mapHeight = 1
		}
		content = m.overviewRenderer.Render(innerWidth, mapHeight)
	} else {
		content = mutedStyle.Render("No data")
	}

	// Pad content to fill space
	lines := strings.Split(content, "\n")
	for len(lines) < innerHeight-1 {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight-1 {
		lines = lines[:innerHeight-1]
	}
	content = strings.Join(lines, "\n")

	inner := lipgloss.JoinVertical(lipgloss.Left, title, content)

	style := overviewStyle.
		Width(innerWidth).
		Height(innerHeight)

	if m.focus == PanelOverview {
		style = style.BorderForeground(highlight)
	}

	return style.Render(inner)
}

// renderSidebar renders the sidebar panel
func (m Model) renderSidebar(width, height int) string {
	innerWidth := width - 4
	innerHeight := height - 2

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	title := titleStyle.Render("Sidebar")
	content := m.sidebarContent

	lines := strings.Split(content, "\n")
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	content = strings.Join(lines[:innerHeight], "\n")

	inner := lipgloss.JoinVertical(lipgloss.Left, title, content)

	style := sidebarStyle.
		Width(innerWidth).
		Height(innerHeight)

	if m.focus == PanelSidebar {
		style = style.BorderForeground(highlight)
	}

	return style.Render(inner)
}

// renderDetails renders the details panel
func (m Model) renderDetails(width, height int) string {
	innerWidth := width - 4
	innerHeight := height - 2

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	title := titleStyle.Render("Details")
	content := m.detailsContent

	lines := strings.Split(content, "\n")
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	content = strings.Join(lines[:innerHeight], "\n")

	inner := lipgloss.JoinVertical(lipgloss.Left, title, content)

	style := detailsStyle.
		Width(innerWidth).
		Height(innerHeight)

	if m.focus == PanelDetails {
		style = style.BorderForeground(highlight)
	}

	return style.Render(inner)
}

// renderFooter renders the footer
func (m Model) renderFooter() string {
	help := mutedStyle.Render("tab: switch panel | ?: help | q: quit")
	return footerStyle.Width(m.width).Render(help)
}

// renderHelpOverlay renders the help/onboarding overlay
func (m Model) renderHelpOverlay() string {
	// Calculate overlay dimensions (80% of screen, centered)
	overlayWidth := m.width * 80 / 100
	overlayHeight := m.height * 80 / 100
	if overlayWidth < 60 {
		overlayWidth = min(60, m.width-4)
	}
	if overlayHeight < 20 {
		overlayHeight = min(20, m.height-4)
	}

	// Help content
	title := helpTitleStyle.Render("Gas Town Dashboard")

	welcomeMsg := ""
	if m.firstRun {
		welcomeMsg = helpSectionStyle.Render("Welcome! Here's how Gas Town works:\n")
	}

	concepts := []string{
		helpHeaderStyle.Render("Core Concepts"),
		"",
		helpKeyStyle.Render("Rigs") + "       Project containers with their own workers",
		"            Each rig has polecats, a witness, and a refinery",
		"",
		helpKeyStyle.Render("Polecats") + "   Worker agents that execute tasks",
		"            Each has its own git worktree for isolation",
		"",
		helpKeyStyle.Render("Witness") + "    Per-rig manager that monitors polecat health",
		"            Nudges stuck workers, handles cleanup",
		"",
		helpKeyStyle.Render("Refinery") + "   Merge queue processor for the rig",
		"            Processes completed work from polecats",
		"",
		helpKeyStyle.Render("Convoys") + "    Groups of related work items",
		"            Track progress across multiple beads",
		"",
		helpKeyStyle.Render("Hooks") + "      Work assignment mechanism",
		"            When work is hooked, the agent executes it",
		"",
		helpKeyStyle.Render("Beads") + "      Issue tracking system (like tickets)",
		"            Track tasks, bugs, and features",
	}

	keymap := []string{
		"",
		helpHeaderStyle.Render("Keymap"),
		"",
		helpKeyStyle.Render("tab") + "        Next panel",
		helpKeyStyle.Render("shift+tab") + "  Previous panel",
		helpKeyStyle.Render("?") + "          Show this help",
		helpKeyStyle.Render("q") + "          Quit",
	}

	dismissMsg := "\n" + mutedStyle.Render("Press any key to dismiss")

	// Combine content
	content := welcomeMsg +
		strings.Join(concepts, "\n") +
		strings.Join(keymap, "\n") +
		dismissMsg

	// Build the overlay box
	innerWidth := overlayWidth - 4
	innerHeight := overlayHeight - 2

	// Truncate content if needed
	lines := strings.Split(content, "\n")
	if len(lines) > innerHeight {
		lines = lines[:innerHeight-1]
		lines = append(lines, mutedStyle.Render("... (press any key)"))
	}
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	content = strings.Join(lines, "\n")

	inner := lipgloss.JoinVertical(lipgloss.Left, title, "", content)

	overlay := helpOverlayStyle.
		Width(innerWidth).
		Height(innerHeight).
		Render(inner)

	// Center the overlay
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

// min returns the smaller of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
