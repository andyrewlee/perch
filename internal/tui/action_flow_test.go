package tui

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andyrewlee/perch/data"
	"github.com/andyrewlee/perch/internal/testutil"
)

// createTestModel creates a model with a mock action runner for testing.
// Returns the model and the mock runner for verification.
func createTestModel(t *testing.T) (Model, *testutil.MockRunner) {
	t.Helper()

	// Create a temp directory that looks like a town
	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/mayor", 0755); err != nil {
		t.Fatalf("failed to create test town: %v", err)
	}

	m := NewWithTownRoot(tmpDir)
	m.ready = true
	m.width = 100
	m.height = 40

	// Create mock runner and inject it
	mock := testutil.NewMockRunner()
	mock.DefaultStdout = []byte("")
	m.actionRunner = NewActionRunnerWithRunner(tmpDir, mock)

	// Set a selected rig for tests that need it
	m.selectedRig = "perch"

	return m, mock
}

// sendKey simulates a key press and returns the updated model and command.
func sendKey(m Model, key string) (Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated.(Model), cmd
}

// TestBootAction tests the boot action flow (key press → status → command).
func TestBootAction(t *testing.T) {
	t.Run("BootRig_Success", func(t *testing.T) {
		m, mock := createTestModel(t)
		mock.On([]string{"gt", "rig", "boot", "perch"}, []byte("Booting..."), nil, nil)

		// Press 'b' to boot
		m, cmd := sendKey(m, "b")

		// Verify status message is set
		if m.statusMessage == nil {
			t.Fatal("expected status message to be set")
		}
		if m.statusMessage.IsError {
			t.Error("status message should not be an error")
		}
		if m.statusMessage.Text != "Booting rig perch..." {
			t.Errorf("expected 'Booting rig perch...', got %q", m.statusMessage.Text)
		}

		// Execute the command to trigger the action
		if cmd == nil {
			t.Fatal("expected command to be returned")
		}
		msg := cmd()

		// Verify mock was called
		if !mock.CalledWith([]string{"gt", "rig", "boot", "perch"}) {
			t.Error("expected gt rig boot perch to be called")
		}

		// Verify action completed message
		completeMsg, ok := msg.(actionCompleteMsg)
		if !ok {
			t.Fatalf("expected actionCompleteMsg, got %T", msg)
		}
		if completeMsg.action != ActionBootRig {
			t.Errorf("expected ActionBootRig, got %v", completeMsg.action)
		}
		if completeMsg.target != "perch" {
			t.Errorf("expected target 'perch', got %q", completeMsg.target)
		}
		if completeMsg.err != nil {
			t.Errorf("expected no error, got %v", completeMsg.err)
		}
	})

	t.Run("BootRig_NoSelection", func(t *testing.T) {
		m, _ := createTestModel(t)
		m.selectedRig = "" // Clear selection

		// Press 'b' to boot with no selection
		m, cmd := sendKey(m, "b")

		// Verify error status message
		if m.statusMessage == nil {
			t.Fatal("expected status message to be set")
		}
		if !m.statusMessage.IsError {
			t.Error("status message should be an error")
		}
		if m.statusMessage.Text != "No rig selected. Use j/k to select a rig." {
			t.Errorf("unexpected error message: %q", m.statusMessage.Text)
		}

		// Command should be status expire, not an action
		if cmd == nil {
			t.Fatal("expected status expire command")
		}
	})

	t.Run("BootRig_Error", func(t *testing.T) {
		m, mock := createTestModel(t)
		mock.On([]string{"gt", "rig", "boot"}, nil, []byte("rig not found"), errors.New("exit status 1"))

		// Press 'b' to boot
		m, cmd := sendKey(m, "b")
		if cmd == nil {
			t.Fatal("expected command")
		}

		// Execute command
		msg := cmd()

		// Process the completion message
		updated, _ := m.Update(msg)
		m = updated.(Model)

		// Verify error status
		if m.statusMessage == nil {
			t.Fatal("expected status message after error")
		}
		if !m.statusMessage.IsError {
			t.Error("status should be error after failed action")
		}
		if m.statusMessage.Text == "" {
			t.Error("error message should not be empty")
		}
	})
}

