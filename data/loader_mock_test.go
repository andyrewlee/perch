package data

import (
	"context"
	"errors"
	"testing"

	"github.com/andyrewlee/perch/internal/testutil"
)

// TestLoaderWithMock demonstrates using MockRunner to test Loader
// without requiring real gt/bd CLI tools.
func TestLoaderWithMock(t *testing.T) {
	fixtures := testutil.NewFixtures()

	t.Run("LoadTownStatus", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "status", "--json"}, fixtures.TownStatusJSON(), nil, nil)

		loader := NewLoaderWithRunner("/tmp/town", mock)
		status, err := loader.LoadTownStatus(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Name != "test-town" {
			t.Errorf("expected town name 'test-town', got %q", status.Name)
		}
		if len(status.Rigs) != 2 {
			t.Errorf("expected 2 rigs, got %d", len(status.Rigs))
		}
		if status.Summary.PolecatCount != 3 {
			t.Errorf("expected 3 polecats, got %d", status.Summary.PolecatCount)
		}

		// Verify the correct command was invoked
		if !mock.CalledWith([]string{"gt", "status", "--json"}) {
			t.Error("expected gt status --json to be called")
		}
	})

	t.Run("LoadPolecats", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "polecat", "list"}, fixtures.PolecatsJSON(), nil, nil)

		loader := NewLoaderWithRunner("/tmp/town", mock)
		polecats, err := loader.LoadPolecats(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(polecats) != 3 {
			t.Errorf("expected 3 polecats, got %d", len(polecats))
		}

		// Check specific polecat
		found := false
		for _, p := range polecats {
			if p.Name == "able" && p.Rig == "perch" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find polecat 'able' in rig 'perch'")
		}
	})

	t.Run("LoadConvoys", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "convoy", "list"}, fixtures.ConvoysJSON(), nil, nil)

		loader := NewLoaderWithRunner("/tmp/town", mock)
		convoys, err := loader.LoadConvoys(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(convoys) != 2 {
			t.Errorf("expected 2 convoys, got %d", len(convoys))
		}
	})

	t.Run("LoadMergeQueue", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		// Use OnFunc to return rig-specific responses
		mock.OnFunc([]string{"gt", "mq", "list"}, func(args []string) ([]byte, []byte, error) {
			if len(args) >= 4 {
				return fixtures.MergeQueueJSON(args[3]), nil, nil
			}
			return []byte("[]"), nil, nil
		})

		loader := NewLoaderWithRunner("/tmp/town", mock)

		// Test perch MQ
		mrs, err := loader.LoadMergeQueue(context.Background(), "perch")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mrs) != 2 {
			t.Errorf("expected 2 merge requests for perch, got %d", len(mrs))
		}

		// Test sidekick MQ
		mrs, err = loader.LoadMergeQueue(context.Background(), "sidekick")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mrs) != 1 {
			t.Errorf("expected 1 merge request for sidekick, got %d", len(mrs))
		}
	})

	t.Run("LoadIssues", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"bd", "list"}, fixtures.IssuesJSON(), nil, nil)

		loader := NewLoaderWithRunner("/tmp/town", mock)
		issues, err := loader.LoadIssues(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(issues) != 3 {
			t.Errorf("expected 3 issues, got %d", len(issues))
		}

		// Check issue priorities
		for _, issue := range issues {
			if issue.ID == "gt-002" && issue.Priority != 0 {
				t.Errorf("expected P0 priority for gt-002, got P%d", issue.Priority)
			}
		}
	})

	t.Run("LoadOpenIssues", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"bd", "list", "--json", "--status", "open"}, fixtures.OpenIssuesJSON(), nil, nil)

		loader := NewLoaderWithRunner("/tmp/town", mock)
		issues, err := loader.LoadOpenIssues(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(issues) != 1 {
			t.Errorf("expected 1 open issue, got %d", len(issues))
		}
	})
}

