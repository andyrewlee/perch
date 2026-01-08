package tui

import (
	"bytes"
	"context"
	"encoding/json"
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
	ActionOpenSession    // Attach to agent's tmux session (advanced)
	ActionRestartSession // Restart agent's session
	ActionPresetNudge    // Nudge with preset message
	ActionCreateBead     // Create a new bead (issue)
	ActionEditBead       // Edit an existing bead
	ActionAddComment     // Add a comment to a bead (issue)
	ActionCloseBead      // Close a bead (mark as resolved)
	ActionReopenBead     // Reopen a closed bead

	// Infrastructure agent controls (Deacon/Witness/Refinery)
	ActionStartDeacon     // Start the Deacon (town-level watchdog)
	ActionStopDeacon      // Stop the Deacon
	ActionRestartDeacon   // Restart the Deacon
	ActionStartWitness    // Start a Witness (rig-specific)
	ActionStopWitness     // Stop a Witness
	ActionRestartWitness  // Restart a Witness
	ActionStartRefinery   // Start a Refinery (rig-specific)
	ActionStopRefinery    // Stop a Refinery
	ActionRestartRefineryAlt // Restart a Refinery (alternative naming for clarity)

	// Dependency management
	ActionManageDeps     // Open dependency management dialog
	ActionAddDependency  // Add a dependency (blocked-by relationship)
	ActionRemoveDependency
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
// Runs: gt log --agent <agent-address> -f
func (r *ActionRunner) OpenLogs(ctx context.Context, agentAddress string) error {
	return r.runCommand(ctx, "gt", "log", "--agent", agentAddress, "-f")
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
func (r *ActionRunner) CreateWork(ctx context.Context, title, description, issueType string, priority int, rig, target string, skipSling bool) error {
	// Step 1: Create the issue and capture the output
	// Runs: bd create --title "..." --description "..." --type <type> --priority <n> --json
	args := []string{"bd", "create", "--title", title}
	if description != "" {
		args = append(args, "--description", description)
	}
	args = append(args, "--type", issueType, "--priority", fmt.Sprintf("%d", priority), "--json")

	output, err := r.runCommandWithOutput(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
	}

	// Parse the issue ID from the JSON output
	// bd create --json outputs: {"id":"pe-xxx","title":...}
	var issue struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(output), &issue); err != nil {
		return fmt.Errorf("failed to parse issue ID from output: %w", err)
	}
	if issue.ID == "" {
		return fmt.Errorf("no issue ID in bd create output")
	}

	// If slinging is skipped, we're done
	if skipSling || rig == "" {
		return nil
	}

	// Step 2: Sling the work to the target
	// Runs: gt sling <issue-id> <rig>
	slingTarget := rig
	if target != "" && target != "(new polecat)" {
		slingTarget = rig + "/" + target
	}

	return r.SlingWork(ctx, issue.ID, slingTarget)
}

// CreateBead creates a new bead (issue).
// Runs: bd create --title "..." --type <type> --priority <n> [--description "..."]
func (r *ActionRunner) CreateBead(ctx context.Context, title, description, issueType string, priority int) error {
	args := []string{"bd", "create",
		"--title", title,
		"--type", issueType,
		"--priority", fmt.Sprintf("%d", priority)}
	if description != "" {
		args = append(args, "--description", description)
	}
	return r.runCommand(ctx, args...)
}

// UpdateBead updates an existing bead.
// Runs: bd update <id> --title "..." [--type <type>] [--priority <n>] [--description "..."]
func (r *ActionRunner) UpdateBead(ctx context.Context, id, title, description, issueType string, priority int) error {
	args := []string{"bd", "update", id,
		"--title", title}
	if description != "" {
		args = append(args, "--description", description)
	}
	if issueType != "" {
		args = append(args, "--type", issueType)
	}
	args = append(args, "--priority", fmt.Sprintf("%d", priority))
	return r.runCommand(ctx, args...)
}

// CloseBead closes a bead (marks it as resolved).
// Runs: bd close <id>
func (r *ActionRunner) CloseBead(ctx context.Context, id string) error {
	return r.runCommand(ctx, "bd", "close", id)
}

// ReopenBead reopens a closed bead.
// Runs: bd update <id> --status open
func (r *ActionRunner) ReopenBead(ctx context.Context, id string) error {
	return r.runCommand(ctx, "bd", "update", id, "--status", "open")
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

// AttachSession attaches to an agent's tmux session.
// Runs: gt session at <agent-address>
func (r *ActionRunner) AttachSession(ctx context.Context, agentAddress string) error {
	return r.runCommand(ctx, "gt", "session", "at", agentAddress)
}

// RestartSession restarts an agent's session.
// Runs: gt session restart <agent-address>
func (r *ActionRunner) RestartSession(ctx context.Context, agentAddress string) error {
	return r.runCommand(ctx, "gt", "session", "restart", agentAddress)
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

// AddDependency adds a dependency relationship between issues.
// blockerID blocks blockedID (blockedID depends on blockerID).
// Runs: bd dep add <blocked-id> <blocker-id>
func (r *ActionRunner) AddDependency(ctx context.Context, blockedID, blockerID string) error {
	return r.runCommand(ctx, "bd", "dep", "add", blockedID, blockerID)
}

// RemoveDependency removes a dependency relationship between issues.
// Runs: bd dep remove <blocked-id> <blocker-id>
func (r *ActionRunner) RemoveDependency(ctx context.Context, blockedID, blockerID string) error {
	return r.runCommand(ctx, "bd", "dep", "remove", blockedID, blockerID)
}

// AddComment adds a comment to an issue.
// Runs: bd comments add <issue-id> <comment>
func (r *ActionRunner) AddComment(ctx context.Context, issueID, comment string) error {
	return r.runCommand(ctx, "bd", "comments", "add", issueID, comment)
}

// OpenSession attaches to an agent's underlying tmux session.
// This is an advanced action for power users who need direct session access.
// Runs: gt session attach <agent-address>
func (r *ActionRunner) OpenSession(ctx context.Context, agentAddress string) error {
	return r.runCommand(ctx, "gt", "session", "attach", agentAddress)
}

// StartDeacon starts the Deacon (town-level watchdog).
// Runs: gt deacon start
func (r *ActionRunner) StartDeacon(ctx context.Context) error {
	return r.runCommand(ctx, "gt", "deacon", "start")
}

// StopDeacon stops the Deacon.
// Runs: gt deacon stop
func (r *ActionRunner) StopDeacon(ctx context.Context) error {
	return r.runCommand(ctx, "gt", "deacon", "stop")
}

// RestartDeacon restarts the Deacon.
// Runs: gt deacon restart
func (r *ActionRunner) RestartDeacon(ctx context.Context) error {
	return r.runCommand(ctx, "gt", "deacon", "restart")
}

// StartWitness starts a Witness for the given rig.
// Runs: gt witness start <rig>
func (r *ActionRunner) StartWitness(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "witness", "start", rig)
}

// StopWitness stops a Witness for the given rig.
// Runs: gt witness stop <rig>
func (r *ActionRunner) StopWitness(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "witness", "stop", rig)
}

// RestartWitness restarts a Witness for the given rig.
// Runs: gt witness restart <rig>
func (r *ActionRunner) RestartWitness(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "witness", "restart", rig)
}

