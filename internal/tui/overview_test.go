package tui

import (
	"strings"
	"testing"
)

func TestOverviewRenderer_Render(t *testing.T) {
	town := MockTown()
	renderer := NewOverviewRenderer(town)

	output := renderer.Render(80, 20)

	// Check that all rig names appear
	for _, rig := range town.Rigs {
		if !strings.Contains(output, rig.Name) {
			t.Errorf("expected rig %q in output", rig.Name)
		}
	}

	// Check legend appears
	if !strings.Contains(output, "polecat") {
		t.Error("expected legend to contain 'polecat'")
	}
	if !strings.Contains(output, "witness") {
		t.Error("expected legend to contain 'witness'")
	}
	if !strings.Contains(output, "refinery") {
		t.Error("expected legend to contain 'refinery'")
	}
}

func TestOverviewRenderer_Deterministic(t *testing.T) {
	town := MockTown()
	renderer := NewOverviewRenderer(town)

	// Render multiple times and check output is identical
	output1 := renderer.Render(80, 20)
	output2 := renderer.Render(80, 20)
	output3 := renderer.Render(80, 20)

	if output1 != output2 || output2 != output3 {
		t.Error("expected deterministic output across renders")
	}
}

func TestOverviewRenderer_EmptyTown(t *testing.T) {
	town := Town{Rigs: []Rig{}}
	renderer := NewOverviewRenderer(town)

	output := renderer.Render(80, 20)

	if !strings.Contains(output, "No rigs") {
		t.Error("expected 'No rigs' message for empty town")
	}
}

func TestAgentSymbol(t *testing.T) {
	renderer := &OverviewRenderer{}

	tests := []struct {
		agent    Agent
		wantType string
	}{
		{Agent{Type: AgentPolecat}, "P"},
		{Agent{Type: AgentWitness}, "W"},
		{Agent{Type: AgentRefinery}, "R"},
	}

	for _, tt := range tests {
		symbol := renderer.agentSymbol(tt.agent)
		if !strings.Contains(symbol, tt.wantType) {
			t.Errorf("expected symbol to contain %q, got %q", tt.wantType, symbol)
		}
	}
}

func TestRigsSortedAlphabetically(t *testing.T) {
	town := Town{
		Rigs: []Rig{
			{Name: "zebra"},
			{Name: "alpha"},
			{Name: "mike"},
		},
	}
	renderer := NewOverviewRenderer(town)
	output := renderer.Render(100, 20)

	// Alpha should appear before mike, mike before zebra
	alphaIdx := strings.Index(output, "alpha")
	mikeIdx := strings.Index(output, "mike")
	zebraIdx := strings.Index(output, "zebra")

	if alphaIdx == -1 || mikeIdx == -1 || zebraIdx == -1 {
		t.Fatal("not all rig names found in output")
	}

	if alphaIdx > mikeIdx || mikeIdx > zebraIdx {
		t.Errorf("rigs not sorted alphabetically: alpha=%d, mike=%d, zebra=%d", alphaIdx, mikeIdx, zebraIdx)
	}
}

func TestRenderRigSummary(t *testing.T) {
	renderer := &OverviewRenderer{}

	tests := []struct {
		name        string
		agents      []Agent
		wantP       bool // expect polecat count
		wantW       bool // expect witness count
		wantR       bool // expect refinery count
		wantWorking bool // expect working badge
		wantIdle    bool // expect idle badge
		wantAttn    bool // expect attention badge
	}{
		{
			name: "mixed agents with statuses",
			agents: []Agent{
				{Type: AgentPolecat, Status: StatusWorking},
				{Type: AgentPolecat, Status: StatusWorking},
				{Type: AgentPolecat, Status: StatusIdle},
				{Type: AgentWitness, Status: StatusWorking},
				{Type: AgentRefinery, Status: StatusIdle},
			},
			wantP: true, wantW: true, wantR: true,
			wantWorking: true, wantIdle: true, wantAttn: false,
		},
		{
			name: "only polecats working",
			agents: []Agent{
				{Type: AgentPolecat, Status: StatusWorking},
				{Type: AgentPolecat, Status: StatusWorking},
			},
			wantP: true, wantW: false, wantR: false,
			wantWorking: true, wantIdle: false, wantAttn: false,
		},
		{
			name: "with attention status",
			agents: []Agent{
				{Type: AgentPolecat, Status: StatusAttention},
				{Type: AgentWitness, Status: StatusIdle},
			},
			wantP: true, wantW: true, wantR: false,
			wantWorking: false, wantIdle: true, wantAttn: true,
		},
		{
			name:   "empty agents",
			agents: []Agent{},
			wantP:  false, wantW: false, wantR: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := renderer.renderRigSummary(tt.agents, 50)

			if tt.wantP && !strings.Contains(summary, "P") {
				t.Error("expected polecat count in summary")
			}
			if tt.wantW && !strings.Contains(summary, "W") {
				t.Error("expected witness count in summary")
			}
			if tt.wantR && !strings.Contains(summary, "R") {
				t.Error("expected refinery count in summary")
			}
			if tt.wantWorking && !strings.Contains(summary, "●") {
				t.Error("expected working badge (●) in summary")
			}
			if tt.wantIdle && !strings.Contains(summary, "○") {
				t.Error("expected idle badge (○) in summary")
			}
			if tt.wantAttn && !strings.Contains(summary, "!") {
				t.Error("expected attention badge (!) in summary")
			}
		})
	}
}

