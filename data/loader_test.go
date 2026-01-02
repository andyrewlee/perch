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
