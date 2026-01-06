package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/andyrewlee/perch/data"
	"github.com/charmbracelet/lipgloss"
)

// SubsystemStatus represents the health status of a subsystem.
type SubsystemStatus int

const (
	SubsystemHealthy SubsystemStatus = iota
	SubsystemWarning
	SubsystemError
	SubsystemUnknown
)

func (s SubsystemStatus) String() string {
	switch s {
	case SubsystemHealthy:
		return "healthy"
	case SubsystemWarning:
		return "warning"
	case SubsystemError:
		return "error"
	default:
		return "unknown"
	}
}

// Badge returns a colored badge for the status.
func (s SubsystemStatus) Badge() string {
	switch s {
	case SubsystemHealthy:
		return operatorHealthyStyle.Render("●")
	case SubsystemWarning:
		return operatorWarningStyle.Render("⚠")
	case SubsystemError:
		return operatorErrorStyle.Render("✗")
	default:
		return operatorUnknownStyle.Render("?")
	}
}

// SubsystemHealth represents the health of a single subsystem.
type SubsystemHealth struct {
	Name        string          // Display name (e.g., "Deacon", "Beads Sync")
	Subsystem   string          // ID (e.g., "deacon", "beads_sync")
	Status      SubsystemStatus // Current health status
	Message     string          // Short status message
	Details     string          // Detailed info (shown in details panel)
	Action      string          // Recommended action if unhealthy
	LastChecked time.Time       // When this was last checked
	Rig         string          // Rig name (for per-rig items)
}

// operatorItem wraps SubsystemHealth for sidebar selection
type operatorItem struct {
	h SubsystemHealth
}

func (o operatorItem) ID() string     { return o.h.Subsystem }
func (o operatorItem) Label() string  { return fmt.Sprintf("%s %s", o.h.Status.Badge(), o.h.Name) }
func (o operatorItem) Status() string { return o.h.Status.String() }

// OperatorState holds the operator console state.
type OperatorState struct {
	Subsystems    []SubsystemHealth
	LastRefresh   time.Time
	HasIssues     bool
	IssueCount    int
	WarningCount  int
}

// BuildOperatorState builds the operator state from a snapshot.
func BuildOperatorState(snap *data.Snapshot) *OperatorState {
	if snap == nil {
		return &OperatorState{}
	}

	state := &OperatorState{
		LastRefresh: time.Now(),
	}

	// 1. Deacon health
	state.Subsystems = append(state.Subsystems, buildDeaconHealth(snap))

	// 2. Beads sync status
	state.Subsystems = append(state.Subsystems, buildBeadsSyncHealth(snap))

	// 3. Per-rig subsystems
	if snap.Town != nil {
		for _, rig := range snap.Town.Rigs {
			// Witness health
			state.Subsystems = append(state.Subsystems, buildWitnessHealth(rig))

			// Refinery health
			state.Subsystems = append(state.Subsystems, buildRefineryHealth(rig, snap.MergeQueues[rig.Name]))

			// Hooks health (stale work detection)
			state.Subsystems = append(state.Subsystems, buildHooksHealth(rig))
		}
	}

	// Count issues
	for _, s := range state.Subsystems {
		if s.Status == SubsystemError {
			state.IssueCount++
			state.HasIssues = true
		} else if s.Status == SubsystemWarning {
			state.WarningCount++
			state.HasIssues = true
		}
	}

	return state
}

