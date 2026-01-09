package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/andyrewlee/perch/data"
	"github.com/charmbracelet/lipgloss"
)

// ActivityType represents the type of activity event.
type ActivityType string

const (
	ActivityMRReady    ActivityType = "mr_ready"
	ActivityMRMerged   ActivityType = "mr_merged"
	ActivityMRConflict ActivityType = "mr_conflict"
	ActivityHook       ActivityType = "hook"
	ActivityPatrol     ActivityType = "patrol"
	ActivityDone       ActivityType = "done"
	ActivityCrash      ActivityType = "crash"
)

// ActivityEvent represents a single activity event in the feed.
type ActivityEvent struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      ActivityType `json:"type"`
	Source    string       `json:"source"`    // Agent or rig name
	Summary   string       `json:"summary"`   // Brief description
	Details   string       `json:"details"`   // Additional details (optional)
	ID        string       `json:"id"`        // Related ID (MR, issue, agent)
}

// activityState holds the activity feed state.
type activityState struct {
	events    []ActivityEvent
	maxEvents int
	lastBuilt time.Time
}

// buildActivityFeed aggregates events from multiple sources into the activity feed.
func buildActivityFeed(snap *data.Snapshot, prevState *activityState) *activityState {
	if snap == nil {
		if prevState != nil {
			return prevState
		}
		return &activityState{maxEvents: 50}
	}

	state := &activityState{
		maxEvents: 50,
	}

	var events []ActivityEvent

	// Add MR events (ready, merged, conflicts)
	for rig, mrs := range snap.MergeQueues {
		for _, mr := range mrs {
			switch mr.Status {
			case "ready", "queued":
				events = append(events, ActivityEvent{
					Timestamp: time.Now(), // MRs don't have timestamps, use now
					Type:      ActivityMRReady,
					Source:    rig,
					Summary:   fmt.Sprintf("%s: %s", mr.Worker, mr.Title),
					Details:   fmt.Sprintf("Branch: %s", mr.Branch),
					ID:        mr.ID,
				})
			case "merged":
				events = append(events, ActivityEvent{
					Timestamp: time.Now(),
					Type:      ActivityMRMerged,
					Source:    rig,
					Summary:   fmt.Sprintf("%s: %s", mr.Worker, mr.Title),
					Details:   "Merged to main",
					ID:        mr.ID,
				})
			}
			if mr.HasConflicts || mr.NeedsRebase {
				events = append(events, ActivityEvent{
					Timestamp: time.Now(),
					Type:      ActivityMRConflict,
					Source:    rig,
					Summary:   fmt.Sprintf("%s: %s", mr.Worker, mr.Title),
					Details:   mr.ConflictInfo,
					ID:        mr.ID,
				})
			}
		}
	}

	// Add hook events from agents
	if snap.Town != nil {
		for _, agent := range snap.Town.Agents {
			if agent.HookedBeadID != "" && agent.HasWork {
				// Check if this is a new hook (compare with previous state)
				isNew := true
				if prevState != nil {
					for _, e := range prevState.events {
						if e.Type == ActivityHook && e.Source == agent.Address && e.ID == agent.HookedBeadID {
							// Found existing hook event, check timestamp
							if time.Since(e.Timestamp) < 5*time.Minute {
								isNew = false
								break
							}
						}
					}
				}
				if isNew {
					events = append(events, ActivityEvent{
						Timestamp: agent.HookedAt,
						Type:      ActivityHook,
						Source:    agent.Address,
						Summary:   fmt.Sprintf("Hooked: %s", agent.HookedBeadID),
						Details:   fmt.Sprintf("Role: %s", agent.Role),
						ID:        agent.HookedBeadID,
					})
				}
			}
		}
	}

	// Add lifecycle events (done, crash, patrol alerts)
	if snap.Lifecycle != nil {
		for _, e := range snap.Lifecycle.Events {
			switch e.EventType {
			case data.EventDone:
				events = append(events, ActivityEvent{
					Timestamp: e.Timestamp,
					Type:      ActivityDone,
					Source:    e.Agent,
					Summary:   "Work completed",
					Details:   e.Message,
					ID:        e.Agent,
				})
			case data.EventCrash:
				events = append(events, ActivityEvent{
					Timestamp: e.Timestamp,
					Type:      ActivityCrash,
					Source:    e.Agent,
					Summary:   "Agent crashed",
					Details:   e.Message,
					ID:        e.Agent,
				})
			case data.EventNudge:
				events = append(events, ActivityEvent{
					Timestamp: e.Timestamp,
					Type:      ActivityPatrol,
					Source:    "deacon",
					Summary:   fmt.Sprintf("Nudge: %s", e.Agent),
					Details:   e.Message,
					ID:        e.Agent,
				})
			}
		}
	}

	// Sort by timestamp (newest first) and limit
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].Timestamp.After(events[i].Timestamp) {
				events[i], events[j] = events[j], events[i]
			}
		}
	}

	if len(events) > state.maxEvents {
		events = events[:state.maxEvents]
	}

	state.events = events
	state.lastBuilt = time.Now()
	return state
}

