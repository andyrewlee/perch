package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/andyrewlee/perch/data"
	"github.com/charmbracelet/lipgloss"
)

// TownMapView represents an interactive town map with rig tiles.
type TownMapView struct {
	rigs      []RigTile
	selected  int    // Index of currently selected rig
	cursorRow int    // Current row for keyboard navigation
	cursorCol int    // Current column for keyboard navigation
	cols      int    // Number of columns in grid
	width     int    // Total width available
	height    int    // Total height available
}

// RigTile represents a single rig's tile with live stats.
type RigTile struct {
	Name         string
	AgentCount   int
	WorkingCount int
	IdleCount    int
	AlertCount   int
	HookedCount  int
	QueueMRCount int
	QueueBlocked int // MRs with conflicts
	LastActivity time.Time
	Health       string // "healthy", "warning", "error"
}

// NewTownMapView creates a new town map view from snapshot data.
func NewTownMapView(snap *data.Snapshot, width, height int) *TownMapView {
	view := &TownMapView{
		width:  width,
		height: height,
	}

	if snap == nil || snap.Town == nil {
		return view
	}

	// Build rig tiles from snapshot data
	for _, rig := range snap.Town.Rigs {
		tile := RigTile{
			Name:        rig.Name,
			AgentCount:  len(rig.Agents),
			HookedCount: rig.ActiveHooks,
		}

		// Count agent statuses
		for _, agent := range rig.Agents {
			if agent.Running {
				if agent.HasWork {
					tile.WorkingCount++
				} else {
					tile.IdleCount++
				}
			}
			if agent.UnreadMail > 0 {
				tile.AlertCount++
			}
		}

		// Get merge queue stats
		if mrs, ok := snap.MergeQueues[rig.Name]; ok {
			tile.QueueMRCount = len(mrs)
			for _, mr := range mrs {
				if mr.HasConflicts || mr.NeedsRebase {
					tile.QueueBlocked++
				}
			}
		}

		// Determine health
		if tile.QueueBlocked > 0 || tile.AlertCount > 3 {
			tile.Health = "error"
		} else if tile.QueueBlocked > 0 || tile.AlertCount > 0 || tile.WorkingCount == 0 && tile.AgentCount > 0 {
			tile.Health = "warning"
		} else {
			tile.Health = "healthy"
		}

		view.rigs = append(view.rigs, tile)
	}

	// Sort rigs by name for deterministic layout
	sort.Slice(view.rigs, func(i, j int) bool {
		return view.rigs[i].Name < view.rigs[j].Name
	})

	// Calculate grid dimensions
	tileWidth := 28 // Each tile is approximately 28 chars wide
	view.cols = imax(1, width/tileWidth)

	return view
}

// Render renders the town map view.
func (v *TownMapView) Render() string {
	if len(v.rigs) == 0 {
		return v.renderEmpty()
	}

	// Calculate tile dimensions
	tileWidth := imin(28, v.width/imax(1, v.cols)-2)
	tileHeight := 8

	// Build grid rows
	var rows []string
	for i := 0; i < len(v.rigs); i += v.cols {
		end := imin(i+v.cols, len(v.rigs))
		rowRigs := v.rigs[i:end]
		rowTiles := make([]string, len(rowRigs))

		for j, rig := range rowRigs {
			globalIdx := i + j
			isSelected := globalIdx == v.selected
			rowTiles[j] = v.renderTile(rig, tileWidth, tileHeight, isSelected)
		}

		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowTiles...))
	}

	// Add legend
	legend := v.renderLegend()
	rows = append(rows, legend)

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderEmpty renders the empty state when no rigs exist.
func (v *TownMapView) renderEmpty() string {
	return mutedStyle.Render("No rigs available in town map")
}

// highlightStyle is a style using the highlight color.
var highlightStyle = lipgloss.NewStyle().Foreground(highlight).Bold(true)

// okStyle indicates healthy/ok status.
var okStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))

