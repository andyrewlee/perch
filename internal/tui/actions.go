package tui

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ActionType identifies the type of action being performed.
type ActionType int

const (
	ActionRefresh ActionType = iota
	ActionBootRig
	ActionShutdownRig
	ActionOpenLogs
	ActionAddRig
)

// Action represents a user-triggered action with its result.
type Action struct {
	Type      ActionType
	Target    string // rig name or agent address
	StartedAt time.Time
	Error     error
	Output    string
}

// ActionRunner executes Gas Town commands.
type ActionRunner struct {
	// TownRoot is the directory where gt commands are executed.
	TownRoot string
}

// NewActionRunner creates a new runner for the given town root.
func NewActionRunner(townRoot string) *ActionRunner {
	return &ActionRunner{TownRoot: townRoot}
}

// BootRig starts a rig.
// Runs: gt rig boot <rig>
func (r *ActionRunner) BootRig(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "rig", "boot", rig)
}

// ShutdownRig stops a rig.
// Runs: gt rig shutdown <rig>
func (r *ActionRunner) ShutdownRig(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "rig", "shutdown", rig)
}

// OpenLogs opens logs for an agent.
// Runs: gt logs <agent-address>
func (r *ActionRunner) OpenLogs(ctx context.Context, agentAddress string) error {
	return r.runCommand(ctx, "gt", "logs", agentAddress)
}

// AddRig adds a new rig by cloning a repository.
// Runs: gt rig add <name> <git-url> [--prefix <prefix>]
func (r *ActionRunner) AddRig(ctx context.Context, name, gitURL, prefix string) error {
	args := []string{"gt", "rig", "add", name, gitURL}
	if prefix != "" {
		args = append(args, "--prefix", prefix)
	}
	return r.runCommand(ctx, args...)
}

// runCommand executes a shell command and returns any error.
func (r *ActionRunner) runCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = r.TownRoot

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			return fmt.Errorf("%s: %s", err, errMsg)
		}
		return err
	}

	return nil
}

// StatusMessage represents a temporary status message shown in the footer.
type StatusMessage struct {
	Text      string
	IsError   bool
	ExpiresAt time.Time
}

// NewStatusMessage creates a status message that expires after the given duration.
func NewStatusMessage(text string, isError bool, duration time.Duration) StatusMessage {
	return StatusMessage{
		Text:      text,
		IsError:   isError,
		ExpiresAt: time.Now().Add(duration),
	}
}

// IsExpired returns true if the message should no longer be displayed.
func (m StatusMessage) IsExpired() bool {
	return time.Now().After(m.ExpiresAt)
}

// ConfirmDialog represents a confirmation dialog for destructive actions.
type ConfirmDialog struct {
	Title   string
	Message string
	Action  ActionType
	Target  string
	OnConfirm func()
}

// IsDestructive returns true if the action type requires confirmation.
func IsDestructive(action ActionType) bool {
	switch action {
	case ActionShutdownRig:
		return true
	default:
		return false
	}
}
