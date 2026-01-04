package tui

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/andyrewlee/perch/data"
	"github.com/andyrewlee/perch/internal/testutil"
	tea "github.com/charmbracelet/bubbletea"
)

// TestRefreshIntegration_NormalCycle tests the full tick loop + refresh pipeline.
// Verifies that:
// - tickMsg triggers a refresh
// - Data flows from mocked loader to model state
// - isRefreshing and lastRefresh are updated correctly
func TestRefreshIntegration_NormalCycle(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	// Set up mock responses for all data sources
	mock.On([]string{"gt", "status"}, fixtures.TownStatusJSON(), nil, nil)
	mock.On([]string{"gt", "polecat"}, fixtures.PolecatsJSON(), nil, nil)
	mock.On([]string{"gt", "convoy"}, fixtures.ConvoysJSON(), nil, nil)
	mock.On([]string{"gt", "mail"}, []byte("[]"), nil, nil)
	mock.On([]string{"bd", "list"}, fixtures.IssuesJSON(), nil, nil)
	mock.On([]string{"gt", "mq"}, []byte("[]"), nil, nil)

	// Create model with mocked loader
	model := newTestModelWithMockRunner(t, mock)
	model.ready = true
	model.refreshInterval = 10 * time.Second

	// Initial state checks
	if model.isRefreshing {
		t.Error("model should not be refreshing initially")
	}
	if !model.lastRefresh.IsZero() {
		t.Error("lastRefresh should be zero initially")
	}

	// Simulate tick message
	updated, cmd := model.Update(tickMsg(time.Now()))
	model = updated.(Model)

	// Should now be refreshing
	if !model.isRefreshing {
		t.Error("tickMsg should set isRefreshing to true")
	}
	if cmd == nil {
		t.Error("tickMsg should return commands (tick + refresh)")
	}

	// Execute the loadData command directly
	refreshResult := model.loadData()

	// Process the refresh result
	updated, _ = model.Update(refreshResult)
	model = updated.(Model)

	// Verify refresh completed
	if model.isRefreshing {
		t.Error("refreshMsg should set isRefreshing to false")
	}
	if model.lastRefresh.IsZero() {
		t.Error("refreshMsg should update lastRefresh")
	}

	// Verify data was loaded
	if model.snapshot == nil {
		t.Fatal("snapshot should be populated after refresh")
	}
	if model.snapshot.Town == nil {
		t.Fatal("town should be loaded")
	}
	if model.snapshot.Town.Name != "test-town" {
		t.Errorf("expected town name 'test-town', got %q", model.snapshot.Town.Name)
	}
	if len(model.snapshot.Town.Rigs) != 2 {
		t.Errorf("expected 2 rigs, got %d", len(model.snapshot.Town.Rigs))
	}

	// Verify sidebar was updated from snapshot
	if len(model.sidebar.Rigs) != 2 {
		t.Errorf("sidebar should have 2 rigs, got %d", len(model.sidebar.Rigs))
	}
}

// TestRefreshIntegration_ErrorPropagation tests that errors from the loader
// are properly propagated to the model state.
func TestRefreshIntegration_ErrorPropagation(t *testing.T) {
	mock := testutil.NewMockRunner()

	// Town status fails
	mock.On([]string{"gt", "status"}, nil, []byte("connection refused"), errors.New("exit status 1"))
	// Other commands succeed (partial data scenario)
	mock.On([]string{"gt", "polecat"}, []byte("[]"), nil, nil)
	mock.On([]string{"gt", "convoy"}, []byte("[]"), nil, nil)
	mock.On([]string{"gt", "mail"}, []byte("[]"), nil, nil)
	mock.On([]string{"bd", "list"}, []byte("[]"), nil, nil)
	mock.On([]string{"gt", "mq"}, []byte("[]"), nil, nil)

	model := newTestModelWithMockRunner(t, mock)
	model.ready = true
	model.refreshInterval = 10 * time.Second

	// Trigger refresh
	model.isRefreshing = true
	refreshResult := model.loadData()
	updated, _ := model.Update(refreshResult)
	model = updated.(Model)

	// Verify error state
	if model.snapshot == nil {
		t.Fatal("snapshot should exist even with errors")
	}
	if !model.snapshot.HasErrors() {
		t.Error("snapshot should have errors")
	}
	if model.snapshot.Town != nil {
		t.Error("town should be nil when loading failed")
	}

	// errorCount should reflect the number of errors
	if model.errorCount != len(model.snapshot.Errors) {
		t.Errorf("errorCount = %d, want %d", model.errorCount, len(model.snapshot.Errors))
	}
}

