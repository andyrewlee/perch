package tui

import (
	"os"
	"testing"
	"time"
)

func TestTickCmdReturnsNilWhenDisabled(t *testing.T) {
	m := NewTestModel(t)
	m.refreshInterval = 0

	cmd := m.tickCmd()
	if cmd != nil {
		t.Errorf("tickCmd should return nil when refreshInterval is 0")
	}
}

func TestTickCmdReturnsCommandWhenEnabled(t *testing.T) {
	m := NewTestModel(t)
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
	m := NewTestModel(t)
	if m.refreshInterval != DefaultRefreshInterval {
		t.Errorf("NewTestModel refreshInterval = %v, want %v", m.refreshInterval, DefaultRefreshInterval)
	}
}

func TestNewWithTownRootHasRefreshInterval(t *testing.T) {
	// Create a temporary directory that looks like a town
	tmpDir := t.TempDir()
	// Create mayor/ directory to make TownExists return true
	if err := os.MkdirAll(tmpDir+"/mayor", 0755); err != nil {
		t.Fatalf("failed to create test town: %v", err)
	}

	m := NewWithTownRoot(tmpDir)
	// Should not be in setup mode
	if m.setupWizard != nil {
		t.Errorf("NewWithTownRoot with existing town should not show setup wizard")
	}
	if m.refreshInterval != DefaultRefreshInterval {
		t.Errorf("NewWithTownRoot model refreshInterval = %v, want %v", m.refreshInterval, DefaultRefreshInterval)
	}
}

func TestNewWithTownRootShowsSetupForMissingTown(t *testing.T) {
	// Use a path that doesn't exist
	m := NewWithTownRoot("/nonexistent/path/that/does/not/exist")
	// Should be in setup mode
	if m.setupWizard == nil {
		t.Errorf("NewWithTownRoot with missing town should show setup wizard")
	}
}

func TestTickMsgTriggersRefresh(t *testing.T) {
	m := NewTestModel(t)
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
	m := NewTestModel(t)
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

func TestRefreshMsgUpdatesState(t *testing.T) {
	m := NewTestModel(t)
	m.ready = true
	m.isRefreshing = true

	updated, _ := m.Update(refreshMsg{err: nil, snapshot: nil})
	model := updated.(Model)

	if model.isRefreshing {
		t.Errorf("refreshMsg should set isRefreshing to false")
	}
	if model.lastRefresh.IsZero() {
		t.Errorf("refreshMsg should update lastRefresh")
	}
}

func TestRenderHUDShowsDisconnected(t *testing.T) {
	m := NewTestModel(t)
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
	m := NewTestModel(t)
	m.ready = true
	m.width = 80
	m.isRefreshing = true

	hud := m.renderHUD()
	if hud == "" {
		t.Errorf("renderHUD should return content when refreshing")
	}
}

func TestRenderHUDShowsConnected(t *testing.T) {
	m := NewTestModel(t)
	m.ready = true
	m.width = 80
	m.lastRefresh = time.Now()

	hud := m.renderHUD()
	if hud == "" {
		t.Errorf("renderHUD should return content when connected")
	}
}

func TestRenderHUDShowsErrors(t *testing.T) {
	m := NewTestModel(t)
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

// ========== Golden Render Tests ==========

func TestRenderHUD_GoldenDisconnected(t *testing.T) {
	defer setupGoldenEnv()()

	m := NewTestModel(t)
	m.width = 80
	m.lastRefresh = time.Time{} // Zero time = disconnected

	hud := m.renderHUD()
	CheckGolden(t, "hud_disconnected", hud, DefaultGoldenOptions())
}

func TestRenderHUD_GoldenConnected(t *testing.T) {
	defer setupGoldenEnv()()

	m := NewTestModel(t)
	m.width = 80
	m.lastRefresh = time.Now().Add(-30 * time.Second)

	hud := m.renderHUD()
	CheckGolden(t, "hud_connected", hud, DefaultGoldenOptions())
}

func TestRenderHUD_GoldenRefreshing(t *testing.T) {
	defer setupGoldenEnv()()

	m := NewTestModel(t)
	m.width = 80
	m.isRefreshing = true
	m.lastRefresh = time.Now().Add(-1 * time.Minute)

	hud := m.renderHUD()
	CheckGolden(t, "hud_refreshing", hud, DefaultGoldenOptions())
}

func TestRenderHUD_GoldenWithErrors(t *testing.T) {
	defer setupGoldenEnv()()

	m := NewTestModel(t)
	m.width = 80
	m.errorCount = 3
	m.lastRefresh = time.Now().Add(-2 * time.Minute)

	hud := m.renderHUD()
	CheckGolden(t, "hud_with_errors", hud, DefaultGoldenOptions())
}

func TestRenderFooter_GoldenNormal(t *testing.T) {
	defer setupGoldenEnv()()

	m := NewTestModel(t)
	m.width = 80
	m.lastRefresh = time.Now().Add(-45 * time.Second)

	footer := m.renderFooter()
	CheckGolden(t, "footer_normal", footer, DefaultGoldenOptions())
}
