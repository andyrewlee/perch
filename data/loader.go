package data

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// CommandRunner abstracts shell command execution.
// Implementations can be real (exec) or mock (for testing).
type CommandRunner interface {
	// Exec runs a command and returns stdout, stderr, and error.
	Exec(ctx context.Context, workDir string, args ...string) (stdout, stderr []byte, err error)
}

// realRunner executes commands using os/exec.
type realRunner struct{}

func (r *realRunner) Exec(ctx context.Context, workDir string, args ...string) ([]byte, []byte, error) {
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

// Loader executes CLI commands and parses their JSON output.
type Loader struct {
	// TownRoot is the Gas Town root directory (where gt commands run).
	TownRoot string

	// Runner executes commands. If nil, uses real exec.
	Runner CommandRunner
}

// NewLoader creates a loader for the given town root.
func NewLoader(townRoot string) *Loader {
	return &Loader{TownRoot: townRoot, Runner: &realRunner{}}
}

// NewLoaderWithRunner creates a loader with a custom command runner.
// Useful for testing with mock responses.
func NewLoaderWithRunner(townRoot string, runner CommandRunner) *Loader {
	return &Loader{TownRoot: townRoot, Runner: runner}
}

// execJSON runs a command and unmarshals its JSON output into dst.
func (l *Loader) execJSON(ctx context.Context, dst any, args ...string) error {
	stdout, stderr, err := l.Runner.Exec(ctx, l.TownRoot, args...)
	if err != nil {
		return fmt.Errorf("%s: %w: %s", args[0], err, string(stderr))
	}

	// Handle null/empty output
	out := bytes.TrimSpace(stdout)
	if len(out) == 0 || string(out) == "null" {
		return nil
	}

	if err := json.Unmarshal(out, dst); err != nil {
		return fmt.Errorf("parsing %s output: %w", args[0], err)
	}

	return nil
}

// LoadTownStatus loads the complete town status.
// Uses --fast to skip mail lookups for faster execution.
func (l *Loader) LoadTownStatus(ctx context.Context) (*TownStatus, error) {
	var status TownStatus
	if err := l.execJSON(ctx, &status, "gt", "status", "--json", "--fast"); err != nil {
		return nil, fmt.Errorf("loading town status: %w", err)
	}
	return &status, nil
}

// LoadPolecats loads all polecats across all rigs.
func (l *Loader) LoadPolecats(ctx context.Context) ([]Polecat, error) {
	var polecats []Polecat
	if err := l.execJSON(ctx, &polecats, "gt", "polecat", "list", "--all", "--json"); err != nil {
		return nil, fmt.Errorf("loading polecats: %w", err)
	}
	return polecats, nil
}

// LoadConvoys loads all convoys.
func (l *Loader) LoadConvoys(ctx context.Context) ([]Convoy, error) {
	var convoys []Convoy
	if err := l.execJSON(ctx, &convoys, "gt", "convoy", "list", "--json"); err != nil {
		return nil, fmt.Errorf("loading convoys: %w", err)
	}
	return convoys, nil
}

// LoadMergeQueue loads merge queue items for a specific rig.
func (l *Loader) LoadMergeQueue(ctx context.Context, rig string) ([]MergeRequest, error) {
	var mrs []MergeRequest
	if err := l.execJSON(ctx, &mrs, "gt", "mq", "list", rig, "--json"); err != nil {
		return nil, fmt.Errorf("loading merge queue for %s: %w", rig, err)
	}
	return mrs, nil
}

// LoadAllMergeQueues loads merge queues for all rigs in the town.
func (l *Loader) LoadAllMergeQueues(ctx context.Context, rigs []string) (map[string][]MergeRequest, error) {
	result := make(map[string][]MergeRequest)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make(chan error, len(rigs))

	for _, rig := range rigs {
		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			mrs, err := l.LoadMergeQueue(ctx, r)
			if err != nil {
				errs <- err
				return
			}
			mu.Lock()
			result[r] = mrs
			mu.Unlock()
		}(rig)
	}

	wg.Wait()
	close(errs)

	// Return first error if any
	for err := range errs {
		return nil, err
	}

	return result, nil
}

