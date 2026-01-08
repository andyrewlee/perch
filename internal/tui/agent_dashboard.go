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
			LastRefresh: time.Now(),
		}
	}

	dash := &AgentDashboard{
		Entries:     []AgentEntry{},
		LastRefresh: time.Now(),
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
			entry.WorkAge = time.Since(agent.HookedAt)
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