// buildDeaconHealth checks deacon/watchdog health.
func buildDeaconHealth(snap *data.Snapshot) SubsystemHealth {
	h := SubsystemHealth{
		Name:        "Deacon",
		Subsystem:   "deacon",
		Status:      SubsystemHealthy,
		LastChecked: time.Now(),
	}

	if snap.OperationalState == nil {
		h.Status = SubsystemUnknown
		h.Message = "Status unknown"
		h.Action = "Refresh to check status"
		return h
	}

	state := snap.OperationalState

	// Check degraded mode first
	if state.DegradedMode {
		h.Status = SubsystemError
		h.Message = "Degraded mode"
		h.Details = state.DegradedReason
		h.Action = state.DegradedAction
		if h.Action == "" {
			h.Action = "Check tmux availability"
		}
		return h
	}

	// Check watchdog health
	if !state.WatchdogHealthy {
		h.Status = SubsystemError
		h.Message = "Watchdog down"
		h.Details = state.WatchdogReason
		h.Action = state.WatchdogAction
		if h.Action == "" {
			h.Action = "Run 'gt deacon start' to start deacon"
		}
		return h
	}

	// Check patrol muted
	if state.PatrolMuted {
		h.Status = SubsystemWarning
		h.Message = "Patrol muted"
		h.Details = "Deacon patrol is muted via GT_PATROL_MUTED"
		h.Action = "unset GT_PATROL_MUTED to resume patrol"
		return h
	}

	// Check heartbeat freshness
	if !state.LastDeaconHeartbeat.IsZero() {
		age := time.Since(state.LastDeaconHeartbeat)
		if age > 5*time.Minute {
			h.Status = SubsystemWarning
			h.Message = fmt.Sprintf("Stale heartbeat (%s ago)", formatDuration(age))
			h.Details = "Deacon hasn't checked in recently"
			h.Action = "Check if deacon process is running"
			return h
		}
		h.Message = fmt.Sprintf("Heartbeat: %s ago", formatDuration(age))
	} else {
		h.Message = "Running"
	}

	h.Details = "Deacon is healthy and monitoring the town"
	return h
}

// buildBeadsSyncHealth checks beads sync status.
func buildBeadsSyncHealth(snap *data.Snapshot) SubsystemHealth {
	h := SubsystemHealth{
		Name:        "Beads Sync",
		Subsystem:   "beads_sync",
		Status:      SubsystemHealthy,
		LastChecked: time.Now(),
	}

	// Check for beads-related load errors
	for _, err := range snap.LoadErrors {
		if err.Source == "issues" || err.Source == "hooked_issues" {
			h.Status = SubsystemError
			h.Message = "Load error"
			h.Details = err.Error
			h.Action = "Run 'bd sync' to sync beads"
			return h
		}
	}

	// If we have issues loaded, sync is working
	if len(snap.Issues) > 0 {
		h.Message = fmt.Sprintf("%d issues loaded", len(snap.Issues))
		h.Details = "Beads are syncing correctly"
		return h
	}

	// No issues loaded could be normal (empty) or an issue
	h.Message = "No issues"
	h.Details = "No beads issues loaded - this may be normal for a new project"
	return h
}

// buildWitnessHealth checks witness health for a rig.
func buildWitnessHealth(rig data.Rig) SubsystemHealth {
	h := SubsystemHealth{
		Name:        fmt.Sprintf("[%s] Witness", rig.Name),
		Subsystem:   fmt.Sprintf("witness_%s", rig.Name),
		Rig:         rig.Name,
		Status:      SubsystemHealthy,
		LastChecked: time.Now(),
	}

	if !rig.HasWitness {
		h.Status = SubsystemUnknown
		h.Message = "Not configured"
		h.Details = "No witness configured for this rig"
		h.Action = "Run 'gt boot " + rig.Name + "' to start witness"
		return h
	}

	// Find witness agent
	var witness *data.Agent
	for i := range rig.Agents {
		if rig.Agents[i].Role == "witness" {
			witness = &rig.Agents[i]
			break
		}
	}

	if witness == nil {
		h.Status = SubsystemError
		h.Message = "Not found"
		h.Details = "Witness is configured but agent not found"
		h.Action = "Run 'gt boot " + rig.Name + "' to start witness"
		return h
	}

	if !witness.Running {
		h.Status = SubsystemError
		h.Message = "Stopped"
		h.Details = "Witness session is not running"
		h.Action = "Run 'gt boot " + rig.Name + "' to start witness"
		return h
	}

	h.Message = "Running"
	h.Details = fmt.Sprintf("Witness is active at %s", witness.Address)
	return h
}

