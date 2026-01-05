package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

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

	result := renderMQEmptyState(snap, nil, false, 40)

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

	result := renderMQEmptyState(snap, nil, false, 40)

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

	result := renderMQEmptyState(snap, nil, false, 40)

	if !strings.Contains(result, "Refinery not configured") {
		t.Errorf("expected 'Refinery not configured' in output, got: %s", result)
	}
}

func TestRenderMQEmptyState_NilSnapshot(t *testing.T) {
	result := renderMQEmptyState(nil, nil, false, 40)

	if !strings.Contains(result, "Refinery not configured") {
		t.Errorf("expected 'Refinery not configured' in output for nil snapshot, got: %s", result)
	}
	if !strings.Contains(result, "Queue clear") {
		t.Errorf("expected 'Queue clear' hint in output, got: %s", result)
	}
}

func TestRenderMQEmptyState_WithLastMergeTime(t *testing.T) {
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "refinery", Role: "refinery", Running: true},
			},
		},
	}
	opts := &SidebarOptions{
		LastMergeTime: time.Now().Add(-5 * time.Minute),
	}

	result := renderMQEmptyState(snap, opts, false, 40)

	if !strings.Contains(result, "Refinery idle") {
		t.Errorf("expected 'Refinery idle' in output, got: %s", result)
	}
	if !strings.Contains(result, "Last merge:") {
		t.Errorf("expected 'Last merge:' in output, got: %s", result)
	}
	if !strings.Contains(result, "ago") {
		t.Errorf("expected 'ago' in last merge time, got: %s", result)
	}
}

func TestRenderMQEmptyState_ActiveShowsHint(t *testing.T) {
	result := renderMQEmptyState(nil, nil, true, 40)

	if !strings.Contains(result, "gt done") {
		t.Errorf("expected 'gt done' hint when section is active, got: %s", result)
	}
}

func TestRenderMQEmptyState_InactiveNoHint(t *testing.T) {
	result := renderMQEmptyState(nil, nil, false, 40)

	if strings.Contains(result, "gt done") {
		t.Errorf("expected no 'gt done' hint when section is inactive, got: %s", result)
	}
}

func TestRenderMQEmptyState_ZeroLastMergeTime(t *testing.T) {
	opts := &SidebarOptions{
		LastMergeTime: time.Time{}, // Zero time
	}
	result := renderMQEmptyState(nil, opts, false, 40)

	// Should not show last merge time for zero time
	if strings.Contains(result, "Last merge:") {
		t.Errorf("expected no 'Last merge:' when time is zero, got: %s", result)
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

func TestAgentsPanelShowsLoadingState(t *testing.T) {
	state := NewSidebarState()
	// New state should start in loading mode
	if !state.AgentsLoading {
		t.Error("expected AgentsLoading to be true initially")
	}

	// Render with empty items while loading
	result := renderAgentsList(state, nil, nil, true, 40, 5)
	if !strings.Contains(result, "Loading") {
		t.Errorf("expected 'Loading' in output during loading state, got: %s", result)
	}
}

func TestAgentsPanelPreservesLastKnownAgents(t *testing.T) {
	state := NewSidebarState()

	// First successful refresh with agents
	snap1 := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "witness", Address: "perch/witness", Running: true},
				{Name: "refinery", Address: "perch/refinery", Running: true},
			},
		},
		LoadedAt: time.Now(),
	}
	state.UpdateFromSnapshot(snap1)

	// Verify agents were loaded
	if len(state.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(state.Agents))
	}
	if state.AgentsLoading {
		t.Error("expected AgentsLoading to be false after successful refresh")
	}

	// Second refresh with Town=nil (simulating failure)
	snap2 := &data.Snapshot{
		Town:     nil, // Town failed to load
		LoadedAt: time.Now(),
		LoadErrors: []data.LoadError{{
			Source:     "Town Status",
			Error:      "connection timeout",
			OccurredAt: time.Now(),
		}},
	}
	state.UpdateFromSnapshot(snap2)

	// Agents should be preserved (last-known value)
	if len(state.Agents) != 2 {
		t.Errorf("expected 2 agents (preserved), got %d", len(state.Agents))
	}
	// Error should be set
	if state.AgentsLoadError == nil {
		t.Error("expected AgentsLoadError to be set after failure")
	}
}

