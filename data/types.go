// Package data provides data loaders for Gas Town dashboard.
// It executes gt/bd CLI commands and parses their JSON output.
package data

import (
	"fmt"
	"strings"
	"time"
)

// TownStatus represents the complete status of a Gas Town workspace.
// Loaded via: gt status --json
type TownStatus struct {
	Name     string   `json:"name"`
	Location string   `json:"location"`
	Overseer Overseer `json:"overseer"`
	Agents   []Agent  `json:"agents"`
	Rigs     []Rig    `json:"rigs"`
	Summary  Summary  `json:"summary"`
}

// Overseer is the human operator of the town.
type Overseer struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	Source     string `json:"source"`
	UnreadMail int    `json:"unread_mail"`
}

// Agent represents a running agent (mayor, deacon, witness, refinery, polecat).
type Agent struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	Session      string `json:"session"`
	Role         string `json:"role"`
	Running      bool   `json:"running"`
	HasWork      bool   `json:"has_work"`
	UnreadMail   int    `json:"unread_mail"`
	FirstSubject string `json:"first_subject,omitempty"`

	// Hook details (enriched from beads data)
	HookedBeadID string    `json:"hooked_bead_id,omitempty"` // ID of the hooked issue
	HookedStatus string    `json:"hooked_status,omitempty"`  // Status of the hooked issue (hooked, in_progress)
	HookedAt     time.Time `json:"hooked_at,omitempty"`      // When the issue was hooked (for age calculation)
}

// Rig represents a project container with workers and infrastructure.
type Rig struct {
	Name         string   `json:"name"`
	Polecats     []string `json:"polecats"`
	PolecatCount int      `json:"polecat_count"`
	Crews        []string `json:"crews"`
	CrewCount    int      `json:"crew_count"`
	HasWitness   bool     `json:"has_witness"`
	HasRefinery  bool     `json:"has_refinery"`
	Hooks        []Hook   `json:"hooks"`
	Agents       []Agent  `json:"agents"`
	ActiveHooks  int      `json:"active_hooks"` // Computed from hooked issues for this rig
}

// Hook represents work hooked to an agent.
type Hook struct {
	Agent   string `json:"agent"`
	Role    string `json:"role"`
	HasWork bool   `json:"has_work"`
}

// Summary provides aggregate counts.
type Summary struct {
	RigCount      int `json:"rig_count"`
	PolecatCount  int `json:"polecat_count"`
	CrewCount     int `json:"crew_count"`
	WitnessCount  int `json:"witness_count"`
	RefineryCount int `json:"refinery_count"`
	ActiveHooks   int `json:"active_hooks"`
}

// Polecat represents a worker agent.
// Loaded via: gt polecat list --all --json
type Polecat struct {
	Rig            string `json:"rig"`
	Name           string `json:"name"`
	State          string `json:"state"`
	SessionRunning bool   `json:"session_running"`
}

// Convoy represents a batch of coordinated work.
// Loaded via: gt convoy list --json (basic) or gt convoy status <id> --json (detailed)
type Convoy struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	// Progress fields (populated from gt convoy status --json)
	Completed int            `json:"completed,omitempty"`
	Total     int            `json:"total,omitempty"`
	Tracked   []TrackedIssue `json:"tracked,omitempty"`
}

// Progress returns the completion percentage (0-100).
func (c *Convoy) Progress() int {
	if c.Total == 0 {
		return 0
	}
	return c.Completed * 100 / c.Total
}

// IsActive returns true if the convoy is open/active.
func (c Convoy) IsActive() bool {
	return c.Status == "open"
}

// HasActiveWork returns true if the convoy has in-progress or hooked issues.
func (c *Convoy) HasActiveWork() bool {
	for _, t := range c.Tracked {
		if t.Status == "in_progress" || t.Status == "hooked" {
			return true
		}
	}
	return false
}

// IsLanded returns true if the convoy is closed/landed.
func (c Convoy) IsLanded() bool {
	return c.Status == "closed"
}

// TrackedIssue represents an issue tracked by a convoy.
// Part of ConvoyStatus from: gt convoy status <id> --json
type TrackedIssue struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	DependencyType string `json:"dependency_type"`
	IssueType      string `json:"issue_type"`
	Assignee       string `json:"assignee"`
	Worker         string `json:"worker"`
	WorkerAge      string `json:"worker_age"`
}

// ConvoyStatus represents detailed convoy status.
// Loaded via: gt convoy status <id> --json
type ConvoyStatus struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Status    string         `json:"status"`
	Tracked   []TrackedIssue `json:"tracked"`
	Completed int            `json:"completed"`
	Total     int            `json:"total"`
}