// TestRefreshIntegration_PartialDataWithErrors tests that partial data is
// still available when some loaders fail.
func TestRefreshIntegration_PartialDataWithErrors(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	// Town and convoys succeed
	mock.On([]string{"gt", "status"}, fixtures.TownStatusJSON(), nil, nil)
	mock.On([]string{"gt", "convoy"}, fixtures.ConvoysJSON(), nil, nil)
	// Others fail
	mock.On([]string{"gt", "polecat"}, nil, []byte("error"), errors.New("failed"))
	mock.On([]string{"gt", "mail"}, nil, []byte("error"), errors.New("failed"))
	mock.On([]string{"bd", "list"}, nil, []byte("error"), errors.New("failed"))
	mock.On([]string{"gt", "mq"}, []byte("[]"), nil, nil)

	model := newTestModelWithMockRunner(t, mock)
	model.ready = true

	// Trigger refresh
	model.isRefreshing = true
	refreshResult := model.loadData()
	updated, _ := model.Update(refreshResult)
	model = updated.(Model)

	// Should have partial data with errors
	if model.snapshot == nil {
		t.Fatal("snapshot should exist")
	}
	if !model.snapshot.HasErrors() {
		t.Error("snapshot should have errors for failed loads")
	}

	// Town should still be available
	if model.snapshot.Town == nil {
		t.Error("town should be loaded despite other errors")
	}
	if model.snapshot.Town.Name != "test-town" {
		t.Errorf("town name = %q, want 'test-town'", model.snapshot.Town.Name)
	}

	// Convoys should still be available
	if len(model.snapshot.Convoys) != 2 {
		t.Errorf("convoys = %d, want 2", len(model.snapshot.Convoys))
	}

	// Polecats should be nil due to error
	if model.snapshot.Polecats != nil {
		t.Error("polecats should be nil due to error")
	}
}

// TestRefreshIntegration_StatusIndicators tests that the HUD status indicators
// reflect the correct state during and after refresh.
func TestRefreshIntegration_StatusIndicators(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	mock.On([]string{"gt", "status"}, fixtures.TownStatusJSON(), nil, nil)
	mock.On([]string{"gt", "polecat"}, fixtures.PolecatsJSON(), nil, nil)
	mock.On([]string{"gt", "convoy"}, fixtures.ConvoysJSON(), nil, nil)
	mock.On([]string{"gt", "mail"}, []byte("[]"), nil, nil)
	mock.On([]string{"bd", "list"}, fixtures.IssuesJSON(), nil, nil)
	mock.On([]string{"gt", "mq"}, []byte("[]"), nil, nil)

	model := newTestModelWithMockRunner(t, mock)
	model.ready = true
	model.width = 80
	model.refreshInterval = 10 * time.Second

	t.Run("disconnected state", func(t *testing.T) {
		// Before any refresh - should be disconnected
		if !model.lastRefresh.IsZero() {
			t.Error("should be disconnected initially (lastRefresh zero)")
		}
		if model.isRefreshing {
			t.Error("should not be refreshing initially")
		}
		if model.errorCount != 0 {
			t.Error("should have no errors initially")
		}
	})

	t.Run("refreshing state", func(t *testing.T) {
		// Trigger tick
		updated, _ := model.Update(tickMsg(time.Now()))
		model = updated.(Model)

		if !model.isRefreshing {
			t.Error("should be refreshing after tick")
		}
	})

	t.Run("connected state", func(t *testing.T) {
		// Complete refresh
		refreshResult := model.loadData()
		updated, _ := model.Update(refreshResult)
		model = updated.(Model)

		if model.isRefreshing {
			t.Error("should not be refreshing after complete")
		}
		if model.lastRefresh.IsZero() {
			t.Error("lastRefresh should be set")
		}
		if model.errorCount != 0 {
			t.Error("should have no errors after successful refresh")
		}
	})
}

