package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/andyrewlee/perch/data"
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

	// Selection state for lists
	sidebarIndex   int
	sidebarItems   []string // Items in sidebar list
	overviewIndex  int      // Selected rig in overview

	// Keymap for vim-style navigation
	keyMap KeyMap

	// Ready indicates the terminal size is known
	ready bool

	// Help overlay state
	showHelp bool
	firstRun bool

	// Data layer
	store  *data.Store
	loader *data.Loader

	// Actions
	actionRunner  *ActionRunner
	statusMessage *StatusMessage
	confirmDialog *ConfirmDialog

	// Selected items for actions
	selectedRig   string
	selectedAgent string
}

// DefaultTownRoot is the default Gas Town root directory.
const DefaultTownRoot = "/Users/andrewlee/gt"

// New creates a new Model
func New() Model {
	town := MockTown()
	km := DefaultKeyMap()
	return Model{
		focus:            PanelOverview,
		town:             town,
		overviewRenderer: NewOverviewRenderer(town),
		sidebarContent:   "Sidebar",
		detailsContent:   "Details",
		sidebarItems:     []string{"Convoys", "Merge Queue", "Agents"},
		keyMap:           km,
		actionRunner:     NewActionRunner(DefaultTownRoot),
		loader:           data.NewLoader(DefaultTownRoot),
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
	km := DefaultKeyMap()
	return Model{
		focus:            PanelOverview,
		town:             town,
		overviewRenderer: NewOverviewRenderer(town),
		sidebarContent:   "Sidebar",
		detailsContent:   "Details",
		sidebarItems:     []string{"Convoys", "Merge Queue", "Agents"},
		keyMap:           km,
		actionRunner:     NewActionRunner(DefaultTownRoot),
		loader:           data.NewLoader(DefaultTownRoot),
	}
}

// NewWithStore creates a new Model with a data store for live data.
func NewWithStore(store *data.Store, townRoot string) Model {
	town := MockTown() // Will be replaced by store data on first refresh
	km := DefaultKeyMap()
	return Model{
		focus:            PanelOverview,
		town:             town,
		overviewRenderer: NewOverviewRenderer(town),
		sidebarContent:   "Sidebar",
		detailsContent:   "Details",
		sidebarItems:     []string{"Convoys", "Merge Queue", "Agents"},
		keyMap:           km,
		store:            store,
		actionRunner:     NewActionRunner(townRoot),
		loader:           data.NewLoader(townRoot),
	}
}

// Message types for async operations
type refreshCompleteMsg struct {
	snapshot *data.Snapshot
	err      error
}

type actionCompleteMsg struct {
	action ActionType
	target string
	err    error
}

type statusExpiredMsg struct{}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	// Trigger initial data refresh
	return m.refreshCmd()
}

// refreshCmd creates a command that refreshes data from the town.
func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if m.loader == nil {
			return refreshCompleteMsg{err: nil}
		}

		snap := m.loader.LoadAll(ctx)
		return refreshCompleteMsg{snapshot: snap}
	}
}

// actionCmd creates a command that executes an action.
func (m Model) actionCmd(action ActionType, target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var err error
		switch action {
		case ActionBootRig:
			err = m.actionRunner.BootRig(ctx, target)
		case ActionShutdownRig:
			err = m.actionRunner.ShutdownRig(ctx, target)
		case ActionOpenLogs:
			err = m.actionRunner.OpenLogs(ctx, target)
		}

		return actionCompleteMsg{action: action, target: target, err: err}
	}
}