// MergeRequest represents an item in the merge queue.
// Loaded via: gt mq list <rig> --json
type MergeRequest struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	Worker       string `json:"worker"`
	Branch       string `json:"branch"`
	Priority     int    `json:"priority"`
	HasConflicts bool   `json:"has_conflicts"`
	NeedsRebase  bool   `json:"needs_rebase"`
	ConflictInfo string `json:"conflict_info,omitempty"`
	LastChecked  string `json:"last_checked,omitempty"`
}

// Worktree represents a cross-rig git worktree.
// Scanned from crew directories across all rigs.
type Worktree struct {
	Rig        string `json:"rig"`         // Target rig where worktree exists
	SourceRig  string `json:"source_rig"`  // Source rig/identity that created it
	SourceName string `json:"source_name"` // Source crew member name
	Path       string `json:"path"`        // Full path to worktree
	Branch     string `json:"branch"`      // Current branch
	Clean      bool   `json:"clean"`       // True if no uncommitted changes
	Status     string `json:"status"`      // Status summary (e.g., "clean", "2 uncommitted")
}

// Issue represents a beads issue.
// Loaded via: bd list --json
type Issue struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	Priority        int       `json:"priority"`
	IssueType       string    `json:"issue_type"`
	Assignee        string    `json:"assignee"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       string    `json:"created_by"`
	UpdatedAt       time.Time `json:"updated_at"`
	Labels          []string  `json:"labels"`
	DependencyCount int       `json:"dependency_count"`
	DependentCount  int       `json:"dependent_count"`
	Ephemeral       bool      `json:"ephemeral"`
}

// IssueDependency represents a dependency relationship between issues.
type IssueDependency struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	IssueType   string    `json:"issue_type"`
	Priority    int       `json:"priority"`
	UpdatedAt   time.Time `json:"updated_at"`
	DepType     string    `json:"dep_type"` // "blocks" or "blocked_by"
}

// IssueDependencies contains all dependencies for an issue.
type IssueDependencies struct {
	IssueID      string
	BlockedBy    []IssueDependency // Issues that block this issue
	Blocking     []IssueDependency // Issues that this issue blocks
	Loading      bool
	LoadError    error
	LastLoadedAt time.Time
}

// Comment represents a comment on a beads issue.
// Loaded via: bd comments <issue-id> --json
type Comment struct {
	ID        string    `json:"id"`
	IssueID   string    `json:"issue_id"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// IssueComments contains all comments for an issue.
type IssueComments struct {
	IssueID      string
	Comments     []Comment
	Loading      bool
	LoadError    error
	LastLoadedAt time.Time
}

// MailMessage represents a mail message in the inbox.
// Loaded via: gt mail inbox --json
type MailMessage struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Timestamp time.Time `json:"timestamp"`
	Read      bool      `json:"read"`
	Priority  string    `json:"priority"`
	Type      string    `json:"type"`
	ThreadID  string    `json:"thread_id"`
}

// LifecycleEventType represents the type of lifecycle event.
type LifecycleEventType string

const (
	EventSpawn   LifecycleEventType = "spawn"
	EventWake    LifecycleEventType = "wake"
	EventNudge   LifecycleEventType = "nudge"
	EventHandoff LifecycleEventType = "handoff"
	EventDone    LifecycleEventType = "done"
	EventCrash   LifecycleEventType = "crash"
	EventKill    LifecycleEventType = "kill"
)

// LifecycleEvent represents a single lifecycle event from town.log.
type LifecycleEvent struct {
	Timestamp time.Time          // When the event occurred
	EventType LifecycleEventType // Type of event (spawn, done, kill, etc.)
	Agent     string             // Agent address (e.g., "perch/dag")
	Message   string             // Full event message/details
}

// LifecycleLog holds parsed lifecycle events from town.log.
type LifecycleLog struct {
	Events   []LifecycleEvent
	LoadedAt time.Time
}