// buildRefineryHealth checks refinery health for a rig.
func buildRefineryHealth(rig data.Rig, mrs []data.MergeRequest) SubsystemHealth {
	h := SubsystemHealth{
		Name:        fmt.Sprintf("[%s] Refinery", rig.Name),
		Subsystem:   fmt.Sprintf("refinery_%s", rig.Name),
		Rig:         rig.Name,
		Status:      SubsystemHealthy,
		LastChecked: time.Now(),
	}

	if !rig.HasRefinery {
		h.Status = SubsystemUnknown
		h.Message = "Not configured"
		h.Details = "No refinery configured for this rig"
		h.Action = "Run 'gt boot " + rig.Name + "' to start refinery"
		return h
	}

	// Find refinery agent
	var refinery *data.Agent
	for i := range rig.Agents {
		if rig.Agents[i].Role == "refinery" {
			refinery = &rig.Agents[i]
			break
		}
	}

	if refinery == nil {
		h.Status = SubsystemError
		h.Message = "Not found"
		h.Details = "Refinery is configured but agent not found"
		h.Action = "Run 'gt boot " + rig.Name + "' to start refinery"
		return h
	}

	if !refinery.Running {
		h.Status = SubsystemError
		h.Message = "Stopped"
		h.Details = "Refinery session is not running"
		h.Action = "Run 'gt boot " + rig.Name + "' to start refinery"
		return h
	}

	// Check merge queue for stalled MRs
	conflictCount := 0
	for _, mr := range mrs {
		if mr.HasConflicts || mr.NeedsRebase {
			conflictCount++
		}
	}

	if conflictCount > 0 {
		h.Status = SubsystemWarning
		h.Message = fmt.Sprintf("Running (%d conflicts)", conflictCount)
		h.Details = fmt.Sprintf("%d merge requests have conflicts or need rebase", conflictCount)
		h.Action = "Nudge polecats to resolve conflicts (press 'n' in MQ section)"
		return h
	}

	if len(mrs) > 0 {
		h.Message = fmt.Sprintf("Running (%d queued)", len(mrs))
	} else {
		h.Message = "Idle"
	}
	h.Details = fmt.Sprintf("Refinery is active at %s", refinery.Address)
	return h
}

// buildHooksHealth checks for stale hooks in a rig.
func buildHooksHealth(rig data.Rig) SubsystemHealth {
	h := SubsystemHealth{
		Name:        fmt.Sprintf("[%s] Hooks", rig.Name),
		Subsystem:   fmt.Sprintf("hooks_%s", rig.Name),
		Rig:         rig.Name,
		Status:      SubsystemHealthy,
		LastChecked: time.Now(),
	}

	activeHooks := 0
	stoppedWithWork := 0

	for _, hook := range rig.Hooks {
		if hook.HasWork {
			activeHooks++
		}
	}

	// Check for agents that have work but aren't running (stale hooks)
	for _, agent := range rig.Agents {
		if agent.HasWork && !agent.Running {
			stoppedWithWork++
		}
	}

	if stoppedWithWork > 0 {
		h.Status = SubsystemWarning
		h.Message = fmt.Sprintf("%d stale (agent stopped)", stoppedWithWork)
		h.Details = fmt.Sprintf("%d agents have hooked work but are not running", stoppedWithWork)
		h.Action = "Nudge agents to resume work or handoff"
		return h
	}

	if activeHooks > 0 {
		h.Message = fmt.Sprintf("%d active", activeHooks)
		h.Details = fmt.Sprintf("%d hooks are active with work assigned", activeHooks)
	} else {
		h.Message = "No active hooks"
		h.Details = "No work is currently hooked to agents"
	}
	return h
}