// statusExpireCmd creates a command that expires the status message after a delay.
func statusExpireCmd(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(time.Time) tea.Msg {
		return statusExpiredMsg{}
	})
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

		// Handle confirmation dialog first
		if m.confirmDialog != nil {
			return m.handleConfirmKey(msg)
		}

		// Handle keybindings (vim-style + action keys)
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.Help):
			m.showHelp = true

		case key.Matches(msg, m.keyMap.NextPanel):
			m.focus = (m.focus + 1) % 3

		case key.Matches(msg, m.keyMap.PrevPanel):
			m.focus = (m.focus + 2) % 3

		case key.Matches(msg, m.keyMap.Left):
			m.movePanelLeft()

		case key.Matches(msg, m.keyMap.Right):
			m.movePanelRight()

		case key.Matches(msg, m.keyMap.Up):
			m.moveUp()

		case key.Matches(msg, m.keyMap.Down):
			m.moveDown()

		case key.Matches(msg, m.keyMap.Select):
			m.handleSelect()

		case key.Matches(msg, m.keyMap.Refresh):
			m.setStatus("Refreshing data...", false)
			return m, m.refreshCmd()

		// Action keys (boot, shutdown, logs)
		case msg.String() == "b":
			// Boot selected rig
			if m.selectedRig == "" {
				m.setStatus("No rig selected. Use j/k to select a rig.", true)
				return m, statusExpireCmd(3 * time.Second)
			}
			m.setStatus("Booting rig "+m.selectedRig+"...", false)
			return m, m.actionCmd(ActionBootRig, m.selectedRig)

		case msg.String() == "s":
			// Shutdown selected rig (requires confirmation)
			if m.selectedRig == "" {
				m.setStatus("No rig selected. Use j/k to select a rig.", true)
				return m, statusExpireCmd(3 * time.Second)
			}
			m.confirmDialog = &ConfirmDialog{
				Title:   "Confirm Shutdown",
				Message: "Shutdown rig '" + m.selectedRig + "'? This will stop all agents. (y/n)",
				Action:  ActionShutdownRig,
				Target:  m.selectedRig,
			}
			return m, nil

		case msg.String() == "o":
			// Open logs for selected agent
			if m.selectedAgent == "" {
				m.setStatus("No agent selected. Use j/k to select an agent.", true)
				return m, statusExpireCmd(3 * time.Second)
			}
			m.setStatus("Opening logs for "+m.selectedAgent+"...", false)
			return m, m.actionCmd(ActionOpenLogs, m.selectedAgent)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case refreshCompleteMsg:
		if msg.err != nil {
			m.setStatus("Refresh failed: "+msg.err.Error(), true)
			return m, statusExpireCmd(5 * time.Second)
		}
		if msg.snapshot != nil {
			m.updateFromSnapshot(msg.snapshot)
			if msg.snapshot.HasErrors() {
				m.setStatus("Refreshed with some errors", true)
			} else {
				m.setStatus("Data refreshed", false)
			}
		}
		return m, statusExpireCmd(3 * time.Second)

	case actionCompleteMsg:
		return m.handleActionComplete(msg)

	case statusExpiredMsg:
		m.statusMessage = nil
	}

	return m, nil
}

// movePanelLeft moves focus to the left panel
func (m *Model) movePanelLeft() {
	switch m.focus {
	case PanelDetails:
		m.focus = PanelSidebar
	case PanelSidebar:
		m.focus = PanelOverview
	}
}

// movePanelRight moves focus to the right panel
func (m *Model) movePanelRight() {
	switch m.focus {
	case PanelOverview:
		m.focus = PanelSidebar
	case PanelSidebar:
		m.focus = PanelDetails
	}
}

// moveUp handles up navigation in the current panel
func (m *Model) moveUp() {
	switch m.focus {
	case PanelOverview:
		if m.overviewIndex > 0 {
			m.overviewIndex--
		}
		// Update selected rig
		if m.overviewIndex < len(m.town.Rigs) {
			m.selectedRig = m.town.Rigs[m.overviewIndex].Name
		}
	case PanelSidebar:
		if m.sidebarIndex > 0 {
			m.sidebarIndex--
		}
	}
}

// moveDown handles down navigation in the current panel
func (m *Model) moveDown() {
	switch m.focus {
	case PanelOverview:
		if m.overviewIndex < len(m.town.Rigs)-1 {
			m.overviewIndex++
		}
		// Update selected rig
		if m.overviewIndex < len(m.town.Rigs) {
			m.selectedRig = m.town.Rigs[m.overviewIndex].Name
		}
	case PanelSidebar:
		if m.sidebarIndex < len(m.sidebarItems)-1 {
			m.sidebarIndex++
		}
	}
}

// handleSelect handles the select action
func (m *Model) handleSelect() {
	switch m.focus {
	case PanelSidebar:
		// Update details based on selected sidebar item
		if m.sidebarIndex < len(m.sidebarItems) {
			m.detailsContent = "Selected: " + m.sidebarItems[m.sidebarIndex]
		}
	case PanelOverview:
		// Update details based on selected rig
		if m.overviewIndex < len(m.town.Rigs) {
			rig := m.town.Rigs[m.overviewIndex]
			m.detailsContent = "Rig: " + rig.Name + "\nAgents: " + string(rune('0'+len(rig.Agents)))
			m.selectedRig = rig.Name
		}
	}
}

// handleConfirmKey handles key presses when a confirmation dialog is shown.
func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		dialog := m.confirmDialog
		m.confirmDialog = nil
		m.setStatus("Executing "+actionName(dialog.Action)+" on "+dialog.Target+"...", false)
		return m, m.actionCmd(dialog.Action, dialog.Target)

	case "n", "N", "esc":
		m.confirmDialog = nil
		m.setStatus("Action cancelled", false)
		return m, statusExpireCmd(2 * time.Second)
	}
	return m, nil
}

