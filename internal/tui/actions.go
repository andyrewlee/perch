package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/andyrewlee/perch/data"
)

// ActionType identifies the type of action being performed.
type ActionType int

const (
	ActionRefresh ActionType = iota
	ActionBootRig
	ActionShutdownRig
	ActionDeleteRig
	ActionOpenLogs
	ActionAddRig
	ActionNudgePolecat
	ActionNudgeRefinery
	ActionRestartRefinery
	ActionStopPolecat    // Stop a single idle polecat
	ActionStopAllIdle    // Stop all idle polecats in a rig
	ActionMarkMailRead   // Mark a mail message as read
	ActionMarkMailUnread // Mark a mail message as unread
	ActionAckMail        // Acknowledge a mail message
	ActionReplyMail      // Quick reply to a mail message
	ActionRemoveWorktree
	ActionCreateWork   // Create issue and optionally sling to polecat
	ActionSlingWork
	ActionHandoff
	ActionStopAgent
	ActionNudgeAgent
	ActionMailAgent
	ActionTogglePlugin
	ActionOpenSession // Attach to agent's tmux session (advanced)
)

// Action represents a user-triggered action with its result.
type Action struct {
	Type      ActionType
	Target    string // rig name or agent address
	StartedAt time.Time
	Error     error
	Output    string
}

// actionRunner implements command execution for actions.
type actionRunner struct{}

func (r *actionRunner) Exec(ctx context.Context, workDir string, args ...string) ([]byte, []byte, error) {
	if len(args) == 0 {
		return nil, nil, fmt.Errorf("no command specified")
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// ActionRunner executes Gas Town commands.
type ActionRunner struct {
	// TownRoot is the directory where gt commands are executed.
	TownRoot string

	// Runner executes commands. If nil, uses real exec.
	Runner data.CommandRunner
}

// NewActionRunner creates a new runner for the given town root.
func NewActionRunner(townRoot string) *ActionRunner {
	return &ActionRunner{TownRoot: townRoot, Runner: &actionRunner{}}
}

// NewActionRunnerWithRunner creates a runner with a custom command runner.
// Useful for testing with mock responses.
func NewActionRunnerWithRunner(townRoot string, runner data.CommandRunner) *ActionRunner {
	return &ActionRunner{TownRoot: townRoot, Runner: runner}
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

// DeleteRig removes a rig from the town.
// Runs: gt rig remove <rig>
// Note: This unregisters the rig but does not delete files.
func (r *ActionRunner) DeleteRig(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "rig", "remove", rig)
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

// NudgePolecat sends a nudge message to a polecat to resolve merge issues.
// Runs: gt mail send <rig>/<worker> -s "Nudge: Resolve merge conflicts" -m "..."
func (r *ActionRunner) NudgePolecat(ctx context.Context, rig, worker, branch string, hasConflicts bool) error {
	subject := "Nudge: Resolve merge conflicts"
	message := fmt.Sprintf("Your branch '%s' needs attention. ", branch)
	if hasConflicts {
		message += "Merge conflicts detected. Please rebase on main and resolve conflicts."
	} else {
		message += "Branch needs to be rebased on main."
	}
	message += "\n\nRun: git fetch origin main && git rebase origin/main"

	return r.runCommand(ctx, "gt", "mail", "send", rig+"/"+worker, "-s", subject, "-m", message)
}

// NudgeRefinery sends a nudge to the refinery to process waiting work.
// Runs: gt mail send <rig>/refinery -s "Nudge" -m "Process waiting MRs"
func (r *ActionRunner) NudgeRefinery(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "mail", "send", rig+"/refinery",
		"-s", "Nudge: Process queue",
		"-m", "Dashboard nudge: Please check and process any waiting merge requests.")
}

// RestartRefinery restarts the refinery agent.
// Runs: gt agent restart <rig>/refinery
func (r *ActionRunner) RestartRefinery(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "agent", "restart", rig+"/refinery")
}

// StopPolecat stops a single polecat.
// Runs: gt polecat stop <agent-address>
// Note: Only idle polecats should be stopped - caller must verify.
func (r *ActionRunner) StopPolecat(ctx context.Context, agentAddress string) error {
	return r.runCommand(ctx, "gt", "polecat", "stop", agentAddress)
}

// StopAllIdlePolecats stops all idle polecats in a rig.
// Runs: gt polecat stop --idle <rig>
// This is a convenience action that only stops polecats without active work.
func (r *ActionRunner) StopAllIdlePolecats(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "polecat", "stop", "--idle", rig)
}

// MarkMailRead marks a mail message as read.
// Runs: gt mail read <mail-id>
func (r *ActionRunner) MarkMailRead(ctx context.Context, mailID string) error {
	return r.runCommand(ctx, "gt", "mail", "read", mailID)
}

// MarkMailUnread marks a mail message as unread.
// Runs: gt mail unread <mail-id>
func (r *ActionRunner) MarkMailUnread(ctx context.Context, mailID string) error {
	return r.runCommand(ctx, "gt", "mail", "unread", mailID)
}

// AckMail acknowledges a mail message (marks as read and archives).
// Runs: gt mail ack <mail-id>
func (r *ActionRunner) AckMail(ctx context.Context, mailID string) error {
	return r.runCommand(ctx, "gt", "mail", "ack", mailID)
}