func TestAgentsPanelShowsErrorState(t *testing.T) {
	state := NewSidebarState()
	state.AgentsLoading = false
	state.AgentsLoadError = errors.New("connection failed")
	state.AgentsLastRefresh = time.Now().Add(-5 * time.Minute)

	// With no cached agents
	result := renderAgentsList(state, nil, nil, true, 40, 5)
	if !strings.Contains(result, "Load error") {
		t.Errorf("expected 'Load error' in output, got: %s", result)
	}
	if !strings.Contains(result, "no cached agents") {
		t.Errorf("expected 'no cached agents' in output, got: %s", result)
	}
}

func TestAgentsPanelShowsAgentsWithError(t *testing.T) {
	state := NewSidebarState()
	state.AgentsLoading = false
	state.AgentsLoadError = errors.New("refresh failed")
	state.AgentsLastRefresh = time.Now().Add(-5 * time.Minute)
	state.Agents = []agentItem{
		{a: data.Agent{Name: "witness", Address: "perch/witness", Running: true}},
	}

	items := make([]SelectableItem, len(state.Agents))
	for i, a := range state.Agents {
		items[i] = a
	}

	// Create a healthy snapshot for the test
	snap := &data.Snapshot{
		Town: &data.TownStatus{},
		OperationalState: &data.OperationalState{
			WatchdogHealthy: true,
		},
	}

	result := renderAgentsList(state, snap, items, true, 40, 10)

	// Should show error indicator
	if !strings.Contains(result, "Load error") {
		t.Errorf("expected 'Load error' in output, got: %s", result)
	}
	// Should still show the agent
	if !strings.Contains(result, "witness") {
		t.Errorf("expected 'witness' agent in output, got: %s", result)
	}
}

func TestAgentsPanelNormalState(t *testing.T) {
	state := NewSidebarState()
	state.AgentsLoading = false
	state.AgentsLoadError = nil
	state.AgentsLastRefresh = time.Now()
	state.Agents = []agentItem{
		{a: data.Agent{Name: "witness", Address: "perch/witness", Running: true}},
		{a: data.Agent{Name: "refinery", Address: "perch/refinery", Running: true}},
	}

	items := make([]SelectableItem, len(state.Agents))
	for i, a := range state.Agents {
		items[i] = a
	}

	// Create a healthy snapshot for the test
	snap := &data.Snapshot{
		Town: &data.TownStatus{},
		OperationalState: &data.OperationalState{
			WatchdogHealthy: true,
		},
	}

	result := renderAgentsList(state, snap, items, true, 40, 10)

	// Should NOT show error or loading
	if strings.Contains(result, "Load error") {
		t.Errorf("should not show 'Load error' in normal state, got: %s", result)
	}
	if strings.Contains(result, "Loading") {
		t.Errorf("should not show 'Loading' in normal state, got: %s", result)
	}
}

func TestAgentsPanelShowsServicesStoppedState(t *testing.T) {
	state := NewSidebarState()
	state.AgentsLoading = false
	state.AgentsLoadError = nil

	// Create a snapshot with unhealthy watchdog (services stopped)
	snap := &data.Snapshot{
		Town: &data.TownStatus{},
		OperationalState: &data.OperationalState{
			WatchdogHealthy: false, // Services are stopped
		},
	}

	// No agents loaded (empty list)
	result := renderAgentsList(state, snap, nil, true, 40, 10)

	// Should show services stopped message
	if !strings.Contains(result, "stopped") {
		t.Errorf("expected 'stopped' in output when services are stopped, got: %s", result)
	}
	// Should show hint to start services (when section is active)
	if !strings.Contains(result, "boot") {
		t.Errorf("expected boot hint in output when services are stopped, got: %s", result)
	}
}