// TestShutdownAction tests the shutdown action flow with confirmation dialog.
func TestShutdownAction(t *testing.T) {
	t.Run("ShutdownRig_ShowsConfirmation", func(t *testing.T) {
		m, _ := createTestModel(t)

		// Press 's' to shutdown
		m, cmd := sendKey(m, "s")

		// Verify confirmation dialog is shown
		if m.confirmDialog == nil {
			t.Fatal("expected confirmation dialog to be shown")
		}
		if m.confirmDialog.Action != ActionShutdownRig {
			t.Errorf("expected ActionShutdownRig, got %v", m.confirmDialog.Action)
		}
		if m.confirmDialog.Target != "perch" {
			t.Errorf("expected target 'perch', got %q", m.confirmDialog.Target)
		}
		if m.confirmDialog.Title != "Confirm Shutdown" {
			t.Errorf("expected 'Confirm Shutdown', got %q", m.confirmDialog.Title)
		}

		// No command should be returned yet (waiting for confirmation)
		if cmd != nil {
			t.Error("expected no command before confirmation")
		}
	})

	t.Run("ShutdownRig_ConfirmWithY", func(t *testing.T) {
		m, mock := createTestModel(t)
		mock.On([]string{"gt", "rig", "shutdown", "perch"}, []byte("Shutting down..."), nil, nil)

		// Press 's' to show confirmation
		m, _ = sendKey(m, "s")
		if m.confirmDialog == nil {
			t.Fatal("confirmation dialog should be shown")
		}

		// Press 'y' to confirm
		m, cmd := sendKey(m, "y")

		// Dialog should be dismissed
		if m.confirmDialog != nil {
			t.Error("confirmation dialog should be dismissed after 'y'")
		}

		// Status message should show action in progress
		if m.statusMessage == nil {
			t.Fatal("expected status message")
		}
		if m.statusMessage.Text == "" {
			t.Error("status text should not be empty")
		}

		// Command should be returned
		if cmd == nil {
			t.Fatal("expected command after confirmation")
		}

		// Execute command
		msg := cmd()

		// Verify mock was called
		if !mock.CalledWith([]string{"gt", "rig", "shutdown", "perch"}) {
			t.Error("expected gt rig shutdown perch to be called")
		}

		// Verify completion message
		completeMsg, ok := msg.(actionCompleteMsg)
		if !ok {
			t.Fatalf("expected actionCompleteMsg, got %T", msg)
		}
		if completeMsg.err != nil {
			t.Errorf("expected no error, got %v", completeMsg.err)
		}
	})

	t.Run("ShutdownRig_CancelWithN", func(t *testing.T) {
		m, mock := createTestModel(t)

		// Press 's' to show confirmation
		m, _ = sendKey(m, "s")
		if m.confirmDialog == nil {
			t.Fatal("confirmation dialog should be shown")
		}

		// Press 'n' to cancel
		m, cmd := sendKey(m, "n")

		// Dialog should be dismissed
		if m.confirmDialog != nil {
			t.Error("confirmation dialog should be dismissed after 'n'")
		}

		// Status message should show cancellation
		if m.statusMessage == nil {
			t.Fatal("expected status message")
		}
		if m.statusMessage.Text != "Action cancelled" {
			t.Errorf("expected 'Action cancelled', got %q", m.statusMessage.Text)
		}

		// No action command should be returned (only status expire)
		if cmd == nil {
			t.Fatal("expected status expire command")
		}

		// Execute the status expire command - it should NOT call the mock
		// (The command is a tick for status expiration, not an action)

		// Verify mock was NOT called
		if mock.CalledWith([]string{"gt", "rig", "shutdown"}) {
			t.Error("shutdown should not be called when cancelled")
		}
	})

	t.Run("ShutdownRig_CancelWithEsc", func(t *testing.T) {
		m, mock := createTestModel(t)

		// Press 's' to show confirmation
		m, _ = sendKey(m, "s")

		// Press 'esc' to cancel
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(Model)

		// Dialog should be dismissed
		if m.confirmDialog != nil {
			t.Error("confirmation dialog should be dismissed after 'esc'")
		}

		// Verify mock was NOT called
		if mock.CalledWith([]string{"gt", "rig", "shutdown"}) {
			t.Error("shutdown should not be called when cancelled")
		}
	})

	t.Run("ShutdownRig_NoSelection", func(t *testing.T) {
		m, _ := createTestModel(t)
		m.selectedRig = ""

		// Press 's' with no selection
		m, _ = sendKey(m, "s")

		// Should show error, not confirmation
		if m.confirmDialog != nil {
			t.Error("confirmation should not be shown without selection")
		}
		if m.statusMessage == nil || !m.statusMessage.IsError {
			t.Error("expected error status message")
		}
	})

	t.Run("ShutdownRig_Error", func(t *testing.T) {
		m, mock := createTestModel(t)
		mock.On([]string{"gt", "rig", "shutdown"}, nil, []byte("agents still running"), errors.New("exit status 1"))

		// Press 's' then 'y'
		m, _ = sendKey(m, "s")
		m, cmd := sendKey(m, "y")
		if cmd == nil {
			t.Fatal("expected command")
		}

		// Execute and process
		msg := cmd()
		updated, _ := m.Update(msg)
		m = updated.(Model)

		// Verify error status
		if m.statusMessage == nil || !m.statusMessage.IsError {
			t.Error("expected error status after failed shutdown")
		}
	})
}

