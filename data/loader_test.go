package data

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLoaderIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if not in Gas Town environment
	townRoot := os.Getenv("GT_ROOT")
	if townRoot == "" {
		// Try to find town root by walking up
		townRoot = "/Users/andrewlee/gt"
		if _, err := os.Stat(townRoot); os.IsNotExist(err) {
			t.Skip("GT_ROOT not set and default town not found")
		}
	}

	// Each subtest gets its own timeout
	ctx := context.Background()
	_ = ctx

	loader := NewLoader(townRoot)

	t.Run("LoadTownStatus", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		status, err := loader.LoadTownStatus(ctx)
		if err != nil {
			t.Fatalf("LoadTownStatus: %v", err)
		}
		if status.Name == "" {
			t.Error("expected non-empty town name")
		}
		t.Logf("Town: %s at %s", status.Name, status.Location)
		t.Logf("Rigs: %d, Polecats: %d", status.Summary.RigCount, status.Summary.PolecatCount)
	})

	t.Run("LoadPolecats", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		polecats, err := loader.LoadPolecats(ctx)
		if err != nil {
			t.Fatalf("LoadPolecats: %v", err)
		}
		t.Logf("Found %d polecats", len(polecats))
		for _, p := range polecats {
			t.Logf("  %s/%s: %s (running=%v)", p.Rig, p.Name, p.State, p.SessionRunning)
		}
	})

	t.Run("LoadConvoys", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		convoys, err := loader.LoadConvoys(ctx)
		if err != nil {
			t.Fatalf("LoadConvoys: %v", err)
		}
		t.Logf("Found %d convoys", len(convoys))
		for _, c := range convoys {
			t.Logf("  %s: %s (%s)", c.ID, c.Title, c.Status)
		}
	})

	t.Run("LoadIssues", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		issues, err := loader.LoadIssues(ctx)
		if err != nil {
			t.Fatalf("LoadIssues: %v", err)
		}
		t.Logf("Found %d issues", len(issues))
		// Show first 5
		for i, issue := range issues {
			if i >= 5 {
				break
			}
			t.Logf("  [%s] %s: %s", issue.Status, issue.ID, issue.Title)
		}
	})

	t.Run("LoadAll", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		snap := loader.LoadAll(ctx)
		if snap.HasErrors() {
			for _, err := range snap.Errors {
				t.Logf("Error: %v", err)
			}
		}
		t.Logf("Snapshot loaded at %s", snap.LoadedAt.Format(time.RFC3339))
		if snap.Town != nil {
			t.Logf("Town: %s", snap.Town.Name)
		}
		t.Logf("Polecats: %d, Convoys: %d, Issues: %d",
			len(snap.Polecats), len(snap.Convoys), len(snap.Issues))
	})

	t.Run("LoadAuditTimeline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Load without actor filter
		entries, err := loader.LoadAuditTimeline(ctx, "", 10)
		if err != nil {
			t.Fatalf("LoadAuditTimeline: %v", err)
		}
		t.Logf("Found %d audit entries (no filter)", len(entries))
		for i, e := range entries {
			if i >= 5 {
				break
			}
			t.Logf("  %s [%s/%s] %s: %s", e.Timestamp.Format("15:04:05"), e.Source, e.Type, e.Actor, e.Summary)
		}
	})

	t.Run("LoadAuditTimelineWithActor", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Load with actor filter
		entries, err := loader.LoadAuditTimeline(ctx, "mayor", 10)
		if err != nil {
			t.Fatalf("LoadAuditTimeline with actor: %v", err)
		}
		t.Logf("Found %d audit entries for 'mayor'", len(entries))
		for i, e := range entries {
			if i >= 5 {
				break
			}
			t.Logf("  %s [%s/%s] %s", e.Timestamp.Format("15:04:05"), e.Source, e.Type, e.Summary)
		}
	})
}