func TestAgentsPanelShowsAllAgentsStopped(t *testing.T) {
	state := NewSidebarState()
	state.AgentsLoading = false
	state.AgentsLoadError = nil
	// All agents are stopped (Running: false)
	state.Agents = []agentItem{
		{a: data.Agent{Name: "witness", Address: "perch/witness", Running: false}},
		{a: data.Agent{Name: "refinery", Address: "perch/refinery", Running: false}},
	}

	items := make([]SelectableItem, len(state.Agents))
	for i, a := range state.Agents {
		items[i] = a
	}

	// Create a healthy snapshot (town exists)
	snap := &data.Snapshot{
		Town: &data.TownStatus{},
		OperationalState: &data.OperationalState{
			WatchdogHealthy: true,
		},
	}

	result := renderAgentsList(state, snap, items, true, 40, 15)

	// Should show services stopped banner
	if !strings.Contains(result, "stopped") {
		t.Errorf("expected 'stopped' banner when all agents are stopped, got: %s", result)
	}
	// Should still show the agents
	if !strings.Contains(result, "witness") {
		t.Errorf("expected 'witness' agent in output, got: %s", result)
	}
}

func TestServicesAppearStopped(t *testing.T) {
	tests := []struct {
		name     string
		snap     *data.Snapshot
		expected bool
	}{
		{
			name:     "nil snapshot",
			snap:     nil,
			expected: true,
		},
		{
			name:     "nil town",
			snap:     &data.Snapshot{Town: nil},
			expected: true,
		},
		{
			name: "unhealthy watchdog",
			snap: &data.Snapshot{
				Town: &data.TownStatus{},
				OperationalState: &data.OperationalState{
					WatchdogHealthy: false,
				},
			},
			expected: true,
		},
		{
			name: "stale deacon heartbeat",
			snap: &data.Snapshot{
				Town: &data.TownStatus{},
				OperationalState: &data.OperationalState{
					WatchdogHealthy:     true,
					LastDeaconHeartbeat: time.Now().Add(-10 * time.Minute), // > 5 min ago
				},
			},
			expected: true,
		},
		{
			name: "healthy services",
			snap: &data.Snapshot{
				Town: &data.TownStatus{},
				OperationalState: &data.OperationalState{
					WatchdogHealthy:     true,
					LastDeaconHeartbeat: time.Now().Add(-1 * time.Minute), // Recent
				},
			},
			expected: false,
		},
		{
			name: "no operational state (legacy)",
			snap: &data.Snapshot{
				Town: &data.TownStatus{},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := servicesAppearStopped(tc.snap)
			if result != tc.expected {
				t.Errorf("servicesAppearStopped() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestAllAgentsStopped(t *testing.T) {
	tests := []struct {
		name     string
		agents   []agentItem
		expected bool
	}{
		{
			name:     "empty list",
			agents:   nil,
			expected: false,
		},
		{
			name: "all stopped",
			agents: []agentItem{
				{a: data.Agent{Running: false}},
				{a: data.Agent{Running: false}},
			},
			expected: true,
		},
		{
			name: "one running",
			agents: []agentItem{
				{a: data.Agent{Running: false}},
				{a: data.Agent{Running: true}},
			},
			expected: false,
		},
		{
			name: "all running",
			agents: []agentItem{
				{a: data.Agent{Running: true}},
				{a: data.Agent{Running: true}},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := allAgentsStopped(tc.agents)
			if result != tc.expected {
				t.Errorf("allAgentsStopped() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestRenderMergeQueueList_Loading(t *testing.T) {
	state := NewSidebarState()
	state.MQsLoading = true

	result := renderMergeQueueList(state, nil, nil, nil, false, 40, 10)

	if !strings.Contains(result, "Loading queue") {
		t.Errorf("expected 'Loading queue' in loading state, got: %s", result)
	}
}

func TestRenderMergeQueueList_ErrorWithCachedItems(t *testing.T) {
	state := NewSidebarState()
	state.MQsLoading = false
	state.MQsLoadError = errors.New("test error")
	state.MQsLastRefresh = time.Now().Add(-5 * time.Minute)
	state.MRs = []mrItem{
		{mr: data.MergeRequest{Title: "Fix: test-branch"}, rig: "perch"},
	}

	items := make([]SelectableItem, len(state.MRs))
	for i, m := range state.MRs {
		items[i] = m
	}

	// Pass a healthy snapshot (services running) so we get "Load error" not "Services stopped"
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "refinery", Role: "refinery", Running: true},
			},
		},
		OperationalState: &data.OperationalState{
			WatchdogHealthy:     true,
			LastDeaconHeartbeat: time.Now(),
		},
	}

	result := renderMergeQueueList(state, snap, nil, items, false, 40, 10)

	// Should show error banner (not "Services stopped" since services are running)
	if !strings.Contains(result, "Load error") {
		t.Errorf("expected 'Load error' when MQsLoadError is set and services running, got: %s", result)
	}
	// Should show stale indicator
	if !strings.Contains(result, "stale") {
		t.Errorf("expected 'stale' indicator, got: %s", result)
	}
	// Should still show the cached item (Label format is "[rig] Title")
	if !strings.Contains(result, "Fix: test-branch") {
		t.Errorf("expected cached item 'Fix: test-branch' to still be visible, got: %s", result)
	}
}

func TestRenderMergeQueueList_ErrorWithoutCachedItems(t *testing.T) {
	state := NewSidebarState()
	state.MQsLoading = false
	state.MQsLoadError = errors.New("test error")

	// Pass a healthy snapshot (services running) so we get "Load error" not "Services stopped"
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "refinery", Role: "refinery", Running: true},
			},
		},
		OperationalState: &data.OperationalState{
			WatchdogHealthy:     true,
			LastDeaconHeartbeat: time.Now(),
		},
	}

	result := renderMergeQueueList(state, snap, nil, nil, false, 40, 10)

	// Should show error
	if !strings.Contains(result, "Load error") {
		t.Errorf("expected 'Load error' when MQsLoadError is set, got: %s", result)
	}
	// Should show no cached items message
	if !strings.Contains(result, "no cached items") {
		t.Errorf("expected 'no cached items' message, got: %s", result)
	}
}

func TestRenderMergeQueueList_NormalWithItems(t *testing.T) {
	state := NewSidebarState()
	state.MQsLoading = false
	state.MQsLoadError = nil
	state.MRs = []mrItem{
		{mr: data.MergeRequest{Title: "Feature 1"}, rig: "perch"},
		{mr: data.MergeRequest{Title: "Feature 2"}, rig: "perch"},
	}

	items := make([]SelectableItem, len(state.MRs))
	for i, m := range state.MRs {
		items[i] = m
	}

	result := renderMergeQueueList(state, nil, nil, items, false, 40, 10)

	// Should NOT show error or loading
	if strings.Contains(result, "Load error") {
		t.Errorf("should not show 'Load error' in normal state, got: %s", result)
	}
	if strings.Contains(result, "Loading") {
		t.Errorf("should not show 'Loading' in normal state, got: %s", result)
	}
	// Should show items (Label format is "[rig] Title")
	if !strings.Contains(result, "Feature 1") {
		t.Errorf("expected 'Feature 1' in output, got: %s", result)
	}
	if !strings.Contains(result, "Feature 2") {
		t.Errorf("expected 'Feature 2' in output, got: %s", result)
	}
}

func TestRenderMergeQueueList_EmptyHealthyState(t *testing.T) {
	state := NewSidebarState()
	state.MQsLoading = false
	state.MQsLoadError = nil

	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "refinery", Role: "refinery", Running: true},
			},
		},
	}

	result := renderMergeQueueList(state, snap, nil, nil, false, 40, 10)

	// Should show healthy empty state from renderMQEmptyState
	if !strings.Contains(result, "Refinery idle") {
		t.Errorf("expected 'Refinery idle' in healthy empty state, got: %s", result)
	}
	if !strings.Contains(result, "Queue clear") {
		t.Errorf("expected 'Queue clear' in healthy empty state, got: %s", result)
	}
}