func TestLoaderErrors(t *testing.T) {
	t.Run("CommandFailure", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "status"}, nil, []byte("connection refused"), errors.New("exit status 1"))

		loader := NewLoaderWithRunner("/tmp/town", mock)
		_, err := loader.LoadTownStatus(context.Background())

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "connection refused") {
			t.Errorf("error should contain stderr: %v", err)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "status"}, []byte("not valid json"), nil, nil)

		loader := NewLoaderWithRunner("/tmp/town", mock)
		_, err := loader.LoadTownStatus(context.Background())

		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})

	t.Run("NullResponse", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		fixtures := testutil.NewFixtures()
		mock.On([]string{"gt", "convoy", "list"}, fixtures.NullResponse(), nil, nil)

		loader := NewLoaderWithRunner("/tmp/town", mock)
		convoys, err := loader.LoadConvoys(context.Background())

		// null is valid - should return empty without error
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if convoys != nil {
			t.Errorf("expected nil convoys for null response, got %v", convoys)
		}
	})

	t.Run("EmptyResponse", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "polecat", "list"}, []byte(""), nil, nil)

		loader := NewLoaderWithRunner("/tmp/town", mock)
		polecats, err := loader.LoadPolecats(context.Background())

		// Empty is valid - should return nil without error
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if polecats != nil {
			t.Errorf("expected nil polecats for empty response, got %v", polecats)
		}
	})
}

func TestLoaderLoadAll(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	// Set up all command responses
	mock.On([]string{"gt", "status", "--json"}, fixtures.TownStatusJSON(), nil, nil)
	mock.On([]string{"gt", "polecat", "list"}, fixtures.PolecatsJSON(), nil, nil)
	mock.On([]string{"gt", "convoy", "list"}, fixtures.ConvoysJSON(), nil, nil)
	mock.On([]string{"bd", "list", "--json", "--limit", "0"}, fixtures.IssuesJSON(), nil, nil)
	mock.OnFunc([]string{"gt", "mq", "list"}, func(args []string) ([]byte, []byte, error) {
		if len(args) >= 4 {
			return fixtures.MergeQueueJSON(args[3]), nil, nil
		}
		return []byte("[]"), nil, nil
	})

	loader := NewLoaderWithRunner("/tmp/town", mock)
	snap := loader.LoadAll(context.Background())

	if snap.HasErrors() {
		for _, err := range snap.Errors {
			t.Errorf("snapshot error: %v", err)
		}
	}

	if snap.Town == nil {
		t.Fatal("expected town in snapshot")
	}
	if snap.Town.Name != "test-town" {
		t.Errorf("expected town name 'test-town', got %q", snap.Town.Name)
	}

	if len(snap.Polecats) != 3 {
		t.Errorf("expected 3 polecats, got %d", len(snap.Polecats))
	}

	if len(snap.Convoys) != 2 {
		t.Errorf("expected 2 convoys, got %d", len(snap.Convoys))
	}

	if len(snap.Issues) != 3 {
		t.Errorf("expected 3 issues, got %d", len(snap.Issues))
	}

	// Check merge queues
	if len(snap.MergeQueues["perch"]) != 2 {
		t.Errorf("expected 2 MRs for perch, got %d", len(snap.MergeQueues["perch"]))
	}

	// Verify RigNames helper
	names := snap.RigNames()
	if len(names) != 2 {
		t.Errorf("expected 2 rig names, got %d", len(names))
	}
}

func TestLoaderLoadAllWithPartialErrors(t *testing.T) {
	fixtures := testutil.NewFixtures()
	mock := testutil.NewMockRunner()

	// Town status works
	mock.On([]string{"gt", "status", "--json"}, fixtures.TownStatusJSON(), nil, nil)
	// Polecats fail
	mock.On([]string{"gt", "polecat", "list"}, nil, []byte("error"), errors.New("failed"))
	// Convoys work
	mock.On([]string{"gt", "convoy", "list"}, fixtures.ConvoysJSON(), nil, nil)
	// Issues work
	mock.On([]string{"bd", "list", "--json", "--limit", "0"}, fixtures.IssuesJSON(), nil, nil)
	// MQ works
	mock.On([]string{"gt", "mq", "list"}, []byte("[]"), nil, nil)

	loader := NewLoaderWithRunner("/tmp/town", mock)
	snap := loader.LoadAll(context.Background())

	// Should have partial data with errors
	if !snap.HasErrors() {
		t.Error("expected snapshot to have errors")
	}

	// Town should still be loaded
	if snap.Town == nil {
		t.Error("expected town despite errors")
	}

	// Polecats should be nil due to error
	if snap.Polecats != nil {
		t.Error("expected nil polecats due to error")
	}

	// Convoys should still be loaded
	if len(snap.Convoys) != 2 {
		t.Errorf("expected 2 convoys despite errors, got %d", len(snap.Convoys))
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