// handleActionComplete processes the result of an action.
func (m Model) handleActionComplete(msg actionCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setStatus(actionName(msg.action)+" failed: "+msg.err.Error(), true)
		return m, statusExpireCmd(5 * time.Second)
	}

	m.setStatus(actionName(msg.action)+" completed for "+msg.target, false)

	// Auto-refresh after successful action
	cmds := []tea.Cmd{
		statusExpireCmd(3 * time.Second),
		m.refreshCmd(),
	}
	return m, tea.Batch(cmds...)
}

// setStatus sets the current status message.
func (m *Model) setStatus(text string, isError bool) {
	msg := NewStatusMessage(text, isError, 5*time.Second)
	m.statusMessage = &msg
}

// actionName returns a human-readable name for an action type.
func actionName(action ActionType) string {
	switch action {
	case ActionRefresh:
		return "Refresh"
	case ActionBootRig:
		return "Boot"
	case ActionShutdownRig:
		return "Shutdown"
	case ActionOpenLogs:
		return "Open logs"
	default:
		return "Action"
	}
}

// updateFromSnapshot updates the model's town data from a snapshot.
func (m *Model) updateFromSnapshot(snap *data.Snapshot) {
	if snap.Town == nil {
		return
	}

	// Convert data.Rig to tui.Rig
	var rigs []Rig
	for _, dr := range snap.Town.Rigs {
		rig := Rig{Name: dr.Name}

		// Add agents from the rig
		for _, da := range dr.Agents {
			agent := Agent{
				Name:   da.Name,
				Type:   agentTypeFromRole(da.Role),
				Status: agentStatusFromRunning(da.Running, da.HasWork),
			}
			rig.Agents = append(rig.Agents, agent)
		}

		rigs = append(rigs, rig)
	}

	m.town = Town{Rigs: rigs}
	m.overviewRenderer = NewOverviewRenderer(m.town)

	// Set default selection if none
	if m.selectedRig == "" && len(rigs) > 0 {
		m.selectedRig = rigs[0].Name
	}
}

// agentTypeFromRole converts a role string to AgentType.
func agentTypeFromRole(role string) AgentType {
	switch role {
	case "witness":
		return AgentWitness
	case "refinery":
		return AgentRefinery
	default:
		return AgentPolecat
	}
}

// agentStatusFromRunning determines agent status from running state.
func agentStatusFromRunning(running, hasWork bool) AgentStatus {
	if !running {
		return StatusIdle
	}
	if hasWork {
		return StatusActive
	}
	return StatusIdle
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

	// Render sidebar items with selection
	var lines []string
	for i, item := range m.sidebarItems {
		line := item
		if i == m.sidebarIndex && m.focus == PanelSidebar {
			// Highlighted selection
			line = selectedItemStyle.Render("> " + item)
		} else if i == m.sidebarIndex {
			// Selected but not focused
			line = dimSelectedStyle.Render("> " + item)
		} else {
			line = "  " + item
		}
		lines = append(lines, line)
	}

	// Pad remaining lines
	for len(lines) < innerHeight-1 {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight-1 {
		lines = lines[:innerHeight-1]
	}

	content := strings.Join(lines, "\n")
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

// renderFooter renders the footer with status message and help
func (m Model) renderFooter() string {
	// Status message takes priority
	if m.statusMessage != nil && !m.statusMessage.IsExpired() {
		style := statusStyle
		if m.statusMessage.IsError {
			style = statusErrorStyle
		}
		return style.Width(m.width).Render(m.statusMessage.Text)
	}

	// Show confirmation dialog prompt
	if m.confirmDialog != nil {
		return confirmStyle.Width(m.width).Render(m.confirmDialog.Message)
	}

	// Default help text with vim keys and action keys
	help := mutedStyle.Render("h/l: panels | j/k: navigate | r: refresh | b: boot | s: shutdown | o: logs | ?: help | q: quit")
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
		helpKeyStyle.Render("h/l") + "        Panel left/right",
		helpKeyStyle.Render("j/k") + "        Navigate up/down",
		helpKeyStyle.Render("tab") + "        Next panel",
		helpKeyStyle.Render("shift+tab") + "  Previous panel",
		helpKeyStyle.Render("enter") + "      Select item",
		helpKeyStyle.Render("r") + "          Refresh data",
		helpKeyStyle.Render("b") + "          Boot selected rig",
		helpKeyStyle.Render("s") + "          Shutdown selected rig",
		helpKeyStyle.Render("o") + "          Open logs for agent",
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
