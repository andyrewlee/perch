package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/andyrewlee/perch/data"
	"github.com/charmbracelet/lipgloss"
)

// AgentHealthStatus represents the health status of an agent.
type AgentHealthStatus int

const (
	AgentHealthy   AgentHealthStatus = iota // Agent is running and working/healthy
	AgentIdle                                // Agent is running but idle
	AgentStopped                             // Agent is not running
	AgentStale                               // Agent has stale work or needs attention
	AgentError                               // Agent has errors
)

// String returns the string representation of the agent health status.
func (s AgentHealthStatus) String() string {
	switch s {
	case AgentHealthy:
		return "healthy"
	case AgentIdle:
		return "idle"
	case AgentStopped:
		return "stopped"
	case AgentStale:
		return "stale"
	case AgentError:
		return "error"
	default:
		return "unknown"
	}
}

// Badge returns a colored unicode badge for the status.
func (s AgentHealthStatus) Badge() string {
	switch s {
	case AgentHealthy:
		return agentHealthyStyle.Render("●")
	case AgentIdle:
		return agentIdleStyle.Render("○")
	case AgentStopped:
		return agentStoppedStyle.Render("◌")
	case AgentStale:
		return agentStaleStyle.Render("⚠")
	case AgentError:
		return agentErrorStyle.Render("✗")
	default:
		return agentUnknownStyle.Render("?")
	}
}

// AgentEntry represents an agent with its health information for the dashboard.
type AgentEntry struct {
	Agent         data.Agent
	RigName       string
	HealthStatus  AgentHealthStatus
	WorkAge       time.Duration // Time since work was hooked
	LastHeartbeat time.Time     // Last heartbeat/check-in time
	MailUnread   int            // Unread mail count
}

// RoleBadge returns a styled badge for the agent role.
func (e AgentEntry) RoleBadge() string {
	switch e.Agent.Role {
	case "polecat":
		return rolePolecatStyle.Render("P")
	case "witness":
		return roleWitnessStyle.Render("W")
	case "refinery":
		return roleRefineryStyle.Render("R")
	case "deacon":
		return roleDeaconStyle.Render("D")
	case "mayor":
		return roleMayorStyle.Render("M")
	default:
		return roleUnknownStyle.Render("?")
	}
}

// AgentDashboard holds agent health data for the dashboard view.
type AgentDashboard struct {
	Entries     []AgentEntry
	Summary     AgentSummary
	LastRefresh time.Time
	SelectedIdx int // Currently selected agent for actions
}

// AgentSummary provides aggregate statistics about agent health.
type AgentSummary struct {
	Total      int
	Running    int
	Working    int
	Idle       int
	Stopped    int
	Stale      int
	WithMail   int // Agents with unread mail
	ByRig      map[string]int // Agent count per rig
	ByRole     map[string]int // Agent count by role
}

// NewAgentDashboard creates a new agent dashboard from a snapshot.
func NewAgentDashboard(snap *data.Snapshot) *AgentDashboard {
	if snap == nil || snap.Town == nil {
		return &AgentDashboard{
			Entries:     []AgentEntry{},
			LastRefresh: now(),
		}
	}

	dash := &AgentDashboard{
		Entries:     []AgentEntry{},
		LastRefresh: now(),
		Summary: AgentSummary{
			ByRig:  make(map[string]int),
			ByRole: make(map[string]int),
		},
	}

	// Collect all agents from all rigs
	for _, rig := range snap.Town.Rigs {
		rigCount := 0
		for _, agent := range rig.Agents {
			entry := buildAgentEntry(agent, rig.Name, snap)
			dash.Entries = append(dash.Entries, entry)
			dash.updateSummary(entry)
			rigCount++
		}
		dash.Summary.ByRig[rig.Name] = rigCount
	}

	dash.Summary.Total = len(dash.Entries)

	return dash
}