// TestRefreshIntegration_ConcurrentTick tests that concurrent ticks don't
// cause multiple simultaneous refreshes.
func TestRefreshIntegration_ConcurrentTick(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	mock.On([]string{"gt", "status"}, fixtures.TownStatusJSON(), nil, nil)
	mock.On([]string{"gt", "polecat"}, fixtures.PolecatsJSON(), nil, nil)
	mock.On([]string{"gt", "convoy"}, fixtures.ConvoysJSON(), nil, nil)
	mock.On([]string{"gt", "mail"}, []byte("[]"), nil, nil)
	mock.On([]string{"bd", "list"}, fixtures.IssuesJSON(), nil, nil)
	mock.On([]string{"gt", "mq"}, []byte("[]"), nil, nil)

	model := newTestModelWithMockRunner(t, mock)
	model.ready = true
	model.refreshInterval = 10 * time.Second

	// First tick - should trigger refresh
	updated, cmd1 := model.Update(tickMsg(time.Now()))
	model = updated.(Model)

	if !model.isRefreshing {
		t.Error("first tick should set isRefreshing")
	}
	if cmd1 == nil {
		t.Error("first tick should return commands")
	}

	// Second tick while still refreshing - should not trigger another refresh
	updated, cmd2 := model.Update(tickMsg(time.Now()))
	model = updated.(Model)

	if !model.isRefreshing {
		t.Error("should still be refreshing")
	}
	// Should still return a tick command (to schedule next tick)
	if cmd2 == nil {
		t.Error("second tick should still return tick command")
	}
}

// TestRefreshIntegration_SidebarSync tests that sidebar state is properly
// synchronized with snapshot data after refresh.
func TestRefreshIntegration_SidebarSync(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	mock.On([]string{"gt", "status"}, fixtures.TownStatusJSON(), nil, nil)
	mock.On([]string{"gt", "polecat"}, fixtures.PolecatsJSON(), nil, nil)
	mock.On([]string{"gt", "convoy"}, fixtures.ConvoysJSON(), nil, nil)
	mock.On([]string{"gt", "mail"}, []byte("[]"), nil, nil)
	mock.On([]string{"bd", "list"}, fixtures.IssuesJSON(), nil, nil)
	mock.On([]string{"gt", "mq"}, []byte("[]"), nil, nil)

	model := newTestModelWithMockRunner(t, mock)
	model.ready = true

	// Initial sidebar should be empty
	if len(model.sidebar.Rigs) != 0 {
		t.Errorf("sidebar rigs should be empty initially, got %d", len(model.sidebar.Rigs))
	}

	// Perform refresh
	model.isRefreshing = true
	refreshResult := model.loadData()
	updated, _ := model.Update(refreshResult)
	model = updated.(Model)

	// Sidebar should now have data
	if len(model.sidebar.Rigs) != 2 {
		t.Errorf("sidebar should have 2 rigs after refresh, got %d", len(model.sidebar.Rigs))
	}

	// Check rig names
	rigNames := make(map[string]bool)
	for _, rig := range model.sidebar.Rigs {
		rigNames[rig.r.Name] = true
	}
	if !rigNames["perch"] {
		t.Error("sidebar should contain rig 'perch'")
	}
	if !rigNames["sidekick"] {
		t.Error("sidebar should contain rig 'sidekick'")
	}
}

// TestRefreshIntegration_QueueHealthUpdate tests that queue health data
// is updated from snapshot after refresh.
func TestRefreshIntegration_QueueHealthUpdate(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	mock.On([]string{"gt", "status"}, fixtures.TownStatusJSON(), nil, nil)
	mock.On([]string{"gt", "polecat"}, fixtures.PolecatsJSON(), nil, nil)
	mock.On([]string{"gt", "convoy"}, fixtures.ConvoysJSON(), nil, nil)
	mock.On([]string{"gt", "mail"}, []byte("[]"), nil, nil)
	mock.On([]string{"bd", "list"}, fixtures.IssuesJSON(), nil, nil)
	// Return merge queue data for perch
	mock.OnFunc([]string{"gt", "mq", "list"}, func(args []string) ([]byte, []byte, error) {
		if len(args) >= 4 {
			return fixtures.MergeQueueJSON(args[3]), nil, nil
		}
		return []byte("[]"), nil, nil
	})

	model := newTestModelWithMockRunner(t, mock)
	model.ready = true

	// Perform refresh
	model.isRefreshing = true
	refreshResult := model.loadData()
	updated, _ := model.Update(refreshResult)
	model = updated.(Model)

	// Queue health should be updated
	if model.queueHealthData == nil {
		t.Fatal("queueHealthData should not be nil")
	}

	// Check perch queue health
	health, ok := model.queueHealthData["perch"]
	if !ok {
		t.Error("queueHealthData should contain 'perch'")
	} else {
		if health.RigName != "perch" {
			t.Errorf("health.RigName = %q, want 'perch'", health.RigName)
		}
		if len(health.MRs) != 2 {
			t.Errorf("perch should have 2 MRs, got %d", len(health.MRs))
		}
	}
}