// StartRefinery starts a Refinery for the given rig.
// Runs: gt refinery start <rig>
func (r *ActionRunner) StartRefinery(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "refinery", "start", rig)
}

// StopRefinery stops a Refinery for the given rig.
// Runs: gt refinery stop <rig>
func (r *ActionRunner) StopRefinery(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "refinery", "stop", rig)
}

// RestartRefineryAgent restarts a Refinery for the given rig.
// Runs: gt refinery restart <rig>
func (r *ActionRunner) RestartRefineryAgent(ctx context.Context, rig string) error {
	return r.runCommand(ctx, "gt", "refinery", "restart", rig)
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

// runCommandWithOutput executes a shell command and returns stdout and any error.
func (r *ActionRunner) runCommandWithOutput(ctx context.Context, args ...string) (string, error) {
	stdout, stderr, err := r.Runner.Exec(ctx, r.TownRoot, args...)
	if err != nil {
		errMsg := string(stderr)
		if errMsg != "" {
			return "", fmt.Errorf("%s: %s", err, errMsg)
		}
		return "", err
	}

	return string(stdout), nil
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
	Input       string // Current text input
	ExtraInput  string // Second input field (e.g., message body for mail)
	ExtraPrompt string // Prompt for second field
	Field       int    // 0 = first field, 1 = second field
}

// DependencyDialog represents a dialog for managing issue dependencies.
type DependencyDialog struct {
	IssueID       string                   // The issue whose dependencies we're managing
	IssueTitle    string                   // Title of the issue (for display)
	Mode          string                   // "add" or "remove"
	SearchQuery   string                   // Current search query
	SearchResults []data.Issue            // Issues matching search
	Dependencies  []data.IssueDependency  // Current dependencies
	Dependents    []data.IssueDependency  // Current dependents (issues we block)
	Selection     int                      // Selected item in results list
	Loading       bool                     // True while loading data
	Status        string                   // Status message
}

// BeadsFilterDialog represents a dialog for filtering beads.
type BeadsFilterDialog struct {
	Step            int      // 0=status, 1=type, 2=priority, 3=assignee, 4=labels
	StatusFilter    string   // Current status filter
	TypeFilter      string   // Current type filter
	PriorityFilter  int      // -1 = all, 0-4 = P0-P4
	AssigneeFilter  string   // Current assignee filter
	LabelsFilter    []string // Current labels filter
	AvailableLabels []string // All available labels from snapshot
	Selection       int      // Selected item in current step
}

// PresetNudge represents a preset nudge message option.
type PresetNudge struct {
	Label   string // Short display label
	Message string // Full nudge message
}

// PresetNudges contains the available preset nudge options.
var PresetNudges = []PresetNudge{
	{"Check mail", "Check your mail and respond to any pending items."},
	{"Status update", "Please provide a status update on your current work."},
	{"Resume work", "Resume working on your hooked task."},
	{"Wrap up", "Please wrap up your current task and prepare for handoff."},
	{"Custom...", ""}, // Empty message signals custom input
}

// PresetNudgeMenu represents the preset nudge selection menu.
type PresetNudgeMenu struct {
	Target    string
	Selection int
}

// IsDestructive returns true if the action type requires confirmation.
func IsDestructive(action ActionType) bool {
	switch action {
	case ActionShutdownRig, ActionDeleteRig, ActionRestartRefinery, ActionRestartRefineryAlt,
		ActionStopPolecat, ActionStopAllIdle, ActionRemoveWorktree,
		ActionStopAgent, ActionRestartSession,
		ActionStopDeacon, ActionRestartDeacon,
		ActionStopWitness, ActionRestartWitness,
		ActionStopRefinery:
		return true
	default:
		return false
	}
}