// RenderOperatorSection renders the operator console sidebar section.
func RenderOperatorSection(state *OperatorState, selection int, isActive bool, width, maxLines int) string {
	if state == nil || len(state.Subsystems) == 0 {
		return mutedStyle.Render("  Loading...")
	}

	var lines []string

	for i, sub := range state.Subsystems {
		if len(lines) >= maxLines {
			remaining := len(state.Subsystems) - i
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("  ... and %d more", remaining)))
			break
		}

		badge := sub.Status.Badge()
		label := sub.Name
		if len(label) > width-8 {
			label = label[:width-11] + "..."
		}

		line := fmt.Sprintf("  %s %s", badge, label)
		if isActive && i == selection {
			lines = append(lines, selectedItemStyle.Render("> "+line[2:]))
		} else {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// RenderOperatorDetails renders the details panel for the operator console.
func RenderOperatorDetails(state *OperatorState, selection int, width int) string {
	var lines []string

	lines = append(lines, headerStyle.Render("Operator Console"))
	lines = append(lines, mutedStyle.Render("Subsystem health and recommended actions"))
	lines = append(lines, "")

	if state == nil || len(state.Subsystems) == 0 {
		lines = append(lines, mutedStyle.Render("No subsystem data available"))
		lines = append(lines, "")
		lines = append(lines, mutedStyle.Render("Press 'r' to refresh"))
		return strings.Join(lines, "\n")
	}

	// Summary
	lines = append(lines, headerStyle.Render("Summary"))
	summaryStyle := healthyStyle
	summaryText := "All systems healthy"
	if state.IssueCount > 0 {
		summaryStyle = statusErrorStyle
		summaryText = fmt.Sprintf("%d errors, %d warnings", state.IssueCount, state.WarningCount)
	} else if state.WarningCount > 0 {
		summaryStyle = warningStyle
		summaryText = fmt.Sprintf("%d warnings", state.WarningCount)
	}
	lines = append(lines, summaryStyle.Render(summaryText))
	lines = append(lines, "")

	// Selected subsystem details
	if selection >= 0 && selection < len(state.Subsystems) {
		sub := state.Subsystems[selection]

		lines = append(lines, headerStyle.Render("Selected: "+sub.Name))
		lines = append(lines, "")

		// Status with badge
		lines = append(lines, fmt.Sprintf("Status:  %s %s", sub.Status.Badge(), sub.Status.String()))
		lines = append(lines, fmt.Sprintf("Message: %s", sub.Message))
		lines = append(lines, "")

		// Details
		if sub.Details != "" {
			lines = append(lines, headerStyle.Render("Details"))
			// Wrap long details
			details := sub.Details
			for len(details) > width-4 {
				lines = append(lines, "  "+details[:width-4])
				details = details[width-4:]
			}
			if details != "" {
				lines = append(lines, "  "+details)
			}
			lines = append(lines, "")
		}

		// Recommended action
		if sub.Action != "" && sub.Status != SubsystemHealthy {
			lines = append(lines, headerStyle.Render("Recommended Action"))
			actionStyle := operatorActionStyle
			if sub.Status == SubsystemError {
				actionStyle = operatorUrgentActionStyle
			}
			lines = append(lines, actionStyle.Render("→ "+sub.Action))
			lines = append(lines, "")
		}
	}

	// Quick actions hint
	lines = append(lines, mutedStyle.Render("Press 'r' to refresh"))

	return strings.Join(lines, "\n")
}

// RenderOperatorEmptyState renders the empty/healthy state for operator section.
func RenderOperatorEmptyState(state *OperatorState, isActive bool) string {
	var lines []string

	if state == nil || !state.HasIssues {
		lines = append(lines, healthyStyle.Render("  ✓ All systems healthy"))
		if isActive {
			lines = append(lines, mutedStyle.Render("  Press 'r' to refresh"))
		}
	} else {
		lines = append(lines, mutedStyle.Render("  (loading...)"))
	}

	return strings.Join(lines, "\n")
}

// Operator-specific styles
var (
	operatorHealthyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00"))

	operatorWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFCC00")).
				Bold(true)

	operatorErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6666")).
				Bold(true)

	operatorUnknownStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	operatorActionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00BFFF"))

	operatorUrgentActionStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FF6666")).
					Bold(true)
)