// TestRefreshIntegration_SelectedRigValidation tests that the selected rig
// is validated after refresh (reset if no longer exists).
func TestRefreshIntegration_SelectedRigValidation(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	mock.On([]string{"gt", "status"}, fixtures.TownStatusJSON(), nil, nil)
	mock.On([]string{"gt", "polecat"}, fixtures.PolecatsJSON(), nil, nil)
	mock.On([]string{"gt", "convoy"}, fixtures.ConvoysJSON(), nil, nil)
	mock.On([]string{"gt", "mail"}, []byte("[]"), nil, nil)
	mock.On([]string{"bd", "list"}, fixtures.IssuesJSON(), nil, nil)
	mock.On([]string{"gt", "mq"}, []byte("[]"), nil, nil)

	model := newTestModelWithMockRunner(t, mock)
	model.ready = true
	model.selectedRig = "nonexistent-rig" // Set to a rig that doesn't exist

	// Perform refresh
	model.isRefreshing = true
	refreshResult := model.loadData()
	updated, _ := model.Update(refreshResult)
	model = updated.(Model)

	// Selected rig should be reset since it doesn't exist in snapshot
	if model.selectedRig == "nonexistent-rig" {
		t.Error("selectedRig should be reset when rig no longer exists")
	}

	// Should default to first rig if available
	if model.selectedRig != "perch" && model.selectedRig != "sidekick" {
		t.Errorf("selectedRig should default to first rig, got %q", model.selectedRig)
	}
}

// newTestModelWithMockRunner creates a Model configured for testing with
// a mocked command runner, avoiding any real CLI calls.
func newTestModelWithMockRunner(t *testing.T, mock *testutil.MockRunner) Model {
	t.Helper()

	// Create a temp directory that looks like a town
	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/mayor", 0755); err != nil {
		t.Fatalf("failed to create test town: %v", err)
	}
	// Create logs directory for lifecycle log
	if err := os.MkdirAll(tmpDir+"/logs", 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create loader with mock runner
	loader := data.NewLoaderWithRunner(tmpDir, mock)

	// Create store with mocked loader
	store := data.NewStoreWithLoader(loader)
	store.RefreshInterval = 5 * time.Second

	// Create model with the mocked store
	return Model{
		focus:           PanelSidebar,
		store:           store,
		townRoot:        tmpDir,
		sidebar:         NewSidebarState(),
		actionRunner:    NewActionRunner(tmpDir),
		refreshInterval: DefaultRefreshInterval,
		queueHealthData: make(map[string]QueueHealth),
	}
}

// TestRefreshIntegration_DisabledTick tests that tick is disabled when
// refreshInterval is zero.
func TestRefreshIntegration_DisabledTick(t *testing.T) {
	mock := testutil.NewMockRunner()
	model := newTestModelWithMockRunner(t, mock)
	model.refreshInterval = 0 // Disable auto-refresh

	cmd := model.tickCmd()
	if cmd != nil {
		t.Error("tickCmd should return nil when refreshInterval is 0")
	}
}

// TestRefreshIntegration_ManualRefresh tests the 'r' key manual refresh.
func TestRefreshIntegration_ManualRefresh(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	mock.On([]string{"gt", "status"}, fixtures.TownStatusJSON(), nil, nil)
	mock.On([]string{"gt", "polecat"}, fixtures.PolecatsJSON(), nil, nil)
	mock.On([]string{"gt", "convoy"}, fixtures.ConvoysJSON(), nil, nil)
	mock.On([]string{"gt", "mail"}, []byte("[]"), nil, nil)
	mock.On([]string{"bd", "list"}, fixtures.IssuesJSON(), nil, nil)
	mock.On([]string{"gt", "mq"}, []byte("[]"), nil, nil)

	model := newTestModelWithMockRunner(t, mock)
	model.ready = true
	model.refreshInterval = 10 * time.Second

	// Press 'r' to trigger manual refresh
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = updated.(Model)

	if !model.isRefreshing {
		t.Error("'r' key should trigger refresh")
	}
	if cmd == nil {
		t.Error("'r' key should return refresh command")
	}
	if model.statusMessage == nil {
		t.Error("'r' key should set status message")
	} else if model.statusMessage.Text != "Refreshing data..." {
		t.Errorf("status message = %q, want 'Refreshing data...'", model.statusMessage.Text)
	}
}
