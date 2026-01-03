package tui

import (
	"strings"
	"testing"

	"github.com/andyrewlee/perch/data"
)

func TestRenderMQEmptyState_RefineryRunning(t *testing.T) {
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "refinery", Role: "refinery", Running: true},
			},
		},
	}

	result := renderMQEmptyState(snap, 40)

	if !strings.Contains(result, "Refinery idle") {
		t.Errorf("expected 'Refinery idle' in output, got: %s", result)
	}
	if !strings.Contains(result, "Queue clear") {
		t.Errorf("expected 'Queue clear' hint in output, got: %s", result)
	}
}

func TestRenderMQEmptyState_RefineryStopped(t *testing.T) {
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "refinery", Role: "refinery", Running: false},
			},
		},
	}

	result := renderMQEmptyState(snap, 40)

	if !strings.Contains(result, "Refinery stopped") {
		t.Errorf("expected 'Refinery stopped' in output, got: %s", result)
	}
	if !strings.Contains(result, "Queue clear") {
		t.Errorf("expected 'Queue clear' hint in output, got: %s", result)
	}
}

func TestRenderMQEmptyState_NoRefinery(t *testing.T) {
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "witness", Role: "witness", Running: true},
			},
		},
	}

	result := renderMQEmptyState(snap, 40)

	if !strings.Contains(result, "No refinery configured") {
		t.Errorf("expected 'No refinery configured' in output, got: %s", result)
	}
}

func TestRenderMQEmptyState_NilSnapshot(t *testing.T) {
	result := renderMQEmptyState(nil, 40)

	if !strings.Contains(result, "No refinery configured") {
		t.Errorf("expected 'No refinery configured' in output for nil snapshot, got: %s", result)
	}
	if !strings.Contains(result, "Queue clear") {
		t.Errorf("expected 'Queue clear' hint in output, got: %s", result)
	}
}

func TestRigItemLabel_ShowsPolecatsAndHooks(t *testing.T) {
	r := rigItem{
		r: data.Rig{
			Name:         "perch",
			PolecatCount: 12,
			Hooks: []data.Hook{
				{Agent: "polecat1", HasWork: true},
				{Agent: "polecat2", HasWork: false},
				{Agent: "polecat3", HasWork: false},
			},
		},
	}

	label := r.Label()

	// Should show polecat count with "pol" suffix
	if !strings.Contains(label, "12pol") {
		t.Errorf("expected label to contain '12pol', got: %s", label)
	}

	// Should show active/total hooks format (1 active of 3 total)
	if !strings.Contains(label, "1/3hk") {
		t.Errorf("expected label to contain '1/3hk' (active/total hooks), got: %s", label)
	}
}

func TestRigItemLabel_ZeroHooks(t *testing.T) {
	r := rigItem{
		r: data.Rig{
			Name:         "test",
			PolecatCount: 5,
			Hooks:        []data.Hook{},
		},
	}

	label := r.Label()

	// Should show 0/0 hooks when no hooks exist
	if !strings.Contains(label, "0/0hk") {
		t.Errorf("expected label to contain '0/0hk', got: %s", label)
	}
}