// OperationalState represents the overall health and operational status of the town.
type OperationalState struct {
	// DegradedMode indicates tmux is unavailable (GT_DEGRADED env var set)
	DegradedMode bool `json:"degraded_mode"`

	// DegradedReason explains why degraded mode is active
	DegradedReason string `json:"degraded_reason,omitempty"`

	// DegradedAction is the recommended action to resolve degraded mode
	DegradedAction string `json:"degraded_action,omitempty"`

	// PatrolMuted indicates the deacon patrol is muted
	PatrolMuted bool `json:"patrol_muted"`

	// WatchdogHealthy indicates the deacon watchdog is running and healthy
	WatchdogHealthy bool `json:"watchdog_healthy"`

	// WatchdogReason explains why the watchdog is unhealthy
	WatchdogReason string `json:"watchdog_reason,omitempty"`

	// WatchdogAction is the recommended action to restore watchdog
	WatchdogAction string `json:"watchdog_action,omitempty"`

	// LastDeaconHeartbeat is when the deacon last checked in
	LastDeaconHeartbeat time.Time `json:"last_deacon_heartbeat,omitempty"`

	// LastWitnessHeartbeat tracks per-rig witness health
	LastWitnessHeartbeat map[string]time.Time `json:"last_witness_heartbeat,omitempty"`

	// LastRefineryHeartbeat tracks per-rig refinery health
	LastRefineryHeartbeat map[string]time.Time `json:"last_refinery_heartbeat,omitempty"`

	// Issues contains any detected operational issues
	Issues []string `json:"issues,omitempty"`
}

// HasIssues returns true if there are any operational issues.
func (o *OperationalState) HasIssues() bool {
	return o.DegradedMode || o.PatrolMuted || !o.WatchdogHealthy || len(o.Issues) > 0
}

// Summary returns a brief status summary.
func (o *OperationalState) Summary() string {
	if o.DegradedMode {
		return "DEGRADED"
	}
	if o.PatrolMuted {
		return "PATROL MUTED"
	}
	if !o.WatchdogHealthy {
		return "WATCHDOG UNHEALTHY"
	}
	if len(o.Issues) > 0 {
		return "ISSUES DETECTED"
	}
	return "HEALTHY"
}

// AuditEntry represents a timeline event from gt audit.
// Loaded via: gt audit --actor=<addr> --json
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"` // events, townlog, git, beads
	Type      string    `json:"type"`   // sling, session_start, done, kill, commit, etc.
	Actor     string    `json:"actor"`
	Summary   string    `json:"summary"`
}

// RigSettings represents the configuration for a rig.
// Combines data from rigs.json and <rig>/settings/config.json
type RigSettings struct {
	// Rig identity
	Name string `json:"name"`

	// From rigs.json
	GitURL string `json:"git_url"`
	Prefix string `json:"prefix"` // Beads issue prefix (e.g., "gt")

	// From settings/config.json
	Theme      string           `json:"theme,omitempty"`       // UI theme for dashboard
	MaxWorkers int              `json:"max_workers,omitempty"` // Maximum concurrent polecats (0 = unlimited)
	MergeQueue MergeQueueConfig `json:"merge_queue"`
}

// MergeQueueConfig contains merge queue settings.
type MergeQueueConfig struct {
	Enabled     bool   `json:"enabled"`
	RunTests    bool   `json:"run_tests"`
	TestCommand string `json:"test_command"`
}

// Validate checks that the rig settings are valid.
func (s *RigSettings) Validate() error {
	if s.Name == "" {
		return ErrEmptyRigName
	}
	if s.Prefix == "" {
		return ErrEmptyPrefix
	}
	if s.MaxWorkers < 0 {
		return ErrInvalidMaxWorkers
	}
	if s.MergeQueue.RunTests && s.MergeQueue.TestCommand == "" {
		return ErrEmptyTestCommand
	}
	return nil
}

// RigSettings validation errors
var (
	ErrEmptyRigName      = &ValidationError{Field: "name", Message: "rig name cannot be empty"}
	ErrEmptyPrefix       = &ValidationError{Field: "prefix", Message: "beads prefix cannot be empty"}
	ErrInvalidMaxWorkers = &ValidationError{Field: "max_workers", Message: "max workers cannot be negative"}
	ErrEmptyTestCommand  = &ValidationError{Field: "test_command", Message: "test command required when run_tests is enabled"}
)

// ValidationError represents a validation error for a specific field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// LoadError represents a structured error from loading a data source.
// This provides actionable context about what failed and how to fix it.
type LoadError struct {
	Source     string    `json:"source"`      // e.g., "town_status", "merge_queue", "mail", "convoys"
	Command    string    `json:"command"`     // e.g., "gt status --json --fast"
	Error      string    `json:"error"`       // The error message
	Stderr     string    `json:"stderr"`      // Raw stderr output (if available)
	OccurredAt time.Time `json:"occurred_at"` // When this error happened
}