// TestRenderMergeQueueList_ServicesStoppedWithError tests that when services are stopped
// and there's a load error, we show "Services stopped / stale" instead of "Load error".
func TestRenderMergeQueueList_ServicesStoppedWithError(t *testing.T) {
	state := NewSidebarState()
	state.MQsLoading = false
	state.MQsLoadError = errors.New("test error")
	state.MQsLastRefresh = time.Now().Add(-5 * time.Minute)

	// nil snap means services appear stopped
	result := renderMergeQueueList(state, nil, nil, nil, false, 40, 10)

	// Should show "Services stopped / stale" instead of "Load error"
	if !strings.Contains(result, "Services stopped") {
		t.Errorf("expected 'Services stopped' when snap is nil, got: %s", result)
	}
	if strings.Contains(result, "Load error") {
		t.Errorf("should NOT show 'Load error' when services are stopped, got: %s", result)
	}
	// Should show last refresh timestamp
	if !strings.Contains(result, "last:") {
		t.Errorf("expected 'last:' timestamp when services stopped, got: %s", result)
	}
}

// TestRenderMergeQueueList_RefineryStoppedWithError tests that when refinery is stopped
// and there's a load error, we show "Services stopped / stale" instead of "Load error".
func TestRenderMergeQueueList_RefineryStoppedWithError(t *testing.T) {
	state := NewSidebarState()
	state.MQsLoading = false
	state.MQsLoadError = errors.New("test error")
	state.MQsLastRefresh = time.Now().Add(-5 * time.Minute)

	// Snap with refinery stopped but watchdog healthy
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "refinery", Role: "refinery", Running: false}, // Refinery stopped
			},
		},
		OperationalState: &data.OperationalState{
			WatchdogHealthy:     true,
			LastDeaconHeartbeat: time.Now(),
		},
	}

	result := renderMergeQueueList(state, snap, nil, nil, false, 40, 10)

	// Should show "Services stopped / stale" because refinery is stopped
	if !strings.Contains(result, "Services stopped") {
		t.Errorf("expected 'Services stopped' when refinery is stopped, got: %s", result)
	}
	if strings.Contains(result, "Load error") {
		t.Errorf("should NOT show 'Load error' when refinery is stopped, got: %s", result)
	}
}