// LoadIssues loads all issues from beads.
func (l *Loader) LoadIssues(ctx context.Context) ([]Issue, error) {
	var issues []Issue
	if err := l.execJSON(ctx, &issues, "bd", "list", "--json", "--limit", "0"); err != nil {
		return nil, fmt.Errorf("loading issues: %w", err)
	}
	return issues, nil
}

// LoadOpenIssues loads only open issues.
func (l *Loader) LoadOpenIssues(ctx context.Context) ([]Issue, error) {
	var issues []Issue
	if err := l.execJSON(ctx, &issues, "bd", "list", "--json", "--status", "open", "--limit", "0"); err != nil {
		return nil, fmt.Errorf("loading open issues: %w", err)
	}
	return issues, nil
}

// LoadHookedIssues loads issues with hooked status.
// These represent active work assigned to agents via beads.
func (l *Loader) LoadHookedIssues(ctx context.Context) ([]Issue, error) {
	var issues []Issue
	if err := l.execJSON(ctx, &issues, "bd", "list", "--json", "--status", "hooked", "--limit", "0"); err != nil {
		return nil, fmt.Errorf("loading hooked issues: %w", err)
	}
	return issues, nil
}

// LoadInProgressIssues loads issues with in_progress status.
// These also represent active work assigned to agents.
func (l *Loader) LoadInProgressIssues(ctx context.Context) ([]Issue, error) {
	var issues []Issue
	if err := l.execJSON(ctx, &issues, "bd", "list", "--json", "--status", "in_progress", "--limit", "0"); err != nil {
		return nil, fmt.Errorf("loading in_progress issues: %w", err)
	}
	return issues, nil
}

// LoadMail loads mail messages from the inbox.
func (l *Loader) LoadMail(ctx context.Context) ([]MailMessage, error) {
	var mail []MailMessage
	if err := l.execJSON(ctx, &mail, "gt", "mail", "inbox", "--json"); err != nil {
		return nil, fmt.Errorf("loading mail: %w", err)
	}
	return mail, nil
}

// LoadOperationalState loads the operational state of the town.
// This checks environment variables, deacon status, and agent health.
func (l *Loader) LoadOperationalState(ctx context.Context, town *TownStatus) *OperationalState {
	state := &OperationalState{
		WatchdogHealthy:       true, // Assume healthy unless proven otherwise
		LastWitnessHeartbeat:  make(map[string]time.Time),
		LastRefineryHeartbeat: make(map[string]time.Time),
	}

	// Check GT_DEGRADED environment variable
	if os.Getenv("GT_DEGRADED") != "" {
		state.DegradedMode = true
		state.Issues = append(state.Issues, "tmux unavailable - running in degraded mode")
	}

	// Check GT_PATROL_MUTED environment variable
	if os.Getenv("GT_PATROL_MUTED") != "" {
		state.PatrolMuted = true
	}

	// Check agent status from town data
	if town != nil {
		for _, agent := range town.Agents {
			switch agent.Role {
			case "health-check": // deacon
				if agent.Running {
					state.LastDeaconHeartbeat = time.Now()
				} else {
					state.WatchdogHealthy = false
					state.Issues = append(state.Issues, "deacon not running - watchdog disabled")
				}
			}
		}

		// Check per-rig agents
		for _, rig := range town.Rigs {
			for _, agent := range rig.Agents {
				switch agent.Role {
				case "witness":
					if agent.Running {
						state.LastWitnessHeartbeat[rig.Name] = time.Now()
					}
				case "refinery":
					if agent.Running {
						state.LastRefineryHeartbeat[rig.Name] = time.Now()
					}
				}
			}
		}
	}

	return state
}

// Snapshot represents a complete snapshot of town data at a point in time.
type Snapshot struct {
	Town             *TownStatus
	Polecats         []Polecat
	Convoys          []Convoy
	MergeQueues      map[string][]MergeRequest
	Issues           []Issue
	HookedIssues     []Issue // Issues with hooked or in_progress status (active work)
	Mail             []MailMessage
	Lifecycle        *LifecycleLog
	OperationalState *OperationalState
	LoadedAt         time.Time
	Errors           []error
}

