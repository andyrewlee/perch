package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/andyrewlee/perch/internal/testutil"
)

// TestActionRunnerWithMock demonstrates using MockRunner to test ActionRunner
// without requiring real gt CLI tools.
func TestActionRunnerWithMock(t *testing.T) {
	t.Run("BootRig_Success", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "rig", "boot", "perch"}, []byte("Booting rig perch..."), nil, nil)

		runner := NewActionRunnerWithRunner("/tmp/town", mock)
		err := runner.BootRig(context.Background(), "perch")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !mock.CalledWith([]string{"gt", "rig", "boot", "perch"}) {
			t.Error("expected gt rig boot perch to be called")
		}
	})

	t.Run("BootRig_Error", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "rig", "boot"}, nil, []byte("rig not found"), errors.New("exit status 1"))

		runner := NewActionRunnerWithRunner("/tmp/town", mock)
		err := runner.BootRig(context.Background(), "unknown-rig")

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "rig not found") {
			t.Errorf("error should contain stderr: %v", err)
		}
	})

	t.Run("ShutdownRig_Success", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "rig", "shutdown", "perch"}, []byte("Shutting down..."), nil, nil)

		runner := NewActionRunnerWithRunner("/tmp/town", mock)
		err := runner.ShutdownRig(context.Background(), "perch")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !mock.CalledWith([]string{"gt", "rig", "shutdown", "perch"}) {
			t.Error("expected gt rig shutdown perch to be called")
		}
	})

	t.Run("ShutdownRig_Error", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "rig", "shutdown"}, nil, []byte("agents still running"), errors.New("exit status 1"))

		runner := NewActionRunnerWithRunner("/tmp/town", mock)
		err := runner.ShutdownRig(context.Background(), "perch")

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("OpenLogs_Success", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "log", "--agent", "perch/polecats/able", "-f"}, []byte(""), nil, nil)

		runner := NewActionRunnerWithRunner("/tmp/town", mock)
		err := runner.OpenLogs(context.Background(), "perch/polecats/able")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !mock.CalledWith([]string{"gt", "log", "--agent", "perch/polecats/able", "-f"}) {
			t.Error("expected gt log --agent to be called with agent address")
		}
	})

	t.Run("OpenLogs_Error", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.On([]string{"gt", "log"}, nil, []byte("agent not found"), errors.New("exit status 1"))

		runner := NewActionRunnerWithRunner("/tmp/town", mock)
		err := runner.OpenLogs(context.Background(), "unknown/agent")

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestActionRunnerCallSequence(t *testing.T) {
	// Test that multiple actions are tracked correctly
	mock := testutil.NewMockRunner()
	mock.DefaultStdout = []byte("")

	runner := NewActionRunnerWithRunner("/tmp/town", mock)

	// Perform a sequence of actions
	runner.BootRig(context.Background(), "rig1")
	runner.BootRig(context.Background(), "rig2")
	runner.ShutdownRig(context.Background(), "rig1")

	calls := mock.Calls()
	if len(calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(calls))
	}

	// Verify call sequence
	if calls[0].Args[3] != "rig1" {
		t.Errorf("first call should be for rig1, got %v", calls[0].Args)
	}
	if calls[1].Args[3] != "rig2" {
		t.Errorf("second call should be for rig2, got %v", calls[1].Args)
	}
	if calls[2].Args[2] != "shutdown" {
		t.Errorf("third call should be shutdown, got %v", calls[2].Args)
	}
}

func TestActionRunnerWorkDir(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.DefaultStdout = []byte("")

	townRoot := "/custom/town/root"
	runner := NewActionRunnerWithRunner(townRoot, mock)

	runner.BootRig(context.Background(), "perch")

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].WorkDir != townRoot {
		t.Errorf("expected workDir %q, got %q", townRoot, calls[0].WorkDir)
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