// TestRenderConvoysList_Loading tests that loading state shows loading indicator.
func TestRenderConvoysList_Loading(t *testing.T) {
	state := NewSidebarState()
	state.ConvoysLoading = true

	result := renderConvoysList(state, nil, nil, false, 40, 10)

	if !strings.Contains(result, "Loading convoys") {
		t.Errorf("expected 'Loading convoys' in loading state, got: %s", result)
	}
}

// TestRenderConvoysList_ErrorWithServicesRunning tests that real errors show "Load error".
func TestRenderConvoysList_ErrorWithServicesRunning(t *testing.T) {
	state := NewSidebarState()
	state.ConvoysLoading = false
	state.ConvoysLoadError = errors.New("test error")
	state.ConvoysLastRefresh = time.Now().Add(-5 * time.Minute)

	// Healthy snapshot (services running)
	snap := &data.Snapshot{
		Town: &data.TownStatus{
			Agents: []data.Agent{
				{Name: "deacon", Role: "deacon", Running: true},
			},
		},
		OperationalState: &data.OperationalState{
			WatchdogHealthy:     true,
			LastDeaconHeartbeat: time.Now(),
		},
	}

	result := renderConvoysList(state, snap, nil, false, 40, 10)

	// Should show "Load error" since services are running
	if !strings.Contains(result, "Load error") {
		t.Errorf("expected 'Load error' when services running but load failed, got: %s", result)
	}
	// Should show last refresh timestamp
	if !strings.Contains(result, "last:") {
		t.Errorf("expected 'last:' timestamp, got: %s", result)
	}
}

