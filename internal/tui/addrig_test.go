package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewAddRigForm(t *testing.T) {
	form := NewAddRigForm()

	if form == nil {
		t.Fatal("NewAddRigForm returned nil")
	}

	if len(form.inputs) != 3 {
		t.Errorf("expected 3 inputs, got %d", len(form.inputs))
	}

	if form.focusIndex != 0 {
		t.Errorf("expected focusIndex=0, got %d", form.focusIndex)
	}

	if form.submitted {
		t.Error("expected submitted=false")
	}

	if form.cancelled {
		t.Error("expected cancelled=false")
	}
}

func TestAddRigFormInitialFocus(t *testing.T) {
	form := NewAddRigForm()

	// First input (name) should be focused
	if !form.inputs[addRigInputName].Focused() {
		t.Error("expected name input to be focused initially")
	}

	if form.inputs[addRigInputURL].Focused() {
		t.Error("expected URL input not to be focused initially")
	}

	if form.inputs[addRigInputPrefix].Focused() {
		t.Error("expected prefix input not to be focused initially")
	}
}

func TestAddRigFormValidation(t *testing.T) {
	tests := []struct {
		name      string
		rigName   string
		gitURL    string
		wantValid bool
	}{
		{"empty both", "", "", false},
		{"name only", "my-rig", "", false},
		{"url only", "", "https://github.com/user/repo", false},
		{"both filled", "my-rig", "https://github.com/user/repo", true},
		{"whitespace name", "  ", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := NewAddRigForm()
			form.inputs[addRigInputName].SetValue(tt.rigName)
			form.inputs[addRigInputURL].SetValue(tt.gitURL)

			got := form.IsValid()
			if got != tt.wantValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestAddRigFormGetters(t *testing.T) {
	form := NewAddRigForm()
	form.inputs[addRigInputName].SetValue("test-rig")
	form.inputs[addRigInputURL].SetValue("git@github.com:user/repo.git")
	form.inputs[addRigInputPrefix].SetValue("tr")

	if form.Name() != "test-rig" {
		t.Errorf("Name() = %q, want %q", form.Name(), "test-rig")
	}

	if form.URL() != "git@github.com:user/repo.git" {
		t.Errorf("URL() = %q, want %q", form.URL(), "git@github.com:user/repo.git")
	}

	if form.Prefix() != "tr" {
		t.Errorf("Prefix() = %q, want %q", form.Prefix(), "tr")
	}
}

func TestAddRigFormEscapeCancels(t *testing.T) {
	form := NewAddRigForm()

	if form.IsCancelled() {
		t.Error("form should not be cancelled initially")
	}

	form.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !form.IsCancelled() {
		t.Error("form should be cancelled after pressing esc")
	}
}

func TestAddRigFormTabNavigation(t *testing.T) {
	form := NewAddRigForm()

	// Initial focus is on name (index 0)
	if form.focusIndex != 0 {
		t.Fatalf("expected focusIndex=0, got %d", form.focusIndex)
	}

	// Tab should move to URL (index 1)
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	if form.focusIndex != 1 {
		t.Errorf("after tab, expected focusIndex=1, got %d", form.focusIndex)
	}

	// Tab should move to prefix (index 2)
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	if form.focusIndex != 2 {
		t.Errorf("after second tab, expected focusIndex=2, got %d", form.focusIndex)
	}

	// Tab should wrap to name (index 0)
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	if form.focusIndex != 0 {
		t.Errorf("after third tab, expected focusIndex=0 (wrap), got %d", form.focusIndex)
	}
}

func TestAddRigFormShiftTabNavigation(t *testing.T) {
	form := NewAddRigForm()

	// Initial focus is on name (index 0)
	// Shift+tab should wrap to prefix (index 2)
	form.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if form.focusIndex != 2 {
		t.Errorf("after shift+tab from 0, expected focusIndex=2, got %d", form.focusIndex)
	}

	// Shift+tab should go to URL (index 1)
	form.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if form.focusIndex != 1 {
		t.Errorf("after second shift+tab, expected focusIndex=1, got %d", form.focusIndex)
	}
}

func TestAddRigFormEnterOnLastFieldSubmits(t *testing.T) {
	form := NewAddRigForm()
	form.inputs[addRigInputName].SetValue("my-rig")
	form.inputs[addRigInputURL].SetValue("https://github.com/user/repo")

	// Navigate to last field (prefix)
	form.focusIndex = 2

	if form.IsSubmitted() {
		t.Error("form should not be submitted initially")
	}

	// Press enter on last field
	form.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !form.IsSubmitted() {
		t.Error("form should be submitted after enter on last field with valid data")
	}
}

func TestAddRigFormEnterOnLastFieldDoesNotSubmitIfInvalid(t *testing.T) {
	form := NewAddRigForm()
	// Leave required fields empty

	// Navigate to last field
	form.focusIndex = 2

	// Press enter on last field
	form.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if form.IsSubmitted() {
		t.Error("form should not be submitted with invalid data")
	}
}

func TestAddRigFormEnterOnNonLastFieldAdvances(t *testing.T) {
	form := NewAddRigForm()

	// Press enter on first field (name)
	form.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if form.focusIndex != 1 {
		t.Errorf("after enter on name field, expected focusIndex=1, got %d", form.focusIndex)
	}

	if form.IsSubmitted() {
		t.Error("form should not be submitted by enter on non-last field")
	}
}

func TestAddRigFormView(t *testing.T) {
	form := NewAddRigForm()
	form.inputs[addRigInputName].SetValue("test-rig")

	view := form.View(80, 24)

	// Check that essential elements are present
	if !strings.Contains(view, "Add New Rig") {
		t.Error("view should contain title 'Add New Rig'")
	}

	if !strings.Contains(view, "Name") {
		t.Error("view should contain 'Name' label")
	}

	if !strings.Contains(view, "Git URL") {
		t.Error("view should contain 'Git URL' label")
	}

	if !strings.Contains(view, "Prefix") {
		t.Error("view should contain 'Prefix' label")
	}

	if !strings.Contains(view, "Tab: next field") {
		t.Error("view should contain help text")
	}
}

func TestAddRigFormViewShowsValidationError(t *testing.T) {
	form := NewAddRigForm()
	// Set name but leave URL empty (triggers validation message)
	form.inputs[addRigInputName].SetValue("test-rig")

	view := form.View(80, 24)

	if !strings.Contains(view, "Git URL is required") {
		t.Error("view should show 'Git URL is required' validation error")
	}
}

func TestAddRigFormViewShowsNameRequiredError(t *testing.T) {
	form := NewAddRigForm()
	// Set URL but leave name empty
	form.inputs[addRigInputURL].SetValue("https://github.com/user/repo")

	view := form.View(80, 24)

	if !strings.Contains(view, "Name is required") {
		t.Error("view should show 'Name is required' validation error")
	}
}

func TestAddRigFormDownKeyNavigation(t *testing.T) {
	form := NewAddRigForm()

	// Down should work like tab
	form.Update(tea.KeyMsg{Type: tea.KeyDown})
	if form.focusIndex != 1 {
		t.Errorf("after down, expected focusIndex=1, got %d", form.focusIndex)
	}
}

func TestAddRigFormUpKeyNavigation(t *testing.T) {
	form := NewAddRigForm()
	form.focusIndex = 1

	// Up should work like shift+tab
	form.Update(tea.KeyMsg{Type: tea.KeyUp})
	if form.focusIndex != 0 {
		t.Errorf("after up from 1, expected focusIndex=0, got %d", form.focusIndex)
	}
}

func TestAddRigFormPrefixIsOptional(t *testing.T) {
	form := NewAddRigForm()
	form.inputs[addRigInputName].SetValue("my-rig")
	form.inputs[addRigInputURL].SetValue("https://github.com/user/repo")
	// Leave prefix empty

	if !form.IsValid() {
		t.Error("form should be valid without prefix (prefix is optional)")
	}

	if form.Prefix() != "" {
		t.Errorf("Prefix() should be empty, got %q", form.Prefix())
	}
}