// Activity styles
var (
	activityTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("242")) // Gray
	activityHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("226")) // Yellow
	activityTimeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")) // Dim gray
	activityMRReadyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))  // Cyan
	activityMRMergedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("142")) // Green
	activityMRConflictStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // Red/pink
	activityHookStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("228")) // Yellow
	activityPatrolStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("213")) // Pink
	activityDoneStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("142")) // Green
	activityCrashStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // Red
)

// formatActivityTime formats a timestamp for display in the activity feed.
func formatActivityTime(t time.Time) string {
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
	return t.Format("Jan 2")
}

// typeIcon returns an icon for the activity type.
func typeIcon(t ActivityType) string {
	switch t {
	case ActivityMRReady:
		return "⏳"
	case ActivityMRMerged:
		return "✓"
	case ActivityMRConflict:
		return "⚠"
	case ActivityHook:
		return "📌"
	case ActivityPatrol:
		return "👀"
	case ActivityDone:
		return "✓"
	case ActivityCrash:
		return "💥"
	default:
		return "•"
	}
}

// typeStyle returns the style for the activity type.
func typeStyle(t ActivityType) lipgloss.Style {
	switch t {
	case ActivityMRReady:
		return activityMRReadyStyle
	case ActivityMRMerged, ActivityDone:
		return activityMRMergedStyle
	case ActivityMRConflict, ActivityCrash:
		return activityMRConflictStyle
	case ActivityHook:
		return activityHookStyle
	case ActivityPatrol:
		return activityPatrolStyle
	default:
		return lipgloss.NewStyle()
	}
}

// RenderActivityFeed renders the activity feed panel.
func RenderActivityFeed(state *activityState, width, height int, focused bool) string {
	// Account for border
	innerWidth := width - 2
	innerHeight := height - 2

	if innerWidth < 10 {
		innerWidth = 10
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	// Build content
	var content strings.Builder

	// Header
	title := activityTitleStyle.Render("Activity Feed")
	content.WriteString(title)
	content.WriteString("\n")

	if state == nil || len(state.events) == 0 {
		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("No activity yet"))
	} else {
		// Render events
		for i, e := range state.events {
			if i >= innerHeight-1 {
				break
			}

			// Time and icon
			timeStr := activityTimeStyle.Render(formatActivityTime(e.Timestamp))
			icon := typeStyle(e.Type).Render(typeIcon(e.Type))

			// Build summary line (truncate if needed)
			summary := e.Summary
			maxSummaryLen := innerWidth - 8 // space for time and icon
			if len(summary) > maxSummaryLen {
				summary = summary[:maxSummaryLen-3] + "..."
			}

			line := fmt.Sprintf("%s %s %s", timeStr, icon, summary)
			content.WriteString(line)

			// Add details on next line if space allows
			if e.Details != "" && i < innerHeight-2 {
				details := e.Details
				maxDetailsLen := innerWidth - 4
				if len(details) > maxDetailsLen {
					details = details[:maxDetailsLen-3] + "..."
				}
				content.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("  "+details))
			}

			if i < len(state.events)-1 && i < innerHeight-2 {
				content.WriteString("\n")
			}
		}
	}

	// Pad to fill height
	lines := strings.Split(content.String(), "\n")
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}

	inner := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(innerWidth).
		Height(innerHeight)

	if focused {
		style = style.BorderForeground(highlight)
	}

	return style.Render(inner)
}
