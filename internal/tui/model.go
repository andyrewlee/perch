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

	// Health check report
	doctorReport *data.DoctorReport

	// Ready indicates the terminal size is known
	ready bool

	// Help overlay state
	showHelp bool
	firstRun bool

	// Setup wizard (shown when no town exists)
	setupWizard *SetupWizard

	// Actions
	actionRunner   *ActionRunner
	statusMessage  *StatusMessage
	confirmDialog   *ConfirmDialog
	addRigForm      *AddRigForm
	createWorkForm  *CreateWorkForm
	beadsForm       *BeadsForm
	inputDialog     *InputDialog
	presetNudgeMenu *PresetNudgeMenu

	// Attach town dialog
	attachDialog *AttachDialog

	// Rig settings form
	rigSettingsForm *RigSettingsForm

	// Selected items for actions
	selectedRig    string
	selectedAgent  string
	selectedConvoy string // Currently selected convoy ID
	selectedPlugin string
	selectedBeadID string // Currently selected bead ID

	// Bead dependencies cache
	beadDependencies    *data.IssueDependencies
	beadDepsLoading     bool
	beadDepsLoadError   error

	// Bead comments cache
	beadComments    *data.IssueComments
	beadCommentsLoading bool

	// Audit timeline for selected agent
	auditTimeline       []data.AuditEntry
	auditTimelineActor  string
	auditTimelineLoading bool

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

// auditTimelineMsg signals that audit timeline has been loaded
type auditTimelineMsg struct {
	actor   string
	entries []data.AuditEntry
	err     error
}

type rigSettingsLoadedMsg struct {
	rigName  string
	settings *data.RigSettings
	err      error
}

type rigSettingsSavedMsg struct {
	rigName string
	err     error
}

type beadDependenciesLoadedMsg struct {
	issueID      string
	dependencies *data.IssueDependencies
	err          error
}

type beadCommentsLoadedMsg struct {
	issueID  string
	comments *data.IssueComments
	err      error
}

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

// loadAuditTimelineCmd loads audit timeline for an actor
func (m Model) loadAuditTimelineCmd(actor string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entries, err := m.store.Loader().LoadAuditTimeline(ctx, actor, 20)
		return auditTimelineMsg{actor: actor, entries: entries, err: err}
	}
}

// loadBeadDependenciesCmd creates a command that loads dependencies for a bead.
func (m Model) loadBeadDependenciesCmd(issueID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		deps, err := m.store.Loader().LoadIssueDependencies(ctx, issueID)
		return beadDependenciesLoadedMsg{issueID: issueID, dependencies: deps, err: err}
	}
}

// loadBeadCommentsCmd creates a command that loads comments for a bead.
func (m Model) loadBeadCommentsCmd(issueID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		comments, err := m.store.Loader().LoadIssueComments(ctx, issueID)
		return beadCommentsLoadedMsg{issueID: issueID, comments: comments, err: err}
	}
}

// actionCmd creates a command that executes an action.
func (m Model) actionCmd(action ActionType, target string) tea.Cmd {
	return m.actionCmdWithInput(action, target, "", "")
}

