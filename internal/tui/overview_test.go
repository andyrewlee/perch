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
