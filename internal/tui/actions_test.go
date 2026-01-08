package tui

import (
	"testing"
	"time"
)

func TestStatusMessage(t *testing.T) {
	t.Run("NewStatusMessage", func(t *testing.T) {
		msg := NewStatusMessage("test message", false, 1*time.Second)

		if msg.Text != "test message" {
			t.Errorf("expected 'test message', got %q", msg.Text)
		}
		if msg.IsError != false {
			t.Error("expected IsError=false")
		}
		if msg.IsExpired() {
			t.Error("message should not be expired immediately")
		}
	})

	t.Run("IsExpired", func(t *testing.T) {
		msg := NewStatusMessage("test", false, 10*time.Millisecond)

		if msg.IsExpired() {
			t.Error("message should not be expired immediately")
		}

		time.Sleep(20 * time.Millisecond)

		if !msg.IsExpired() {
			t.Error("message should be expired after duration")
		}
	})

	t.Run("ErrorMessage", func(t *testing.T) {
		msg := NewStatusMessage("error", true, 1*time.Second)

		if !msg.IsError {
			t.Error("expected IsError=true")
		}
	})
}

func TestIsDestructive(t *testing.T) {
	tests := []struct {
		action ActionType
		want   bool
	}{
		{ActionRefresh, false},
		{ActionBootRig, false},
		{ActionShutdownRig, true},
		{ActionDeleteRig, true},
		{ActionOpenLogs, false},
	}

	for _, tt := range tests {
		got := IsDestructive(tt.action)
		if got != tt.want {
			t.Errorf("IsDestructive(%v) = %v, want %v", tt.action, got, tt.want)
		}
	}
}

func TestActionName(t *testing.T) {
	tests := []struct {
		action ActionType
		want   string
	}{
		{ActionRefresh, "Refresh"},
		{ActionBootRig, "Boot"},
		{ActionShutdownRig, "Shutdown"},
		{ActionDeleteRig, "Delete"},
		{ActionOpenLogs, "Open logs"},
	}

	for _, tt := range tests {
		got := actionName(tt.action)
		if got != tt.want {
			t.Errorf("actionName(%v) = %q, want %q", tt.action, got, tt.want)
		}
	}
}

func TestConfirmDialog(t *testing.T) {
	dialog := ConfirmDialog{
		Title:   "Test Title",
		Message: "Test message?",
		Action:  ActionShutdownRig,
		Target:  "test-rig",
	}

	if dialog.Title != "Test Title" {
		t.Errorf("expected 'Test Title', got %q", dialog.Title)
	}
	if dialog.Action != ActionShutdownRig {
		t.Errorf("expected ActionShutdownRig, got %v", dialog.Action)
	}
	if dialog.Target != "test-rig" {
		t.Errorf("expected 'test-rig', got %q", dialog.Target)
	}
}

func TestIsTownLevelBead(t *testing.T) {
	tests := []struct {
		beadID string
		want   bool
	}{
		// Town-level beads (hq- prefix)
		{"hq-mayor", true},
		{"hq-deacon", true},
		{"hq-3c4", true},
		{"hq-witness-role", true},
		{"hq-", true}, // Has prefix (edge case but technically correct)
		// Rig-level beads (other prefixes)
		{"pe-001", false},
		{"pe-m054", false},
		{"gt-123", false},
		{"test-abc", false},
		// Edge cases
		{"hq", false},      // Just prefix, no hyphen
		{"Hq-abc", false},  // Wrong case
		{"", false},        // Empty
	}

	for _, tt := range tests {
		t.Run(tt.beadID, func(t *testing.T) {
			got := IsTownLevelBead(tt.beadID)
			if got != tt.want {
				t.Errorf("IsTownLevelBead(%q) = %v, want %v", tt.beadID, got, tt.want)
			}
		})
	}
}

func TestAgentTypeFromRole(t *testing.T) {
	tests := []struct {
		role string
		want AgentType
	}{
		{"witness", AgentWitness},
		{"refinery", AgentRefinery},
		{"polecat", AgentPolecat},
		{"unknown", AgentPolecat}, // defaults to polecat
	}

	for _, tt := range tests {
		got := agentTypeFromRole(tt.role)
		if got != tt.want {
			t.Errorf("agentTypeFromRole(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestAgentStatusFromState(t *testing.T) {
	tests := []struct {
		running    bool
		hasWork    bool
		unreadMail int
		want       AgentStatus
	}{
		{false, false, 0, StatusStopped},
		{false, true, 0, StatusStopped},
		{true, false, 0, StatusIdle},
		{true, true, 0, StatusWorking},
		{true, true, 1, StatusAttention}, // Mail takes precedence
		{true, false, 2, StatusAttention},
	}

	for _, tt := range tests {
		got := agentStatusFromState(tt.running, tt.hasWork, tt.unreadMail)
		if got != tt.want {
			t.Errorf("agentStatusFromState(%v, %v, %d) = %v, want %v",
				tt.running, tt.hasWork, tt.unreadMail, got, tt.want)
		}
	}
}