// buildAgentEntry creates an AgentEntry from an agent and snapshot data.
func buildAgentEntry(agent data.Agent, rigName string, snap *data.Snapshot) AgentEntry {
	entry := AgentEntry{
		Agent:       agent,
		RigName:     rigName,
		MailUnread:  agent.UnreadMail,
	}

	// Determine health status
	if !agent.Running {
		entry.HealthStatus = AgentStopped
	} else if agent.HasWork {
		// Check work age for staleness
		if !agent.HookedAt.IsZero() {
			entry.WorkAge = since(agent.HookedAt)
			if entry.WorkAge > 2*time.Hour {
				entry.HealthStatus = AgentStale
			} else {
				entry.HealthStatus = AgentHealthy
			}
		} else {
			entry.HealthStatus = AgentHealthy
		}
	} else if agent.UnreadMail > 0 {
		entry.HealthStatus = AgentStale // Needs attention due to mail
	} else {
		entry.HealthStatus = AgentIdle
	}

	return entry
}

// updateSummary updates the summary statistics with an agent entry.
func (d *AgentDashboard) updateSummary(entry AgentEntry) {
	switch entry.HealthStatus {
	case AgentHealthy:
		d.Summary.Running++
		d.Summary.Working++
	case AgentIdle:
		d.Summary.Running++
		d.Summary.Idle++
	case AgentStopped:
		d.Summary.Stopped++
	case AgentStale:
		d.Summary.Running++
		d.Summary.Stale++
	}

	if entry.MailUnread > 0 {
		d.Summary.WithMail++
	}

	role := entry.Agent.Role
	d.Summary.ByRole[role]++
}

// Render renders the agent dashboard view.
func (d *AgentDashboard) Render(width, height int) string {
	if width < 20 || height < 5 {
		return "..."
	}

	var lines []string

	// Header
	lines = append(lines, renderDashboardHeader(d.Summary))

	// Summary bar
	lines = append(lines, renderSummaryBar(d.Summary, width))

	// Agent list
	maxLines := height - len(lines) - 3 // Reserve space for footer
	agentLines := d.renderAgentList(width, maxLines)
	lines = append(lines, agentLines)

	// Footer with hints
	lines = append(lines, "")
	lines = append(lines, renderDashboardHints())

	return strings.Join(lines, "\n")
}

// renderDashboardHeader renders the dashboard title.
func renderDashboardHeader(summary AgentSummary) string {
	return dashboardTitleStyle.Render("Agent Status Dashboard")
}

// renderSummaryBar renders the summary statistics bar.
func renderSummaryBar(summary AgentSummary, width int) string {
	var parts []string

	// Total count
	parts = append(parts, fmt.Sprintf("%d agents", summary.Total))

	// Status breakdown
	if summary.Working > 0 {
		parts = append(parts, agentHealthyStyle.Render(fmt.Sprintf("%d working", summary.Working)))
	}
	if summary.Idle > 0 {
		parts = append(parts, agentIdleStyle.Render(fmt.Sprintf("%d idle", summary.Idle)))
	}
	if summary.Stopped > 0 {
		parts = append(parts, agentStoppedStyle.Render(fmt.Sprintf("%d stopped", summary.Stopped)))
	}
	if summary.Stale > 0 {
		parts = append(parts, agentStaleStyle.Render(fmt.Sprintf("%d stale", summary.Stale)))
	}

	// Mail alert
	if summary.WithMail > 0 {
		parts = append(parts, agentMailStyle.Render(fmt.Sprintf("%d with mail", summary.WithMail)))
	}

	line := strings.Join(parts, " • ")

	// Truncate if too long
	if len(line) > width-4 {
		line = line[:width-7] + "..."
	}

	return mutedStyle.Render(line)
}

