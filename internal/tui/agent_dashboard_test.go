package tui

import (
	"testing"
	"time"

	"github.com/andyrewlee/perch/data"
)

// TestNewAgentDashboard tests creating an agent dashboard from a snapshot.
func TestNewAgentDashboard(t *testing.T) {
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Rigs: []data.Rig{
				{
					Name: "testrig",
					Agents: []data.Agent{
						{
							Name:       "test-polecat-1",
							Address:    "testrig/polecat-1",
							Session:    "session-1",
							Role:       "polecat",
							Running:    true,
							HasWork:    true,
							UnreadMail: 0,
							HookedAt:   time.Now().Add(-30 * time.Minute),
							HookedBeadID: "pe-test1",
						},
						{
							Name:       "test-witness",
							Address:    "testrig/witness",
							Session:    "session-2",
							Role:       "witness",
							Running:    true,
							HasWork:    false,
							UnreadMail: 0,
						},
						{
							Name:       "stopped-polecat",
							Address:    "testrig/polecat-2",
							Role:       "polecat",
							Running:    false,
							HasWork:    false,
							UnreadMail: 1,
						},
					},
				},
			},
		},
	}

	dash := NewAgentDashboard(snap)

	if dash == nil {
		t.Fatal("NewAgentDashboard returned nil")
	}

	// Check summary counts
	if dash.Summary.Total != 3 {
		t.Errorf("Expected 3 total agents, got %d", dash.Summary.Total)
	}

	if dash.Summary.Running != 2 {
		t.Errorf("Expected 2 running agents, got %d", dash.Summary.Running)
	}

	if dash.Summary.Working != 1 {
		t.Errorf("Expected 1 working agent, got %d", dash.Summary.Working)
	}

	if dash.Summary.Idle != 1 {
		t.Errorf("Expected 1 idle agent, got %d", dash.Summary.Idle)
	}

	if dash.Summary.Stopped != 1 {
		t.Errorf("Expected 1 stopped agent, got %d", dash.Summary.Stopped)
	}

	if dash.Summary.WithMail != 1 {
		t.Errorf("Expected 1 agent with mail, got %d", dash.Summary.WithMail)
	}

	// Check rig count
	if dash.Summary.ByRig["testrig"] != 3 {
		t.Errorf("Expected 3 agents in testrig, got %d", dash.Summary.ByRig["testrig"])
	}

	// Check role counts
	if dash.Summary.ByRole["polecat"] != 2 {
		t.Errorf("Expected 2 polecats, got %d", dash.Summary.ByRole["polecat"])
	}

	if dash.Summary.ByRole["witness"] != 1 {
		t.Errorf("Expected 1 witness, got %d", dash.Summary.ByRole["witness"])
	}
}

// TestNewAgentDashboardNilSnapshot tests creating an agent dashboard with a nil snapshot.
func TestNewAgentDashboardNilSnapshot(t *testing.T) {
	dash := NewAgentDashboard(nil)

	if dash == nil {
		t.Fatal("NewAgentDashboard returned nil")
	}

	if len(dash.Entries) != 0 {
		t.Errorf("Expected 0 entries for nil snapshot, got %d", len(dash.Entries))
	}

	if dash.Summary.Total != 0 {
		t.Errorf("Expected 0 total agents for nil snapshot, got %d", dash.Summary.Total)
	}
}

// TestNewAgentDashboardNoTown tests creating an agent dashboard with no town data.
func TestNewAgentDashboardNoTown(t *testing.T) {
	snap := &data.Snapshot{}
	dash := NewAgentDashboard(snap)

	if dash == nil {
		t.Fatal("NewAgentDashboard returned nil")
	}

	if len(dash.Entries) != 0 {
		t.Errorf("Expected 0 entries for no town data, got %d", len(dash.Entries))
	}
}

// TestAgentEntryRoleBadge tests the role badge rendering.
func TestAgentEntryRoleBadge(t *testing.T) {
	tests := []struct {
		role        string
		expectedLen  int // Badge should be non-empty
	}{
		{"polecat", 1},
		{"witness", 1},
		{"refinery", 1},
		{"deacon", 1},
		{"mayor", 1},
		{"unknown", 1},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			entry := AgentEntry{
				Agent: data.Agent{Role: tt.role},
			}
			badge := entry.RoleBadge()
			if len(badge) == 0 {
				t.Errorf("RoleBadge() returned empty string for role %s", tt.role)
			}
		})
	}
}

// TestAgentHealthStatusBadge tests the health status badge rendering.
func TestAgentHealthStatusBadge(t *testing.T) {
	tests := []struct {
		status     AgentHealthStatus
		expectedLen int // Badge should be non-empty
	}{
		{AgentHealthy, 1},
		{AgentIdle, 1},
		{AgentStopped, 1},
		{AgentStale, 1},
		{AgentError, 1},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			badge := tt.status.Badge()
			if len(badge) == 0 {
				t.Errorf("Badge() returned empty string for status %v", tt.status)
			}
		})
	}
}

// TestAgentDashboardRender tests rendering the agent dashboard.
func TestAgentDashboardRender(t *testing.T) {
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Rigs: []data.Rig{
				{
					Name: "testrig",
					Agents: []data.Agent{
						{
							Name:    "test-polecat",
							Address: "testrig/polecat",
							Role:    "polecat",
							Running: true,
							HasWork: true,
						},
					},
				},
			},
		},
	}

	dash := NewAgentDashboard(snap)
	rendered := dash.Render(80, 20)

	if len(rendered) == 0 {
		t.Error("Render() returned empty string")
	}

	// Check that key elements are present
	if !contains(rendered, "Agent Status Dashboard") {
		t.Error("Render() missing title")
	}

	if !contains(rendered, "agents") {
		t.Error("Render() missing agent count")
	}
}
