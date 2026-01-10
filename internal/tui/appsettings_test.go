package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewAppSettingsForm(t *testing.T) {
	interval := 10 * time.Second
	form := NewAppSettingsForm(interval, true)

	if form == nil {
		t.Fatal("NewAppSettingsForm returned nil")
	}

	if form.RefreshInterval() != interval {
		t.Errorf("expected refresh interval %v, got %v", interval, form.RefreshInterval())
	}

	if !form.AutoRefresh() {
		t.Error("expected AutoRefresh to be true")
	}
}

func TestAppSettingsFormRefreshInterval(t *testing.T) {
	form := NewAppSettingsForm(5*time.Second, true)

	// Test setting valid intervals
	testCases := []struct {
		input    string
		expected time.Duration
	}{
		{"10", 10 * time.Second},
		{"30", 30 * time.Second},
		{"60", 60 * time.Second},
		{"", 5 * time.Second}, // empty returns default
		{"0", 5 * time.Second}, // 0 returns default
	}

	for _, tc := range testCases {
		form.inputs[appSettingsInputRefresh].SetValue(tc.input)
		result := form.RefreshInterval()
		if result != tc.expected {
			t.Errorf("input %q: expected %v, got %v", tc.input, tc.expected, result)
		}
	}
}

func TestAppSettingsFormValidation(t *testing.T) {
	form := NewAppSettingsForm(5*time.Second, true)

	// Test valid intervals
	validInputs := []string{"5", "10", "60", "300"}
	for _, input := range validInputs {
		form.inputs[appSettingsInputRefresh].SetValue(input)
		if !form.IsValid() {
			t.Errorf("input %q should be valid, got error: %s", input, form.ValidationError())
		}
	}

	// Test invalid intervals
	invalidInputs := []string{"0", "301", "abc", "-5"}
	for _, input := range invalidInputs {
		form.inputs[appSettingsInputRefresh].SetValue(input)
		if form.IsValid() {
			t.Errorf("input %q should be invalid", input)
		}
	}
}

func TestAppSettingsFormNavigation(t *testing.T) {
	form := NewAppSettingsForm(5*time.Second, true)

	// Initial focus on first input
	if form.focusIndex != 0 {
		t.Errorf("expected initial focus index 0, got %d", form.focusIndex)
	}

	// Tab moves to toggle
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	if form.focusIndex != appSettingsFieldCount {
		t.Errorf("expected focus index %d after tab, got %d", appSettingsFieldCount, form.focusIndex)
	}

	// Space toggles the checkbox (on toggle field)
	initialState := form.toggles[appSettingsToggleAutoRefresh]
	form.Update(tea.KeyMsg{Type: tea.KeySpace})
	if form.toggles[appSettingsToggleAutoRefresh] == initialState {
		t.Error("space should toggle the checkbox")
	}

	// Esc cancels
	form.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !form.cancelled {
		t.Error("esc should cancel the form")
	}
}

func TestAppSettingsFormAutoRefreshToggle(t *testing.T) {
	form := NewAppSettingsForm(5*time.Second, true)

	// Toggle is initially true
	if !form.AutoRefresh() {
		t.Error("expected initial AutoRefresh to be true")
	}

	// Toggle it off
	form.toggles[appSettingsToggleAutoRefresh] = false
	if form.AutoRefresh() {
		t.Error("expected AutoRefresh to be false after toggle")
	}
}
