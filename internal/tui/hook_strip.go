package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/andyrewlee/perch/data"
	"github.com/charmbracelet/lipgloss"
)

// HookEntry represents a single hook entry in the strip.
type HookEntry struct {
	Agent      string    `json:"agent"`
	BeadID     string    `json:"bead_id"`
	Status     string    `json:"status"`     // "active" or "completed"
	Timestamp  time.Time `json:"timestamp"`
	Role       string    `json:"role"`
	HasWork    bool      `json:"has_work"`
	IsStale    bool      `json:"is_stale"`
}

// hookActivityStripState holds the hook activity strip state.
type hookActivityStripState struct {
	activeHooks      []HookEntry
	completedHooks   []HookEntry
	lastBuilt        time.Time
	maxActiveToShow  int
	maxCompletedToShow int
}

// buildHookActivityStrip aggregates hook data from the snapshot.
func buildHookActivityStrip(snap *data.Snapshot, prevState *hookActivityStripState) *hookActivityStripState {
	if snap == nil || snap.Town == nil {
		if prevState != nil {
			return prevState
		}
		return &hookActivityStripState{
			maxActiveToShow:   10,
			maxCompletedToShow: 5,
		}
	}

	state := &hookActivityStripState{
		maxActiveToShow:    10,
		maxCompletedToShow: 5,
	}

	var activeHooks []HookEntry
	var completedHooks []HookEntry

	// Build active hooks from agents with work
	for _, agent := range snap.Town.Agents {
		if agent.HookedBeadID != "" && agent.HasWork {
			// Check if hook is stale (no activity for 2+ hours)
			isStale := !agent.HookedAt.IsZero() && time.Since(agent.HookedAt) > 2*time.Hour

			activeHooks = append(activeHooks, HookEntry{
				Agent:     agent.Name,
				BeadID:    agent.HookedBeadID,
				Status:    "active",
				Timestamp: agent.HookedAt,
				Role:      agent.Role,
				HasWork:   agent.HasWork,
				IsStale:   isStale,
			})
		}
	}

	// Add recently completed hooks from lifecycle events
	if snap.Lifecycle != nil {
		now := time.Now()
		for _, e := range snap.Lifecycle.Events {
			if e.EventType == data.EventDone {
				// Only show completions from last hour
				if now.Sub(e.Timestamp) < time.Hour {
					completedHooks = append(completedHooks, HookEntry{
						Agent:     extractAgentName(e.Agent),
						Status:    "completed",
						Timestamp: e.Timestamp,
					})
				}
			}
		}
	}

	// Sort active hooks by timestamp (oldest first - they've been waiting)
	for i := 0; i < len(activeHooks); i++ {
		for j := i + 1; j < len(activeHooks); j++ {
			if activeHooks[j].Timestamp.Before(activeHooks[i].Timestamp) {
				activeHooks[i], activeHooks[j] = activeHooks[j], activeHooks[i]
			}
		}
	}

	// Sort completed hooks by timestamp (newest first)
	for i := 0; i < len(completedHooks); i++ {
		for j := i + 1; j < len(completedHooks); j++ {
			if completedHooks[j].Timestamp.After(completedHooks[i].Timestamp) {
				completedHooks[i], completedHooks[j] = completedHooks[j], completedHooks[i]
			}
		}
	}

	// Limit active hooks
	if len(activeHooks) > state.maxActiveToShow {
		activeHooks = activeHooks[:state.maxActiveToShow]
	}
	// Limit completed hooks
	if len(completedHooks) > state.maxCompletedToShow {
		completedHooks = completedHooks[:state.maxCompletedToShow]
	}

	state.activeHooks = activeHooks
	state.completedHooks = completedHooks
	state.lastBuilt = time.Now()
	return state
}

// extractAgentName extracts the short name from a full agent address.
func extractAgentName(address string) string {
	parts := strings.Split(address, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return address
}

// Hook strip styles
var (
	hookStripTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("242")) // Gray
	hookStripAgentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("228")) // Yellow
	hookStripBeadStyle  = lipgloss.NewStyle().
			Foreground(lipgloss.Color("159")) // Cyan
	hookStripAgeStyle   = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")) // Dim gray
	hookStripDoneStyle  = lipgloss.NewStyle().
			Foreground(lipgloss.Color("142")) // Green
	hookStripStaleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")) // Red
	hookStripEmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")) // Dim gray
)

// RenderHookActivityStrip renders the hook activity strip as a horizontal bar.
// Returns a string that can be embedded in the layout.
func RenderHookActivityStrip(state *hookActivityStripState, width int) string {
	if width < 20 {
		width = 20
	}

	// Inner width accounting for borders
	innerWidth := width - 4

	// Build content
	var content strings.Builder

	// Count active hooks
	activeCount := 0
	if state != nil {
		activeCount = len(state.activeHooks)
	}

	// Title
	title := hookStripTitleStyle.Render(fmt.Sprintf("⚓ Hooks (%d)", activeCount))
	content.WriteString(title)

	if state == nil || len(state.activeHooks) == 0 && len(state.completedHooks) == 0 {
		content.WriteString(" " + hookStripEmptyStyle.Render("No active hooks"))
	} else {
		// Build hook entries
		var entries []string

		// Add active hooks first
		for _, h := range state.activeHooks {
			age := formatHookAge(h.Timestamp)
			var entry strings.Builder

			// Agent name
			agentStyle := hookStripAgentStyle
			if h.IsStale {
				agentStyle = hookStripStaleStyle
			}
			entry.WriteString(agentStyle.Render(h.Agent))

			// Bead ID (truncated)
			beadID := h.BeadID
			if len(beadID) > 12 {
				beadID = beadID[:9] + "..."
			}
			entry.WriteString(":")
			entry.WriteString(hookStripBeadStyle.Render(beadID))

			// Age
			entry.WriteString(" ")
			entry.WriteString(hookStripAgeStyle.Render(age))

			entries = append(entries, entry.String())
		}

		// Add separator and completed hooks if any
		if len(state.completedHooks) > 0 {
			entries = append(entries, "✓")
			for _, h := range state.completedHooks {
				age := formatHookAge(h.Timestamp)
				entry := hookStripDoneStyle.Render(fmt.Sprintf("%s✓%s", h.Agent, age))
				entries = append(entries, entry)
			}
		}

		// Join entries with spacing
		if len(entries) > 0 {
			// Calculate available width for entries
			availableWidth := innerWidth - len(title) - 2
			if availableWidth < 20 {
				availableWidth = 20
			}

			// Build the line with entries
			line := strings.Join(entries, " ")
			if len(line) > availableWidth {
				// Truncate if too long
				line = line[:availableWidth-3] + "..."
			}
			content.WriteString(" " + line)
		}
	}

	// Create the strip as a single-line bordered box
	inner := content.String()

	// Pad to full width
	if len(inner) < innerWidth {
		inner = inner + strings.Repeat(" ", innerWidth-len(inner))
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(innerWidth).
		Height(1).
		BorderForeground(lipgloss.Color("240"))

	return style.Render(inner)
}

// formatHookAge formats a timestamp for the hook strip.
func formatHookAge(t time.Time) string {
	if t.IsZero() {
		return "?"
	}
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh", int(diff.Hours()))
	}
	return fmt.Sprintf("%dd", int(diff.Hours()/24))
}