// TestDeleteAction tests the delete action flow with confirmation dialog.
func TestDeleteAction(t *testing.T) {
	t.Run("DeleteRig_ShowsConfirmation", func(t *testing.T) {
		m, _ := createTestModel(t)

		// Press 'd' to delete
		m, _ = sendKey(m, "d")

		// Verify confirmation dialog
		if m.confirmDialog == nil {
			t.Fatal("expected confirmation dialog")
		}
		if m.confirmDialog.Action != ActionDeleteRig {
			t.Errorf("expected ActionDeleteRig, got %v", m.confirmDialog.Action)
		}
		if m.confirmDialog.Title != "Confirm Delete" {
			t.Errorf("expected 'Confirm Delete', got %q", m.confirmDialog.Title)
		}
	})

	t.Run("DeleteRig_ConfirmExecutes", func(t *testing.T) {
		m, mock := createTestModel(t)
		mock.On([]string{"gt", "rig", "remove", "perch"}, []byte(""), nil, nil)

		// Press 'd' then 'y'
		m, _ = sendKey(m, "d")
		m, cmd := sendKey(m, "y")

		if cmd == nil {
			t.Fatal("expected command")
		}

		// Execute
		cmd()

		// Verify mock
		if !mock.CalledWith([]string{"gt", "rig", "remove", "perch"}) {
			t.Error("expected gt rig remove perch to be called")
		}
	})
}

