package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andyrewlee/perch/data"
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

	// Data store
	store    *data.Store
	snapshot *data.Snapshot
	townRoot string

	// Sidebar state
	sidebar *SidebarState

	// Ready indicates the terminal size is known
	ready bool

	// Help overlay state
	showHelp bool
	firstRun bool

	// Setup wizard (shown when no town exists)
	setupWizard *SetupWizard

	// Actions
	actionRunner  *ActionRunner
	statusMessage *StatusMessage
	confirmDialog *ConfirmDialog
	addRigForm    *AddRigForm

	// Attach town dialog
	attachDialog *AttachDialog

	// Selected items for actions
	selectedRig   string
	selectedAgent string

	// Tick loop state
	refreshInterval time.Duration
	lastRefresh     time.Time
	errorCount      int
	isRefreshing    bool

	// Queue health panel (shown when Merge Queue selected in sidebar)
	queueHealthPanel *QueueHealthPanel
	queueHealthData  map[string]QueueHealth // Per-rig queue health
}

// GetDefaultTownRoot returns the default Gas Town root directory.
// It checks GT_ROOT env var first, then falls back to $HOME/gt.
func GetDefaultTownRoot() string {
	if root := os.Getenv("GT_ROOT"); root != "" {
		return root
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to a reasonable default if we can't get home dir
		return "/tmp/gt"
	}
	return filepath.Join(home, "gt")
}

// DefaultRefreshInterval is how often to auto-refresh data.
const DefaultRefreshInterval = 10 * time.Second

// refreshMsg signals that data has been refreshed
type refreshMsg struct {
	snapshot *data.Snapshot
	err      error
}

// tickMsg triggers periodic refresh
type tickMsg time.Time

// New creates a new Model with the default town root.
func New() Model {
	return NewWithTownRoot(GetDefaultTownRoot())
}

// NewWithTownRoot creates a new Model with a custom town root.
func NewWithTownRoot(townRoot string) Model {
	// Check if town exists
	if !TownExists(townRoot) {
		// Show setup wizard for first-run
		return Model{
			townRoot:    townRoot,
			setupWizard: NewSetupWizard(),
		}
	}

	store := data.NewStore(townRoot)
	store.RefreshInterval = 5 * time.Second

	return Model{
		focus:           PanelSidebar,
		store:           store,
		townRoot:        townRoot,
		sidebar:         NewSidebarState(),
		actionRunner:    NewActionRunner(townRoot),
		refreshInterval: DefaultRefreshInterval,
		queueHealthData: make(map[string]QueueHealth),
	}
}

// NewFirstRun creates a new Model with help overlay shown (for first-time users).
func NewFirstRun() Model {
	m := New()
	m.firstRun = true
	m.showHelp = true
	return m
}

// NewWithStore creates a new Model with a provided data store.
func NewWithStore(store *data.Store, townRoot string) Model {
	return Model{
		focus:           PanelSidebar,
		store:           store,
		sidebar:         NewSidebarState(),
		actionRunner:    NewActionRunner(townRoot),
		refreshInterval: DefaultRefreshInterval,
		queueHealthData: make(map[string]QueueHealth),
	}
}

// Message types for async operations
type actionCompleteMsg struct {
	action ActionType
	target string
	err    error
}

type statusExpiredMsg struct{}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	// If setup wizard is active, initialize it instead
	if m.setupWizard != nil {
		return m.setupWizard.Init()
	}

	return tea.Batch(
		m.loadData,
		m.tickCmd(),
	)
}

// loadData loads data from the store
func (m Model) loadData() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	snap := m.store.Refresh(ctx)
	return refreshMsg{snapshot: snap, err: nil}
}

