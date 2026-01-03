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
