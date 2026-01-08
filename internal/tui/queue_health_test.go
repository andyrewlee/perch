package tui

import (
	"strings"
	"testing"
	"time"
)

func TestRefineryStateString(t *testing.T) {
	tests := []struct {
		state    RefineryState
		expected string
	}{
		{RefineryIdle, "Idle"},
		{RefineryProcessing, "Processing"},
		{RefineryStalled, "Stalled"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("RefineryState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestQueueMRAgeBadge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		age      time.Duration
		contains string
	}{
		{"fresh", 5 * time.Minute, "fresh"},
		{"ok", 20 * time.Minute, "ok"},
		{"waiting", 45 * time.Minute, "waiting"},
		{"stale", 2 * time.Hour, "stale"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := QueueMR{
				ID:        "test-1",
				Title:     "Test MR",
				CreatedAt: now.Add(-tt.age),
			}
			badge := mr.AgeBadge()
			if !strings.Contains(badge, tt.contains) {
				t.Errorf("AgeBadge() = %q, want to contain %q", badge, tt.contains)
			}
		})
	}
}

func TestQueueHealthGuidance(t *testing.T) {
	tests := []struct {
		name        string
		health      QueueHealth
		shouldNudge bool
	}{
		{
			name: "processing refinery",
			health: QueueHealth{
				State: RefineryProcessing,
			},
			shouldNudge: false,
		},
		{
			name: "stalled refinery",
			health: QueueHealth{
				State: RefineryStalled,
			},
			shouldNudge: true,
		},
		{
			name: "idle with no MRs",
			health: QueueHealth{
				State: RefineryIdle,
				MRs:   []QueueMR{},
			},
			shouldNudge: false,
		},
		{
			name: "idle with fresh MRs",
			health: QueueHealth{
				State: RefineryIdle,
				MRs: []QueueMR{
					{Age: 5 * time.Minute},
				},
			},
			shouldNudge: false,
		},
		{
			name: "idle with old MRs",
			health: QueueHealth{
				State: RefineryIdle,
				MRs: []QueueMR{
					{Age: 45 * time.Minute},
				},
			},
			shouldNudge: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, shouldNudge := tt.health.Guidance()
			if shouldNudge != tt.shouldNudge {
				t.Errorf("Guidance() shouldNudge = %v, want %v", shouldNudge, tt.shouldNudge)
			}
		})
	}
}

func TestQueueHealthTimeSinceLastMerge(t *testing.T) {
	tests := []struct {
		name      string
		lastMerge time.Time
		contains  string
	}{
		{
			name:      "never merged",
			lastMerge: time.Time{},
			contains:  "never",
		},
		{
			name:      "recent merge",
			lastMerge: time.Now().Add(-5 * time.Minute),
			contains:  "m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := QueueHealth{LastMergeTime: tt.lastMerge}
			result := health.TimeSinceLastMerge()
			if !strings.Contains(result, tt.contains) {
				t.Errorf("TimeSinceLastMerge() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

func TestQueueHealthPanelRender(t *testing.T) {
	health := QueueHealth{
		RigName: "perch",
		State:   RefineryIdle,
		MRs: []QueueMR{
			{ID: "mr-1", Title: "Fix bug", Worker: "toast", Status: "pending"},
			{ID: "mr-2", Title: "Add feature", Worker: "nux", Status: "processing"},
		},
	}

	panel := NewQueueHealthPanel(health)
	output := panel.Render(80, 20)

	// Check that key elements are present
	checks := []string{
		"Queue Health",
		"Refinery:",
		"Last merge:",
		"Merge Requests:",
		"Fix bug",
		"nudge",
		"retry",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("Render() output missing %q", check)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{48 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
		}
	}
}