// renderAgentList renders the list of agents grouped by rig.
func (d *AgentDashboard) renderAgentList(width, maxLines int) string {
	if len(d.Entries) == 0 {
		return mutedStyle.Render("\nNo agents found")
	}

	if maxLines < 2 {
		maxLines = 2
	}

	var lines []string
	lines = append(lines, "")

	// Group by rig
	byRig := make(map[string][]AgentEntry)
	for _, entry := range d.Entries {
		byRig[entry.RigName] = append(byRig[entry.RigName], entry)
	}

	// Render each rig's agents
	for rigName, entries := range byRig {
		if len(lines) >= maxLines {
			remaining := len(d.Entries) - (len(lines) - 1) // Approximate
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("  ... and %d more agents", remaining)))
			break
		}

		// Rig header
		rigHeader := fmt.Sprintf("  [%s]", rigName)
		lines = append(lines, dashboardRigHeaderStyle.Render(rigHeader))

		// Agents in this rig
		for _, entry := range entries {
			if len(lines) >= maxLines {
				break
			}
			lines = append(lines, d.renderAgentLine(entry, width))
		}

		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// renderAgentLine renders a single agent line.
func (d *AgentDashboard) renderAgentLine(entry AgentEntry, width int) string {
	// Format: [role] name status [work info]
	roleBadge := entry.RoleBadge()
	statusBadge := entry.HealthStatus.Badge()

	name := entry.Agent.Name
	if len(name) > 20 {
		name = name[:17] + "..."
	}

	// Work info
	var workInfo string
	if entry.Agent.HasWork {
		workInfo = fmt.Sprintf("(%s)", entry.Agent.HookedBeadID)
		if len(workInfo) > 20 {
			workInfo = workInfo[:17] + "...)"
		}
	} else if entry.HealthStatus == AgentIdle {
		workInfo = "(idle)"
	} else if entry.HealthStatus == AgentStopped {
		workInfo = "(stopped)"
	}

	// Mail indicator
	mailIndicator := ""
	if entry.MailUnread > 0 {
		mailIndicator = agentMailStyle.Render(fmt.Sprintf(" ✉%d", entry.MailUnread))
	}

	line := fmt.Sprintf("    %s %s %s %s%s",
		roleBadge, name, statusBadge, workInfo, mailIndicator)

	// Truncate if too long
	if len(line) > width-4 {
		line = line[:width-7] + "..."
	}

	// Highlight if selected
	if d.SelectedIdx >= 0 && d.SelectedIdx < len(d.Entries) {
		if d.Entries[d.SelectedIdx].Agent.Name == entry.Agent.Name &&
			d.Entries[d.SelectedIdx].RigName == entry.RigName {
			return selectedItemStyle.Render("  > " + line[4:])
		}
	}

	return line
}

// renderDashboardHints renders the keyboard hints at the bottom.
func renderDashboardHints() string {
	return mutedStyle.Render("enter: details | a: attach | n: nudge | r: refresh")
}

// AgentDetailDialog shows detailed information about a single agent.
type AgentDetailDialog struct {
	Agent         data.Agent
	RigName       string
	HealthStatus  AgentHealthStatus
	WorkAge       time.Duration
	LastHeartbeat time.Time
	MailUnread    int
	SelectedAction int // 0=nudge, 1=attach, 2=mail, 3=handoff/stop/start
	ShowActions   bool // Toggle action menu visibility
}

// NewAgentDetailDialog creates a new agent detail dialog from an AgentEntry.
func NewAgentDetailDialog(entry AgentEntry) *AgentDetailDialog {
	return &AgentDetailDialog{
		Agent:         entry.Agent,
		RigName:       entry.RigName,
		HealthStatus:  entry.HealthStatus,
		WorkAge:       entry.WorkAge,
		LastHeartbeat: entry.LastHeartbeat,
		MailUnread:    entry.MailUnread,
		SelectedAction: 0,
		ShowActions:   true,
	}
}