// tickCmd creates a tick command for periodic refresh
func (m Model) tickCmd() tea.Cmd {
	if m.refreshInterval <= 0 {
		return nil
	}
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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
		case ActionDeleteRig:
			err = m.actionRunner.DeleteRig(ctx, target)
		case ActionOpenLogs:
			err = m.actionRunner.OpenLogs(ctx, target)
		case ActionNudgeRefinery:
			err = m.actionRunner.NudgeRefinery(ctx, target)
		case ActionRestartRefinery:
			err = m.actionRunner.RestartRefinery(ctx, target)
		case ActionStopPolecat:
			err = m.actionRunner.StopPolecat(ctx, target)
		case ActionStopAllIdle:
			err = m.actionRunner.StopAllIdlePolecats(ctx, target)
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
	// Handle setup wizard if active
	if m.setupWizard != nil {
		return m.handleSetupWizard(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case refreshMsg:
		m.isRefreshing = false
		m.lastRefresh = time.Now()

		if msg.err != nil {
			m.errorCount++
			m.setStatus("Refresh failed: "+msg.err.Error(), true)
			return m, statusExpireCmd(5 * time.Second)
		}

		m.snapshot = msg.snapshot
		m.sidebar.UpdateFromSnapshot(msg.snapshot)
		m.updateQueueHealth(msg.snapshot)

		// Validate selected rig still exists, reset if not
		if m.selectedRig != "" && msg.snapshot != nil && msg.snapshot.Town != nil {
			found := false
			for _, rig := range msg.snapshot.Town.Rigs {
				if rig.Name == m.selectedRig {
					found = true
					break
				}
			}
			if !found {
				m.selectedRig = ""
			}
		}

		// Set default selection if none
		if m.selectedRig == "" && msg.snapshot != nil && msg.snapshot.Town != nil && len(msg.snapshot.Town.Rigs) > 0 {
			m.selectedRig = msg.snapshot.Town.Rigs[0].Name
		}

		if msg.snapshot != nil && msg.snapshot.HasErrors() {
			m.errorCount = len(msg.snapshot.Errors)
		} else {
			m.errorCount = 0
		}
		return m, nil

	case tickMsg:
		// Auto-refresh on tick, schedule next tick
		if !m.isRefreshing {
			m.isRefreshing = true
			return m, tea.Batch(m.tickCmd(), m.loadData)
		}
		// Already refreshing, just schedule next tick
		return m, m.tickCmd()

	case actionCompleteMsg:
		return m.handleActionComplete(msg)

	case statusExpiredMsg:
		m.statusMessage = nil
	}

	return m, nil
}

// handleSetupWizard delegates to the setup wizard and handles transitions.
func (m Model) handleSetupWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Forward window size to both wizard and main model
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wsm.Width
		m.height = wsm.Height
		m.ready = true
	}

	// Update the wizard
	wizardModel, cmd := m.setupWizard.Update(msg)
	wizard, ok := wizardModel.(*SetupWizard)
	if ok {
		m.setupWizard = wizard
	}

	// Check if setup is complete
	if m.setupWizard.IsComplete() {
		// Transition to main dashboard
		townRoot := m.setupWizard.TownRoot()
		m.setupWizard = nil
		m.townRoot = townRoot
		m.store = data.NewStore(townRoot)
		m.store.RefreshInterval = 5 * time.Second
		m.sidebar = NewSidebarState()
		m.actionRunner = NewActionRunner(townRoot)
		m.refreshInterval = DefaultRefreshInterval
		m.firstRun = true
		m.showHelp = true // Show help on first run

		// Start loading data
		return m, tea.Batch(m.loadData, m.tickCmd())
	}

	return m, cmd
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If help is showing, any key dismisses it
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	// Handle attach dialog first
	if m.attachDialog != nil {
		return m.handleAttachKey(msg)
	}

	// Handle confirmation dialog
	if m.confirmDialog != nil {
		return m.handleConfirmKey(msg)
	}

	// Handle add rig form
	if m.addRigForm != nil {
		return m.handleAddRigFormKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		m.showHelp = true
		return m, nil

	case "A":
		// Show attach town dialog (Shift+A to switch towns)
		m.attachDialog = NewAttachDialog()
		return m, nil

	case "tab":
		m.focus = (m.focus + 1) % 3
		return m, nil

	case "shift+tab":
		m.focus = (m.focus + 2) % 3
		return m, nil

	case "r":
		// Manual refresh
		m.isRefreshing = true
		m.setStatus("Refreshing data...", false)
		return m, m.loadData

	case "b":
		// Boot selected rig
		if m.selectedRig == "" {
			m.setStatus("No rig selected. Use j/k to select a rig.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.setStatus("Booting rig "+m.selectedRig+"...", false)
		return m, m.actionCmd(ActionBootRig, m.selectedRig)

	case "s":
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

	case "d":
		// Delete selected rig (requires confirmation)
		if m.selectedRig == "" {
			m.setStatus("No rig selected. Use j/k to select a rig.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.confirmDialog = &ConfirmDialog{
			Title:   "Confirm Delete",
			Message: "Delete rig '" + m.selectedRig + "'? This unregisters it from the town (files not deleted). (y/n)",
			Action:  ActionDeleteRig,
			Target:  m.selectedRig,
		}
		return m, nil

	case "o":
		// Open logs for selected agent
		if m.selectedAgent == "" {
			m.setStatus("No agent selected. Use j/k to select an agent.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.setStatus("Opening logs for "+m.selectedAgent+"...", false)
		return m, m.actionCmd(ActionOpenLogs, m.selectedAgent)

	case "n":
		// Nudge polecat to resolve merge issues
		if m.sidebar.Section != SectionMergeQueue {
			m.setStatus("Switch to Merge Queue section (press 2) to nudge", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		if m.sidebar.Selection < 0 || m.sidebar.Selection >= len(m.sidebar.MRs) {
			m.setStatus("No merge request selected", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		mr := m.sidebar.MRs[m.sidebar.Selection]
		if !mr.mr.HasConflicts && !mr.mr.NeedsRebase {
			m.setStatus("MR has no conflicts or rebase needed", false)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.setStatus("Nudging "+mr.mr.Worker+"...", false)
		return m, m.nudgeCmd(mr.rig, mr.mr.Worker, mr.mr.Branch, mr.mr.HasConflicts)

	case "a":
		// Open add rig form
		m.addRigForm = NewAddRigForm()
		return m, nil

	case "c":
		// Stop selected idle polecat (only when Agents section is active)
		if m.sidebar.Section != SectionAgents {
			m.setStatus("Switch to Agents section (press 4) to stop polecats", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		if m.sidebar.Selection < 0 || m.sidebar.Selection >= len(m.sidebar.Agents) {
			m.setStatus("No agent selected", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		agent := m.sidebar.Agents[m.sidebar.Selection].a
		// Safety check: only allow stopping polecats, not witness/refinery
		if agent.Role != "polecat" {
			m.setStatus("Can only stop polecats, not "+agent.Role, true)
			return m, statusExpireCmd(3 * time.Second)
		}
		// Safety check: don't stop polecats with active work
		if agent.HasWork {
			m.setStatus("Polecat has active work! Nudge it first or wait for completion.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		if !agent.Running {
			m.setStatus("Polecat is already stopped", false)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.confirmDialog = &ConfirmDialog{
			Title:   "Confirm Stop Polecat",
			Message: "Stop idle polecat '" + agent.Name + "'? (y/n)",
			Action:  ActionStopPolecat,
			Target:  agent.Address,
		}
		return m, nil

	case "C":
		// Stop all idle polecats in selected rig
		if m.selectedRig == "" {
			m.setStatus("No rig selected. Use j/k to select a rig.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.confirmDialog = &ConfirmDialog{
			Title:   "Confirm Stop All Idle",
			Message: "Stop all idle polecats in '" + m.selectedRig + "'? Only idle polecats will be stopped. (y/n)",
			Action:  ActionStopAllIdle,
			Target:  m.selectedRig,
		}
		return m, nil

	// Sidebar navigation (only when sidebar focused)
	case "j", "down":
		if m.focus == PanelSidebar {
			m.sidebar.SelectNext()
			m.syncSelectedRig()
		}
		return m, nil

	case "k", "up":
		if m.focus == PanelSidebar {
			m.sidebar.SelectPrev()
			m.syncSelectedRig()
		}
		return m, nil

	case "h", "left":
		if m.focus == PanelSidebar {
			m.sidebar.PrevSection()
			m.syncSelectedRig()
		}
		return m, nil

	case "l", "right":
		if m.focus == PanelSidebar {
			m.sidebar.NextSection()
			m.syncSelectedRig()
		}
		return m, nil

	case "1":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionRigs
			m.sidebar.Selection = 0
			m.syncSelectedRig()
		}
		return m, nil

	case "2":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionConvoys
			m.sidebar.Selection = 0
		}
		return m, nil

	case "3":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionMergeQueue
			m.sidebar.Selection = 0
		}
		return m, nil

	case "4":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionAgents
			m.sidebar.Selection = 0
		}
		return m, nil

	case "5":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionMail
			m.sidebar.Selection = 0
		}
		return m, nil

	case "6":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionLifecycle
			m.sidebar.Selection = 0
		}
		return m, nil

	case "e":
		// Cycle lifecycle type filter (only in Lifecycle section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionLifecycle {
			m.cycleLifecycleTypeFilter()
			m.sidebar.UpdateFromSnapshot(m.snapshot)
		}
		return m, nil

	case "g":
		// Set agent filter to current selection's agent (only in Lifecycle section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionLifecycle {
			m.setLifecycleAgentFilter()
			m.sidebar.UpdateFromSnapshot(m.snapshot)
		}
		return m, nil

	case "x":
		// Clear lifecycle filters (only in Lifecycle section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionLifecycle {
			m.sidebar.LifecycleFilter = ""
			m.sidebar.LifecycleAgentFilter = ""
			m.sidebar.UpdateFromSnapshot(m.snapshot)
		}
		return m, nil
	}

	return m, nil
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

// handleAddRigFormKey handles key presses when the add rig form is shown.
func (m Model) handleAddRigFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmd := m.addRigForm.Update(msg)

	if m.addRigForm.IsCancelled() {
		m.addRigForm = nil
		m.setStatus("Add rig cancelled", false)
		return m, statusExpireCmd(2 * time.Second)
	}

	if m.addRigForm.IsSubmitted() {
		name := m.addRigForm.Name()
		url := m.addRigForm.URL()
		prefix := m.addRigForm.Prefix()
		m.addRigForm = nil
		m.setStatus("Adding rig '"+name+"'...", false)
		return m, m.addRigCmd(name, url, prefix)
	}

	return m, cmd
}

// addRigCmd creates a command that executes the add rig action.
func (m Model) addRigCmd(name, url, prefix string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		err := m.actionRunner.AddRig(ctx, name, url, prefix)
		return actionCompleteMsg{action: ActionAddRig, target: name, err: err}
	}
}

// nudgeCmd creates a command that nudges a polecat to resolve merge issues.
func (m Model) nudgeCmd(rig, worker, branch string, hasConflicts bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := m.actionRunner.NudgePolecat(ctx, rig, worker, branch, hasConflicts)
		return actionCompleteMsg{action: ActionNudgePolecat, target: worker, err: err}
	}
}

// handleAttachKey handles key presses when the attach dialog is shown.
func (m Model) handleAttachKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.attachDialog.IsValid() {
			newPath := m.attachDialog.ExpandedPath()
			m.attachDialog = nil
			// Reinitialize with new town root
			m.townRoot = newPath
			m.store = data.NewStore(newPath)
			m.store.RefreshInterval = 5 * time.Second
			m.actionRunner = NewActionRunner(newPath)
			m.snapshot = nil
			m.sidebar = NewSidebarState()
			m.selectedRig = ""
			m.selectedAgent = ""
			m.setStatus("Attached to town: "+newPath, false)
			return m, tea.Batch(m.loadData, statusExpireCmd(3*time.Second))
		}
		// Invalid - just show error (already visible)
		return m, nil

	case "esc":
		m.attachDialog = nil
		m.setStatus("Attach cancelled", false)
		return m, statusExpireCmd(2 * time.Second)

	case "tab":
		// Autocomplete - use first suggestion if available
		if len(m.attachDialog.suggestions) > 0 {
			m.attachDialog.SetValue(m.attachDialog.suggestions[0])
		}
		return m, nil

	default:
		// Pass key to text input
		m.attachDialog.Update(msg)
		return m, nil
	}
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
		m.loadData,
	}
	return m, tea.Batch(cmds...)
}

// setStatus sets the current status message.
func (m *Model) setStatus(text string, isError bool) {
	msg := NewStatusMessage(text, isError, 5*time.Second)
	m.statusMessage = &msg
}

// syncSelectedRig updates selectedRig when navigating in the Rigs section.
func (m *Model) syncSelectedRig() {
	if m.sidebar.Section == SectionRigs && len(m.sidebar.Rigs) > 0 {
		if m.sidebar.Selection >= 0 && m.sidebar.Selection < len(m.sidebar.Rigs) {
			m.selectedRig = m.sidebar.Rigs[m.sidebar.Selection].r.Name
		}
	}
}

// cycleLifecycleTypeFilter cycles through lifecycle event type filters.
func (m *Model) cycleLifecycleTypeFilter() {
	types := []data.LifecycleEventType{
		"", // all
		data.EventSpawn,
		data.EventWake,
		data.EventNudge,
		data.EventHandoff,
		data.EventDone,
		data.EventCrash,
		data.EventKill,
	}

	current := m.sidebar.LifecycleFilter
	for i, t := range types {
		if t == current {
			m.sidebar.LifecycleFilter = types[(i+1)%len(types)]
			m.sidebar.Selection = 0
			return
		}
	}
	m.sidebar.LifecycleFilter = types[1] // Default to first filter
	m.sidebar.Selection = 0
}

// setLifecycleAgentFilter sets the agent filter to the currently selected event's agent.
func (m *Model) setLifecycleAgentFilter() {
	if m.sidebar.Selection >= 0 && m.sidebar.Selection < len(m.sidebar.LifecycleEvents) {
		agent := m.sidebar.LifecycleEvents[m.sidebar.Selection].e.Agent
		if m.sidebar.LifecycleAgentFilter == agent {
			// Toggle off if already set to this agent
			m.sidebar.LifecycleAgentFilter = ""
		} else {
			m.sidebar.LifecycleAgentFilter = agent
		}
		m.sidebar.Selection = 0
	}
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
	case ActionDeleteRig:
		return "Delete"
	case ActionOpenLogs:
		return "Open logs"
	case ActionAddRig:
		return "Add rig"
	case ActionNudgePolecat:
		return "Nudge"
	case ActionNudgeRefinery:
		return "Nudge refinery"
	case ActionRestartRefinery:
		return "Restart refinery"
	case ActionStopPolecat:
		return "Stop polecat"
	case ActionStopAllIdle:
		return "Stop all idle"
	default:
		return "Action"
	}
}

// updateQueueHealth populates queue health data from snapshot.
func (m *Model) updateQueueHealth(snap *data.Snapshot) {
	if snap == nil {
		return
	}
	if m.queueHealthData == nil {
		m.queueHealthData = make(map[string]QueueHealth)
	}

	for rigName, mrs := range snap.MergeQueues {
		health := QueueHealth{
			RigName: rigName,
			State:   RefineryIdle,
		}

		// Determine refinery state from agents
		if snap.Town != nil {
			for _, rig := range snap.Town.Rigs {
				if rig.Name == rigName {
					for _, agent := range rig.Agents {
						if agent.Role == "refinery" {
							health.RefineryAgent = agent.Address
							if agent.Running && agent.HasWork {
								health.State = RefineryProcessing
							} else if !agent.Running {
								health.State = RefineryStalled
							}
						}
					}
				}
			}
		}

		// Convert MergeRequests to QueueMRs
		for _, mr := range mrs {
			qmr := QueueMR{
				ID:     mr.ID,
				Title:  mr.Title,
				Worker: mr.Worker,
				Status: mr.Status,
			}
			// Age is calculated from current time if not set
			health.MRs = append(health.MRs, qmr)

			// Check if any MR is stale (indicates potential stall)
			if qmr.Age > 1*time.Hour && health.State == RefineryIdle {
				health.State = RefineryStalled
			}
		}

		m.queueHealthData[rigName] = health
	}
}

// View implements tea.Model
func (m Model) View() string {
	// Show setup wizard if active
	if m.setupWizard != nil {
		return m.setupWizard.View()
	}

	if !m.ready {
		return "Initializing..."
	}

	if m.showHelp {
		return m.renderHelpOverlay()
	}

	if m.addRigForm != nil {
		return m.addRigForm.View(m.width, m.height)
	}

	if m.attachDialog != nil {
		return m.attachDialog.Render(m.width, m.height)
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
	sidebar := RenderSidebar(m.sidebar, m.snapshot, sidebarWidth, bodyHeight, m.focus == PanelSidebar)
	details := RenderDetails(m.sidebar, m.snapshot, detailsWidth, bodyHeight, m.focus == PanelDetails)
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
	content := m.buildOverviewContent()

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

func (m Model) buildOverviewContent() string {
	if m.snapshot == nil {
		return mutedStyle.Render("Loading...")
	}
	if m.snapshot.Town == nil {
		// Town failed to load - surface the error
		if m.snapshot.HasErrors() {
			errMsg := "Failed to load town status"
			for _, err := range m.snapshot.Errors {
				if err != nil {
					errMsg = err.Error()
					break
				}
			}
			return statusErrorStyle.Render("Error: " + errMsg)
		}
		return mutedStyle.Render("Loading...")
	}

	town := m.snapshot.Town
	var lines []string

	// Header with last refresh time
	headerLine := headerStyle.Render(town.Name)
	if !m.lastRefresh.IsZero() {
		refreshStr := m.lastRefresh.Format("15:04:05")
		headerLine += "  " + mutedStyle.Render("updated "+refreshStr)
	}
	lines = append(lines, headerLine)

	// Operational state banner (only show if issues detected)
	if m.snapshot.OperationalState != nil && m.snapshot.OperationalState.HasIssues() {
		lines = append(lines, m.buildOperationalBanner())
	}
	lines = append(lines, "")

	// Compact rig clusters visualization
	rigLine := m.buildRigClusters()
	if rigLine != "" {
		lines = append(lines, rigLine)
		lines = append(lines, "")
	}

	// Summary stats in compact form
	s := town.Summary
	statsLine := fmt.Sprintf("%d rigs  %d polecats  %d crews  %d hooks active",
		s.RigCount, s.PolecatCount, s.CrewCount, s.ActiveHooks)
	lines = append(lines, mutedStyle.Render(statsLine))

	// Top alerts section
	alerts := m.buildAlerts()
	if len(alerts) > 0 {
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Alerts"))
		for _, alert := range alerts {
			lines = append(lines, alert)
		}
	}

	// Errors if any (less prominent now that we have alerts)
	if m.snapshot.HasErrors() && len(alerts) == 0 {
		lines = append(lines, "")
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("(%d load errors)", len(m.snapshot.Errors))))
	}

	return strings.Join(lines, "\n")
}

// buildOperationalBanner creates a status banner for operational issues
func (m Model) buildOperationalBanner() string {
	state := m.snapshot.OperationalState
	if state == nil {
		return ""
	}

	var parts []string

	// Degraded mode - most severe
	if state.DegradedMode {
		parts = append(parts, degradedStyle.Render("⚠ DEGRADED MODE"))
	}

	// Patrol muted
	if state.PatrolMuted {
		parts = append(parts, mutedBannerStyle.Render("⏸ PATROL MUTED"))
	}

	// Watchdog unhealthy
	if !state.WatchdogHealthy {
		parts = append(parts, warningStyle.Render("⚠ WATCHDOG DOWN"))
	}

	// Show individual issues if not covered above
	if len(state.Issues) > 0 && !state.DegradedMode && state.WatchdogHealthy {
		for _, issue := range state.Issues {
			parts = append(parts, warningStyle.Render("• "+issue))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "  ")
}

// buildRigClusters creates a compact visual representation of rig status
func (m Model) buildRigClusters() string {
	if m.snapshot == nil || m.snapshot.Town == nil {
		return ""
	}

	var clusters []string
	for _, rig := range m.snapshot.Town.Rigs {
		cluster := m.renderRigCluster(rig)
		clusters = append(clusters, cluster)
	}

	if len(clusters) == 0 {
		return mutedStyle.Render("No rigs")
	}

	return strings.Join(clusters, "  ")
}

// renderRigCluster renders a single rig as a compact status cluster
func (m Model) renderRigCluster(rig data.Rig) string {
	// Count agent states
	var working, idle, attention, stopped int
	for _, agent := range rig.Agents {
		if !agent.Running {
			stopped++
		} else if agent.UnreadMail > 0 {
			attention++
		} else if agent.HasWork {
			working++
		} else {
			idle++
		}
	}

	// Build status indicators
	var indicators []string
	if working > 0 {
		indicators = append(indicators, workingStyle.Render(fmt.Sprintf("%d●", working)))
	}
	if idle > 0 {
		indicators = append(indicators, idleStyle.Render(fmt.Sprintf("%d○", idle)))
	}
	if attention > 0 {
		indicators = append(indicators, attentionStyle.Render(fmt.Sprintf("%d!", attention)))
	}
	if stopped > 0 {
		indicators = append(indicators, stoppedStyle.Render(fmt.Sprintf("%d◌", stopped)))
	}

	// Check merge queue for conflicts
	if mrs, ok := m.snapshot.MergeQueues[rig.Name]; ok {
		conflicts := 0
		for _, mr := range mrs {
			if mr.HasConflicts || mr.NeedsRebase {
				conflicts++
			}
		}
		if conflicts > 0 {
			indicators = append(indicators, conflictStyle.Render(fmt.Sprintf("%d~", conflicts)))
		}
	}

	indicatorStr := ""
	if len(indicators) > 0 {
		indicatorStr = " " + strings.Join(indicators, " ")
	}

	return fmt.Sprintf("[%s%s]", rig.Name, indicatorStr)
}

// buildAlerts generates the top alerts for quick attention
func (m Model) buildAlerts() []string {
	var alerts []string
	maxAlerts := 3 // Limit to keep overview compact

	if m.snapshot == nil || m.snapshot.Town == nil {
		return alerts
	}

	// Check for agents needing attention (unread mail)
	for _, agent := range m.snapshot.Town.Agents {
		if agent.UnreadMail > 0 && len(alerts) < maxAlerts {
			alerts = append(alerts, attentionStyle.Render("!")+
				fmt.Sprintf(" %s has %d unread mail", agent.Name, agent.UnreadMail))
		}
	}

	// Check for merge queue issues
	for rigName, mrs := range m.snapshot.MergeQueues {
		for _, mr := range mrs {
			if len(alerts) >= maxAlerts {
				break
			}
			if mr.HasConflicts {
				alerts = append(alerts, conflictStyle.Render("~")+
					fmt.Sprintf(" [%s] %s has conflicts", rigName, truncate(mr.Title, 25)))
			} else if mr.NeedsRebase {
				alerts = append(alerts, rebaseStyle.Render("~")+
					fmt.Sprintf(" [%s] %s needs rebase", rigName, truncate(mr.Title, 25)))
			}
		}
	}

	// Check for stopped infrastructure (witness/refinery)
	for _, rig := range m.snapshot.Town.Rigs {
		if len(alerts) >= maxAlerts {
			break
		}
		for _, agent := range rig.Agents {
			if len(alerts) >= maxAlerts {
				break
			}
			if !agent.Running && (agent.Role == "witness" || agent.Role == "refinery") {
				alerts = append(alerts, stoppedStyle.Render("◌")+
					fmt.Sprintf(" [%s] %s is stopped", rig.Name, agent.Role))
			}
		}
	}

	// Check for load errors
	if m.snapshot.HasErrors() && len(alerts) < maxAlerts {
		alerts = append(alerts, statusErrorStyle.Render("●")+
			fmt.Sprintf(" %d data load error(s)", len(m.snapshot.Errors)))
	}

	return alerts
}

// renderFooter renders the footer with HUD indicators, status message, and help
func (m Model) renderFooter() string {
	// Build HUD section (left side)
	hud := m.renderHUD()

	// Status message takes priority over help text
	var rightSide string
	if m.statusMessage != nil && !m.statusMessage.IsExpired() {
		style := statusStyle
		if m.statusMessage.IsError {
			style = statusErrorStyle
		}
		rightSide = style.Render(m.statusMessage.Text)
	} else if m.confirmDialog != nil {
		rightSide = confirmStyle.Render(m.confirmDialog.Message)
	} else {
		// Context-aware help
		var helpItems []string
		switch m.focus {
		case PanelSidebar:
			helpItems = append(helpItems, "j/k: select", "h/l: section", "1-6: jump")
			if m.sidebar.Section == SectionMergeQueue {
				helpItems = append(helpItems, "n: nudge")
			}
			if m.sidebar.Section == SectionAgents {
				helpItems = append(helpItems, "c: stop idle", "C: stop all idle")
			}
			if m.sidebar.Section == SectionLifecycle {
				helpItems = append(helpItems, "e: type filter", "g: agent filter", "x: clear")
			}
		}
		helpItems = append(helpItems, "a: add rig", "A: attach", "r: refresh", "b: boot", "s: stop", "d: delete", "o: logs", "?: help", "q: quit")
		rightSide = mutedStyle.Render(strings.Join(helpItems, " | "))
	}

	// Calculate spacing between HUD and right side
	hudWidth := lipgloss.Width(hud)
	rightWidth := lipgloss.Width(rightSide)
	spacing := m.width - hudWidth - rightWidth - 2
	if spacing < 1 {
		spacing = 1
	}
	spacer := lipgloss.NewStyle().Width(spacing).Render("")

	return footerStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, hud, spacer, rightSide),
	)
}