func TestEnrichWithHookedBeads(t *testing.T) {
	// Create a snapshot with mock data
	snap := &Snapshot{
		Town: &TownStatus{
			Name: "test-town",
			Summary: Summary{
				ActiveHooks: 0, // Initially 0
			},
			Agents: []Agent{
				{Name: "mayor", Address: "mayor/", HasWork: false},
			},
			Rigs: []Rig{
				{
					Name: "perch",
					Hooks: []Hook{
						{Agent: "perch/ace", Role: "polecat", HasWork: false},
						{Agent: "perch/nux", Role: "polecat", HasWork: false},
						{Agent: "perch/slit", Role: "polecat", HasWork: false},
					},
					Agents: []Agent{
						{Name: "witness", Address: "perch/witness", HasWork: false},
						{Name: "refinery", Address: "perch/refinery", HasWork: false},
					},
				},
			},
		},
		HookedIssues: []Issue{
			{ID: "gt-001", Title: "Task A", Assignee: "perch/polecats/ace"},
			{ID: "gt-002", Title: "Task B", Assignee: "perch/polecats/slit"},
		},
	}

	// Run enrichment
	snap.EnrichWithHookedBeads()

	// Verify ActiveHooks is updated
	if snap.Town.Summary.ActiveHooks != 2 {
		t.Errorf("expected ActiveHooks=2, got %d", snap.Town.Summary.ActiveHooks)
	}

	// Verify hooks are updated correctly
	hooks := snap.Town.Rigs[0].Hooks
	// ace should have work (matched gt-001)
	if !hooks[0].HasWork {
		t.Errorf("expected perch/ace hook to have work")
	}
	// nux should NOT have work (no matching bead)
	if hooks[1].HasWork {
		t.Errorf("expected perch/nux hook to NOT have work")
	}
	// slit should have work (matched gt-002)
	if !hooks[2].HasWork {
		t.Errorf("expected perch/slit hook to have work")
	}
}

func TestEnrichWithHookedBeads_EmptyHooks(t *testing.T) {
	// Snapshot with no hooked issues
	snap := &Snapshot{
		Town: &TownStatus{
			Summary: Summary{ActiveHooks: 0},
		},
		HookedIssues: nil,
	}

	// Should not panic
	snap.EnrichWithHookedBeads()

	// ActiveHooks should remain 0
	if snap.Town.Summary.ActiveHooks != 0 {
		t.Errorf("expected ActiveHooks=0, got %d", snap.Town.Summary.ActiveHooks)
	}
}

func TestEnrichWithHookedBeads_NilTown(t *testing.T) {
	// Snapshot with nil town
	snap := &Snapshot{
		Town: nil,
		HookedIssues: []Issue{
			{ID: "gt-001", Title: "Task A", Assignee: "perch/polecats/ace"},
		},
	}

	// Should not panic
	snap.EnrichWithHookedBeads()
}

func TestSplitAgentAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"perch/ace", []string{"perch", "ace"}},
		{"perch/polecats/ace", []string{"perch", "polecats/ace"}},
		{"norig", []string{"norig"}},
		{"a/b/c/d", []string{"a", "b/c/d"}},
	}

	for _, tc := range tests {
		result := splitAgentAddress(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("splitAgentAddress(%q): expected %v, got %v", tc.input, tc.expected, result)
			continue
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Errorf("splitAgentAddress(%q)[%d]: expected %q, got %q", tc.input, i, tc.expected[i], result[i])
			}
		}
	}
}

func TestStoreRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	townRoot := os.Getenv("GT_ROOT")
	if townRoot == "" {
		townRoot = "/Users/andrewlee/gt"
		if _, err := os.Stat(townRoot); os.IsNotExist(err) {
			t.Skip("GT_ROOT not set and default town not found")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	store := NewStore(townRoot)

	// Initially nil
	if store.Snapshot() != nil {
		t.Error("expected nil snapshot before refresh")
	}

	// Refresh
	snap := store.Refresh(ctx)
	if snap == nil {
		t.Fatal("expected non-nil snapshot after refresh")
	}

	// Accessors should work
	if store.Town() == nil && !snap.HasErrors() {
		t.Error("expected town after successful refresh")
	}

	t.Logf("Rigs: %v", store.Snapshot().RigNames())
	t.Logf("Open issues: %d", len(store.OpenIssues()))
	t.Logf("Last refresh: %s", store.LastRefresh().Format(time.RFC3339))
}