// Render renders the agent detail dialog.
func (d *AgentDetailDialog) Render(width, height int) string {
	if width < 40 || height < 10 {
		return "Window too small for detail view"
	}

	// Dialog dimensions
	dialogWidth := min(width-4, 70)
	dialogHeight := min(height-4, 25)

	// Build content
	var lines []string

	// Title with role badge
	roleBadge := d.roleBadge()
	title := dialogTitleStyle.Render(fmt.Sprintf("┤ %s %s ├", d.Agent.Name, roleBadge))
	lines = append(lines, title)
	lines = append(lines, dialogBorderStyle.Render(strings.Repeat("─", dialogWidth-2)))

	// Agent info section
	lines = append(lines, "")
	lines = append(lines, dialogLabelStyle.Render("Address:   ")+dialogValueStyle.Render(d.Agent.Address))
	lines = append(lines, dialogLabelStyle.Render("Rig:       ")+dialogValueStyle.Render(d.RigName))
	lines = append(lines, dialogLabelStyle.Render("Role:      ")+dialogValueStyle.Render(d.Agent.Role))
	lines = append(lines, dialogLabelStyle.Render("Status:    ")+d.healthStatusLine())
	lines = append(lines, dialogLabelStyle.Render("Session:   ")+dialogValueStyle.Render(d.sessionStatus()))

	// Work hook section
	lines = append(lines, "")
	lines = append(lines, dialogSectionStyle.Render("┌─ Work Hook ─────────────────────"))
	if d.Agent.HasWork {
		lines = append(lines, "")
		lines = append(lines, dialogLabelStyle.Render("Bead ID:   ")+dialogValueStyle.Render(d.Agent.HookedBeadID))
		lines = append(lines, dialogLabelStyle.Render("Status:    ")+dialogValueStyle.Render(d.Agent.HookedStatus))
		if !d.Agent.HookedAt.IsZero() {
			lines = append(lines, dialogLabelStyle.Render("Hooked:    ")+dialogValueStyle.Render(formatTimestamp(d.Agent.HookedAt)))
			lines = append(lines, dialogLabelStyle.Render("Age:       ")+dialogValueStyle.Render(formatDurationAge(d.WorkAge)))
		}
	} else {
		lines = append(lines, "")
		lines = append(lines, mutedStyle.Render("  No work hooked"))
	}

	// Mail section
	lines = append(lines, "")
	lines = append(lines, dialogSectionStyle.Render("┌─ Mail ──────────────────────────"))
	if d.MailUnread > 0 {
		lines = append(lines, "")
		mailLine := dialogValueStyle.Render(fmt.Sprintf("%d unread message", d.MailUnread))
		if d.MailUnread > 1 {
			mailLine = dialogValueStyle.Render(fmt.Sprintf("%d unread messages", d.MailUnread))
		}
		if d.Agent.FirstSubject != "" {
			mailLine += mutedStyle.Render(fmt.Sprintf(" (\"%s\")", truncateString(d.Agent.FirstSubject, 30)))
		}
		lines = append(lines, "  ✉ " + mailLine)
	} else {
		lines = append(lines, "")
		lines = append(lines, mutedStyle.Render("  No unread mail"))
	}

	// Actions section
	if d.ShowActions {
		lines = append(lines, "")
		lines = append(lines, dialogSectionStyle.Render("┌─ Quick Actions ────────────────"))
		lines = append(lines, "")
		actions := d.getActions()
		for i, action := range actions {
			if i == d.SelectedAction {
				lines = append(lines, selectedItemStyle.Render("▶ ")+action)
			} else {
				lines = append(lines, "  "+action)
			}
		}
	}

	// Footer hint
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("enter: execute action • esc: close • q: quit"))

	// Render dialog box
	return renderDialogBox(lines, dialogWidth, dialogHeight)
}

// roleBadge returns a styled badge for the agent role.
func (d *AgentDetailDialog) roleBadge() string {
	switch d.Agent.Role {
	case "polecat":
		return rolePolecatStyle.Render("[P]")
	case "witness":
		return roleWitnessStyle.Render("[W]")
	case "refinery":
		return roleRefineryStyle.Render("[R]")
	case "deacon":
		return roleDeaconStyle.Render("[D]")
	case "mayor":
		return roleMayorStyle.Render("[M]")
	default:
		return roleUnknownStyle.Render("[?]")
	}
}

// healthStatusLine returns a formatted health status line.
func (d *AgentDetailDialog) healthStatusLine() string {
	status := d.HealthStatus.String()
	badge := d.HealthStatus.Badge()
	return badge + " " + status
}