// TestLogsAction tests the logs action flow.
func TestLogsAction(t *testing.T) {
	t.Run("OpenLogs_Success", func(t *testing.T) {
		m, mock := createTestModel(t)
		mock.On([]string{"gt", "logs", "perch/polecats/able"}, []byte(""), nil, nil)

		// Set selected agent
		m.selectedAgent = "perch/polecats/able"

		// Press 'o' to open logs
		m, cmd := sendKey(m, "o")

		// Verify status message
		if m.statusMessage == nil {
			t.Fatal("expected status message")
		}
		if m.statusMessage.Text != "Opening logs for perch/polecats/able..." {
			t.Errorf("expected 'Opening logs for perch/polecats/able...', got %q", m.statusMessage.Text)
		}

		// Execute command
		if cmd == nil {
			t.Fatal("expected command")
		}
		msg := cmd()

		// Verify mock
		if !mock.CalledWith([]string{"gt", "logs", "perch/polecats/able"}) {
			t.Error("expected gt logs to be called")
		}

		// Verify completion
		completeMsg, ok := msg.(actionCompleteMsg)
		if !ok {
			t.Fatalf("expected actionCompleteMsg, got %T", msg)
		}
		if completeMsg.action != ActionOpenLogs {
			t.Errorf("expected ActionOpenLogs, got %v", completeMsg.action)
		}
	})

	t.Run("OpenLogs_NoSelection", func(t *testing.T) {
		m, _ := createTestModel(t)
		m.selectedAgent = ""

		// Press 'o' with no agent selected
		m, _ = sendKey(m, "o")

		// Verify error status
		if m.statusMessage == nil || !m.statusMessage.IsError {
			t.Error("expected error status message")
		}
		if m.statusMessage.Text != "No agent selected. Use j/k to select an agent." {
			t.Errorf("unexpected error: %q", m.statusMessage.Text)
		}
	})

	t.Run("OpenLogs_Error", func(t *testing.T) {
		m, mock := createTestModel(t)
		mock.On([]string{"gt", "logs"}, nil, []byte("agent not found"), errors.New("exit status 1"))
		m.selectedAgent = "unknown/agent"

		// Press 'o'
		m, cmd := sendKey(m, "o")
		if cmd == nil {
			t.Fatal("expected command")
		}

		// Execute and process
		msg := cmd()
		updated, _ := m.Update(msg)
		m = updated.(Model)

		// Verify error status
		if m.statusMessage == nil || !m.statusMessage.IsError {
			t.Error("expected error status")
		}
	})
}

// TestActionCompleteHandling tests the handleActionComplete flow.
func TestActionCompleteHandling(t *testing.T) {
	t.Run("SuccessUpdatesStatus", func(t *testing.T) {
		m, _ := createTestModel(t)

		// Send a successful completion
		msg := actionCompleteMsg{
			action: ActionBootRig,
			target: "perch",
			err:    nil,
		}
		updated, cmd := m.Update(msg)
		model := updated.(Model)

		// Status should show success
		if model.statusMessage == nil {
			t.Fatal("expected status message")
		}
		if model.statusMessage.IsError {
			t.Error("status should not be error for success")
		}
		if model.statusMessage.Text != "Boot completed for perch" {
			t.Errorf("expected 'Boot completed for perch', got %q", model.statusMessage.Text)
		}

		// Should trigger refresh
		if cmd == nil {
			t.Error("expected commands after successful action")
		}
	})

	t.Run("ErrorUpdatesStatus", func(t *testing.T) {
		m, _ := createTestModel(t)

		// Send a failed completion
		msg := actionCompleteMsg{
			action: ActionShutdownRig,
			target: "perch",
			err:    errors.New("agents still running"),
		}
		updated, _ := m.Update(msg)
		model := updated.(Model)

		// Status should show error
		if model.statusMessage == nil {
			t.Fatal("expected status message")
		}
		if !model.statusMessage.IsError {
			t.Error("status should be error for failure")
		}
		if model.statusMessage.Text == "" {
			t.Error("error message should not be empty")
		}
	})
}

// TestStatusMessageExpiration tests that status messages expire correctly.
func TestStatusMessageExpiration(t *testing.T) {
	t.Run("StatusExpiredMsgClearsStatus", func(t *testing.T) {
		m, _ := createTestModel(t)
		m.setStatus("Test message", false)

		if m.statusMessage == nil {
			t.Fatal("status should be set")
		}

		// Send status expired message
		updated, _ := m.Update(statusExpiredMsg{})
		model := updated.(Model)

		if model.statusMessage != nil {
			t.Error("status should be cleared after expiration")
		}
	})

	t.Run("StatusExpireCmd", func(t *testing.T) {
		cmd := statusExpireCmd(10 * time.Millisecond)
		if cmd == nil {
			t.Fatal("expected command")
		}

		// Execute the command (it's a tick)
		// The actual tick timing is internal to bubbletea
	})
}