// TestRenderConvoysList_ServicesStoppedWithError tests that services stopped shows stale indicator.
func TestRenderConvoysList_ServicesStoppedWithError(t *testing.T) {
	state := NewSidebarState()
	state.ConvoysLoading = false
	state.ConvoysLoadError = errors.New("test error")
	state.ConvoysLastRefresh = time.Now().Add(-5 * time.Minute)

	// nil snap means services appear stopped
	result := renderConvoysList(state, nil, nil, false, 40, 10)

	// Should show "Services stopped / stale" instead of "Load error"
	if !strings.Contains(result, "Services stopped") {
		t.Errorf("expected 'Services stopped' when snap is nil, got: %s", result)
	}
	if strings.Contains(result, "Load error") {
		t.Errorf("should NOT show 'Load error' when services are stopped, got: %s", result)
	}
	// Should show last refresh timestamp
	if !strings.Contains(result, "last:") {
		t.Errorf("expected 'last:' timestamp when services stopped, got: %s", result)
	}
}

// TestRenderConvoysList_NormalWithItems tests normal convoy display.
func TestRenderConvoysList_NormalWithItems(t *testing.T) {
	state := NewSidebarState()
	state.ConvoysLoading = false
	state.ConvoysLoadError = nil
	state.Convoys = []convoyItem{
		{c: data.Convoy{Title: "Deploy feature X"}},
		{c: data.Convoy{Title: "Fix bug Y"}},
	}

	items := make([]SelectableItem, len(state.Convoys))
	for i, c := range state.Convoys {
		items[i] = c
	}

	result := renderConvoysList(state, nil, items, false, 40, 10)

	// Should NOT show error or loading
	if strings.Contains(result, "Load error") {
		t.Errorf("should not show 'Load error' in normal state, got: %s", result)
	}
	if strings.Contains(result, "Loading") {
		t.Errorf("should not show 'Loading' in normal state, got: %s", result)
	}
	// Should show items
	if !strings.Contains(result, "Deploy feature X") {
		t.Errorf("expected 'Deploy feature X' in output, got: %s", result)
	}
	if !strings.Contains(result, "Fix bug Y") {
		t.Errorf("expected 'Fix bug Y' in output, got: %s", result)
	}
}

// TestRefineryRunning tests the refineryRunning helper function.
func TestRefineryRunning(t *testing.T) {
	tests := []struct {
		name     string
		snap     *data.Snapshot
		expected bool
	}{
		{
			name:     "nil snapshot",
			snap:     nil,
			expected: false,
		},
		{
			name:     "nil town",
			snap:     &data.Snapshot{Town: nil},
			expected: false,
		},
		{
			name: "no refinery agents",
			snap: &data.Snapshot{
				Town: &data.TownStatus{
					Agents: []data.Agent{
						{Name: "deacon", Role: "deacon", Running: true},
					},
				},
			},
			expected: false,
		},
		{
			name: "refinery stopped",
			snap: &data.Snapshot{
				Town: &data.TownStatus{
					Agents: []data.Agent{
						{Name: "refinery", Role: "refinery", Running: false},
					},
				},
			},
			expected: false,
		},
		{
			name: "refinery running",
			snap: &data.Snapshot{
				Town: &data.TownStatus{
					Agents: []data.Agent{
						{Name: "refinery", Role: "refinery", Running: true},
					},
				},
			},
			expected: true,
		},
		{
			name: "multiple refineries one running",
			snap: &data.Snapshot{
				Town: &data.TownStatus{
					Agents: []data.Agent{
						{Name: "refinery-a", Role: "refinery", Running: false},
						{Name: "refinery-b", Role: "refinery", Running: true},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := refineryRunning(tt.snap)
			if result != tt.expected {
				t.Errorf("refineryRunning() = %v, want %v", result, tt.expected)
			}
		})
	}
}
