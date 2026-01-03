package tui

import (
	"strings"
	"testing"

	"github.com/andyrewlee/perch/data"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewRigSettingsForm(t *testing.T) {
	t.Run("with nil settings", func(t *testing.T) {
		form := NewRigSettingsForm("test-rig", nil)

		if form.RigName() != "test-rig" {
			t.Errorf("expected rig name 'test-rig', got %q", form.RigName())
		}

		// Defaults should be set
		if !form.MQEnabled() {
			t.Error("expected MQ enabled by default")
		}
		if !form.RunTests() {
			t.Error("expected run tests by default")
		}
		if form.Prefix() != "" {
			t.Errorf("expected empty prefix, got %q", form.Prefix())
		}
	})

	t.Run("with existing settings", func(t *testing.T) {
		settings := &data.RigSettings{
			Name:       "my-rig",
			Prefix:     "mr",
			Theme:      "dark",
			MaxWorkers: 5,
			MergeQueue: data.MergeQueueConfig{
				Enabled:     false,
				RunTests:    true,
				TestCommand: "make test",
			},
		}

		form := NewRigSettingsForm("my-rig", settings)

		if form.Prefix() != "mr" {
			t.Errorf("expected prefix 'mr', got %q", form.Prefix())
		}
		if form.Theme() != "dark" {
			t.Errorf("expected theme 'dark', got %q", form.Theme())
		}
		if form.MaxWorkers() != 5 {
			t.Errorf("expected max workers 5, got %d", form.MaxWorkers())
		}
		if form.MQEnabled() {
			t.Error("expected MQ disabled")
		}
		if !form.RunTests() {
			t.Error("expected run tests enabled")
		}
		if form.TestCommand() != "make test" {
			t.Errorf("expected test command 'make test', got %q", form.TestCommand())
		}
	})
}

func TestRigSettingsFormValidation(t *testing.T) {
	t.Run("valid with prefix", func(t *testing.T) {
		settings := &data.RigSettings{
			Prefix: "tr",
			MergeQueue: data.MergeQueueConfig{
				RunTests:    false,
			},
		}
		form := NewRigSettingsForm("test-rig", settings)

		if !form.IsValid() {
			t.Errorf("expected valid, got error: %s", form.ValidationError())
		}
	})

	t.Run("invalid without prefix", func(t *testing.T) {
		form := NewRigSettingsForm("test-rig", nil)

		if form.IsValid() {
			t.Error("expected invalid when prefix is empty")
		}
		if form.ValidationError() != "Prefix is required" {
			t.Errorf("expected prefix required error, got: %s", form.ValidationError())
		}
	})

	t.Run("invalid max workers", func(t *testing.T) {
		settings := &data.RigSettings{
			Prefix: "tr",
		}
		form := NewRigSettingsForm("test-rig", settings)

		// Simulate entering invalid max workers (we test via form state)
		// The actual input would be validated in the form
		// For now we test the ToSettings conversion
		result := form.ToSettings()
		if result.Prefix != "tr" {
			t.Errorf("expected prefix 'tr', got %q", result.Prefix)
		}
	})
}

func TestRigSettingsFormToSettings(t *testing.T) {
	settings := &data.RigSettings{
		Prefix:     "tr",
		Theme:      "light",
		MaxWorkers: 3,
		MergeQueue: data.MergeQueueConfig{
			Enabled:     true,
			RunTests:    true,
			TestCommand: "npm test",
		},
	}

	form := NewRigSettingsForm("test-rig", settings)
	result := form.ToSettings()

	if result.Name != "test-rig" {
		t.Errorf("expected name 'test-rig', got %q", result.Name)
	}
	if result.Prefix != "tr" {
		t.Errorf("expected prefix 'tr', got %q", result.Prefix)
	}
	if result.Theme != "light" {
		t.Errorf("expected theme 'light', got %q", result.Theme)
	}
	if result.MaxWorkers != 3 {
		t.Errorf("expected max workers 3, got %d", result.MaxWorkers)
	}
	if !result.MergeQueue.Enabled {
		t.Error("expected MQ enabled")
	}
	if !result.MergeQueue.RunTests {
		t.Error("expected run tests enabled")
	}
	if result.MergeQueue.TestCommand != "npm test" {
		t.Errorf("expected test command 'npm test', got %q", result.MergeQueue.TestCommand)
	}
}

func TestRigSettingsFormNavigation(t *testing.T) {
	settings := &data.RigSettings{
		Prefix: "tr",
	}
	form := NewRigSettingsForm("test-rig", settings)

	// Initial focus should be on first input
	if form.focusIndex != 0 {
		t.Errorf("expected initial focus 0, got %d", form.focusIndex)
	}

	// Tab moves to next field
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	if form.focusIndex != 1 {
		t.Errorf("expected focus 1 after tab, got %d", form.focusIndex)
	}

	// Shift+tab moves back
	form.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if form.focusIndex != 0 {
		t.Errorf("expected focus 0 after shift+tab, got %d", form.focusIndex)
	}

	// Navigate to toggle fields (beyond text inputs)
	// Starting at 0, tab settingsFieldCount times to reach index settingsFieldCount
	for i := 0; i < settingsFieldCount; i++ {
		form.Update(tea.KeyMsg{Type: tea.KeyTab})
	}

	// Should now be on first toggle
	if form.focusIndex != settingsFieldCount {
		t.Errorf("expected focus on first toggle (%d), got %d", settingsFieldCount, form.focusIndex)
	}
}

func TestRigSettingsFormToggles(t *testing.T) {
	settings := &data.RigSettings{
		Prefix: "tr",
		MergeQueue: data.MergeQueueConfig{
			Enabled:  true,
			RunTests: true,
		},
	}
	form := NewRigSettingsForm("test-rig", settings)

	// Navigate to MQ enabled toggle
	for i := 0; i < settingsFieldCount; i++ {
		form.Update(tea.KeyMsg{Type: tea.KeyTab})
	}

	// Verify we're on the toggle
	if !form.isOnToggle() {
		t.Error("expected to be on toggle field")
	}

	// Initial state
	if !form.MQEnabled() {
		t.Error("expected MQ enabled initially")
	}

	// Space should toggle
	form.Update(tea.KeyMsg{Type: tea.KeySpace})
	if form.MQEnabled() {
		t.Error("expected MQ disabled after toggle")
	}

	// Toggle again
	form.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !form.MQEnabled() {
		t.Error("expected MQ enabled after second toggle")
	}
}

func TestRigSettingsFormSubmit(t *testing.T) {
	settings := &data.RigSettings{
		Prefix: "tr",
		MergeQueue: data.MergeQueueConfig{
			RunTests: false,
		},
	}
	form := NewRigSettingsForm("test-rig", settings)

	// Should not be submitted initially
	if form.IsSubmitted() {
		t.Error("form should not be submitted initially")
	}

	// Navigate to last field and press enter
	totalFields := form.totalFields()
	for i := 0; i < totalFields-1; i++ {
		form.Update(tea.KeyMsg{Type: tea.KeyTab})
	}

	// Now on last field, enter should submit
	form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !form.IsSubmitted() {
		t.Error("form should be submitted after enter on last field")
	}
}

func TestRigSettingsFormCancel(t *testing.T) {
	form := NewRigSettingsForm("test-rig", nil)

	// Should not be cancelled initially
	if form.IsCancelled() {
		t.Error("form should not be cancelled initially")
	}

	// Escape should cancel
	form.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !form.IsCancelled() {
		t.Error("form should be cancelled after escape")
	}
}

func TestRigSettingsFormView(t *testing.T) {
	settings := &data.RigSettings{
		Prefix:     "tr",
		Theme:      "dark",
		MaxWorkers: 3,
		MergeQueue: data.MergeQueueConfig{
			Enabled:     true,
			RunTests:    true,
			TestCommand: "go test ./...",
		},
	}
	form := NewRigSettingsForm("test-rig", settings)

	view := form.View(80, 40)

	// Check that the view contains expected elements
	if !strings.Contains(view, "Edit Rig:") {
		t.Error("expected view to contain 'Edit Rig:'")
	}
	if !strings.Contains(view, "test-rig") {
		t.Error("expected view to contain rig name 'test-rig'")
	}
	if !strings.Contains(view, "Prefix") {
		t.Error("expected view to contain 'Prefix' label")
	}
	if !strings.Contains(view, "Theme") {
		t.Error("expected view to contain 'Theme' label")
	}
	if !strings.Contains(view, "Max Workers") {
		t.Error("expected view to contain 'Max Workers' label")
	}
	if !strings.Contains(view, "Merge Queue") {
		t.Error("expected view to contain 'Merge Queue' section")
	}
	if !strings.Contains(view, "Enabled") {
		t.Error("expected view to contain 'Enabled' toggle")
	}
	if !strings.Contains(view, "Run Tests") {
		t.Error("expected view to contain 'Run Tests' toggle")
	}
	if !strings.Contains(view, "Test Command") {
		t.Error("expected view to contain 'Test Command' label")
	}
}