// LoadAll loads all data sources into a snapshot.
// Errors for individual sources are collected but don't stop other loads.
func (l *Loader) LoadAll(ctx context.Context) *Snapshot {
	snap := &Snapshot{
		LoadedAt:    time.Now(),
		MergeQueues: make(map[string][]MergeRequest),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Load town status first (we need rig names for MQ)
	town, err := l.LoadTownStatus(ctx)
	if err != nil {
		mu.Lock()
		snap.Errors = append(snap.Errors, err)
		mu.Unlock()
	} else {
		snap.Town = town
	}

	// Parallel loads
	wg.Add(6)

	go func() {
		defer wg.Done()
		polecats, err := l.LoadPolecats(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			snap.Errors = append(snap.Errors, err)
		} else {
			snap.Polecats = polecats
		}
	}()

	go func() {
		defer wg.Done()
		convoys, err := l.LoadConvoys(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			snap.Errors = append(snap.Errors, err)
		} else {
			snap.Convoys = convoys
		}
	}()

	go func() {
		defer wg.Done()
		issues, err := l.LoadIssues(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			snap.Errors = append(snap.Errors, err)
		} else {
			snap.Issues = issues
		}
	}()

	go func() {
		defer wg.Done()
		mail, err := l.LoadMail(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			snap.Errors = append(snap.Errors, err)
		} else {
			snap.Mail = mail
		}
	}()

	go func() {
		defer wg.Done()
		// Load both hooked and in_progress issues as active work
		hooked, err := l.LoadHookedIssues(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			snap.Errors = append(snap.Errors, err)
		} else {
			snap.HookedIssues = hooked
		}
	}()

	go func() {
		defer wg.Done()
		lifecycle, err := l.LoadLifecycleLog(ctx, 100) // Load last 100 events
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			snap.Errors = append(snap.Errors, err)
		} else {
			snap.Lifecycle = lifecycle
		}
	}()

	wg.Wait()

	// Load operational state (requires town status)
	snap.OperationalState = l.LoadOperationalState(ctx, snap.Town)

	// Load MQ for each rig (requires town status)
	if snap.Town != nil {
		for _, rig := range snap.Town.Rigs {
			mrs, err := l.LoadMergeQueue(ctx, rig.Name)
			if err != nil {
				snap.Errors = append(snap.Errors, err)
			} else {
				snap.MergeQueues[rig.Name] = mrs
			}
		}
	}

	// Enrich town status with bead-based hook data
	snap.EnrichWithHookedBeads()

	return snap
}

// HasErrors returns true if the snapshot has any load errors.
func (s *Snapshot) HasErrors() bool {
	return len(s.Errors) > 0
}

// RigNames returns the names of all rigs.
func (s *Snapshot) RigNames() []string {
	if s.Town == nil {
		return nil
	}
	names := make([]string, len(s.Town.Rigs))
	for i, r := range s.Town.Rigs {
		names[i] = r.Name
	}
	return names
}

// UnreadMailCount returns the number of unread mail messages.
func (s *Snapshot) UnreadMailCount() int {
	count := 0
	for _, m := range s.Mail {
		if !m.Read {
			count++
		}
	}
	return count
}

// UnreadMail returns only unread mail messages.
func (s *Snapshot) UnreadMail() []MailMessage {
	var unread []MailMessage
	for _, m := range s.Mail {
		if !m.Read {
			unread = append(unread, m)
		}
	}
	return unread
}