// actionCmdWithInput creates a command that executes an action with additional input.
func (m Model) actionCmdWithInput(action ActionType, target, input, extraInput string) tea.Cmd {
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
		case ActionRestartRefinery, ActionRestartRefineryAlt:
			err = m.actionRunner.RestartRefinery(ctx, target)
		case ActionStopPolecat:
			err = m.actionRunner.StopPolecat(ctx, target)
		case ActionStopAllIdle:
			err = m.actionRunner.StopAllIdlePolecats(ctx, target)
		case ActionRemoveWorktree:
			err = m.actionRunner.RemoveWorktree(ctx, target)
		case ActionSlingWork:
			err = m.actionRunner.SlingWork(ctx, input, target)
		case ActionHandoff:
			err = m.actionRunner.Handoff(ctx, target)
		case ActionStopAgent:
			err = m.actionRunner.StopAgent(ctx, target)
		case ActionNudgeAgent:
			err = m.actionRunner.NudgeAgent(ctx, target, input)
		case ActionMailAgent:
			err = m.actionRunner.MailAgent(ctx, target, input, extraInput)
		case ActionTogglePlugin:
			err = m.actionRunner.TogglePlugin(ctx, target)
		case ActionOpenSession:
			err = m.actionRunner.OpenSession(ctx, target)
		case ActionRestartSession:
			err = m.actionRunner.RestartSession(ctx, target)
		case ActionPresetNudge:
			err = m.actionRunner.NudgeAgent(ctx, target, input)
		// Infrastructure agent controls
		case ActionStartDeacon:
			err = m.actionRunner.StartDeacon(ctx)
		case ActionStopDeacon:
			err = m.actionRunner.StopDeacon(ctx)
		case ActionRestartDeacon:
			err = m.actionRunner.RestartDeacon(ctx)
		case ActionStartWitness:
			err = m.actionRunner.StartWitness(ctx, target)
		case ActionStopWitness:
			err = m.actionRunner.StopWitness(ctx, target)
		case ActionRestartWitness:
			err = m.actionRunner.RestartWitness(ctx, target)
		case ActionStartRefinery:
			err = m.actionRunner.StartRefinery(ctx, target)
		case ActionStopRefinery:
			err = m.actionRunner.StopRefinery(ctx, target)
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

		// Update selected agent from sidebar
		m.updateSelectedFromSidebar()

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

	case auditTimelineMsg:
		m.auditTimelineLoading = false
		if msg.err != nil {
			// Silent failure - just show empty timeline
			m.auditTimeline = nil
		} else if msg.actor == m.auditTimelineActor {
			// Only update if still looking at same actor
			m.auditTimeline = msg.entries
		}
		return m, nil

	case rigSettingsLoadedMsg:
		if msg.err != nil {
			m.setStatus("Failed to load settings: "+msg.err.Error(), true)
			return m, statusExpireCmd(5 * time.Second)
		}
		m.rigSettingsForm = NewRigSettingsForm(msg.rigName, msg.settings)
		return m, nil

	case rigSettingsSavedMsg:
		if msg.err != nil {
			m.setStatus("Failed to save settings: "+msg.err.Error(), true)
			return m, statusExpireCmd(5 * time.Second)
		}
		m.setStatus("Settings saved for "+msg.rigName, false)
		return m, tea.Batch(statusExpireCmd(3*time.Second), m.loadData)

	case beadDependenciesLoadedMsg:
		m.beadDepsLoading = false
		if msg.err != nil {
			m.beadDepsLoadError = msg.err
			m.setStatus("Failed to load dependencies: "+msg.err.Error(), true)
			return m, statusExpireCmd(3 * time.Second)
		}
		if msg.dependencies != nil {
			m.beadDependencies = msg.dependencies
		}
		return m, nil

	case beadCommentsLoadedMsg:
		m.beadCommentsLoading = false
		if msg.err != nil {
			m.setStatus("Failed to load comments: "+msg.err.Error(), true)
			return m, statusExpireCmd(3 * time.Second)
		}
		if msg.comments != nil {
			m.beadComments = msg.comments
		}
		return m, nil
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

	// Handle input dialog first
	if m.inputDialog != nil {
		return m.handleInputKey(msg)
	}

	// Handle preset nudge menu
	if m.presetNudgeMenu != nil {
		return m.handlePresetNudgeMenuKey(msg)
	}

	// Handle attach dialog
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

	// Handle create work form
	if m.createWorkForm != nil {
		return m.handleCreateWorkFormKey(msg)
	}

	// Handle beads form
	if m.beadsForm != nil {
		return m.handleBeadsFormKey(msg)
	}

	// Handle rig settings form
	if m.rigSettingsForm != nil {
		return m.handleRigSettingsFormKey(msg)
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
		// Context-dependent: Manual refresh OR restart subsystem (Operator section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionOperator {
			// Restart selected infrastructure subsystem
			return m.handleInfrastructureRestart()
		}
		// Manual refresh
		m.isRefreshing = true
		m.setStatus("Refreshing data...", false)
		return m, m.loadData

	case "b":
		// Context-dependent: Beads form (Beads section), start infrastructure (Operator section), or boot rig (Rigs section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionBeads {
			// Open beads form for create or edit
			var selectedBead *data.Issue
			if m.sidebar.Selection >= 0 && m.sidebar.Selection < len(m.sidebar.Beads) {
				selectedBead = &m.sidebar.Beads[m.sidebar.Selection].issue
			}

			if selectedBead != nil {
				// Edit mode
				m.beadsForm = NewBeadsFormEdit(
					selectedBead.ID,
					selectedBead.Title,
					selectedBead.Description,
					selectedBead.IssueType,
					selectedBead.Priority,
				)
			} else {
				// Create mode
				m.beadsForm = NewBeadsFormCreate()
			}
			return m, nil
		}
		if m.focus == PanelSidebar && m.sidebar.Section == SectionOperator {
			// Start selected infrastructure subsystem
			return m.handleInfrastructureStart()
		}
		// Boot selected rig
		if m.selectedRig == "" {
			m.setStatus("No rig selected. Use j/k to select a rig.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.setStatus("Booting rig "+m.selectedRig+"...", false)
		return m, m.actionCmd(ActionBootRig, m.selectedRig)

	case "s":
		// Context-dependent: Stop infrastructure (Operator section), toggle beads scope (Beads section), or shutdown rig (Rigs section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionOperator {
			// Stop selected infrastructure subsystem
			return m.handleInfrastructureStop()
		}
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

	case "a":
		// Open add rig form
		m.addRigForm = NewAddRigForm()
		return m, nil

	case "w":
		// Open create work form
		var rigs []string
		if m.snapshot != nil && m.snapshot.Town != nil {
			for _, rig := range m.snapshot.Town.Rigs {
				rigs = append(rigs, rig.Name)
			}
		}
		m.createWorkForm = NewCreateWorkForm(rigs)
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

	case "n":
		// Context-sensitive nudge: merge queue or agents section
		if m.sidebar.Section == SectionMergeQueue {
			// Nudge polecat to resolve merge issues
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
		} else if m.sidebar.Section == SectionAgents {
			// Nudge selected agent (opens preset nudge menu)
			if m.selectedAgent == "" {
				m.setStatus("No agent selected. Use j/k to select an agent.", true)
				return m, statusExpireCmd(3 * time.Second)
			}
			m.presetNudgeMenu = &PresetNudgeMenu{
				Target:    m.selectedAgent,
				Selection: 0,
			}
			return m, nil
		}
		m.setStatus("Switch to Merge Queue or Agents section to nudge", true)
		return m, statusExpireCmd(3 * time.Second)

	case "S":
		// Sling work to selected agent (opens input dialog)
		if m.selectedAgent == "" {
			m.setStatus("No agent selected. Use j/k to select an agent.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.inputDialog = &InputDialog{
			Title:  "Sling Work",
			Prompt: "Bead ID to sling to " + m.selectedAgent + ": ",
			Action: ActionSlingWork,
			Target: m.selectedAgent,
		}
		return m, nil

	case "H":
		// Context-dependent: Handoff (Agents section) or toggle convoy history (Convoys section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionConvoys {
			m.sidebar.ToggleConvoyHistory()
			viewName := "active"
			if m.sidebar.ShowConvoyHistory {
				viewName = "history"
			}
			m.setStatus("Showing "+viewName+" convoys", false)
			return m, statusExpireCmd(2 * time.Second)
		}
		// Handoff selected agent's work
		if m.selectedAgent == "" {
			m.setStatus("No agent selected. Use j/k to select an agent.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.setStatus("Handing off work for "+m.selectedAgent+"...", false)
		return m, m.actionCmd(ActionHandoff, m.selectedAgent)

	case "K":
		// Kill/stop selected agent (requires confirmation)
		if m.selectedAgent == "" {
			m.setStatus("No agent selected. Use j/k to select an agent.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.confirmDialog = &ConfirmDialog{
			Title:   "Confirm Stop",
			Message: "Stop agent '" + m.selectedAgent + "'? This will nuke the polecat. (y/n)",
			Action:  ActionStopAgent,
			Target:  m.selectedAgent,
		}
		return m, nil

	case "T":
		// Open agent's underlying session (advanced/hidden action for power users)
		// Only works in Agents section
		if m.sidebar.Section != SectionAgents {
			m.setStatus("Switch to Agents section (press 4) to open session", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		if m.selectedAgent == "" {
			m.setStatus("No agent selected. Use j/k to select an agent.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.setStatus("Opening session for "+m.selectedAgent+"...", false)
		return m, m.actionCmd(ActionOpenSession, m.selectedAgent)

	case "m":
		// Context-dependent: Mail agent (Agents section) or toggle mail read (Mail section)
		if m.sidebar != nil && m.sidebar.Section == SectionMail {
			// Toggle read/unread for selected mail (only in Mail section)
			if m.sidebar.Selection < 0 || m.sidebar.Selection >= len(m.sidebar.Mail) {
				m.setStatus("No mail selected", true)
				return m, statusExpireCmd(3 * time.Second)
			}
			mail := m.sidebar.Mail[m.sidebar.Selection].m
			if mail.Read {
				m.setStatus("Marking mail as unread...", false)
				return m, m.mailActionCmd(ActionMarkMailUnread, mail.ID)
			}
			m.setStatus("Marking mail as read...", false)
			return m, m.mailActionCmd(ActionMarkMailRead, mail.ID)
		}
		// Mail selected agent (opens input dialog)
		if m.selectedAgent == "" {
			m.setStatus("No agent selected. Use j/k to select an agent.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.inputDialog = &InputDialog{
			Title:       "Mail Agent",
			Prompt:      "Subject: ",
			ExtraPrompt: "Message: ",
			Action:      ActionMailAgent,
			Target:      m.selectedAgent,
		}
		return m, nil

	case "t":
		// Attach to agent's terminal session
		if m.selectedAgent == "" {
			m.setStatus("No agent selected. Use j/k to select an agent.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.setStatus("Attaching to "+m.selectedAgent+"...", false)
		return m, m.actionCmd(ActionOpenSession, m.selectedAgent)

	case "R":
		// Restart agent's session (requires confirmation)
		if m.selectedAgent == "" {
			m.setStatus("No agent selected. Use j/k to select an agent.", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		m.confirmDialog = &ConfirmDialog{
			Title:   "Confirm Restart",
			Message: "Restart session for '" + m.selectedAgent + "'? This stops and starts the agent. (y/n)",
			Action:  ActionRestartSession,
			Target:  m.selectedAgent,
		}
		return m, nil

	// Sidebar navigation (only when sidebar focused)
	case "j", "down":
		if m.focus == PanelSidebar {
			m.sidebar.SelectNext()
			m.syncSelectedRig()
			if cmd := m.syncSelectedAgent(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case "k", "up":
		if m.focus == PanelSidebar {
			m.sidebar.SelectPrev()
			m.syncSelectedRig()
			if cmd := m.syncSelectedAgent(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case "h", "left":
		if m.focus == PanelSidebar {
			m.sidebar.PrevSection()
			m.syncSelectedRig()
			if cmd := m.syncSelectedAgent(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case "l", "right":
		if m.focus == PanelSidebar {
			m.sidebar.NextSection()
			m.syncSelectedRig()
			if cmd := m.syncSelectedAgent(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case "0":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionIdentity
			m.sidebar.Selection = 0
		}
		return m, nil

	case "1":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionRigs
			m.sidebar.Selection = 0
			if cmd := m.syncSelection(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case "2":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionConvoys
			m.sidebar.Selection = 0
			m.updateSelectedFromSidebar()
		}
		return m, nil

	case "3":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionMergeQueue
			m.sidebar.Selection = 0
			m.updateSelectedFromSidebar()
		}
		return m, nil

	case "4":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionAgents
			m.sidebar.Selection = 0
			if cmd := m.syncSelectedAgent(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case "5":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionMail
			m.sidebar.Selection = 0
		}
		return m, nil

	case "y":
		// Acknowledge selected mail (only in Mail section)
		if m.sidebar.Section != SectionMail {
			m.setStatus("Switch to Mail section (press 5) to ack mail", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		if m.sidebar.Selection < 0 || m.sidebar.Selection >= len(m.sidebar.Mail) {
			m.setStatus("No mail selected", true)
			return m, statusExpireCmd(3 * time.Second)
		}
		mail := m.sidebar.Mail[m.sidebar.Selection].m
		m.setStatus("Acknowledging mail...", false)
		return m, m.mailActionCmd(ActionAckMail, mail.ID)

	case "6":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionLifecycle
			m.sidebar.Selection = 0
		}
		return m, nil

	case "7":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionWorktrees
			m.sidebar.Selection = 0
		}
		return m, nil

	case "8":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionPlugins
			m.sidebar.Selection = 0
			m.updateSelectedFromSidebar()
		}
		return m, nil

	case "9":
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionAlerts
			m.sidebar.Selection = 0
		}
		return m, nil

	case "-":
		// Operator console (subsystem health)
		if m.focus == PanelSidebar {
			m.sidebar.Section = SectionOperator
			m.sidebar.Selection = 0
		}
		return m, nil

	case "e":
		// Edit rig settings (only when in Rigs section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionRigs {
			if m.selectedRig == "" {
				m.setStatus("No rig selected. Use j/k to select a rig.", true)
				return m, statusExpireCmd(3 * time.Second)
			}
			return m, m.openRigSettingsCmd(m.selectedRig)
		}
		// Cycle lifecycle type filter (only in Lifecycle section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionLifecycle {
			m.cycleLifecycleTypeFilter()
			m.sidebar.UpdateFromSnapshot(m.snapshot)
		}
		// Toggle plugin enabled/disabled (only in Plugins section)
		if m.sidebar != nil && m.sidebar.Section == SectionPlugins && m.selectedPlugin != "" {
			m.setStatus("Toggling plugin...", false)
			return m, m.actionCmd(ActionTogglePlugin, m.selectedPlugin)
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
			return m, nil
		}
		// Remove worktree (only when in Worktrees section)
		if m.focus == PanelSidebar && m.sidebar.Section == SectionWorktrees {
			if m.sidebar.Selection < 0 || m.sidebar.Selection >= len(m.sidebar.Worktrees) {
				m.setStatus("No worktree selected", true)
				return m, statusExpireCmd(3 * time.Second)
			}
			wt := m.sidebar.Worktrees[m.sidebar.Selection]
			if !wt.wt.Clean {
				m.setStatus("Worktree has uncommitted changes. Use --force to remove.", true)
				return m, statusExpireCmd(3 * time.Second)
			}
			m.confirmDialog = &ConfirmDialog{
				Title:   "Confirm Remove",
				Message: "Remove worktree '" + wt.wt.SourceRig + "-" + wt.wt.SourceName + "' from " + wt.wt.Rig + "? (y/n)",
				Action:  ActionRemoveWorktree,
				Target:  wt.wt.Path,
			}
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

// renderHealthDetails renders detailed health check information.
func (m Model) renderHealthDetails() string {
	if m.doctorReport == nil {
		return "Health checks not yet loaded.\n\nPress 'r' to refresh."
	}

	var b strings.Builder

	// Summary header
	b.WriteString("Health Check Report\n")
	b.WriteString("═══════════════════\n\n")
	b.WriteString(fmt.Sprintf("Total checks: %d\n", m.doctorReport.TotalChecks))
	b.WriteString(fmt.Sprintf("Passed: %d  ", m.doctorReport.PassedCount))
	b.WriteString(fmt.Sprintf("Warnings: %d  ", m.doctorReport.WarningCount))
	b.WriteString(fmt.Sprintf("Errors: %d\n\n", m.doctorReport.ErrorCount))

	// Show errors first
	if len(m.doctorReport.Errors()) > 0 {
		b.WriteString("❌ ERRORS\n")
		b.WriteString("─────────\n")
		for _, check := range m.doctorReport.Errors() {
			b.WriteString(fmt.Sprintf("✗ %s: %s\n", check.Name, check.Message))
			for _, detail := range check.Details {
				b.WriteString(fmt.Sprintf("    %s\n", detail))
			}
			if check.SuggestFix != "" {
				b.WriteString(fmt.Sprintf("  → %s\n", check.SuggestFix))
			}
			b.WriteString("\n")
		}
	}

	// Show warnings
	if len(m.doctorReport.Warnings()) > 0 {
		b.WriteString("⚠️  WARNINGS\n")
		b.WriteString("───────────\n")
		for _, check := range m.doctorReport.Warnings() {
			b.WriteString(fmt.Sprintf("⚠ %s: %s\n", check.Name, check.Message))
			for _, detail := range check.Details {
				b.WriteString(fmt.Sprintf("    %s\n", detail))
			}
			if check.SuggestFix != "" {
				b.WriteString(fmt.Sprintf("  → %s\n", check.SuggestFix))
			}
			b.WriteString("\n")
		}
	}

	// If no issues
	if !m.doctorReport.HasIssues() {
		b.WriteString("✓ All health checks passed!\n")
	}

	return b.String()
}

// renderConvoyDetails renders detailed view of a convoy for the details panel.
func (m Model) renderConvoyDetails(convoy *data.Convoy) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("Convoy: %s\n", convoy.ID))
	b.WriteString(fmt.Sprintf("Title: %s\n", convoy.Title))
	b.WriteString(fmt.Sprintf("Status: %s\n", convoy.Status))
	b.WriteString(fmt.Sprintf("Progress: %d/%d (%d%%)\n", convoy.Completed, convoy.Total, convoy.Progress()))
	b.WriteString("\n")

	// Tracked issues
	if len(convoy.Tracked) > 0 {
		b.WriteString("Tracked Issues:\n")
		for _, t := range convoy.Tracked {
			statusIcon := "○"
			switch t.Status {
			case "in_progress":
				statusIcon = "▶"
			case "hooked":
				statusIcon = "⊙"
			case "closed":
				statusIcon = "✓"
			}
			b.WriteString(fmt.Sprintf("  %s %s: %s\n", statusIcon, t.ID, t.Title))
			if t.Worker != "" {
				b.WriteString(fmt.Sprintf("    Worker: %s (%s)\n", t.Worker, t.WorkerAge))
			}
		}
	} else {
		b.WriteString("No tracked issues.\n")
	}

	return b.String()
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

// handlePresetNudgeMenuKey handles key presses when the preset nudge menu is shown.
func (m Model) handlePresetNudgeMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.presetNudgeMenu.Selection < len(PresetNudges)-1 {
			m.presetNudgeMenu.Selection++
		}
		return m, nil

	case "k", "up":
		if m.presetNudgeMenu.Selection > 0 {
			m.presetNudgeMenu.Selection--
		}
		return m, nil

	case "enter":
		selected := PresetNudges[m.presetNudgeMenu.Selection]
		target := m.presetNudgeMenu.Target

		// If "Custom..." selected, open custom input dialog
		if selected.Message == "" {
			m.presetNudgeMenu = nil
			m.inputDialog = &InputDialog{
				Title:  "Custom Nudge",
				Prompt: "Message to " + target + ": ",
				Action: ActionNudgeAgent,
				Target: target,
			}
			return m, nil
		}

		// Execute preset nudge
		m.presetNudgeMenu = nil
		m.setStatus("Nudging "+target+"...", false)
		return m, m.actionCmdWithInput(ActionPresetNudge, target, selected.Message, "")

	case "esc", "q":
		m.presetNudgeMenu = nil
		m.setStatus("Nudge cancelled", false)
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

// handleCreateWorkFormKey handles key presses when the create work form is shown.
func (m Model) handleCreateWorkFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle priority keys (1-5)
	if len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '5' {
		if m.createWorkForm.Step() == StepIssueDetails {
			// Set priority (1-5 maps to P0-P4)
			priority := int(msg.String()[0] - '1')
			m.createWorkForm.priority = priority
			return m, nil
		}
	}

	// Check if we need to update targets when step changes
	prevStep := m.createWorkForm.Step()

	cmd := m.createWorkForm.Update(msg)

	// If step changed to target selection, populate targets
	if prevStep == StepSelectRig && m.createWorkForm.Step() == StepSelectTarget {
		m.populateTargets()
	}

	if m.createWorkForm.IsCancelled() {
		m.createWorkForm = nil
		m.setStatus("Create work cancelled", false)
		return m, statusExpireCmd(2 * time.Second)
	}

	if m.createWorkForm.IsSubmitted() {
		title := m.createWorkForm.Title()
		issueType := m.createWorkForm.Type()
		priority := m.createWorkForm.Priority()
		rig := m.createWorkForm.SelectedRig()
		target := m.createWorkForm.SelectedTarget()
		skipSling := m.createWorkForm.SkipSling()

		m.createWorkForm = nil
		m.setStatus("Creating issue '"+title+"'...", false)
		return m, m.createWorkCmd(title, string(issueType), priority, rig, target, skipSling)
	}

	return m, cmd
}

// handleBeadsFormKey handles key presses when the beads form is shown.
func (m Model) handleBeadsFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmd := m.beadsForm.Update(msg)

	if m.beadsForm.IsCancelled() {
		m.beadsForm = nil
		m.setStatus("Bead form cancelled", false)
		return m, statusExpireCmd(2 * time.Second)
	}

	if m.beadsForm.IsSubmitted() {
		title := m.beadsForm.Title()
		description := m.beadsForm.Description()
		issueType := string(m.beadsForm.Type())
		priority := m.beadsForm.Priority()

		if m.beadsForm.Mode() == BeadsModeEdit {
			// Edit mode
			id := m.beadsForm.EditID()
			m.beadsForm = nil
			m.setStatus("Updating bead '"+id+"'...", false)
			return m, m.updateBeadCmd(id, title, description, issueType, priority)
		}
		// Create mode
		m.beadsForm = nil
		m.setStatus("Creating bead '"+title+"'...", false)
		return m, m.createBeadCmd(title, description, issueType, priority)
	}

	return m, cmd
}

// openRigSettingsCmd loads settings and opens the settings form.
func (m Model) openRigSettingsCmd(rigName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		loader := data.NewLoader(m.townRoot)
		settings, err := loader.LoadRigSettings(ctx, rigName)
		return rigSettingsLoadedMsg{rigName: rigName, settings: settings, err: err}
	}
}

// saveRigSettingsCmd saves rig settings.
func (m Model) saveRigSettingsCmd(settings *data.RigSettings) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		loader := data.NewLoader(m.townRoot)
		err := loader.SaveRigSettings(ctx, settings)
		return rigSettingsSavedMsg{rigName: settings.Name, err: err}
	}
}

// handleRigSettingsFormKey handles key presses when the rig settings form is shown.
func (m Model) handleRigSettingsFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmd := m.rigSettingsForm.Update(msg)

	if m.rigSettingsForm.IsCancelled() {
		m.rigSettingsForm = nil
		m.setStatus("Edit cancelled", false)
		return m, statusExpireCmd(2 * time.Second)
	}

	if m.rigSettingsForm.IsSubmitted() {
		settings := m.rigSettingsForm.ToSettings()
		m.rigSettingsForm = nil
		m.setStatus("Saving settings for '"+settings.Name+"'...", false)
		return m, m.saveRigSettingsCmd(settings)
	}

	return m, cmd
}

// populateTargets populates the target selection with polecats from the selected rig.
func (m *Model) populateTargets() {
	if m.createWorkForm == nil || m.snapshot == nil || m.snapshot.Town == nil {
		m.createWorkForm.SetTargets(nil)
		return
	}

	rig := m.createWorkForm.SelectedRig()
	var polecats []string

	for _, r := range m.snapshot.Town.Rigs {
		if r.Name == rig {
			polecats = r.Polecats
			break
		}
	}

	m.createWorkForm.SetTargets(polecats)
}

// createWorkCmd creates a command that creates an issue and optionally slings it.
func (m Model) createWorkCmd(title, issueType string, priority int, rig, target string, skipSling bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := m.actionRunner.CreateWork(ctx, title, issueType, priority, rig, target, skipSling)
		return actionCompleteMsg{action: ActionCreateWork, target: title, err: err}
	}
}

// createBeadCmd creates a command that creates a new bead.
func (m Model) createBeadCmd(title, description, issueType string, priority int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := m.actionRunner.CreateBead(ctx, title, description, issueType, priority)
		return actionCompleteMsg{action: ActionCreateBead, target: title, err: err}
	}
}

// updateBeadCmd creates a command that updates an existing bead.
func (m Model) updateBeadCmd(id, title, description, issueType string, priority int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := m.actionRunner.UpdateBead(ctx, id, title, description, issueType, priority)
		return actionCompleteMsg{action: ActionEditBead, target: id, err: err}
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

// mailActionCmd creates a command that executes a mail action.
func (m Model) mailActionCmd(action ActionType, mailID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var err error
		switch action {
		case ActionMarkMailRead:
			err = m.actionRunner.MarkMailRead(ctx, mailID)
		case ActionMarkMailUnread:
			err = m.actionRunner.MarkMailUnread(ctx, mailID)
		case ActionAckMail:
			err = m.actionRunner.AckMail(ctx, mailID)
		}

		return actionCompleteMsg{action: action, target: mailID, err: err}
	}
}

// handleInputKey handles key presses when an input dialog is shown.
func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		dialog := m.inputDialog
		// If there's an extra field and we're on the first field, move to second
		if dialog.ExtraPrompt != "" && dialog.Field == 0 {
			dialog.Field = 1
			return m, nil
		}
		// Execute the action
		m.inputDialog = nil
		if dialog.Input == "" {
			m.setStatus("Input cancelled (empty)", false)
			return m, statusExpireCmd(2 * time.Second)
		}
		m.setStatus("Executing "+actionName(dialog.Action)+"...", false)
		return m, m.actionCmdWithInput(dialog.Action, dialog.Target, dialog.Input, dialog.ExtraInput)

	case "esc":
		m.inputDialog = nil
		m.setStatus("Input cancelled", false)
		return m, statusExpireCmd(2 * time.Second)

	case "backspace":
		dialog := m.inputDialog
		if dialog.Field == 0 && len(dialog.Input) > 0 {
			dialog.Input = dialog.Input[:len(dialog.Input)-1]
		} else if dialog.Field == 1 && len(dialog.ExtraInput) > 0 {
			dialog.ExtraInput = dialog.ExtraInput[:len(dialog.ExtraInput)-1]
		}
		return m, nil

	case "tab":
		// Toggle between fields if there are two
		dialog := m.inputDialog
		if dialog.ExtraPrompt != "" {
			dialog.Field = (dialog.Field + 1) % 2
		}
		return m, nil

	default:
		// Add character to current input field
		key := msg.String()
		if len(key) == 1 {
			dialog := m.inputDialog
			if dialog.Field == 0 {
				dialog.Input += key
			} else {
				dialog.ExtraInput += key
			}
		}
		return m, nil
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

	case "r":
		// Recheck dependencies
		m.attachDialog.RecheckDependencies()
		return m, nil

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

// syncSelection updates selectedRig, selectedAgent, selectedPlugin, and selectedBeadID based on sidebar selection.
// Returns a command if dependency loading needs to happen.
func (m *Model) syncSelection() tea.Cmd {
	if m.sidebar == nil {
		return nil
	}
	switch m.sidebar.Section {
	case SectionRigs:
		if len(m.sidebar.Rigs) > 0 && m.sidebar.Selection >= 0 && m.sidebar.Selection < len(m.sidebar.Rigs) {
			m.selectedRig = m.sidebar.Rigs[m.sidebar.Selection].r.Name
		}
	case SectionAgents:
		if item := m.sidebar.SelectedItem(); item != nil {
			m.selectedAgent = item.ID()
		}
	case SectionPlugins:
		if item := m.sidebar.SelectedItem(); item != nil {
			m.selectedPlugin = item.ID()
		}
	case SectionBeads:
		if item := m.sidebar.SelectedItem(); item != nil {
			newBeadID := item.ID()
			// Load dependencies and comments if bead selection changed
			if newBeadID != m.selectedBeadID {
				m.selectedBeadID = newBeadID
				m.beadDependencies = nil
				m.beadDepsLoadError = nil
				m.beadDepsLoading = true
				m.beadComments = nil
				m.beadCommentsLoading = true
				return tea.Batch(
					m.loadBeadDependenciesCmd(newBeadID),
					m.loadBeadCommentsCmd(newBeadID),
				)
			}
		}
	}
	return nil
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

// syncSelectedAgent updates selectedAgent and loads audit timeline when navigating in the Agents section.
// Returns a command to load the audit timeline if needed.
func (m *Model) syncSelectedAgent() tea.Cmd {
	if m.sidebar.Section != SectionAgents || len(m.sidebar.Agents) == 0 {
		return nil
	}
	if m.sidebar.Selection < 0 || m.sidebar.Selection >= len(m.sidebar.Agents) {
		return nil
	}

	agent := m.sidebar.Agents[m.sidebar.Selection].a
	m.selectedAgent = agent.Address

	// Load audit timeline if actor changed
	if m.auditTimelineActor != agent.Address {
		m.auditTimelineActor = agent.Address
		m.auditTimeline = nil
		m.auditTimelineLoading = true
		return m.loadAuditTimelineCmd(agent.Address)
	}
	return nil
}

// updateSelectedFromSidebar updates selectedRig, selectedAgent, and selectedPlugin based on sidebar.
func (m *Model) updateSelectedFromSidebar() {
	_ = m.syncSelection()
}

// handleInfrastructureStart handles starting the selected infrastructure subsystem.
func (m Model) handleInfrastructureStart() (tea.Model, tea.Cmd) {
	if m.sidebar.Selection < 0 || m.sidebar.Selection >= len(m.sidebar.Operator) {
		m.setStatus("No subsystem selected", true)
		return m, statusExpireCmd(3 * time.Second)
	}

	subsystem := m.sidebar.Operator[m.sidebar.Selection].h
	var action ActionType
	var target string

	switch subsystem.Subsystem {
	case "deacon":
		action = ActionStartDeacon
		target = "deacon"
	case "witness":
		// Extract rig name from subsystem ID (e.g., "witness_perch")
		if subsystem.Rig != "" {
			action = ActionStartWitness
			target = subsystem.Rig
		} else {
			m.setStatus("Cannot start witness: no rig specified", true)
			return m, statusExpireCmd(3 * time.Second)
		}
	case "refinery":
		if subsystem.Rig != "" {
			action = ActionStartRefinery
			target = subsystem.Rig
		} else {
			m.setStatus("Cannot start refinery: no rig specified", true)
			return m, statusExpireCmd(3 * time.Second)
		}
	default:
		m.setStatus("Cannot start subsystem: "+subsystem.Name, true)
		return m, statusExpireCmd(3 * time.Second)
	}

	m.setStatus("Starting "+subsystem.Name+"...", false)
	return m, m.actionCmd(action, target)
}

// handleInfrastructureStop handles stopping the selected infrastructure subsystem.
func (m Model) handleInfrastructureStop() (tea.Model, tea.Cmd) {
	if m.sidebar.Selection < 0 || m.sidebar.Selection >= len(m.sidebar.Operator) {
		m.setStatus("No subsystem selected", true)
		return m, statusExpireCmd(3 * time.Second)
	}

	subsystem := m.sidebar.Operator[m.sidebar.Selection].h
	var action ActionType
	var target string

	switch subsystem.Subsystem {
	case "deacon":
		action = ActionStopDeacon
		target = "deacon"
	case "witness":
		if subsystem.Rig != "" {
			action = ActionStopWitness
			target = subsystem.Rig
		} else {
			m.setStatus("Cannot stop witness: no rig specified", true)
			return m, statusExpireCmd(3 * time.Second)
		}
	case "refinery":
		if subsystem.Rig != "" {
			action = ActionStopRefinery
			target = subsystem.Rig
		} else {
			m.setStatus("Cannot stop refinery: no rig specified", true)
			return m, statusExpireCmd(3 * time.Second)
		}
	default:
		m.setStatus("Cannot stop subsystem: "+subsystem.Name, true)
		return m, statusExpireCmd(3 * time.Second)
	}

	m.confirmDialog = &ConfirmDialog{
		Title:   "Confirm Stop",
		Message: "Stop " + subsystem.Name + "? (y/n)",
		Action:  action,
		Target:  target,
	}
	return m, nil
}

// handleInfrastructureRestart handles restarting the selected infrastructure subsystem.
func (m Model) handleInfrastructureRestart() (tea.Model, tea.Cmd) {
	if m.sidebar.Selection < 0 || m.sidebar.Selection >= len(m.sidebar.Operator) {
		m.setStatus("No subsystem selected", true)
		return m, statusExpireCmd(3 * time.Second)
	}

	subsystem := m.sidebar.Operator[m.sidebar.Selection].h
	var action ActionType
	var target string

	switch subsystem.Subsystem {
	case "deacon":
		action = ActionRestartDeacon
		target = "deacon"
	case "witness":
		if subsystem.Rig != "" {
			action = ActionRestartWitness
			target = subsystem.Rig
		} else {
			m.setStatus("Cannot restart witness: no rig specified", true)
			return m, statusExpireCmd(3 * time.Second)
		}
	case "refinery":
		if subsystem.Rig != "" {
			action = ActionRestartRefineryAlt
			target = subsystem.Rig
		} else {
			m.setStatus("Cannot restart refinery: no rig specified", true)
			return m, statusExpireCmd(3 * time.Second)
		}
	default:
		m.setStatus("Cannot restart subsystem: "+subsystem.Name, true)
		return m, statusExpireCmd(3 * time.Second)
	}

	m.confirmDialog = &ConfirmDialog{
		Title:   "Confirm Restart",
		Message: "Restart " + subsystem.Name + "? (y/n)",
		Action:  action,
		Target:  target,
	}
	return m, nil
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
	case ActionSlingWork:
		return "Sling work"
	case ActionHandoff:
		return "Handoff"
	case ActionStopAgent:
		return "Stop agent"
	case ActionNudgeAgent:
		return "Nudge"
	case ActionMailAgent:
		return "Mail"
	case ActionNudgeRefinery:
		return "Nudge refinery"
	case ActionRestartRefinery, ActionRestartRefineryAlt:
		return "Restart refinery"
	case ActionStopPolecat:
		return "Stop polecat"
	case ActionStopAllIdle:
		return "Stop all idle"
	case ActionMarkMailRead:
		return "Mark read"
	case ActionMarkMailUnread:
		return "Mark unread"
	case ActionAckMail:
		return "Acknowledge"
	case ActionReplyMail:
		return "Reply"
	case ActionRemoveWorktree:
		return "Remove worktree"
	case ActionCreateWork:
		return "Create work"
	case ActionTogglePlugin:
		return "Toggle plugin"
	case ActionOpenSession:
		return "Attach session"
	case ActionRestartSession:
		return "Restart session"
	case ActionPresetNudge:
		return "Nudge"
	case ActionCreateBead:
		return "Create bead"
	case ActionEditBead:
		return "Update bead"
	// Infrastructure agent controls
	case ActionStartDeacon:
		return "Start deacon"
	case ActionStopDeacon:
		return "Stop deacon"
	case ActionRestartDeacon:
		return "Restart deacon"
	case ActionStartWitness:
		return "Start witness"
	case ActionStopWitness:
		return "Stop witness"
	case ActionRestartWitness:
		return "Restart witness"
	case ActionStartRefinery:
		return "Start refinery"
	case ActionStopRefinery:
		return "Stop refinery"
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

	// Update doctor report from snapshot
	m.doctorReport = snap.DoctorReport

	// Set default convoy selection if none and we have convoys
	if m.selectedConvoy == "" && m.sidebar != nil && len(m.sidebar.Convoys) > 0 {
		m.selectedConvoy = m.sidebar.Convoys[0].ID()
	}
}

// buildSidebarOptions creates sidebar options with queue health info.
func (m Model) buildSidebarOptions() *SidebarOptions {
	opts := &SidebarOptions{}

	// Find the most recent merge time across all rigs
	for _, health := range m.queueHealthData {
		if !health.LastMergeTime.IsZero() {
			if opts.LastMergeTime.IsZero() || health.LastMergeTime.After(opts.LastMergeTime) {
				opts.LastMergeTime = health.LastMergeTime
			}
		}
	}

	return opts
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

	if m.createWorkForm != nil {
		return m.createWorkForm.View(m.width, m.height)
	}

	if m.beadsForm != nil {
		return m.beadsForm.View(m.width, m.height)
	}

	if m.rigSettingsForm != nil {
		return m.rigSettingsForm.View(m.width, m.height)
	}

	if m.attachDialog != nil {
		return m.attachDialog.Render(m.width, m.height)
	}

	if m.presetNudgeMenu != nil {
		return m.renderPresetNudgeMenu()
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

	// Build sidebar options with queue health info
	sidebarOpts := m.buildSidebarOptions()
	sidebar := RenderSidebar(m.sidebar, m.snapshot, sidebarWidth, bodyHeight, m.focus == PanelSidebar, sidebarOpts)

	// Build audit state for agent details
	var auditState *AuditTimelineState
	if m.sidebar.Section == SectionAgents {
		auditState = &AuditTimelineState{
			Actor:   m.auditTimelineActor,
			Entries: m.auditTimeline,
			Loading: m.auditTimelineLoading,
		}
	}

	// Only pass dependencies when viewing a bead (SectionBeads)
	var deps *data.IssueDependencies
	if m.sidebar != nil && m.sidebar.Section == SectionBeads && m.selectedBeadID == m.beadDependencies.IssueID {
		deps = m.beadDependencies
	}
	// Only pass comments when viewing a bead (SectionBeads)
	var comments *data.IssueComments
	if m.sidebar != nil && m.sidebar.Section == SectionBeads && m.selectedBeadID == m.beadComments.IssueID {
		comments = m.beadComments
	}
	details := RenderDetails(m.sidebar, m.snapshot, auditState, detailsWidth, bodyHeight, m.focus == PanelDetails, deps, comments)
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
			for _, loadErr := range m.snapshot.LoadErrors {
				if loadErr.Error != "" {
					errMsg = loadErr.Error
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

	// Operational state banner - always show health status
	lines = append(lines, m.buildOperationalBanner())
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
	// Add stale marker if hook data may be stale (watchdog down or hooked issues failed to load)
	if m.snapshot.HooksDataStale() {
		statsLine += " (stale)"
	}
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

// buildOperationalBanner creates a status banner for operational state
func (m Model) buildOperationalBanner() string {
	state := m.snapshot.OperationalState
	if state == nil {
		return mutedStyle.Render("○ Loading operational state...")
	}

	var lines []string
	var actions []string

	// Degraded mode - most severe
	if state.DegradedMode {
		badge := "⚠ DEGRADED MODE"
		if state.DegradedReason != "" {
			badge += ": " + state.DegradedReason
		}
		lines = append(lines, degradedStyle.Render(badge))
		if state.DegradedAction != "" {
			actions = append(actions, state.DegradedAction)
		}
	}

	// Patrol muted
	if state.PatrolMuted {
		badge := "⏸ PATROL MUTED"
		lines = append(lines, mutedBannerStyle.Render(badge))
		actions = append(actions, "unset GT_PATROL_MUTED to resume")
	}

	// Watchdog status
	if !state.WatchdogHealthy {
		badge := "⚠ WATCHDOG DOWN"
		if state.WatchdogReason != "" {
			badge += ": " + state.WatchdogReason
		}
		lines = append(lines, warningStyle.Render(badge))
		if state.WatchdogAction != "" {
			actions = append(actions, state.WatchdogAction)
		}
	}

	// Show individual issues if not covered above
	if len(state.Issues) > 0 && !state.DegradedMode && state.WatchdogHealthy && !state.PatrolMuted {
		for _, issue := range state.Issues {
			lines = append(lines, warningStyle.Render("• "+issue))
		}
	}

	// If no issues, show healthy status with heartbeat info
	if len(lines) == 0 {
		healthLine := healthyStyle.Render("✓ Healthy")

		// Add heartbeat info
		var heartbeats []string

		// Deacon heartbeat
		if !state.LastDeaconHeartbeat.IsZero() {
			ago := formatDuration(time.Since(state.LastDeaconHeartbeat))
			heartbeats = append(heartbeats, "deacon: "+ago)
		}

		// Witness heartbeats (summarize)
		witnessCount := len(state.LastWitnessHeartbeat)
		if witnessCount > 0 {
			heartbeats = append(heartbeats, fmt.Sprintf("%d witness", witnessCount))
		}

		// Refinery heartbeats (summarize)
		refineryCount := len(state.LastRefineryHeartbeat)
		if refineryCount > 0 {
			heartbeats = append(heartbeats, fmt.Sprintf("%d refinery", refineryCount))
		}

		if len(heartbeats) > 0 {
			healthLine += "  " + mutedStyle.Render("("+strings.Join(heartbeats, ", ")+")")
		}

		return healthLine
	}

	// Build the banner output
	result := strings.Join(lines, "  ")

	// Add action hints on a new line if any
	if len(actions) > 0 {
		actionLine := mutedStyle.Render("  → " + strings.Join(actions, ", "))
		result += "\n" + actionLine
	}

	return result
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

	// Check for watchdog down (deacon not running) - highest priority alert
	if m.snapshot.OperationalState != nil && !m.snapshot.OperationalState.WatchdogHealthy {
		if len(alerts) < maxAlerts {
			reason := "deacon stopped"
			if m.snapshot.OperationalState.WatchdogReason != "" {
				reason = m.snapshot.OperationalState.WatchdogReason
			}
			action := "gt boot"
			if m.snapshot.OperationalState.WatchdogAction != "" {
				action = m.snapshot.OperationalState.WatchdogAction
			}
			alerts = append(alerts, warningStyle.Render("⚠")+
				fmt.Sprintf(" Watchdog down: %s (%s)", reason, action))
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
					fmt.Sprintf(" [%s] %s stopped (select rig, b=boot)", rig.Name, agent.Role))
			}
		}
	}

	// Check for load errors - show which subsystems failed or are stale
	if len(m.snapshot.LoadErrors) > 0 && len(alerts) < maxAlerts {
		// Determine if services are paused (watchdog down) vs actual failures
		servicesDown := m.snapshot.OperationalState != nil && !m.snapshot.OperationalState.WatchdogHealthy

		// Collect unique sources with their last success time
		type sourceInfo struct {
			label       string
			lastSuccess time.Time
		}
		var sources []sourceInfo
		seen := make(map[string]bool)
		for _, err := range m.snapshot.LoadErrors {
			if !seen[err.Source] {
				lastSuccess := m.snapshot.LastSuccess[err.Source]
				sources = append(sources, sourceInfo{
					label:       err.SourceLabel(),
					lastSuccess: lastSuccess,
				})
				seen[err.Source] = true
			}
		}

		if servicesDown {
			// Services paused - show stale state with last refresh times
			var staleInfo []string
			for _, src := range sources {
				info := src.label
				if !src.lastSuccess.IsZero() {
					info += " " + src.lastSuccess.Format("15:04")
				}
				staleInfo = append(staleInfo, info)
			}
			staleList := strings.Join(staleInfo, ", ")
			if len(staleList) > 40 {
				staleList = staleList[:37] + "..."
			}
			alerts = append(alerts, stoppedStyle.Render("◌")+
				fmt.Sprintf(" Data stale: %s", staleList))
		} else {
			// Actual failures - show error state
			var labels []string
			for _, src := range sources {
				labels = append(labels, src.label)
			}
			sourceList := strings.Join(labels, ", ")
			if len(sourceList) > 30 {
				sourceList = sourceList[:27] + "..."
			}
			alerts = append(alerts, statusErrorStyle.Render("●")+
				fmt.Sprintf(" %s failed (press 8 for details)", sourceList))
		}
	}

	return alerts
}

// renderFooter renders the footer with HUD indicators, status message, and help
func (m Model) renderFooter() string {
	// Build HUD section (left side)
	hud := m.renderHUD()

	// Status message takes priority over help text
	var rightSide string
	if m.inputDialog != nil {
		// Show input dialog prompt with current input
		dialog := m.inputDialog
		var prompt, input string
		if dialog.Field == 0 {
			prompt = dialog.Prompt
			input = dialog.Input
		} else {
			prompt = dialog.ExtraPrompt
			input = dialog.ExtraInput
		}
		rightSide = inputStyle.Render(prompt + input + "_")
	} else if m.statusMessage != nil && !m.statusMessage.IsExpired() {
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
			helpItems = append(helpItems, "j/k: select", "h/l: section", "0-9: jump")
			if m.sidebar.Section == SectionRigs {
				helpItems = append(helpItems, "e: edit settings")
			}
			if m.sidebar.Section == SectionMergeQueue {
				helpItems = append(helpItems, "n: nudge")
			}
			if m.sidebar.Section == SectionConvoys {
				helpItems = append(helpItems, "H: history")
			}
			if m.sidebar.Section == SectionAgents {
				helpItems = append(helpItems, "c: stop idle", "C: stop all idle")
			}
			if m.sidebar.Section == SectionLifecycle {
				helpItems = append(helpItems, "e: type filter", "g: agent filter", "x: clear")
			}
			if m.sidebar.Section == SectionMail {
				helpItems = append(helpItems, "m: read/unread", "y: ack")
			}
			if m.sidebar.Section == SectionWorktrees {
				helpItems = append(helpItems, "x: remove")
			}
			if m.sidebar.Section == SectionErrors {
				helpItems = append(helpItems, "r: retry")
			}
			if m.sidebar.Section == SectionOperator {
				helpItems = append(helpItems, "b: start", "s: stop", "r: restart")
			}
		}
		helpItems = append(helpItems, "w: new work", "a: add rig", "A: attach", "r: refresh", "b: boot", "s: stop", "d: delete", "o: logs")
		if m.sidebar != nil && m.sidebar.Section == SectionAgents {
			helpItems = append(helpItems, "n: nudge", "t: attach", "R: restart", "K: kill", "m: mail", "S: sling", "H: handoff")
		}
		if m.sidebar != nil && m.sidebar.Section == SectionPlugins {
			helpItems = append(helpItems, "e: toggle")
		}
		helpItems = append(helpItems, "?: help", "q: quit")
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

// renderPresetNudgeMenu renders the preset nudge selection menu overlay
func (m Model) renderPresetNudgeMenu() string {
	// Calculate menu dimensions (centered, compact)
	menuWidth := 50
	if menuWidth > m.width-4 {
		menuWidth = m.width - 4
	}

	// Build menu content
	var b strings.Builder

	// Title
	title := helpTitleStyle.Render("Nudge " + m.presetNudgeMenu.Target)
	b.WriteString(title)
	b.WriteString("\n\n")

	// Options
	for i, preset := range PresetNudges {
		var line string
		if i == m.presetNudgeMenu.Selection {
			line = selectedItemStyle.Render("> " + preset.Label)
		} else {
			line = mutedStyle.Render("  " + preset.Label)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("j/k: select • enter: send • esc: cancel"))

	content := b.String()

	// Wrap in box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Width(menuWidth)

	box := boxStyle.Render(content)

	// Center on screen
	boxHeight := lipgloss.Height(box)
	boxWidth := lipgloss.Width(box)

	paddingTop := (m.height - boxHeight) / 2
	paddingLeft := (m.width - boxWidth) / 2

	if paddingTop < 0 {
		paddingTop = 0
	}
	if paddingLeft < 0 {
		paddingLeft = 0
	}

	return strings.Repeat("\n", paddingTop) + strings.Repeat(" ", paddingLeft) + box
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
		"            Press H to toggle active/history view",
		helpKeyStyle.Render("Worktrees") + "  Cross-rig git worktrees",
		"            Press x to remove a worktree",
		helpKeyStyle.Render("Beads") + "      Issue tracking (tasks, bugs, features)",
		"",
		helpKeyStyle.Render("Plugins") + "    Town/rig extensions run during patrol",
		"            Automated tasks with cooldown, cron, or event gates",
		"",
		helpHeaderStyle.Render("Behind the Scenes"),
		"",
		helpKeyStyle.Render("Sessions") + "   Background processes for each agent",
		"            Sessions persist even if Perch closes",
	}

	keymap := []string{
		"",
		helpHeaderStyle.Render("Keymap"),
		"",
		helpKeyStyle.Render("?") + "          Show this help",
		helpKeyStyle.Render("q") + "          Quit",
		helpKeyStyle.Render("tab") + "        Next panel",
		helpKeyStyle.Render("shift+tab") + "  Previous panel",
		helpKeyStyle.Render("h/l") + "        Panel left/right, section switch",
		helpKeyStyle.Render("j/k") + "        Navigate up/down",
		helpKeyStyle.Render("0-9") + "        Jump to section (0=Identity...9=Alerts)",
		helpKeyStyle.Render("-") + "          Operator console (system health)",
		helpKeyStyle.Render("H") + "          Toggle convoy active/history view",
		helpKeyStyle.Render("x") + "          Remove worktree / clear lifecycle filters",
		helpKeyStyle.Render("e") + "          Edit rig settings (when in Rigs)",
		helpKeyStyle.Render("a") + "          Add new rig",
		helpKeyStyle.Render("A") + "          Attach to a different town",
		helpKeyStyle.Render("n") + "          Nudge polecat (merge queue)",
		helpKeyStyle.Render("e") + "          Cycle type filter (lifecycle)",
		helpKeyStyle.Render("g") + "          Filter by agent (lifecycle)",
		helpKeyStyle.Render("x") + "          Remove worktree / clear lifecycle filters",
		helpKeyStyle.Render("c") + "          Stop idle polecat (agents)",
		helpKeyStyle.Render("C") + "          Stop all idle polecats in rig",
		helpKeyStyle.Render("r") + "          Refresh data",
		helpKeyStyle.Render("b") + "          Boot rig / Create-edit bead (beads)",
		helpKeyStyle.Render("s") + "          Shutdown rig / Toggle scope (beads)",
		helpKeyStyle.Render("d") + "          Delete selected rig",
		helpKeyStyle.Render("o") + "          Open logs for agent",
		"",
		helpHeaderStyle.Render("Agent Actions (when in Agents section)"),
		"",
		helpKeyStyle.Render("S") + "          Sling work to agent",
		helpKeyStyle.Render("H") + "          Handoff agent's work",
		helpKeyStyle.Render("K") + "          Kill/stop agent",
		helpKeyStyle.Render("n") + "          Nudge agent with message",
		helpKeyStyle.Render("m") + "          Mail agent",
		helpKeyStyle.Render("T") + "          Open session (advanced)",
		"",
		helpHeaderStyle.Render("Plugin Actions (when in Plugins section)"),
		"",
		helpKeyStyle.Render("e") + "          Toggle plugin enabled/disabled",
		"",
		helpHeaderStyle.Render("Infrastructure Actions (when in Operator section)"),
		"",
		helpKeyStyle.Render("b") + "          Start selected subsystem (Deacon/Witness/Refinery)",
		helpKeyStyle.Render("s") + "          Stop selected subsystem",
		helpKeyStyle.Render("r") + "          Restart selected subsystem",
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
		lines = append(lines, mutedStyle.Render("... (Press any key)"))
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
