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
	StatusIdle AgentStatus = iota
	StatusActive
	StatusError
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

// agentSymbol returns the colored symbol for an agent
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

	// Status color
	var style lipgloss.Style
	switch agent.Status {
	case StatusActive:
		style = activeStyle
	case StatusError:
		style = errorStyle
	default:
		style = idleStyle
	}

	return style.Render(typeChar)
}

// renderLegend shows the symbol meanings
func (r *OverviewRenderer) renderLegend() string {
	legend := []string{
		activeStyle.Render("P") + "=polecat",
		activeStyle.Render("W") + "=witness",
		activeStyle.Render("R") + "=refinery",
		"|",
		activeStyle.Render("*") + "=active",
		idleStyle.Render("*") + "=idle",
		errorStyle.Render("*") + "=error",
	}

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

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	idleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

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
					{Name: "furiosa", Type: AgentPolecat, Status: StatusActive},
					{Name: "nux", Type: AgentPolecat, Status: StatusActive},
					{Name: "slit", Type: AgentPolecat, Status: StatusIdle},
					{Name: "witness", Type: AgentWitness, Status: StatusActive},
					{Name: "refinery", Type: AgentRefinery, Status: StatusIdle},
				},
			},
			{
				Name: "gastown",
				Agents: []Agent{
					{Name: "immortan", Type: AgentPolecat, Status: StatusActive},
					{Name: "witness", Type: AgentWitness, Status: StatusActive},
					{Name: "refinery", Type: AgentRefinery, Status: StatusActive},
				},
			},
			{
				Name: "citadel",
				Agents: []Agent{
					{Name: "warboy", Type: AgentPolecat, Status: StatusError},
					{Name: "witness", Type: AgentWitness, Status: StatusIdle},
					{Name: "refinery", Type: AgentRefinery, Status: StatusIdle},
				},
			},
		},
	}
}