// EnrichWithHookedBeads reconciles bead-based hook state with town status.
// This updates:
// - Summary.ActiveHooks to reflect actual hooked beads
// - Agent.HasWork and Agent.FirstSubject based on bead assignees
// - Rig.Hooks to reflect which agents have hooked work
func (s *Snapshot) EnrichWithHookedBeads() {
	if s.Town == nil || len(s.HookedIssues) == 0 {
		return
	}

	// Build map of assignee address -> hooked issue
	// Assignee format: "perch/polecats/slit" or "rig/role/name"
	hookedByAssignee := make(map[string]*Issue)
	for i := range s.HookedIssues {
		issue := &s.HookedIssues[i]
		if issue.Assignee != "" {
			hookedByAssignee[issue.Assignee] = issue
		}
	}

	// Update Summary.ActiveHooks
	s.Town.Summary.ActiveHooks = len(s.HookedIssues)

	// Update agents at the town level
	for i := range s.Town.Agents {
		agent := &s.Town.Agents[i]
		if issue, ok := hookedByAssignee[agent.Address]; ok {
			agent.HasWork = true
			agent.FirstSubject = issue.Title
		}
	}

	// Update agents and hooks within each rig
	for i := range s.Town.Rigs {
		rig := &s.Town.Rigs[i]

		// Update rig-level agents
		for j := range rig.Agents {
			agent := &rig.Agents[j]
			if issue, ok := hookedByAssignee[agent.Address]; ok {
				agent.HasWork = true
				agent.FirstSubject = issue.Title
			}
		}

		// Update rig hooks
		for j := range rig.Hooks {
			hook := &rig.Hooks[j]
			// Hook agent format: "perch/ace" -> try "perch/polecats/ace"
			// Check both formats
			if _, ok := hookedByAssignee[hook.Agent]; ok {
				hook.HasWork = true
			}
			// Try polecat format: rig/polecats/name
			parts := splitAgentAddress(hook.Agent)
			if len(parts) == 2 {
				polecatAddr := parts[0] + "/polecats/" + parts[1]
				if _, ok := hookedByAssignee[polecatAddr]; ok {
					hook.HasWork = true
				}
			}
		}
	}
}

// splitAgentAddress splits "rig/name" into ["rig", "name"]
func splitAgentAddress(addr string) []string {
	for i := 0; i < len(addr); i++ {
		if addr[i] == '/' {
			return []string{addr[:i], addr[i+1:]}
		}
	}
	return []string{addr}
}

// LoadLifecycleLog loads and parses the town.log file.
// It reads the last 'limit' events from the log file.
func (l *Loader) LoadLifecycleLog(_ context.Context, limit int) (*LifecycleLog, error) {
	logPath := filepath.Join(l.TownRoot, "logs", "town.log")

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Log file doesn't exist yet - not an error
			return &LifecycleLog{LoadedAt: time.Now()}, nil
		}
		return nil, fmt.Errorf("opening town.log: %w", err)
	}
	defer file.Close()

	// Read all lines first (we need to get the last N)
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading town.log: %w", err)
	}

	// Get last 'limit' lines
	start := 0
	if len(allLines) > limit {
		start = len(allLines) - limit
	}
	lines := allLines[start:]

	// Parse events (in reverse order so newest is first)
	events := make([]LifecycleEvent, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		if event, ok := parseLifecycleEvent(lines[i]); ok {
			events = append(events, event)
		}
	}

	return &LifecycleLog{
		Events:   events,
		LoadedAt: time.Now(),
	}, nil
}

// parseLifecycleEvent parses a single line from town.log.
// Format: "2026-01-02 07:09:03 [done] gastown-ui/rictus completed rictus-mjx03nhm"
func parseLifecycleEvent(line string) (LifecycleEvent, bool) {
	// Pattern: timestamp [event_type] agent_or_message rest_of_message
	pattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) \[(\w+)\] (.+)$`)
	matches := pattern.FindStringSubmatch(line)
	if matches == nil {
		return LifecycleEvent{}, false
	}

	timestamp, err := time.Parse("2006-01-02 15:04:05", matches[1])
	if err != nil {
		return LifecycleEvent{}, false
	}

	eventType := LifecycleEventType(matches[2])
	message := matches[3]

	// Extract agent from message based on event type
	agent := extractAgentFromMessage(eventType, message)

	return LifecycleEvent{
		Timestamp: timestamp,
		EventType: eventType,
		Agent:     agent,
		Message:   message,
	}, true
}

// extractAgentFromMessage extracts the agent address from the event message.
func extractAgentFromMessage(eventType LifecycleEventType, message string) string {
	// Different event types have different formats:
	// [nudge] deacon nudged with "..."
	// [done] gastown-ui/rictus completed rictus-mjx03nhm
	// [kill] gastown-ui/rictus killed (gt session stop)
	// [handoff] deacon handed off (...)
	// [spawn] perch/furiosa spawned by witness

	// First word is usually the agent
	parts := strings.Fields(message)
	if len(parts) == 0 {
		return ""
	}

	agent := parts[0]

	// Some agents don't have the rig/ prefix (like "deacon")
	// Keep them as-is for now
	return agent
}
