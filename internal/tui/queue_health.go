package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RefineryState represents the state of a refinery.
type RefineryState int

const (
	RefineryIdle RefineryState = iota
	RefineryProcessing
	RefineryStalled
)

// String returns the string representation of the RefineryState.
func (s RefineryState) String() string {
	switch s {
	case RefineryIdle:
		return "Idle"
	case RefineryProcessing:
		return "Processing"
	case RefineryStalled:
		return "Stalled"
	default:
		return "Unknown"
	}
}

// QueueMR represents a merge request in the queue with age info.
type QueueMR struct {
	ID        string
	Title     string
	Worker    string
	Status    string
	CreatedAt time.Time
	Age       time.Duration
}

// AgeBadge returns a colored badge based on MR age.
func (m QueueMR) AgeBadge() string {
	age := m.Age
	if age == 0 {
		age = time.Since(m.CreatedAt)
	}

	switch {
	case age < 10*time.Minute:
		return queueFreshStyle.Render("fresh")
	case age < 30*time.Minute:
		return queueOkStyle.Render("ok")
	case age < 1*time.Hour:
		return queueWaitingStyle.Render("waiting")
	default:
		return queueStaleStyle.Render("stale")
	}
}

// QueueHealth represents the health status of a rig's merge queue.
type QueueHealth struct {
	RigName       string
	State         RefineryState
	LastMergeTime time.Time
	MRs           []QueueMR
	RefineryAgent string // Agent address for actions
}

// TimeSinceLastMerge returns formatted duration since last merge.
func (q QueueHealth) TimeSinceLastMerge() string {
	if q.LastMergeTime.IsZero() {
		return "never"
	}
	return formatDuration(time.Since(q.LastMergeTime))
}

// Guidance returns the recommended action based on queue state.
func (q QueueHealth) Guidance() (text string, shouldNudge bool) {
	switch q.State {
	case RefineryProcessing:
		return "Refinery is actively processing. Wait for completion.", false
	case RefineryStalled:
		return "Refinery appears stalled. Consider nudging or restarting.", true
	default: // Idle
		if len(q.MRs) == 0 {
			return "Queue is empty. No action needed.", false
		}
		// Check oldest MR age
		for _, mr := range q.MRs {
			age := mr.Age
			if age == 0 {
				age = time.Since(mr.CreatedAt)
			}
			if age > 30*time.Minute {
				return "MRs waiting > 30min. Consider nudging refinery.", true
			}
		}
		return "Queue looks healthy. Work should process soon.", false
	}
}

// QueueHealthPanel renders the queue health information.
type QueueHealthPanel struct {
	health       QueueHealth
	selectedMR   int // Index of selected MR for actions
	showActions  bool
	width        int
	height       int
}

// NewQueueHealthPanel creates a new queue health panel.
func NewQueueHealthPanel(health QueueHealth) *QueueHealthPanel {
	return &QueueHealthPanel{health: health}
}

// Update updates the panel with new health data.
func (p *QueueHealthPanel) Update(health QueueHealth) {
	p.health = health
}

// Render renders the queue health panel.
func (p *QueueHealthPanel) Render(width, height int) string {
	p.width = width
	p.height = height

	if width < 10 || height < 5 {
		return "..."
	}

	var sections []string

	// Header with refinery state
	sections = append(sections, p.renderHeader())

	// Last merge time
	sections = append(sections, p.renderLastMerge())

	// Guidance banner
	sections = append(sections, p.renderGuidance())

	// MR list with age badges
	sections = append(sections, p.renderMRList(height-8)) // Reserve space for header/guidance

	// Actions hint
	sections = append(sections, p.renderActionsHint())

	return strings.Join(sections, "\n")
}

func (p *QueueHealthPanel) renderHeader() string {
	stateStyle := idleStyle
	switch p.health.State {
	case RefineryProcessing:
		stateStyle = workingStyle
	case RefineryStalled:
		stateStyle = stalledStyle
	}

	state := stateStyle.Render(p.health.State.String())
	return fmt.Sprintf("%s  Refinery: %s", queueTitleStyle.Render("Queue Health"), state)
}

func (p *QueueHealthPanel) renderLastMerge() string {
	label := mutedStyle.Render("Last merge: ")
	value := p.health.TimeSinceLastMerge()
	if value == "never" {
		value = mutedStyle.Render(value)
	} else {
		value = queueTimeStyle.Render(value + " ago")
	}
	return label + value
}

func (p *QueueHealthPanel) renderGuidance() string {
	text, shouldNudge := p.health.Guidance()
	if shouldNudge {
		return queueNudgeBannerStyle.Render("! " + text)
	}
	return queueOkBannerStyle.Render(text)
}

func (p *QueueHealthPanel) renderMRList(maxLines int) string {
	if len(p.health.MRs) == 0 {
		return mutedStyle.Render("\nNo merge requests in queue")
	}

	if maxLines < 1 {
		maxLines = 1
	}

	var lines []string
	lines = append(lines, queueSubtitleStyle.Render("\nMerge Requests:"))

	for i, mr := range p.health.MRs {
		if len(lines) >= maxLines {
			remaining := len(p.health.MRs) - i
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("  ... and %d more", remaining)))
			break
		}

		line := p.renderMRLine(mr, i == p.selectedMR)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (p *QueueHealthPanel) renderMRLine(mr QueueMR, selected bool) string {
	// Format: [status] title (worker) [age badge]
	status := p.formatStatus(mr.Status)
	title := truncate(mr.Title, p.width-35)
	worker := mutedStyle.Render(fmt.Sprintf("(%s)", mr.Worker))
	badge := mr.AgeBadge()

	line := fmt.Sprintf("  %s %s %s %s", status, title, worker, badge)

	if selected {
		return selectedItemStyle.Render("> " + line[2:])
	}
	return line
}

func (p *QueueHealthPanel) formatStatus(status string) string {
	switch status {
	case "pending":
		return queuePendingStyle.Render("[pending]")
	case "processing":
		return queueProcessingStyle.Render("[processing]")
	case "ready":
		return queueReadyStyle.Render("[ready]")
	case "failed":
		return queueFailedStyle.Render("[failed]")
	default:
		return mutedStyle.Render("[" + status + "]")
	}
}

func (p *QueueHealthPanel) renderActionsHint() string {
	return mutedStyle.Render("\nn: nudge refinery | x: restart refinery")
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// Styles for queue health panel
var (
	// Refinery state styles (use existing idleStyle, workingStyle from overview.go)
	stalledStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6666")).
			Bold(true)

	queueTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight)

	queueSubtitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(text)

	queueTimeStyle = lipgloss.NewStyle().
			Foreground(text)

	queueFreshStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	queueOkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88FF88"))

	queueWaitingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFCC00"))

	queueStaleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6666")).
			Bold(true)

	queueOkBannerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#88FF88")).
				Italic(true).
				MarginTop(1)

	queueNudgeBannerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFCC00")).
				Bold(true).
				MarginTop(1)

	queuePendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	queueProcessingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00AAFF"))

	queueReadyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	queueFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF0000"))
)