// sessionStatus returns the session status.
func (d *AgentDetailDialog) sessionStatus() string {
	if d.Agent.Session == "" {
		return "none"
	}
	if d.Agent.Running {
		return dialogValueStyle.Render(d.Agent.Session + " (running)")
	}
	return mutedStyle.Render(d.Agent.Session + " (stopped)")
}

// getActions returns the list of available actions.
func (d *AgentDetailDialog) getActions() []string {
	actions := []string{
		"n: Send nudge",
		"a: Attach session",
		"m: Send mail",
	}

	if d.Agent.Running {
		actions = append(actions, "h: Handoff work")
		actions = append(actions, "s: Stop agent")
	} else {
		actions = append(actions, "t: Start session")
	}

	return actions
}

// ActionCount returns the number of available actions.
func (d *AgentDetailDialog) ActionCount() int {
	if d.Agent.Running {
		return 5 // nudge, attach, mail, handoff, stop
	}
	return 4 // nudge, attach, mail, start
}

// ExecuteAction runs the action at the given index.
func (d *AgentDetailDialog) ExecuteAction(idx int) (ActionType, string) {
	actions := d.getActions()
	if idx < 0 || idx >= len(actions) {
		return ActionRefresh, ""
	}

	// Map index to action type
	if idx == 0 {
		return ActionPresetNudge, d.Agent.Address
	}
	if idx == 1 {
		return ActionOpenSession, d.Agent.Address
	}
	if idx == 2 {
		return ActionMailAgent, d.Agent.Address
	}
	if d.Agent.Running {
		if idx == 3 {
			return ActionHandoff, d.Agent.Address
		}
		if idx == 4 {
			return ActionStopAgent, d.Agent.Address
		}
	} else {
		if idx == 3 {
			return ActionStartSession, d.Agent.Address
		}
	}

	return ActionRefresh, ""
}

// SelectNext moves to the next action.
func (d *AgentDetailDialog) SelectNext() {
	if d.SelectedAction < d.ActionCount()-1 {
		d.SelectedAction++
	}
}

// SelectPrev moves to the previous action.
func (d *AgentDetailDialog) SelectPrev() {
	if d.SelectedAction > 0 {
		d.SelectedAction--
	}
}

// renderDialogBox renders content inside a dialog box.
func renderDialogBox(lines []string, width, height int) string {
	// Pad content to desired height
	for len(lines) < height {
		lines = append(lines, "")
	}

	// Truncate or pad each line to width
	var contentLines []string
	for _, line := range lines {
		if len(line) > width-2 {
			line = line[:width-5] + "..."
		}
		contentLines = append(contentLines, lipgloss.NewStyle().Width(width-2).Render(line))
	}

	// Build box with border
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(highlight).
		Width(width).
		Height(height).
		Padding(0, 1).
		Render(strings.Join(contentLines, "\n"))

	// Center the box
	return lipgloss.NewStyle().
		Align(lipgloss.Center, lipgloss.Center).
		Render(box)
}

// formatTimestamp formats a timestamp for display.
func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	age := since(t)
	if age < time.Minute {
		return "just now"
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	}
	if age < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	}
	return t.Format("Jan 02 15:04")
}

// formatDurationAge formats a duration for display as work age.
func formatDurationAge(d time.Duration) string {
	if d < time.Minute {
		return "< 1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// truncateString truncates a string to max length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Dialog styles
var (
	dialogTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginBottom(1)

	dialogBorderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88AADD"))

	dialogLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88CCFF")).
			Width(11)

	dialogValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	dialogSectionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88AADD")).
			Bold(true)
)

// Dashboard styles
var (
	agentHealthyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	agentIdleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	agentStoppedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	agentStaleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFCC00"))

	agentErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6666")).
			Bold(true)

	agentUnknownStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	agentMailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAAFF"))

	rolePolecatStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88CCFF")).
			Bold(true)

	roleWitnessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFCC88")).
			Bold(true)

	roleRefineryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CC88FF")).
			Bold(true)

	roleDeaconStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88FF88")).
			Bold(true)

	roleMayorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF88CC")).
			Bold(true)

	roleUnknownStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	dashboardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight)

	dashboardRigHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#88AADD"))
)
