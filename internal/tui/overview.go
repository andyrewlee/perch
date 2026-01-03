package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// AgentType represents the type of agent
type AgentType int

const (
	AgentPolecat AgentType = iota
	AgentWitness
	AgentRefinery
)

// AgentStatus represents agent health status
type AgentStatus int

const (
	StatusStopped   AgentStatus = iota // Not running
	StatusIdle                         // Running, no work hooked
	StatusWorking                      // Running, actively working
	StatusAttention                    // Has unread mail, may need attention
)

// Agent represents a worker in a rig
type Agent struct {
	Name   string
	Type   AgentType
	Status AgentStatus
}

// Rig represents a project container
type Rig struct {
	Name   string
	Agents []Agent
}

// Town represents the entire Gas Town workspace
type Town struct {
	Rigs []Rig
}

// OverviewRenderer handles rendering the town map
type OverviewRenderer struct {
	town Town
}

// NewOverviewRenderer creates a new renderer with the given town data
func NewOverviewRenderer(town Town) *OverviewRenderer {
	return &OverviewRenderer{town: town}
}

// Render generates the town overview map
func (r *OverviewRenderer) Render(width, height int) string {
	if len(r.town.Rigs) == 0 {
		return r.renderEmpty(width, height)
	}

	// Sort rigs deterministically by name
	rigs := make([]Rig, len(r.town.Rigs))
	copy(rigs, r.town.Rigs)
	sort.Slice(rigs, func(i, j int) bool {
		return rigs[i].Name < rigs[j].Name
	})

	// Calculate layout
	rigBoxes := r.renderRigBoxes(rigs, width, height)
	legend := r.renderLegend()

	// Combine map and legend
	return lipgloss.JoinVertical(lipgloss.Left, rigBoxes, legend)
}

// renderEmpty shows placeholder when no rigs exist
func (r *OverviewRenderer) renderEmpty(width, height int) string {
	msg := "No rigs detected"
	return mutedStyle.Render(msg)
}

// renderRigBoxes creates the visual grid of rig clusters
func (r *OverviewRenderer) renderRigBoxes(rigs []Rig, width, height int) string {
	if len(rigs) == 0 {
		return ""
	}

	// Reserve height for legend (2 lines)
	mapHeight := height - 3
	if mapHeight < 3 {
		mapHeight = 3
	}

	// Calculate grid dimensions
	// Each rig box is approximately 20 chars wide, 5 lines tall
	boxWidth := 22
	boxHeight := 5
	rigsPerRow := width / boxWidth
	if rigsPerRow < 1 {
		rigsPerRow = 1
	}

	var rows []string
	for i := 0; i < len(rigs); i += rigsPerRow {
		end := i + rigsPerRow
		if end > len(rigs) {
			end = len(rigs)
		}

		rowRigs := rigs[i:end]
		rowBoxes := make([]string, len(rowRigs))
		for j, rig := range rowRigs {
			rowBoxes[j] = r.renderRigBox(rig, boxWidth-2, boxHeight)
		}

		row := lipgloss.JoinHorizontal(lipgloss.Top, rowBoxes...)
		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderRigBox creates a single rig cluster visualization
func (r *OverviewRenderer) renderRigBox(rig Rig, width, height int) string {
	// Rig header
	header := rigHeaderStyle.Render(truncate(rig.Name, width-2))

	// Render agents
	agentLine := r.renderAgents(rig.Agents, width-2)

	// Build box content
	content := lipgloss.JoinVertical(lipgloss.Left, header, agentLine)

	// Apply box style
	style := rigBoxStyle.
		Width(width).
		Height(height)

	return style.Render(content)
}

// renderAgents creates a compact line of agent symbols
func (r *OverviewRenderer) renderAgents(agents []Agent, maxWidth int) string {
	if len(agents) == 0 {
		return mutedStyle.Render("(empty)")
	}

	// Sort agents deterministically: witnesses first, then refineries, then polecats
	sorted := make([]Agent, len(agents))
	copy(sorted, agents)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Type != sorted[j].Type {
			return sorted[i].Type > sorted[j].Type // W > R > P
		}
		return sorted[i].Name < sorted[j].Name
	})

	var parts []string
	for _, agent := range sorted {
		symbol := r.agentSymbol(agent)
		parts = append(parts, symbol)
	}

	line := strings.Join(parts, " ")

	// Truncate if too long
	if len(line) > maxWidth {
		line = line[:maxWidth-3] + "..."
	}

	return line
}

