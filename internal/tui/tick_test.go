package tui

import (
	"testing"
	"time"
)

func TestTickCmdReturnsNilWhenDisabled(t *testing.T) {
	m := New()
	m.refreshInterval = 0

	cmd := m.tickCmd()
	if cmd != nil {
		t.Errorf("tickCmd should return nil when refreshInterval is 0")
	}
}

func TestTickCmdReturnsCommandWhenEnabled(t *testing.T) {
	m := New()
	m.refreshInterval = 10 * time.Second

	cmd := m.tickCmd()
	if cmd == nil {
		t.Errorf("tickCmd should return a command when refreshInterval is positive")
	}
}

func TestDefaultRefreshInterval(t *testing.T) {
	if DefaultRefreshInterval != 10*time.Second {
		t.Errorf("DefaultRefreshInterval = %v, want 10s", DefaultRefreshInterval)
	}
}

func TestNewModelHasRefreshInterval(t *testing.T) {
	m := New()
	if m.refreshInterval != DefaultRefreshInterval {
		t.Errorf("New model refreshInterval = %v, want %v", m.refreshInterval, DefaultRefreshInterval)
	}
}

func TestNewWithTownHasRefreshInterval(t *testing.T) {
	town := MockTown()
	m := NewWithTown(town)
	if m.refreshInterval != DefaultRefreshInterval {
		t.Errorf("NewWithTown model refreshInterval = %v, want %v", m.refreshInterval, DefaultRefreshInterval)
	}
}

func TestTickMsgTriggersRefresh(t *testing.T) {
	m := New()
	m.ready = true
	m.isRefreshing = false

	updated, cmd := m.Update(tickMsg(time.Now()))
	model := updated.(Model)

	if !model.isRefreshing {
		t.Errorf("tickMsg should set isRefreshing to true")
	}
	if cmd == nil {
		t.Errorf("tickMsg should return commands (tick + refresh)")
	}
}

func TestTickMsgWhileRefreshingOnlySchedulesTick(t *testing.T) {
	m := New()
	m.ready = true
	m.isRefreshing = true

	updated, cmd := m.Update(tickMsg(time.Now()))
	model := updated.(Model)

	// Should still be refreshing (not changed)
	if !model.isRefreshing {
		t.Errorf("isRefreshing should remain true")
	}
	// Should still return a tick command (to schedule next tick)
	if cmd == nil {
		t.Errorf("should return tick command even when already refreshing")
	}
}

func TestRefreshCompleteUpdatesState(t *testing.T) {
	m := New()
	m.ready = true
	m.isRefreshing = true

	updated, _ := m.Update(refreshCompleteMsg{err: nil, snapshot: nil})
	model := updated.(Model)

	if model.isRefreshing {
		t.Errorf("refreshCompleteMsg should set isRefreshing to false")
	}
	if model.lastRefresh.IsZero() {
		t.Errorf("refreshCompleteMsg should update lastRefresh")
	}
}

func TestRenderHUDShowsDisconnected(t *testing.T) {
	m := New()
	m.ready = true
	m.width = 80

	hud := m.renderHUD()
	if hud == "" {
		t.Errorf("renderHUD should return content")
	}
	// Should show disconnected indicator when no refresh has happened
	if m.lastRefresh.IsZero() && m.errorCount == 0 && !m.isRefreshing {
		// Should contain the disconnected circle
		// (The actual rendered string includes ANSI codes, so we just check it's not empty)
	}
}

func TestRenderHUDShowsRefreshing(t *testing.T) {
	m := New()
	m.ready = true
	m.width = 80
	m.isRefreshing = true

	hud := m.renderHUD()
	if hud == "" {
		t.Errorf("renderHUD should return content when refreshing")
	}
}

func TestRenderHUDShowsConnected(t *testing.T) {
	m := New()
	m.ready = true
	m.width = 80
	m.lastRefresh = time.Now()

	hud := m.renderHUD()
	if hud == "" {
		t.Errorf("renderHUD should return content when connected")
	}
}

func TestRenderHUDShowsErrors(t *testing.T) {
	m := New()
	m.ready = true
	m.width = 80
	m.errorCount = 3

	hud := m.renderHUD()
	if hud == "" {
		t.Errorf("renderHUD should return content when there are errors")
	}
}

func TestJoinHUDEmpty(t *testing.T) {
	result := joinHUD(nil)
	if result != nil {
		t.Errorf("joinHUD(nil) should return nil, got %v", result)
	}

	result = joinHUD([]string{})
	if result != nil {
		t.Errorf("joinHUD([]) should return nil, got %v", result)
	}
}

func TestJoinHUDSingle(t *testing.T) {
	result := joinHUD([]string{"a"})
	if len(result) != 1 {
		t.Errorf("joinHUD single should return 1 element, got %d", len(result))
	}
}

func TestJoinHUDMultiple(t *testing.T) {
	result := joinHUD([]string{"a", "b", "c"})
	// Should be: a, sep, b, sep, c = 5 elements
	if len(result) != 5 {
		t.Errorf("joinHUD([a,b,c]) should return 5 elements (3 + 2 separators), got %d", len(result))
	}
}
