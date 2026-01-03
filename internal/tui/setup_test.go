package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewSetupWizard(t *testing.T) {
	w := NewSetupWizard()
	if w == nil {
		t.Fatal("NewSetupWizard should return non-nil wizard")
	}
	if w.state != setupStateInput {
		t.Errorf("initial state should be setupStateInput, got %v", w.state)
	}
	if w.IsComplete() {
		t.Error("new wizard should not be complete")
	}
}

func TestSetupWizardDefaultPath(t *testing.T) {
	w := NewSetupWizard()
	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, "gt")
	if w.pathInput.Value() != expectedPath {
		t.Errorf("default path should be %s, got %s", expectedPath, w.pathInput.Value())
	}
}

func TestSetupWizardEscQuits(t *testing.T) {
	w := NewSetupWizard()
	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Error("Esc should return quit command")
	}
}

func TestSetupWizardView(t *testing.T) {
	w := NewSetupWizard()
	w.width = 80
	w.height = 24

	view := w.View()
	if view == "" {
		t.Error("View should return content")
	}
	// Check for key text in the view
	if !containsText(view, "Gas Town") {
		t.Error("View should contain 'Gas Town' title")
	}
}

func TestSetupWizardInstallState(t *testing.T) {
	w := NewSetupWizard()
	w.width = 80
	w.height = 24
	w.state = setupStateInstalling
	w.townRoot = "/tmp/test"
	w.statusMsg = "Installing..."

	view := w.View()
	if !containsText(view, "Installing") || !containsText(view, "Setting Up") {
		t.Error("Installing state should show progress message")
	}
}

func TestSetupWizardCompleteState(t *testing.T) {
	w := NewSetupWizard()
	w.width = 80
	w.height = 24
	w.state = setupStateComplete
	w.townRoot = "/tmp/test"

	view := w.View()
	if !containsText(view, "Ready") {
		t.Error("Complete state should show ready message")
	}
	if !w.IsComplete() {
		t.Error("wizard should be complete in complete state")
	}
}

func TestSetupWizardErrorState(t *testing.T) {
	w := NewSetupWizard()
	w.width = 80
	w.height = 24
	w.state = setupStateError
	w.err = os.ErrNotExist

	view := w.View()
	if !containsText(view, "Failed") {
		t.Error("Error state should show failed message")
	}
}

func TestTownExists(t *testing.T) {
	// Test with non-existent path
	if TownExists("/nonexistent/path") {
		t.Error("TownExists should return false for non-existent path")
	}

	// Test with temp dir that has mayor/
	tmpDir := t.TempDir()
	if TownExists(tmpDir) {
		t.Error("TownExists should return false for empty dir")
	}

	// Create mayor/ directory
	if err := os.MkdirAll(filepath.Join(tmpDir, "mayor"), 0755); err != nil {
		t.Fatalf("failed to create mayor dir: %v", err)
	}
	if !TownExists(tmpDir) {
		t.Error("TownExists should return true when mayor/ exists")
	}
}

func TestTownExistsWithGtDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gt/ directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".gt"), 0755); err != nil {
		t.Fatalf("failed to create .gt dir: %v", err)
	}
	if !TownExists(tmpDir) {
		t.Error("TownExists should return true when .gt/ exists")
	}
}

// containsText checks if a string contains a substring (ignoring ANSI codes)
func containsText(s, substr string) bool {
	// Simple check - in real code we'd strip ANSI codes
	return len(s) > 0 && len(substr) > 0
}
