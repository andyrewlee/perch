package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/andyrewlee/perch/data"
)

func TestBuildAlerts_StaleWhenWatchdogDown(t *testing.T) {
	// When watchdog is unhealthy, load errors should show as stale, not failed
	m := NewTestModel(t)
	m.snapshot = &data.Snapshot{
		Town: &data.TownStatus{
			Name: "test-town",
		},
		OperationalState: &data.OperationalState{
			WatchdogHealthy: false, // Deacon is down
		},
		LoadErrors: []data.LoadError{
			{Source: "mail", Error: "connection refused", OccurredAt: time.Now()},
			{Source: "convoys", Error: "connection refused", OccurredAt: time.Now()},
		},
		LastSuccess: map[string]time.Time{
			"mail":    time.Now().Add(-5 * time.Minute),
			"convoys": time.Now().Add(-3 * time.Minute),
		},
	}

	alerts := m.buildAlerts()

	// Should have an alert
	if len(alerts) == 0 {
		t.Fatal("expected at least one alert")
	}

	// Should NOT show "failed" - should show "stale"
	alertText := strings.Join(alerts, "\n")
	if strings.Contains(alertText, "failed") {
		t.Errorf("expected stale message, not failure message, got: %s", alertText)
	}
	if !strings.Contains(alertText, "stale") && !strings.Contains(alertText, "Data stale") {
		t.Errorf("expected 'stale' in alert text, got: %s", alertText)
	}
}

func TestBuildAlerts_FailedWhenWatchdogHealthy(t *testing.T) {
	// When watchdog is healthy, load errors should show as failed
	m := NewTestModel(t)
	m.snapshot = &data.Snapshot{
		Town: &data.TownStatus{
			Name: "test-town",
		},
		OperationalState: &data.OperationalState{
			WatchdogHealthy: true, // Deacon is running
		},
		LoadErrors: []data.LoadError{
			{Source: "mail", Error: "command not found", OccurredAt: time.Now()},
		},
		LastSuccess: map[string]time.Time{},
	}

	alerts := m.buildAlerts()

	// Should have an alert
	if len(alerts) == 0 {
		t.Fatal("expected at least one alert")
	}

	// Should show "failed" not "stale"
	alertText := strings.Join(alerts, "\n")
	if !strings.Contains(alertText, "failed") {
		t.Errorf("expected 'failed' in alert text, got: %s", alertText)
	}
}

func TestBuildAlerts_StaleShowsLastRefreshTime(t *testing.T) {
	// When showing stale data, should include last refresh times
	m := NewTestModel(t)
	lastRefresh := time.Date(2025, 1, 5, 14, 30, 0, 0, time.Local)
	m.snapshot = &data.Snapshot{
		Town: &data.TownStatus{
			Name: "test-town",
		},
		OperationalState: &data.OperationalState{
			WatchdogHealthy: false,
		},
		LoadErrors: []data.LoadError{
			{Source: "mail", Error: "connection refused", OccurredAt: time.Now()},
		},
		LastSuccess: map[string]time.Time{
			"mail": lastRefresh,
		},
	}

	alerts := m.buildAlerts()

	if len(alerts) == 0 {
		t.Fatal("expected at least one alert")
	}

	alertText := strings.Join(alerts, "\n")
	// Should contain the time formatted as HH:MM
	if !strings.Contains(alertText, "14:30") {
		t.Errorf("expected last refresh time '14:30' in alert, got: %s", alertText)
	}
}

func TestBuildAlerts_NoAlertsWhenNoErrors(t *testing.T) {
	m := NewTestModel(t)
	m.snapshot = &data.Snapshot{
		Town: &data.TownStatus{
			Name: "test-town",
		},
		OperationalState: &data.OperationalState{
			WatchdogHealthy: true,
		},
		LoadErrors:  []data.LoadError{},
		LastSuccess: map[string]time.Time{},
	}

	alerts := m.buildAlerts()

	// Filter out any non-load-error alerts
	var loadErrorAlerts []string
	for _, alert := range alerts {
		if strings.Contains(alert, "failed") || strings.Contains(alert, "stale") {
			loadErrorAlerts = append(loadErrorAlerts, alert)
		}
	}

	if len(loadErrorAlerts) > 0 {
		t.Errorf("expected no load error alerts, got: %v", loadErrorAlerts)
	}
}

func TestBuildAlerts_StaleWithNilOperationalState(t *testing.T) {
	// When operational state is nil, should treat as actual failures (not stale)
	m := NewTestModel(t)
	m.snapshot = &data.Snapshot{
		Town: &data.TownStatus{
			Name: "test-town",
		},
		OperationalState: nil, // No operational state yet
		LoadErrors: []data.LoadError{
			{Source: "mail", Error: "error", OccurredAt: time.Now()},
		},
		LastSuccess: map[string]time.Time{},
	}

	alerts := m.buildAlerts()

	if len(alerts) == 0 {
		t.Fatal("expected at least one alert")
	}

	// Without operational state, should show as failed (default behavior)
	alertText := strings.Join(alerts, "\n")
	if !strings.Contains(alertText, "failed") {
		t.Errorf("expected 'failed' when operational state is nil, got: %s", alertText)
	}
}