// renderHUD renders the HUD indicators (connection, refresh time, errors)
func (m Model) renderHUD() string {
	var parts []string

	// Connection/refresh status indicator
	if m.isRefreshing {
		parts = append(parts, hudRefreshingStyle.Render("◐"))
	} else if m.errorCount > 0 {
		parts = append(parts, hudErrorStyle.Render("●"))
	} else if !m.lastRefresh.IsZero() {
		parts = append(parts, hudConnectedStyle.Render("●"))
	} else {
		parts = append(parts, hudDisconnectedStyle.Render("○"))
	}

	// Last refresh time
	if !m.lastRefresh.IsZero() {
		ago := time.Since(m.lastRefresh)
		var timeStr string
		switch {
		case ago < time.Minute:
			timeStr = fmt.Sprintf("%ds", int(ago.Seconds()))
		case ago < time.Hour:
			timeStr = fmt.Sprintf("%dm", int(ago.Minutes()))
		default:
			timeStr = m.lastRefresh.Format("15:04")
		}
		parts = append(parts, mutedStyle.Render(timeStr))
	}

	// Error count
	if m.errorCount > 0 {
		errStr := fmt.Sprintf("%d err", m.errorCount)
		if m.errorCount > 1 {
			errStr += "s"
		}
		parts = append(parts, hudErrorStyle.Render(errStr))
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, joinHUD(parts)...)
}

