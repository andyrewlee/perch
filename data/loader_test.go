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

func TestRigSettingsValidation(t *testing.T) {
	tests := []struct {
		name        string
		settings    RigSettings
		expectError bool
		errorField  string
	}{
		{
			name: "valid settings",
			settings: RigSettings{
				Name:       "test-rig",
				Prefix:     "tr",
				MaxWorkers: 5,
				MergeQueue: MergeQueueConfig{
					Enabled:     true,
					RunTests:    true,
					TestCommand: "go test ./...",
				},
			},
			expectError: false,
		},
		{
			name: "empty name",
			settings: RigSettings{
				Name:   "",
				Prefix: "tr",
			},
			expectError: true,
			errorField:  "name",
		},
		{
			name: "empty prefix",
			settings: RigSettings{
				Name:   "test-rig",
				Prefix: "",
			},
			expectError: true,
			errorField:  "prefix",
		},
		{
			name: "negative max workers",
			settings: RigSettings{
				Name:       "test-rig",
				Prefix:     "tr",
				MaxWorkers: -1,
			},
			expectError: true,
			errorField:  "max_workers",
		},
		{
			name: "run tests enabled but no command",
			settings: RigSettings{
				Name:   "test-rig",
				Prefix: "tr",
				MergeQueue: MergeQueueConfig{
					Enabled:     true,
					RunTests:    true,
					TestCommand: "",
				},
			},
			expectError: true,
			errorField:  "test_command",
		},
		{
			name: "run tests disabled - no command needed",
			settings: RigSettings{
				Name:   "test-rig",
				Prefix: "tr",
				MergeQueue: MergeQueueConfig{
					Enabled:     true,
					RunTests:    false,
					TestCommand: "",
				},
			},
			expectError: false,
		},
		{
			name: "zero max workers is valid (unlimited)",
			settings: RigSettings{
				Name:       "test-rig",
				Prefix:     "tr",
				MaxWorkers: 0,
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.settings.Validate()
			if tc.expectError {
				if err == nil {
					t.Errorf("expected validation error for %s", tc.errorField)
				} else if verr, ok := err.(*ValidationError); ok {
					if verr.Field != tc.errorField {
						t.Errorf("expected error field %q, got %q", tc.errorField, verr.Field)
					}
				} else {
					t.Errorf("expected ValidationError, got %T: %v", err, err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidationErrorString(t *testing.T) {
	err := &ValidationError{Field: "test_field", Message: "test message"}
	expected := "test_field: test message"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
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

func TestConvoyProgress(t *testing.T) {
	tests := []struct {
		name       string
		convoy     Convoy
		wantPct    int
		wantActive bool
	}{
		{
			name: "empty convoy",
			convoy: Convoy{
				ID:        "test-1",
				Status:    "open",
				Completed: 0,
				Total:     0,
			},
			wantPct:    0,
			wantActive: true, // status is "open"
		},
		{
			name: "half complete",
			convoy: Convoy{
				ID:        "test-2",
				Status:    "open",
				Completed: 3,
				Total:     6,
			},
			wantPct:    50,
			wantActive: true, // status is "open"
		},
		{
			name: "all complete",
			convoy: Convoy{
				ID:        "test-3",
				Status:    "closed",
				Completed: 5,
				Total:     5,
			},
			wantPct:    100,
			wantActive: false, // status is "closed"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.convoy.Progress()
			if got != tt.wantPct {
				t.Errorf("Progress() = %d, want %d", got, tt.wantPct)
			}
			gotActive := tt.convoy.IsActive()
			if gotActive != tt.wantActive {
				t.Errorf("IsActive() = %v, want %v", gotActive, tt.wantActive)
			}
		})
	}
}

func TestConvoyHasActiveWork(t *testing.T) {
	tests := []struct {
		name   string
		convoy Convoy
		want   bool
	}{
		{
			name: "no tracked issues",
			convoy: Convoy{
				ID:      "test-1",
				Tracked: nil,
			},
			want: false,
		},
		{
			name: "active with in_progress",
			convoy: Convoy{
				ID: "test-2",
				Tracked: []TrackedIssue{
					{ID: "a", Status: "closed"},
					{ID: "b", Status: "in_progress"},
					{ID: "c", Status: "open"},
				},
			},
			want: true,
		},
		{
			name: "active with hooked",
			convoy: Convoy{
				ID: "test-3",
				Tracked: []TrackedIssue{
					{ID: "a", Status: "hooked"},
					{ID: "b", Status: "open"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.convoy.HasActiveWork()
			if got != tt.want {
				t.Errorf("HasActiveWork() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDoctorOutput(t *testing.T) {
	input := `✓ town-config-exists: mayor/town.json exists
✓ town-config-valid: mayor/town.json valid (name=gt, version=2)
⚠ town-git: Town root is not under version control
    Your town harness contains personal configuration
    → Run 'git init' in your town root
✗ bd-daemon: bd daemon failed to start
    Error: not in a git repository
    → Check 'bd daemon --status' and logs

38 checks, 30 passed, 7 warnings, 1 errors`

	report, err := parseDoctorOutput(input)
	if err != nil {
		t.Fatalf("parseDoctorOutput: %v", err)
	}

	// Check summary
	if report.TotalChecks != 38 {
		t.Errorf("TotalChecks = %d, want 38", report.TotalChecks)
	}
	if report.PassedCount != 30 {
		t.Errorf("PassedCount = %d, want 30", report.PassedCount)
	}
	if report.WarningCount != 7 {
		t.Errorf("WarningCount = %d, want 7", report.WarningCount)
	}
	if report.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", report.ErrorCount)
	}

	// Check parsed checks
	if len(report.Checks) != 4 {
		t.Errorf("len(Checks) = %d, want 4", len(report.Checks))
	}

	// Verify first check (passed)
	if report.Checks[0].Name != "town-config-exists" {
		t.Errorf("Checks[0].Name = %q, want 'town-config-exists'", report.Checks[0].Name)
	}
	if report.Checks[0].Status != CheckPassed {
		t.Errorf("Checks[0].Status = %v, want CheckPassed", report.Checks[0].Status)
	}

	// Verify warning check
	warningCheck := report.Checks[2]
	if warningCheck.Name != "town-git" {
		t.Errorf("Warning check name = %q, want 'town-git'", warningCheck.Name)
	}
	if warningCheck.Status != CheckWarning {
		t.Errorf("Warning check status = %v, want CheckWarning", warningCheck.Status)
	}
	if warningCheck.SuggestFix == "" {
		t.Error("Warning check should have SuggestFix")
	}

	// Verify error check
	errorCheck := report.Checks[3]
	if errorCheck.Name != "bd-daemon" {
		t.Errorf("Error check name = %q, want 'bd-daemon'", errorCheck.Name)
	}
	if errorCheck.Status != CheckError {
		t.Errorf("Error check status = %v, want CheckError", errorCheck.Status)
	}
}

func TestDoctorReportHelpers(t *testing.T) {
	report := &DoctorReport{
		Checks: []DoctorCheck{
			{Name: "e1", Status: CheckError},
			{Name: "w1", Status: CheckWarning},
			{Name: "w2", Status: CheckWarning},
			{Name: "p1", Status: CheckPassed},
		},
		ErrorCount:   1,
		WarningCount: 2,
		PassedCount:  1,
	}

	if !report.HasIssues() {
		t.Error("HasIssues should be true")
	}

	errors := report.Errors()
	if len(errors) != 1 {
		t.Errorf("len(Errors()) = %d, want 1", len(errors))
	}

	warnings := report.Warnings()
	if len(warnings) != 2 {
		t.Errorf("len(Warnings()) = %d, want 2", len(warnings))
	}

	// Test report with no issues
	okReport := &DoctorReport{
		ErrorCount:   0,
		WarningCount: 0,
	}
	if okReport.HasIssues() {
		t.Error("HasIssues should be false for healthy report")
	}
}

func TestLoadConvoysWithDetails(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	loader := NewLoader(townRoot)
	convoys, err := loader.LoadConvoysWithDetails(ctx)
	if err != nil {
		t.Fatalf("LoadConvoysWithDetails: %v", err)
	}

	t.Logf("Found %d convoys with details", len(convoys))
	for _, c := range convoys {
		t.Logf("  %s: %s (%s) - %d/%d (%d%%) active=%v",
			c.ID, c.Title, c.Status, c.Completed, c.Total, c.Progress(), c.IsActive())
		for _, tr := range c.Tracked {
			t.Logf("    [%s] %s: %s", tr.Status, tr.ID, tr.Title)
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