// SuggestedAction returns a suggested action for this error.
func (e *LoadError) SuggestedAction() string {
	switch e.Source {
	case "town_status":
		return "Check if gt is installed and $GT_ROOT is set correctly"
	case "merge_queue":
		return "Verify the rig exists and refinery is configured"
	case "mail":
		return "Check mail configuration with 'gt mail inbox'"
	case "convoys":
		return "Check convoy status with 'gt convoy list'"
	case "issues":
		return e.beadsSuggestedAction()
	case "polecats":
		return "Check polecat status with 'gt polecat list --all'"
	case "lifecycle":
		return "Check logs directory exists at $GT_ROOT/logs/town.log"
	case "doctor":
		return "Run 'gt doctor' manually to see full output"
	case "worktrees":
		return "Check crew directories exist in the rig"
	default:
		return "Check command manually: " + e.Command
	}
}

// beadsSuggestedAction returns a specific action for beads load errors.
func (e *LoadError) beadsSuggestedAction() string {
	// Check stderr for specific error patterns
	errorText := e.Stderr
	if errorText == "" {
		errorText = e.Error
	}

	// Detect prefix mismatch: "prefix mismatch", "wrong prefix", or "Configured prefix"
	if containsAny(errorText, []string{"prefix mismatch", "wrong prefix", "Configured prefix"}) {
		return "Run: bd sync --import-only --rename-on-import"
	}

	// Detect out-of-sync or import errors
	if containsAny(errorText, []string{"out of sync", "out-of-sync", "import failed", "import error"}) {
		return "Run: bd sync --import-only"
	}

	// Detect stale JSONL or database issues
	if containsAny(errorText, []string{"stale", "newer data available", "auto-import disabled"}) {
		return "Run: bd sync --import-only"
	}

	// Default suggestion
	return "Verify beads configuration with 'bd list'"
}