func TestLegendContainsStatusSymbols(t *testing.T) {
	renderer := &OverviewRenderer{}
	legend := renderer.renderLegend()

	// Check new status symbols are in legend
	if !strings.Contains(legend, "●") {
		t.Error("expected working symbol (●) in legend")
	}
	if !strings.Contains(legend, "○") {
		t.Error("expected idle symbol (○) in legend")
	}
	if !strings.Contains(legend, "!") {
		t.Error("expected attention symbol (!) in legend")
	}
	if !strings.Contains(legend, "◌") {
		t.Error("expected stopped symbol (◌) in legend")
	}
}

// ========== Golden Render Tests ==========

func TestOverviewRenderer_GoldenFullTown(t *testing.T) {
	defer setupGoldenEnv()()

	town := MockTown()
	renderer := NewOverviewRenderer(town)

	output := renderer.Render(100, 40)
	CheckGolden(t, "overview_full_town", output, DefaultGoldenOptions())
}

func TestOverviewRenderer_GoldenEmptyTown(t *testing.T) {
	defer setupGoldenEnv()()

	town := Town{Rigs: []Rig{}}
	renderer := NewOverviewRenderer(town)

	output := renderer.Render(100, 40)
	CheckGolden(t, "overview_empty_town", output, DefaultGoldenOptions())
}

func TestOverviewRenderer_GoldenSingleRig(t *testing.T) {
	defer setupGoldenEnv()()

	town := Town{
		Rigs: []Rig{
			{
				Name: "perch",
				Agents: []Agent{
					{Name: "furiosa", Type: AgentPolecat, Status: StatusWorking},
					{Name: "nux", Type: AgentPolecat, Status: StatusIdle},
					{Name: "witness", Type: AgentWitness, Status: StatusWorking},
					{Name: "refinery", Type: AgentRefinery, Status: StatusIdle},
				},
			},
		},
	}
	renderer := NewOverviewRenderer(town)

	output := renderer.Render(100, 40)
	CheckGolden(t, "overview_single_rig", output, DefaultGoldenOptions())
}

func TestOverviewRenderer_GoldenAllStopped(t *testing.T) {
	defer setupGoldenEnv()()

	town := Town{
		Rigs: []Rig{
			{
				Name: "perch",
				Agents: []Agent{
					{Name: "furiosa", Type: AgentPolecat, Status: StatusStopped},
					{Name: "witness", Type: AgentWitness, Status: StatusStopped},
					{Name: "refinery", Type: AgentRefinery, Status: StatusStopped},
				},
			},
		},
	}
	renderer := NewOverviewRenderer(town)

	output := renderer.Render(100, 40)
	CheckGolden(t, "overview_all_stopped", output, DefaultGoldenOptions())
}

func TestOverviewRenderer_GoldenMixedStatus(t *testing.T) {
	defer setupGoldenEnv()()

	town := Town{
		Rigs: []Rig{
			{
				Name: "perch",
				Agents: []Agent{
					{Name: "furiosa", Type: AgentPolecat, Status: StatusWorking},
					{Name: "nux", Type: AgentPolecat, Status: StatusIdle},
					{Name: "slit", Type: AgentPolecat, Status: StatusAttention},
					{Name: "toast", Type: AgentPolecat, Status: StatusStopped},
				},
			},
		},
	}
	renderer := NewOverviewRenderer(town)

	output := renderer.Render(100, 40)
	CheckGolden(t, "overview_mixed_status", output, DefaultGoldenOptions())
}