// joinHUD joins HUD parts with separators
func joinHUD(parts []string) []string {
	if len(parts) == 0 {
		return nil
	}
	result := make([]string, 0, len(parts)*2-1)
	for i, p := range parts {
		result = append(result, p)
		if i < len(parts)-1 {
			result = append(result, mutedStyle.Render(" "))
		}
	}
	return result
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
		helpHeaderStyle.Render("Rigs & Agents"),
		"",
		helpKeyStyle.Render("Rigs") + "       Project containers (own workers, MQ)",
		"            Each rig has polecats, a witness, and refinery",
		"",
		helpKeyStyle.Render("Polecats") + "   Witness-managed workers (auto-started)",
		"            Autonomous agents, auto-cleanup when idle",
		"",
		helpKeyStyle.Render("Crew") + "       Human-managed workers (you run sessions)",
		"            Same isolation, but you control lifecycle",
		"",
		helpKeyStyle.Render("Witness") + "    Polecat lifecycle manager",
		"            Starts/nudges/cleans up polecats",
		"",
		helpKeyStyle.Render("Refinery") + "   Merge queue processor",
		"            Rebases and merges completed work",
		"",
		helpHeaderStyle.Render("Work & Status"),
		"",
		helpKeyStyle.Render("Hooks") + "      Work assignment mechanism",
		"            Hooked work = agent executes immediately",
		"",
		helpKeyStyle.Render("●=working") + "  Agent running with hooked work",
		helpKeyStyle.Render("○=idle") + "     Agent running, waiting for work",
		helpKeyStyle.Render("!=attention") + " Has unread mail (may need help)",
		helpKeyStyle.Render("◌=stopped") + "  Agent session not running",
		"",
		helpKeyStyle.Render("Convoys") + "    Groups of related work items",
		helpKeyStyle.Render("Beads") + "      Issue tracking (tasks, bugs, features)",
		"",
		helpHeaderStyle.Render("Behind the Scenes"),
		"",
		helpKeyStyle.Render("Sessions") + "   Powered by tmux (internal, no setup needed)",
		"            Sessions persist even if Perch closes",
	}

	keymap := []string{
		"",
		helpHeaderStyle.Render("Keymap"),
		"",
		helpKeyStyle.Render("h/l") + "        Panel left/right, section switch",
		helpKeyStyle.Render("j/k") + "        Navigate up/down",
		helpKeyStyle.Render("tab") + "        Next panel",
		helpKeyStyle.Render("shift+tab") + "  Previous panel",
		helpKeyStyle.Render("1-6") + "        Jump to section (1=Rigs, 2=Convoys, 3=MQ, 4=Agents, 5=Mail, 6=Lifecycle)",
		helpKeyStyle.Render("a") + "          Add new rig",
		helpKeyStyle.Render("A") + "          Attach to a different town",
		helpKeyStyle.Render("n") + "          Nudge polecat (merge queue)",
		helpKeyStyle.Render("e") + "          Cycle type filter (lifecycle)",
		helpKeyStyle.Render("g") + "          Filter by agent (lifecycle)",
		helpKeyStyle.Render("x") + "          Clear filters (lifecycle)",
		helpKeyStyle.Render("c") + "          Stop idle polecat (agents)",
		helpKeyStyle.Render("C") + "          Stop all idle polecats in rig",
		helpKeyStyle.Render("r") + "          Refresh data",
		helpKeyStyle.Render("b") + "          Boot selected rig",
		helpKeyStyle.Render("s") + "          Shutdown selected rig",
		helpKeyStyle.Render("d") + "          Delete selected rig",
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

// agentStatusFromState determines agent status from running state and work.
func agentStatusFromState(running, hasWork bool, unreadMail int) AgentStatus {
	if !running {
		return StatusStopped
	}
	// Running: check for attention-needed conditions
	if unreadMail > 0 {
		return StatusAttention
	}
	if hasWork {
		return StatusWorking
	}
	return StatusIdle
}