// renderTile renders a single rig tile.
func (v *TownMapView) renderTile(rig RigTile, width, height int, isSelected bool) string {
	var lines []string

	// Header with rig name
	headerStyle := townMapHeaderStyle
	if isSelected {
		headerStyle = townMapHeaderSelectedStyle
	}
	lines = append(lines, headerStyle.Render(truncate(rig.Name, width-2)))

	// Agent summary
	agentLine := fmt.Sprintf("Agents: %d", rig.AgentCount)
	if rig.WorkingCount > 0 {
		agentLine += workingStyle.Render(fmt.Sprintf(" ●%d", rig.WorkingCount))
	}
	if rig.IdleCount > 0 {
		agentLine += idleStyle.Render(fmt.Sprintf(" ○%d", rig.IdleCount))
	}
	lines = append(lines, agentLine)

	// Hooks
	if rig.HookedCount > 0 {
		lines = append(lines, fmt.Sprintf("Hooks: %s", highlightStyle.Render(fmt.Sprintf("%d active", rig.HookedCount))))
	} else {
		lines = append(lines, mutedStyle.Render("Hooks: none"))
	}

	// Merge queue
	if rig.QueueMRCount > 0 {
		queueLine := fmt.Sprintf("Queue: %d MR", rig.QueueMRCount)
		if rig.QueueBlocked > 0 {
			queueLine += " " + conflictStyle.Render(fmt.Sprintf("!%d blocked", rig.QueueBlocked))
		}
		lines = append(lines, queueLine)
	} else {
		lines = append(lines, mutedStyle.Render("Queue: empty"))
	}

	// Alerts
	if rig.AlertCount > 0 {
		lines = append(lines, attentionStyle.Render(fmt.Sprintf("! %d alert(s)", rig.AlertCount)))
	} else if rig.Health == "healthy" {
		lines = append(lines, okStyle.Render("✓ Healthy"))
	} else {
		lines = append(lines, mutedStyle.Render("-"))
	}

	// Selection indicator
	if isSelected {
		lines = append(lines, selectedIndicatorStyle.Render("Press Enter for details"))
	} else {
		lines = append(lines, "")
	}

	// Apply border style
	borderStyle := townMapBorderStyle
	if isSelected {
		borderStyle = townMapBorderSelectedStyle
	}

	content := strings.Join(lines, "\n")
	return borderStyle.Width(width).Height(height).Render(content)
}

// renderLegend renders the legend below the grid.
func (v *TownMapView) renderLegend() string {
	items := []string{
		"j/k or ↑/↓: move focus",
		"Enter: rig details",
		"Esc: back to overview",
	}
	return legendStyle.Render(strings.Join(items, " | "))
}

// MoveSelection moves the selection in the given direction.
// Returns true if selection changed.
func (v *TownMapView) MoveSelection(dir string) bool {
	if len(v.rigs) == 0 {
		return false
	}

	oldSelected := v.selected

	switch dir {
	case "up", "k":
		v.selected = imax(0, v.selected-v.cols)
	case "down", "j":
		v.selected = imin(len(v.rigs)-1, v.selected+v.cols)
	case "left", "h":
		if v.selected > 0 {
			v.selected--
		}
	case "right", "l":
		if v.selected < len(v.rigs)-1 {
			v.selected++
		}
	}

	return v.selected != oldSelected
}

// SelectedRig returns the name of the currently selected rig.
func (v *TownMapView) SelectedRig() string {
	if v.selected < 0 || v.selected >= len(v.rigs) {
		return ""
	}
	return v.rigs[v.selected].Name
}

// Styles for town map rendering (prefixed with townMap to avoid conflicts).
var (
	townMapBorderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1).
			MarginRight(1)

	townMapBorderSelectedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(highlight).
				Padding(0, 1).
				MarginRight(1)

	townMapHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text)

	townMapHeaderSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(highlight)

	selectedIndicatorStyle = lipgloss.NewStyle().
				Foreground(highlight).
				Italic(true)
)

// Helper functions for min/max (prefixed to avoid conflicts).
func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func imax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
