package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/andyrewlee/perch/internal/testutil"
)

func TestCheckDependencies_AllFound(t *testing.T) {
	mock := testutil.NewMockRunner()

	// All dependencies found
	mock.OnMatcher(
		func(args []string) bool { return len(args) >= 2 && args[0] == "which" },
		func(args []string) ([]byte, []byte, error) {
			return []byte("/usr/local/bin/" + args[1]), nil, nil
		},
	)

	result := CheckDependencies(mock, "/tmp")

	if !result.AllFound {
		t.Error("expected all dependencies to be found")
	}

	for _, status := range result.Statuses {
		if !status.Found {
			t.Errorf("expected %s to be found", status.Dependency.Name)
		}
	}

	// Verify all deps were checked
	if !mock.CalledWith([]string{"which", "tmux"}) {
		t.Error("expected which tmux to be called")
	}
	if !mock.CalledWith([]string{"which", "gt"}) {
		t.Error("expected which gt to be called")
	}
	if !mock.CalledWith([]string{"which", "bd"}) {
		t.Error("expected which bd to be called")
	}
}

func TestCheckDependencies_MissingTmux(t *testing.T) {
	mock := testutil.NewMockRunner()

	// tmux not found, others found
	mock.OnMatcher(
		func(args []string) bool { return len(args) >= 2 && args[0] == "which" },
		func(args []string) ([]byte, []byte, error) {
			if args[1] == "tmux" {
				return nil, []byte("tmux not found"), errors.New("exit status 1")
			}
			return []byte("/usr/local/bin/" + args[1]), nil, nil
		},
	)

	result := CheckDependencies(mock, "/tmp")

	if result.AllFound {
		t.Error("expected not all dependencies to be found")
	}

	missing := result.Missing()
	if len(missing) != 1 {
		t.Errorf("expected 1 missing dependency, got %d", len(missing))
	}
	if missing[0].Name != "tmux" {
		t.Errorf("expected tmux to be missing, got %s", missing[0].Name)
	}
}

func TestCheckDependencies_AllMissing(t *testing.T) {
	mock := testutil.NewMockRunner()

	// All dependencies missing
	mock.OnMatcher(
		func(args []string) bool { return len(args) >= 2 && args[0] == "which" },
		func(args []string) ([]byte, []byte, error) {
			return nil, []byte("not found"), errors.New("exit status 1")
		},
	)

	result := CheckDependencies(mock, "/tmp")

	if result.AllFound {
		t.Error("expected not all dependencies to be found")
	}

	missing := result.Missing()
	if len(missing) != 3 {
		t.Errorf("expected 3 missing dependencies, got %d", len(missing))
	}
}

func TestFormatDependencyError(t *testing.T) {
	mock := testutil.NewMockRunner()

	// gt missing
	mock.OnMatcher(
		func(args []string) bool { return len(args) >= 2 && args[0] == "which" },
		func(args []string) ([]byte, []byte, error) {
			if args[1] == "gt" {
				return nil, nil, errors.New("not found")
			}
			return []byte("/usr/local/bin/" + args[1]), nil, nil
		},
	)

	result := CheckDependencies(mock, "/tmp")

	errMsg := FormatDependencyError(result)
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
	if !strings.Contains(errMsg, "gt") {
		t.Error("expected error message to contain 'gt'")
	}
}

func TestFormatDependencyError_AllFound(t *testing.T) {
	mock := testutil.NewMockRunner()

	// All found
	mock.OnMatcher(
		func(args []string) bool { return len(args) >= 2 && args[0] == "which" },
		func(args []string) ([]byte, []byte, error) {
			return []byte("/usr/local/bin/" + args[1]), nil, nil
		},
	)

	result := CheckDependencies(mock, "/tmp")

	errMsg := FormatDependencyError(result)
	if errMsg != "" {
		t.Errorf("expected empty error message, got %q", errMsg)
	}
}

func TestRenderDependencyStatus(t *testing.T) {
	mock := testutil.NewMockRunner()

	// tmux found, gt missing, bd found
	mock.OnMatcher(
		func(args []string) bool { return len(args) >= 2 && args[0] == "which" },
		func(args []string) ([]byte, []byte, error) {
			if args[1] == "gt" {
				return nil, nil, errors.New("not found")
			}
			return []byte("/usr/local/bin/" + args[1]), nil, nil
		},
	)

	result := CheckDependencies(mock, "/tmp")
	output := RenderDependencyStatus(result)

	// Check that all dependencies are listed
	if !strings.Contains(output, "tmux") {
		t.Error("expected output to contain 'tmux'")
	}
	if !strings.Contains(output, "gt") {
		t.Error("expected output to contain 'gt'")
	}
	if !strings.Contains(output, "bd") {
		t.Error("expected output to contain 'bd'")
	}
}

func TestRenderInstallGuidance(t *testing.T) {
	mock := testutil.NewMockRunner()

	// All missing
	mock.OnMatcher(
		func(args []string) bool { return len(args) >= 2 && args[0] == "which" },
		func(args []string) ([]byte, []byte, error) {
			return nil, nil, errors.New("not found")
		},
	)

	result := CheckDependencies(mock, "/tmp")
	output := RenderInstallGuidance(result)

	// Check that install hints are included
	if !strings.Contains(output, "brew install tmux") {
		t.Error("expected output to contain brew install hint for tmux")
	}
}

func TestRenderInstallGuidance_AllFound(t *testing.T) {
	mock := testutil.NewMockRunner()

	// All found
	mock.OnMatcher(
		func(args []string) bool { return len(args) >= 2 && args[0] == "which" },
		func(args []string) ([]byte, []byte, error) {
			return []byte("/usr/local/bin/" + args[1]), nil, nil
		},
	)

	result := CheckDependencies(mock, "/tmp")
	output := RenderInstallGuidance(result)

	if output != "" {
		t.Errorf("expected empty guidance when all deps found, got %q", output)
	}
}