// containsAny checks if the text contains any of the substrings (case-insensitive).
func containsAny(text string, substrings []string) bool {
	textLower := strings.ToLower(text)
	for _, s := range substrings {
		if strings.Contains(textLower, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// SourceLabel returns a human-readable label for the source.
func (e *LoadError) SourceLabel() string {
	switch e.Source {
	case "town_status":
		return "Town Status"
	case "merge_queue":
		return "Merge Queue"
	case "mail":
		return "Mail"
	case "convoys":
		return "Convoys"
	case "closed_convoys":
		return "Convoy History"
	case "issues":
		return "Issues"
	case "hooked_issues":
		return "Hooked Issues"
	case "polecats":
		return "Polecats"
	case "lifecycle":
		return "Lifecycle Log"
	case "doctor":
		return "Health Checks"
	case "worktrees":
		return "Worktrees"
	default:
		return e.Source
	}
}

// CheckStatus represents the result status of a doctor check.
type CheckStatus string

const (
	CheckPassed  CheckStatus = "passed"
	CheckWarning CheckStatus = "warning"
	CheckError   CheckStatus = "error"
)

// DoctorCheck represents a single health check from gt doctor.
// Loaded by parsing gt doctor output (no JSON available).
type DoctorCheck struct {
	Name       string      `json:"name"`
	Status     CheckStatus `json:"status"`
	Message    string      `json:"message"`
	Details    []string    `json:"details,omitempty"`
	SuggestFix string      `json:"suggest_fix,omitempty"`
}

// DoctorReport represents the full gt doctor output.
type DoctorReport struct {
	Checks       []DoctorCheck `json:"checks"`
	TotalChecks  int           `json:"total_checks"`
	PassedCount  int           `json:"passed_count"`
	WarningCount int           `json:"warning_count"`
	ErrorCount   int           `json:"error_count"`
	LoadedAt     time.Time     `json:"loaded_at"`
}

// HasIssues returns true if there are any warnings or errors.
func (r *DoctorReport) HasIssues() bool {
	return r.WarningCount > 0 || r.ErrorCount > 0
}

// Errors returns only the checks with error status.
func (r *DoctorReport) Errors() []DoctorCheck {
	var errs []DoctorCheck
	for _, c := range r.Checks {
		if c.Status == CheckError {
			errs = append(errs, c)
		}
	}
	return errs
}

// Warnings returns only the checks with warning status.
func (r *DoctorReport) Warnings() []DoctorCheck {
	var warns []DoctorCheck
	for _, c := range r.Checks {
		if c.Status == CheckWarning {
			warns = append(warns, c)
		}
	}
	return warns
}

// Plugin represents a Gas Town plugin.
// Loaded by scanning ~/gt/plugins/ and <rig>/plugins/ directories.
type Plugin struct {
	Name        string    `json:"name"`        // Directory name
	Path        string    `json:"path"`        // Full path to plugin directory
	Title       string    `json:"title"`       // From plugin.md frontmatter
	Description string    `json:"description"` // From plugin.md frontmatter
	GateType    string    `json:"gate_type"`   // cooldown, cron, condition, event
	Schedule    string    `json:"schedule"`    // Cron or cooldown value
	Enabled     bool      `json:"enabled"`     // Whether plugin is enabled
	Scope       string    `json:"scope"`       // "town" or rig name
	LastRun     time.Time `json:"last_run"`    // Last execution time
	LastError   string    `json:"last_error"`  // Last error message if any
	HasError    bool      `json:"has_error"`   // Whether plugin has errors
}

// Identity represents the current actor's identity and provenance.
// Combines whoami info with recent activity.
type Identity struct {
	// Actor info (from whoami/overseer)
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Source   string `json:"source"` // git, env, etc.

	// Rig/role context
	CurrentRig  string `json:"current_rig,omitempty"`
	CurrentRole string `json:"current_role,omitempty"` // mayor, witness, polecat, crew

	// Recent activity (provenance)
	LastCommits []CommitInfo `json:"last_commits,omitempty"`
	LastBeads   []BeadInfo   `json:"last_beads,omitempty"`
}

// CommitInfo represents a recent commit.
type CommitInfo struct {
	Hash    string    `json:"hash"`
	Subject string    `json:"subject"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Rig     string    `json:"rig,omitempty"`
}

// BeadInfo represents a recently touched bead.
type BeadInfo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
	Action    string    `json:"action,omitempty"` // created, updated, closed
}

// BeadRoute represents a prefix-to-location mapping for beads routing.
// Loaded from ~/gt/.beads/routes.jsonl
type BeadRoute struct {
	Prefix  string `json:"prefix"`           // e.g., "hq-", "pe-", "gt-"
	Location string `json:"location"`         // Absolute path to beads directory
	Path     string `json:"path,omitempty"`   // Legacy field name for location (for compatibility)
	Rig     string `json:"rig,omitempty"`     // Rig name if applicable (empty for town)
}

// GetLocation returns the location, using Path as fallback for backward compatibility.
func (br *BeadRoute) GetLocation() string {
	if br.Location != "" {
		return br.Location
	}
	return br.Path
}

// Routes maps bead ID prefixes to their beads locations.
// This enables commands like "bd show pe-123" to route to the correct rig.
type Routes struct {
	// Routes maps prefix (e.g., "pe-") to route info
	Entries map[string]BeadRoute `json:"entries"`
}

// PatrolFormulasHealth represents the health status of patrol formula molecules.
// These formulas are required for refinery/witness to auto-start patrols.
type PatrolFormulasHealth struct {
	// HasFormulas indicates if the formula files exist in .beads/formulas/
	HasFormulas bool `json:"has_formulas"`

	// HasMolecules indicates if the molecules exist in .beads/molecules.jsonl
	HasMolecules bool `json:"has_molecules"`

	// MissingFormulas lists the patrol formulas that are missing
	MissingFormulas []string `json:"missing_formulas,omitempty"`

	// FormulasPath is the path where formulas should exist
	FormulasPath string `json:"formulas_path,omitempty"`

	// MoleculesPath is the path to the molecules catalog
	MoleculesPath string `json:"molecules_path,omitempty"`
}

// Status returns a human-readable status message.
func (h *PatrolFormulasHealth) Status() string {
	if h.HasMolecules {
		return "OK"
	}
	if h.HasFormulas {
		return "Formulas exist but not in catalog"
	}
	return "Missing"
}

// NeedsFix returns true if the patrol formulas need to be installed.
func (h *PatrolFormulasHealth) NeedsFix() bool {
	return !h.HasMolecules
}

// FixMessage returns a suggested fix for missing patrol formulas.
func (h *PatrolFormulasHealth) FixMessage() string {
	if !h.HasFormulas {
		return "Run 'gt install' to restore formula files"
	}
	return "Run 'bd formula cook' to add formulas to molecule catalog"
}

// Details returns detailed information about the patrol formulas status.
func (h *PatrolFormulasHealth) Details() []string {
	var details []string
	if len(h.MissingFormulas) > 0 {
		for _, formula := range h.MissingFormulas {
			details = append(details, fmt.Sprintf("Missing: %s", formula))
		}
	}
	if !h.HasFormulas {
		details = append(details, fmt.Sprintf("Formulas directory: %s", h.FormulasPath))
	}
	if !h.HasMolecules {
		details = append(details, fmt.Sprintf("Molecules catalog: %s", h.MoleculesPath))
	}
	return details
}