// agentSymbol returns the colored symbol for an agent with status badge
func (r *OverviewRenderer) agentSymbol(agent Agent) string {
	// Type symbol
	var typeChar string
	switch agent.Type {
	case AgentWitness:
		typeChar = "W"
	case AgentRefinery:
		typeChar = "R"
	case AgentPolecat:
		typeChar = "P"
	}

	// Status badge and color
	var style lipgloss.Style
	var badge string
	switch agent.Status {
	case StatusWorking:
		style = workingStyle
		badge = "●" // Working indicator
	case StatusAttention:
		style = attentionStyle
		badge = "!" // Needs attention
	case StatusIdle:
		style = idleStyle
		badge = "○" // Idle/ready
	case StatusStopped:
		style = stoppedStyle
		badge = "◌" // Stopped
	default:
		style = stoppedStyle
		badge = "◌"
	}

	return style.Render(typeChar + badge)
}

// renderLegend shows the symbol meanings with clear status badges
func (r *OverviewRenderer) renderLegend() string {
	// Agent types
	types := []string{
		idleStyle.Render("P") + "=polecat",
		idleStyle.Render("W") + "=witness",
		idleStyle.Render("R") + "=refinery",
	}

	// Status badges
	statuses := []string{
		workingStyle.Render("●") + "=working",
		idleStyle.Render("○") + "=idle",
		attentionStyle.Render("!") + "=attention",
		stoppedStyle.Render("◌") + "=stopped",
	}

	legend := append(types, "|")
	legend = append(legend, statuses...)

	return legendStyle.Render(strings.Join(legend, "  "))
}

// truncate shortens a string to fit within maxLen
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Styles for overview rendering
var (
	rigBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1).
			MarginRight(1)

	rigHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text)

	// Status styles for agents
	workingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")) // Green - actively working

	idleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")) // Grey - idle but running

	attentionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFCC00")) // Yellow - needs attention

	stoppedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")) // Dim grey - stopped

	legendStyle = lipgloss.NewStyle().
			Foreground(muted).
			MarginTop(1)
)

// MockTown creates sample data for testing
func MockTown() Town {
	return Town{
		Rigs: []Rig{
			{
				Name: "perch",
				Agents: []Agent{
					{Name: "furiosa", Type: AgentPolecat, Status: StatusWorking},
					{Name: "nux", Type: AgentPolecat, Status: StatusWorking},
					{Name: "slit", Type: AgentPolecat, Status: StatusIdle},
					{Name: "witness", Type: AgentWitness, Status: StatusWorking},
					{Name: "refinery", Type: AgentRefinery, Status: StatusIdle},
				},
			},
			{
				Name: "gastown",
				Agents: []Agent{
					{Name: "immortan", Type: AgentPolecat, Status: StatusWorking},
					{Name: "witness", Type: AgentWitness, Status: StatusWorking},
					{Name: "refinery", Type: AgentRefinery, Status: StatusWorking},
				},
			},
			{
				Name: "citadel",
				Agents: []Agent{
					{Name: "warboy", Type: AgentPolecat, Status: StatusAttention},
					{Name: "witness", Type: AgentWitness, Status: StatusStopped},
					{Name: "refinery", Type: AgentRefinery, Status: StatusStopped},
				},
			},
		},
	}
}
