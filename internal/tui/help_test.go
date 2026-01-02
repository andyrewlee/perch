package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHelpOverlay_Toggle(t *testing.T) {
	m := New()
	m.width = 100
	m.height = 40
	m.ready = true

	// Initially help should not be shown
	if m.showHelp {
		t.Error("expected showHelp to be false initially")
	}

	// Press '?' to show help
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if !m.showHelp {
		t.Error("expected showHelp to be true after pressing '?'")
	}

	// Press any key to dismiss
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(Model)
	if m.showHelp {
		t.Error("expected showHelp to be false after pressing any key")
	}
}

func TestHelpOverlay_FirstRun(t *testing.T) {
	m := NewFirstRun()
	m.width = 100
	m.height = 40
	m.ready = true

	if !m.firstRun {
		t.Error("expected firstRun to be true for NewFirstRun()")
	}
	if !m.showHelp {
		t.Error("expected showHelp to be true for NewFirstRun()")
	}

	// View should show the help overlay
	view := m.View()
	if !strings.Contains(view, "Welcome") {
		t.Error("expected welcome message in first run help overlay")
	}
}

func TestHelpOverlay_Content(t *testing.T) {
	m := New()
	m.width = 100
	m.height = 40
	m.ready = true
	m.showHelp = true

	view := m.View()

	// Check for key concepts
	concepts := []string{
		"Rigs",
		"Polecats",
		"Witness",
		"Refinery",
		"Convoys",
		"Hooks",
		"Beads",
	}

	for _, concept := range concepts {
		if !strings.Contains(view, concept) {
			t.Errorf("expected help overlay to contain %q", concept)
		}
	}

	// Check for keymap
	keys := []string{
		"tab",
		"shift+tab",
		"?",
		"q",
	}

	for _, key := range keys {
		if !strings.Contains(view, key) {
			t.Errorf("expected help overlay to contain keymap entry %q", key)
		}
	}
}

func TestHelpOverlay_DismissMessage(t *testing.T) {
	m := New()
	m.width = 100
	m.height = 40
	m.ready = true
	m.showHelp = true

	view := m.View()

	if !strings.Contains(view, "Press any key") {
		t.Error("expected dismiss message in help overlay")
	}
}

func TestHelpOverlay_BlocksOtherKeys(t *testing.T) {
	m := New()
	m.width = 100
	m.height = 40
	m.ready = true
	m.showHelp = true
	originalFocus := m.focus

	// Try to switch panels with tab while help is shown
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	// Tab should dismiss help, not change focus
	if m.showHelp {
		t.Error("expected help to be dismissed")
	}
	if m.focus != originalFocus {
		t.Error("expected focus to remain unchanged when dismissing help")
	}
}

func TestFooterShowsHelpHint(t *testing.T) {
	m := New()
	m.width = 100
	m.height = 40
	m.ready = true

	view := m.View()

	// Footer should show help key hint
	if !strings.Contains(view, "?:") || !strings.Contains(view, "help") {
		t.Error("expected footer to show help key hint")
	}
}