// TestIsDestructiveComplete tests all destructive action types.
func TestIsDestructiveComplete(t *testing.T) {
	tests := []struct {
		action ActionType
		want   bool
	}{
		{ActionRefresh, false},
		{ActionBootRig, false},
		{ActionShutdownRig, true},
		{ActionDeleteRig, true},
		{ActionOpenLogs, false},
		{ActionAddRig, false},
		{ActionNudgePolecat, false},
		{ActionNudgeRefinery, false},
		{ActionRestartRefinery, true},
		{ActionStopPolecat, true},
		{ActionStopAllIdle, true},
		{ActionMarkMailRead, false},
		{ActionMarkMailUnread, false},
		{ActionAckMail, false},
		{ActionReplyMail, false},
	}

	for _, tt := range tests {
		t.Run(actionName(tt.action), func(t *testing.T) {
			got := IsDestructive(tt.action)
			if got != tt.want {
				t.Errorf("IsDestructive(%v) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

// TestActionNameComplete tests all action name mappings.
func TestActionNameComplete(t *testing.T) {
	tests := []struct {
		action ActionType
		want   string
	}{
		{ActionRefresh, "Refresh"},
		{ActionBootRig, "Boot"},
		{ActionShutdownRig, "Shutdown"},
		{ActionDeleteRig, "Delete"},
		{ActionOpenLogs, "Open logs"},
		{ActionAddRig, "Add rig"},
		{ActionNudgePolecat, "Nudge"},
		{ActionNudgeRefinery, "Nudge refinery"},
		{ActionRestartRefinery, "Restart refinery"},
		{ActionStopPolecat, "Stop polecat"},
		{ActionStopAllIdle, "Stop all idle"},
		{ActionMarkMailRead, "Mark read"},
		{ActionMarkMailUnread, "Mark unread"},
		{ActionAckMail, "Acknowledge"},
		{ActionReplyMail, "Reply"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := actionName(tt.action)
			if got != tt.want {
				t.Errorf("actionName(%v) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

// TestRefreshAction tests the manual refresh action.
func TestRefreshAction(t *testing.T) {
	t.Run("RefreshSetsState", func(t *testing.T) {
		m, _ := createTestModel(t)
		m.isRefreshing = false

		// Press 'r' to refresh
		m, cmd := sendKey(m, "r")

		// Verify state
		if !m.isRefreshing {
			t.Error("expected isRefreshing to be true")
		}
		if m.statusMessage == nil {
			t.Fatal("expected status message")
		}
		if m.statusMessage.Text != "Refreshing data..." {
			t.Errorf("expected 'Refreshing data...', got %q", m.statusMessage.Text)
		}

		// Should return load command
		if cmd == nil {
			t.Error("expected load command")
		}
	})
}

// TestStopPolecatAction tests the stop polecat action flow.
func TestStopPolecatAction(t *testing.T) {
	t.Run("StopPolecat_RequiresAgentsSection", func(t *testing.T) {
		m, _ := createTestModel(t)
		m.sidebar.Section = SectionRigs // Wrong section

		// Press 'c'
		m, _ = sendKey(m, "c")

		// Should show error about section
		if m.statusMessage == nil || !m.statusMessage.IsError {
			t.Error("expected error status")
		}
		if m.confirmDialog != nil {
			t.Error("confirmation should not show in wrong section")
		}
	})

	t.Run("StopAllIdle_ShowsConfirmation", func(t *testing.T) {
		m, _ := createTestModel(t)

		// Press 'C' (uppercase)
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
		m = updated.(Model)

		// Should show confirmation
		if m.confirmDialog == nil {
			t.Fatal("expected confirmation dialog")
		}
		if m.confirmDialog.Action != ActionStopAllIdle {
			t.Errorf("expected ActionStopAllIdle, got %v", m.confirmDialog.Action)
		}
	})
}

// TestNudgeAction tests the nudge action flow.
func TestNudgeAction(t *testing.T) {
	t.Run("NudgePolecat_Success", func(t *testing.T) {
		m, mock := createTestModel(t)
		mock.On([]string{"gt", "mail", "send"}, []byte(""), nil, nil)

		// Set up for nudge action
		m.sidebar.Section = SectionMergeQueue
		m.sidebar.MRs = []mrItem{
			{
				rig: "perch",
				mr: data.MergeRequest{
					Worker:       "able",
					Branch:       "feat/test",
					HasConflicts: true,
				},
			},
		}
		m.sidebar.Selection = 0

		// Press 'n'
		m, cmd := sendKey(m, "n")

		// Verify status
		if m.statusMessage == nil {
			t.Fatal("expected status message")
		}

		// Execute
		if cmd == nil {
			t.Fatal("expected command")
		}
		cmd()

		// Verify mail was sent
		if !mock.CalledWith([]string{"gt", "mail", "send"}) {
			t.Error("expected mail send to be called")
		}
	})

	t.Run("NudgePolecat_RequiresMergeQueueSection", func(t *testing.T) {
		m, _ := createTestModel(t)
		m.sidebar.Section = SectionRigs

		// Press 'n'
		m, _ = sendKey(m, "n")

		// Should show error
		if m.statusMessage == nil || !m.statusMessage.IsError {
			t.Error("expected error status")
		}
	})
}

// TestActionCmdContextTimeout tests that action commands use context with timeout.
func TestActionCmdContextTimeout(t *testing.T) {
	m, mock := createTestModel(t)

	// Set up a slow response handler
	mock.OnFunc([]string{"gt", "rig", "boot"}, func(args []string) ([]byte, []byte, error) {
		// Simulate work
		return []byte("done"), nil, nil
	})

	// Create action command
	cmd := m.actionCmd(ActionBootRig, "perch")
	if cmd == nil {
		t.Fatal("expected command")
	}

	// Execute - this should complete (not hang)
	done := make(chan bool)
	go func() {
		cmd()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(35 * time.Second):
		t.Error("action command timed out (should use internal timeout)")
	}
}

// TestConfirmDialogStruct tests ConfirmDialog fields.
func TestConfirmDialogStruct(t *testing.T) {
	var called bool
	dialog := ConfirmDialog{
		Title:   "Test Title",
		Message: "Test message?",
		Action:  ActionDeleteRig,
		Target:  "test-rig",
		OnConfirm: func() {
			called = true
		},
	}

	if dialog.Title != "Test Title" {
		t.Errorf("Title = %q, want 'Test Title'", dialog.Title)
	}
	if dialog.Message != "Test message?" {
		t.Errorf("Message = %q, want 'Test message?'", dialog.Message)
	}
	if dialog.Action != ActionDeleteRig {
		t.Errorf("Action = %v, want ActionDeleteRig", dialog.Action)
	}
	if dialog.Target != "test-rig" {
		t.Errorf("Target = %q, want 'test-rig'", dialog.Target)
	}

	// Test OnConfirm callback
	if dialog.OnConfirm != nil {
		dialog.OnConfirm()
		if !called {
			t.Error("OnConfirm callback was not executed")
		}
	}
}

// TestActionRunnerCommands tests that ActionRunner calls correct commands.
func TestActionRunnerCommands(t *testing.T) {
	tests := []struct {
		name     string
		action   func(r *ActionRunner) error
		expected []string
	}{
		{
			name:     "BootRig",
			action:   func(r *ActionRunner) error { return r.BootRig(context.Background(), "perch") },
			expected: []string{"gt", "rig", "boot", "perch"},
		},
		{
			name:     "ShutdownRig",
			action:   func(r *ActionRunner) error { return r.ShutdownRig(context.Background(), "perch") },
			expected: []string{"gt", "rig", "shutdown", "perch"},
		},
		{
			name:     "DeleteRig",
			action:   func(r *ActionRunner) error { return r.DeleteRig(context.Background(), "perch") },
			expected: []string{"gt", "rig", "remove", "perch"},
		},
		{
			name:     "OpenLogs",
			action:   func(r *ActionRunner) error { return r.OpenLogs(context.Background(), "perch/polecats/able") },
			expected: []string{"gt", "logs", "perch/polecats/able"},
		},
		{
			name:     "NudgeRefinery",
			action:   func(r *ActionRunner) error { return r.NudgeRefinery(context.Background(), "perch") },
			expected: []string{"gt", "mail", "send", "perch/refinery"},
		},
		{
			name:     "RestartRefinery",
			action:   func(r *ActionRunner) error { return r.RestartRefinery(context.Background(), "perch") },
			expected: []string{"gt", "agent", "restart", "perch/refinery"},
		},
		{
			name:     "StopPolecat",
			action:   func(r *ActionRunner) error { return r.StopPolecat(context.Background(), "perch/polecats/able") },
			expected: []string{"gt", "polecat", "stop", "perch/polecats/able"},
		},
		{
			name:     "StopAllIdlePolecats",
			action:   func(r *ActionRunner) error { return r.StopAllIdlePolecats(context.Background(), "perch") },
			expected: []string{"gt", "polecat", "stop", "--idle", "perch"},
		},
		{
			name:     "AddRig",
			action:   func(r *ActionRunner) error { return r.AddRig(context.Background(), "newrig", "git@github.com:test/repo", "") },
			expected: []string{"gt", "rig", "add", "newrig", "git@github.com:test/repo"},
		},
		{
			name:     "AddRigWithPrefix",
			action:   func(r *ActionRunner) error { return r.AddRig(context.Background(), "newrig", "git@github.com:test/repo", "nr") },
			expected: []string{"gt", "rig", "add", "newrig", "git@github.com:test/repo", "--prefix", "nr"},
		},
		{
			name:     "MarkMailRead",
			action:   func(r *ActionRunner) error { return r.MarkMailRead(context.Background(), "hq-123") },
			expected: []string{"gt", "mail", "read", "hq-123"},
		},
		{
			name:     "MarkMailUnread",
			action:   func(r *ActionRunner) error { return r.MarkMailUnread(context.Background(), "hq-456") },
			expected: []string{"gt", "mail", "unread", "hq-456"},
		},
		{
			name:     "AckMail",
			action:   func(r *ActionRunner) error { return r.AckMail(context.Background(), "hq-789") },
			expected: []string{"gt", "mail", "ack", "hq-789"},
		},
		{
			name:     "ReplyMail",
			action:   func(r *ActionRunner) error { return r.ReplyMail(context.Background(), "hq-abc", "Got it, thanks!") },
			expected: []string{"gt", "mail", "reply", "hq-abc", "-m", "Got it, thanks!"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := testutil.NewMockRunner()
			mock.DefaultStdout = []byte("")
			runner := NewActionRunnerWithRunner("/tmp/town", mock)

			tt.action(runner)

			if !mock.CalledWith(tt.expected) {
				calls := mock.Calls()
				if len(calls) > 0 {
					t.Errorf("expected %v, got %v", tt.expected, calls[0].Args)
				} else {
					t.Errorf("expected %v, but no calls made", tt.expected)
				}
			}
		})
	}
}

// TestNudgePolecatMessage tests the nudge message content.
func TestNudgePolecatMessage(t *testing.T) {
	t.Run("WithConflicts", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.DefaultStdout = []byte("")
		runner := NewActionRunnerWithRunner("/tmp/town", mock)

		runner.NudgePolecat(context.Background(), "perch", "able", "feat/test", true)

		calls := mock.Calls()
		if len(calls) == 0 {
			t.Fatal("expected call")
		}

		// Check subject contains "Nudge"
		args := calls[0].Args
		found := false
		for _, arg := range args {
			if arg == "Nudge: Resolve merge conflicts" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subject 'Nudge: Resolve merge conflicts' in args %v", args)
		}
	})

	t.Run("WithoutConflicts", func(t *testing.T) {
		mock := testutil.NewMockRunner()
		mock.DefaultStdout = []byte("")
		runner := NewActionRunnerWithRunner("/tmp/town", mock)

		runner.NudgePolecat(context.Background(), "perch", "able", "feat/test", false)

		if !mock.CalledWith([]string{"gt", "mail", "send"}) {
			t.Error("expected mail send to be called")
		}
	})
}