// ReplyMail sends a quick reply to a mail message.
// Runs: gt mail reply <mail-id> -m "<message>"
func (r *ActionRunner) ReplyMail(ctx context.Context, mailID, message string) error {
	return r.runCommand(ctx, "gt", "mail", "reply", mailID, "-m", message)
}

// RemoveWorktree removes a git worktree.
// Uses git worktree remove directly since gt worktree remove requires crew context.
func (r *ActionRunner) RemoveWorktree(ctx context.Context, worktreePath string) error {
	return r.runCommand(ctx, "git", "worktree", "remove", worktreePath)
}

// CreateWork creates an issue and optionally slings it to a polecat.
// Step 1: Create issue with bd create
// Step 2: If not skipSling, sling to target with gt sling
func (r *ActionRunner) CreateWork(ctx context.Context, title, issueType string, priority int, rig, target string, skipSling bool) error {
	// Step 1: Create the issue
	// Runs: bd create --title "..." --type <type> --priority <n>
	err := r.runCommand(ctx, "bd", "create",
		"--title", title,
		"--type", issueType,
		"--priority", fmt.Sprintf("%d", priority))
	if err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
	}

	// If slinging is skipped, we're done
	if skipSling || rig == "" {
		return nil
	}

	// Step 2: Sling the work to the target
	// Note: We need to get the issue ID from the create output
	// For now, we'll use a workaround - sling the most recent issue
	// Runs: gt sling <issue-id> <rig>
	// TODO: Parse the issue ID from bd create output

	// For MVP, we'll sling by finding the latest issue
	// This is a simplification - in a full implementation we'd parse the bd create output
	slingTarget := rig
	if target != "" && target != "(new polecat)" {
		slingTarget = rig + "/" + target
	}

	// Note: gt sling needs the issue ID. For now, we skip the sling step
	// and just create the issue. The user can sling manually.
	// TODO: Implement proper sling with issue ID from bd create output
	_ = slingTarget

	return nil
}

// SlingWork assigns work to an agent.
// Runs: gt sling <bead> <agent-address>
func (r *ActionRunner) SlingWork(ctx context.Context, bead, agentAddress string) error {
	return r.runCommand(ctx, "gt", "sling", bead, agentAddress)
}

// Handoff hands off work to a fresh session.
// Runs: gt handoff for the specified agent
func (r *ActionRunner) Handoff(ctx context.Context, agentAddress string) error {
	return r.runCommand(ctx, "gt", "handoff", "--target", agentAddress)
}

// StopAgent stops/nukes a polecat agent.
// Runs: gt polecat nuke <agent-address>
func (r *ActionRunner) StopAgent(ctx context.Context, agentAddress string) error {
	return r.runCommand(ctx, "gt", "polecat", "nuke", agentAddress)
}

// NudgeAgent sends a nudge message to an agent.
// Runs: gt nudge <agent-address> <message>
func (r *ActionRunner) NudgeAgent(ctx context.Context, agentAddress, message string) error {
	return r.runCommand(ctx, "gt", "nudge", agentAddress, "-m", message)
}

// MailAgent opens mail composition for an agent.
// Runs: gt mail send <agent-address> -s "<subject>" -m "<message>"
func (r *ActionRunner) MailAgent(ctx context.Context, agentAddress, subject, message string) error {
	return r.runCommand(ctx, "gt", "mail", "send", agentAddress, "-s", subject, "-m", message)
}

// TogglePlugin enables or disables a plugin by creating/removing a .disabled marker file.
// pluginPath is the full path to the plugin directory.
func (r *ActionRunner) TogglePlugin(ctx context.Context, pluginPath string) error {
	disabledPath := filepath.Join(pluginPath, ".disabled")

	if _, err := os.Stat(disabledPath); os.IsNotExist(err) {
		// Plugin is enabled, disable it
		return os.WriteFile(disabledPath, []byte("disabled\n"), 0644)
	}
	// Plugin is disabled, enable it by removing the marker
	return os.Remove(disabledPath)
}

// OpenSession attaches to an agent's underlying tmux session.
// This is an advanced action for power users who need direct session access.
// Runs: gt session attach <agent-address>
func (r *ActionRunner) OpenSession(ctx context.Context, agentAddress string) error {
	return r.runCommand(ctx, "gt", "session", "attach", agentAddress)
}

// runCommand executes a shell command and returns any error.
func (r *ActionRunner) runCommand(ctx context.Context, args ...string) error {
	_, stderr, err := r.Runner.Exec(ctx, r.TownRoot, args...)
	if err != nil {
		errMsg := string(stderr)
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
	Title     string
	Message   string
	Action    ActionType
	Target    string
	OnConfirm func()
}

// InputDialog represents a dialog for text input (sling, nudge, mail).
type InputDialog struct {
	Title       string
	Prompt      string
	Action      ActionType
	Target      string
	Input       string      // Current text input
	ExtraInput  string      // Second input field (e.g., message body for mail)
	ExtraPrompt string      // Prompt for second field
	Field       int         // 0 = first field, 1 = second field
}

// IsDestructive returns true if the action type requires confirmation.
func IsDestructive(action ActionType) bool {
	switch action {
	case ActionShutdownRig, ActionDeleteRig, ActionRestartRefinery,
		ActionStopPolecat, ActionStopAllIdle, ActionRemoveWorktree, ActionStopAgent:
		return true
	default:
		return false
	}
}
