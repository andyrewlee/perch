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
	"strconv"
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

// LoadConvoys loads open (active) convoys.
func (l *Loader) LoadConvoys(ctx context.Context) ([]Convoy, error) {
	var convoys []Convoy
	if err := l.execJSON(ctx, &convoys, "gt", "convoy", "list", "--json"); err != nil {
		return nil, fmt.Errorf("loading convoys: %w", err)
	}
	return convoys, nil
}

// LoadClosedConvoys loads recently landed (closed) convoys.
func (l *Loader) LoadClosedConvoys(ctx context.Context) ([]Convoy, error) {
	var convoys []Convoy
	if err := l.execJSON(ctx, &convoys, "gt", "convoy", "list", "--status=closed", "--json"); err != nil {
		return nil, fmt.Errorf("loading closed convoys: %w", err)
	}
	return convoys, nil
}

// LoadConvoyDetails loads detailed info for a single convoy including progress.
func (l *Loader) LoadConvoyDetails(ctx context.Context, id string) (*Convoy, error) {
	var convoy Convoy
	if err := l.execJSON(ctx, &convoy, "gt", "convoy", "status", id, "--json"); err != nil {
		return nil, fmt.Errorf("loading convoy %s: %w", id, err)
	}
	return &convoy, nil
}

// LoadConvoysWithDetails loads all convoys with full progress details.
// This makes one call per convoy but provides complete information.
func (l *Loader) LoadConvoysWithDetails(ctx context.Context) ([]Convoy, error) {
	// First get the list of convoys
	convoys, err := l.LoadConvoys(ctx)
	if err != nil {
		return nil, err
	}

	if len(convoys) == 0 {
		return convoys, nil
	}

	// Load details for each convoy in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	detailed := make([]Convoy, len(convoys))
	errs := make(chan error, len(convoys))

	for i, c := range convoys {
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()
			detail, err := l.LoadConvoyDetails(ctx, id)
			if err != nil {
				errs <- err
				return
			}
			mu.Lock()
			detailed[idx] = *detail
			mu.Unlock()
		}(i, c.ID)
	}

	wg.Wait()
	close(errs)

	// Collect errors but don't fail - use basic convoy data as fallback
	for err := range errs {
		// Log error but continue - we'll have partial data
		_ = err
	}

	return detailed, nil
}

// LoadConvoyStatus loads detailed status for a specific convoy.
func (l *Loader) LoadConvoyStatus(ctx context.Context, convoyID string) (*ConvoyStatus, error) {
	var status ConvoyStatus
	if err := l.execJSON(ctx, &status, "gt", "convoy", "status", convoyID, "--json"); err != nil {
		return nil, fmt.Errorf("loading convoy status for %s: %w", convoyID, err)
	}
	return &status, nil
}

// LoadAllConvoyStatuses loads detailed status for all convoys.
func (l *Loader) LoadAllConvoyStatuses(ctx context.Context, convoys []Convoy) (map[string]*ConvoyStatus, error) {
	result := make(map[string]*ConvoyStatus)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make(chan error, len(convoys))

	for _, convoy := range convoys {
		wg.Add(1)
		go func(c Convoy) {
			defer wg.Done()
			status, err := l.LoadConvoyStatus(ctx, c.ID)
			if err != nil {
				errs <- err
				return
			}
			mu.Lock()
			result[c.ID] = status
			mu.Unlock()
		}(convoy)
	}

	wg.Wait()
	close(errs)

	// Return first error if any
	for err := range errs {
		return nil, err
	}

	return result, nil
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

// LoadIssueDependencies loads dependencies for a specific issue.
// Returns both blocked-by (dependencies) and blocking (dependents) issues.
func (l *Loader) LoadIssueDependencies(ctx context.Context, issueID string) (*IssueDependencies, error) {
	result := &IssueDependencies{
		IssueID:      issueID,
		LastLoadedAt: time.Now(),
	}

	// Load blocked-by issues (dependencies)
	// Command: bd dep list <issue-id>
	var blockedBy []struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Status     string `json:"status"`
		IssueType  string `json:"issue_type"`
		Priority   int    `json:"priority"`
		UpdatedAt  string `json:"updated_at"`
		DepType    string `json:"dependency_type"`
	}

	stdout, _, err := l.Runner.Exec(ctx, l.TownRoot, "bd", "dep", "list", issueID)
	if err != nil {
		// Dependency list may not be implemented yet, return empty result
		result.LoadError = fmt.Errorf("listing dependencies: %w", err)
		return result, nil
	}

	if err := json.Unmarshal(bytes.TrimSpace(stdout), &blockedBy); err != nil {
		result.LoadError = fmt.Errorf("parsing dependencies: %w", err)
		return result, nil
	}

	// Parse blocked-by and blocking from the output
	for _, dep := range blockedBy {
		updatedAt, _ := time.Parse(time.RFC3339, dep.UpdatedAt)
		issueDep := IssueDependency{
			ID:        dep.ID,
			Title:     dep.Title,
			Status:    dep.Status,
			IssueType: dep.IssueType,
			Priority:  dep.Priority,
			UpdatedAt: updatedAt,
		}

		if dep.DepType == "blocks" {
			// This issue is blocked by dep.ID
			result.BlockedBy = append(result.BlockedBy, issueDep)
		} else if dep.DepType == "blocked_by" {
			// This issue blocks dep.ID
			result.Blocking = append(result.Blocking, issueDep)
		}
	}

	return result, nil
}

// AddDependency adds a dependency relationship between issues.
// blockerID blocks blockedID (blockedID depends on blockerID).
// Command: bd dep add <blocked-id> <blocker-id>
func (l *Loader) AddDependency(ctx context.Context, blockedID, blockerID string) error {
	_, _, err := l.Runner.Exec(ctx, l.TownRoot, "bd", "dep", "add", blockedID, blockerID)
	return err
}

// RemoveDependency removes a dependency relationship between issues.
// Command: bd dep remove <blocked-id> <blocker-id>
func (l *Loader) RemoveDependency(ctx context.Context, blockedID, blockerID string) error {
	_, _, err := l.Runner.Exec(ctx, l.TownRoot, "bd", "dep", "remove", blockedID, blockerID)
	return err
}

// LoadIssueComments loads comments for a specific issue.
// Command: bd comments <issue-id> --json
func (l *Loader) LoadIssueComments(ctx context.Context, issueID string) (*IssueComments, error) {
	result := &IssueComments{
		IssueID:      issueID,
		LastLoadedAt: time.Now(),
	}

	var comments []Comment
	if err := l.execJSON(ctx, &comments, "bd", "comments", issueID, "--json"); err != nil {
		result.LoadError = fmt.Errorf("loading comments: %w", err)
		return result, nil
	}

	result.Comments = comments
	return result, nil
}

// AddComment adds a comment to an issue.
// Command: bd comments add <issue-id> <comment>
func (l *Loader) AddComment(ctx context.Context, issueID, comment string) error {
	_, _, err := l.Runner.Exec(ctx, l.TownRoot, "bd", "comments", "add", issueID, comment)
	return err
}

// LoadMail loads mail messages from the inbox.
func (l *Loader) LoadMail(ctx context.Context) ([]MailMessage, error) {
	var mail []MailMessage
	if err := l.execJSON(ctx, &mail, "gt", "mail", "inbox", "--json"); err != nil {
		return nil, fmt.Errorf("loading mail: %w", err)
	}
	return mail, nil
}

// LoadDoctorReport runs gt doctor and parses the output.
func (l *Loader) LoadDoctorReport(ctx context.Context) (*DoctorReport, error) {
	cmd := exec.CommandContext(ctx, "gt", "doctor")
	cmd.Dir = l.TownRoot

	// Capture both stdout and stderr (gt doctor writes to both)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Run command - it may exit with error if there are issues
	_ = cmd.Run() // Ignore error since gt doctor exits 1 on issues

	return parseDoctorOutput(output.String())
}

// parseDoctorOutput parses the text output from gt doctor.
func parseDoctorOutput(output string) (*DoctorReport, error) {
	report := &DoctorReport{
		LoadedAt: time.Now(),
	}

	// Regex patterns for parsing
	// Check line: ✓/⚠/✗ check-name: message
	checkPattern := regexp.MustCompile(`^([✓⚠✗])\s+([a-z0-9-]+):\s+(.*)$`)
	// Detail line: starts with spaces, contains actual info
	detailPattern := regexp.MustCompile(`^\s{4}(.+)$`)
	// Fix suggestion: → message
	fixPattern := regexp.MustCompile(`^\s*→\s+(.+)$`)
	// Summary line: N checks, N passed, N warnings, N errors
	summaryPattern := regexp.MustCompile(`^(\d+)\s+checks?,\s+(\d+)\s+passed,\s+(\d+)\s+warnings?,\s+(\d+)\s+errors?`)

	var currentCheck *DoctorCheck

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// Try to match summary line
		if matches := summaryPattern.FindStringSubmatch(line); matches != nil {
			report.TotalChecks, _ = strconv.Atoi(matches[1])
			report.PassedCount, _ = strconv.Atoi(matches[2])
			report.WarningCount, _ = strconv.Atoi(matches[3])
			report.ErrorCount, _ = strconv.Atoi(matches[4])
			continue
		}

		// Try to match a check line
		if matches := checkPattern.FindStringSubmatch(line); matches != nil {
			// Save previous check if any
			if currentCheck != nil {
				report.Checks = append(report.Checks, *currentCheck)
			}

			status := CheckPassed
			switch matches[1] {
			case "⚠":
				status = CheckWarning
			case "✗":
				status = CheckError
			}

			currentCheck = &DoctorCheck{
				Name:    matches[2],
				Status:  status,
				Message: matches[3],
			}
			continue
		}

		// Try to match fix suggestion
		if matches := fixPattern.FindStringSubmatch(line); matches != nil && currentCheck != nil {
			currentCheck.SuggestFix = matches[1]
			continue
		}

		// Try to match detail line
		if matches := detailPattern.FindStringSubmatch(line); matches != nil && currentCheck != nil {
			detail := strings.TrimSpace(matches[1])
			if detail != "" && !strings.HasPrefix(detail, "→") {
				currentCheck.Details = append(currentCheck.Details, detail)
			}
			continue
		}
	}

	// Save last check
	if currentCheck != nil {
		report.Checks = append(report.Checks, *currentCheck)
	}

	return report, nil
}

// LoadIdentity loads the current actor's identity and provenance data.
// Combines whoami info with recent git commits and bead activity.
func (l *Loader) LoadIdentity(ctx context.Context, overseer *Overseer, issues []Issue) *Identity {
	identity := &Identity{}

	// Copy overseer info if available
	if overseer != nil {
		identity.Name = overseer.Name
		identity.Email = overseer.Email
		identity.Username = overseer.Username
		identity.Source = overseer.Source
	}

	// Load recent commits (last 5)
	commits := l.loadRecentCommits(ctx, 5)
	if len(commits) > 0 {
		identity.LastCommits = commits
	}

	// Extract recent beads from issues (sorted by UpdatedAt)
	beads := extractRecentBeads(issues, 5)
	if len(beads) > 0 {
		identity.LastBeads = beads
	}

	return identity
}

// loadRecentCommits loads the most recent git commits.
func (l *Loader) loadRecentCommits(ctx context.Context, limit int) []CommitInfo {
	// Use git log with JSON-like format
	format := `{"hash":"%h","subject":"%s","author":"%an","date":"%aI"}`
	stdout, _, err := l.Runner.Exec(ctx, l.TownRoot,
		"git", "log", fmt.Sprintf("-n%d", limit), fmt.Sprintf("--pretty=format:%s,", format))
	if err != nil {
		return nil
	}

	// Parse the output (comma-separated JSON objects)
	output := bytes.TrimSpace(stdout)
	if len(output) == 0 {
		return nil
	}

	// Wrap in array and parse
	output = bytes.TrimSuffix(output, []byte(","))
	jsonArray := append([]byte("["), output...)
	jsonArray = append(jsonArray, ']')

	var commits []CommitInfo
	if err := json.Unmarshal(jsonArray, &commits); err != nil {
		return nil
	}

	return commits
}

// extractRecentBeads extracts the most recently updated beads from issues.
func extractRecentBeads(issues []Issue, limit int) []BeadInfo {
	if len(issues) == 0 {
		return nil
	}

	// Sort by UpdatedAt descending (make a copy to avoid mutating original)
	sorted := make([]Issue, len(issues))
	copy(sorted, issues)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].UpdatedAt.After(sorted[i].UpdatedAt) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Take top N
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	beads := make([]BeadInfo, len(sorted))
	for i, issue := range sorted {
		beads[i] = BeadInfo{
			ID:        issue.ID,
			Title:     issue.Title,
			Status:    issue.Status,
			UpdatedAt: issue.UpdatedAt,
		}
	}

	return beads
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
		state.DegradedReason = "tmux unavailable"
		state.DegradedAction = "run 'gt boot' with tmux installed"
		state.Issues = append(state.Issues, "tmux unavailable - running in degraded mode")
	}

	// Check GT_PATROL_MUTED environment variable
	if os.Getenv("GT_PATROL_MUTED") != "" {
		state.PatrolMuted = true
	}

	// Check agent status from town data
	deaconFound := false
	if town != nil {
		for _, agent := range town.Agents {
			switch agent.Role {
			case "health-check": // deacon
				deaconFound = true
				if agent.Running {
					state.LastDeaconHeartbeat = time.Now()
				} else {
					state.WatchdogHealthy = false
					state.WatchdogReason = "deacon stopped"
					state.WatchdogAction = "run 'gt deacon start'"
					state.Issues = append(state.Issues, "deacon not running - watchdog disabled")
				}
			}
		}

		// If no deacon found at all, watchdog is also unhealthy
		if !deaconFound {
			state.WatchdogHealthy = false
			state.WatchdogReason = "deacon not registered"
			state.WatchdogAction = "run 'gt boot' to initialize"
			state.Issues = append(state.Issues, "deacon not found - run 'gt boot' to initialize")
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


// LoadWorktrees scans crew directories across all rigs to find cross-rig worktrees.
func (l *Loader) LoadWorktrees(ctx context.Context, rigs []string) ([]Worktree, error) {
	var worktrees []Worktree

	for _, rig := range rigs {
		crewDir := fmt.Sprintf("%s/%s/crew", l.TownRoot, rig)

		// Read crew directory entries
		entries, err := os.ReadDir(crewDir)
		if err != nil {
			// Crew directory might not exist or be empty
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			wtPath := fmt.Sprintf("%s/%s", crewDir, name)

			// Check if this is a git worktree (has .git file, not directory)
			gitPath := fmt.Sprintf("%s/.git", wtPath)
			info, err := os.Stat(gitPath)
			if err != nil {
				continue
			}

			// Worktrees have a .git file, not a .git directory
			if info.IsDir() {
				continue
			}

			// Parse the worktree name to extract source rig and name
			// Format: <source-rig>-<name> e.g., "gastown-joe"
			sourceRig, sourceName := parseWorktreeName(name)

			// Get git status for the worktree
			branch, status, clean := getWorktreeStatus(ctx, wtPath)

			worktrees = append(worktrees, Worktree{
				Rig:        rig,
				SourceRig:  sourceRig,
				SourceName: sourceName,
				Path:       wtPath,
				Branch:     branch,
				Clean:      clean,
				Status:     status,
			})
		}
	}

	return worktrees, nil
}

// parseWorktreeName extracts source rig and name from worktree directory name.
// Format: <source-rig>-<name> e.g., "gastown-joe" -> ("gastown", "joe")
func parseWorktreeName(name string) (sourceRig, sourceName string) {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", name
}

// getWorktreeStatus gets branch and status for a worktree.
func getWorktreeStatus(ctx context.Context, path string) (branch, status string, clean bool) {
	// Get current branch
	cmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err == nil {
		branch = strings.TrimSpace(string(out))
	} else {
		branch = "unknown"
	}

	// Get status summary
	cmd = exec.CommandContext(ctx, "git", "-C", path, "status", "--porcelain")
	out, err = cmd.Output()
	if err != nil {
		status = "unknown"
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		status = "clean"
		clean = true
	} else {
		count := len(lines)
		if count == 1 {
			status = "1 uncommitted"
		} else {
			status = fmt.Sprintf("%d uncommitted", count)
		}
		clean = false
	}

	return
}

// LoadAuditTimeline loads audit entries for a specific actor.
func (l *Loader) LoadAuditTimeline(ctx context.Context, actor string, limit int) ([]AuditEntry, error) {
	var entries []AuditEntry
	args := []string{"gt", "audit", "--json"}
	if actor != "" {
		args = append(args, "--actor="+actor)
	}
	if limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", limit))
	}
	if err := l.execJSON(ctx, &entries, args...); err != nil {
		return nil, fmt.Errorf("loading audit timeline for %s: %w", actor, err)
	}
	return entries, nil
}

// LoadPlugins scans town and rig plugin directories and returns plugin info.
func (l *Loader) LoadPlugins(ctx context.Context, rigNames []string) ([]Plugin, error) {
	var plugins []Plugin

	// Load town-level plugins
	townPluginsDir := filepath.Join(l.TownRoot, "plugins")
	townPlugins, err := l.scanPluginDir(townPluginsDir, "town")
	if err == nil {
		plugins = append(plugins, townPlugins...)
	}

	// Load rig-level plugins
	for _, rigName := range rigNames {
		rigPluginsDir := filepath.Join(l.TownRoot, rigName, "plugins")
		rigPlugins, err := l.scanPluginDir(rigPluginsDir, rigName)
		if err == nil {
			plugins = append(plugins, rigPlugins...)
		}
	}

	return plugins, nil
}

// scanPluginDir scans a plugins directory and returns plugin info.
func (l *Loader) scanPluginDir(dir string, scope string) ([]Plugin, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var plugins []Plugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(dir, entry.Name())
		plugin := l.loadPluginInfo(pluginPath, entry.Name(), scope)
		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// loadPluginInfo reads plugin.md and extracts frontmatter info.
func (l *Loader) loadPluginInfo(pluginPath, name, scope string) Plugin {
	plugin := Plugin{
		Name:    name,
		Path:    pluginPath,
		Scope:   scope,
		Enabled: true, // Default to enabled
	}

	// Check for disabled marker file
	disabledPath := filepath.Join(pluginPath, ".disabled")
	if _, err := os.Stat(disabledPath); err == nil {
		plugin.Enabled = false
	}

	// Check for error file
	errorPath := filepath.Join(pluginPath, ".last_error")
	if data, err := os.ReadFile(errorPath); err == nil {
		plugin.LastError = strings.TrimSpace(string(data))
		plugin.HasError = plugin.LastError != ""
	}

	// Check for last run file
	lastRunPath := filepath.Join(pluginPath, ".last_run")
	if data, err := os.ReadFile(lastRunPath); err == nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
			plugin.LastRun = t
		}
	}

	// Parse plugin.md for metadata
	pluginMdPath := filepath.Join(pluginPath, "plugin.md")
	file, err := os.Open(pluginMdPath)
	if err != nil {
		plugin.Title = name // Use directory name as fallback title
		return plugin
	}
	defer file.Close()

	// Parse TOML frontmatter (between +++ markers)
	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	var frontmatterLines []string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "+++" {
			if inFrontmatter {
				break // End of frontmatter
			}
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		}
	}

	// Parse frontmatter lines (simple key = "value" parsing)
	for _, line := range frontmatterLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove quotes
		value = strings.Trim(value, `"'`)

		switch key {
		case "title":
			plugin.Title = value
		case "description":
			plugin.Description = value
		case "gate":
			plugin.GateType = value
		case "schedule", "cooldown", "cron":
			plugin.Schedule = value
		}
	}

	if plugin.Title == "" {
		plugin.Title = name
	}

	return plugin
}

// Snapshot represents a complete snapshot of town data at a point in time.
type Snapshot struct {
	Town             *TownStatus
	Polecats         []Polecat
	Convoys          []Convoy                  // Active/open convoys
	ClosedConvoys    []Convoy                  // Recently landed convoys
	ConvoyStatuses   map[string]*ConvoyStatus  // Detailed convoy status by ID
	Worktrees        []Worktree
	MergeQueues      map[string][]MergeRequest
	Issues           []Issue
	HookedIssues     []Issue // Issues with hooked or in_progress status (active work)
	HookedLoaded     bool    // True if HookedIssues loaded successfully (false on error)
	Mail             []MailMessage
	Plugins          []Plugin
	Identity         *Identity
	Lifecycle        *LifecycleLog
	OperationalState *OperationalState
	DoctorReport     *DoctorReport
	Routes           *Routes // Beads prefix-to-location routing table
	LoadedAt         time.Time
	Errors           []error // Deprecated: use LoadErrors for structured error info
	LoadErrors       []LoadError          // Structured errors with source context
	LastSuccess      map[string]time.Time // Per-source last successful load time
}

// LoadAll loads all data sources into a snapshot.
// Errors for individual sources are collected but don't stop other loads.
func (l *Loader) LoadAll(ctx context.Context) *Snapshot {
	now := time.Now()
	snap := &Snapshot{
		LoadedAt:    now,
		MergeQueues: make(map[string][]MergeRequest),
		LastSuccess: make(map[string]time.Time),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Helper to add a structured error
	addError := func(source, command string, err error) {
		loadErr := LoadError{
			Source:     source,
			Command:    command,
			Error:      err.Error(),
			OccurredAt: now,
		}
		mu.Lock()
		snap.LoadErrors = append(snap.LoadErrors, loadErr)
		snap.Errors = append(snap.Errors, err) // Keep for backwards compat
		mu.Unlock()
	}

	// Helper to mark success
	markSuccess := func(source string) {
		mu.Lock()
		snap.LastSuccess[source] = now
		mu.Unlock()
	}

	// Load town status first (we need rig names for MQ)
	town, err := l.LoadTownStatus(ctx)
	if err != nil {
		addError("town_status", "gt status --json --fast", err)
	} else {
		snap.Town = town
		markSuccess("town_status")
	}

	// Parallel loads (polecats, convoys, closedConvoys, issues, mail, hookedIssues, lifecycle, doctor)
	wg.Add(8)

	go func() {
		defer wg.Done()
		polecats, err := l.LoadPolecats(ctx)
		if err != nil {
			addError("polecats", "gt polecat list --all --json", err)
		} else {
			mu.Lock()
			snap.Polecats = polecats
			mu.Unlock()
			markSuccess("polecats")
		}
	}()

	go func() {
		defer wg.Done()
		convoys, err := l.LoadConvoysWithDetails(ctx)
		if err != nil {
			addError("convoys", "gt convoy list --json", err)
		} else {
			mu.Lock()
			snap.Convoys = convoys
			mu.Unlock()
			markSuccess("convoys")
		}
	}()

	go func() {
		defer wg.Done()
		closedConvoys, err := l.LoadClosedConvoys(ctx)
		if err != nil {
			addError("closed_convoys", "gt convoy list --status=closed --json", err)
		} else {
			mu.Lock()
			snap.ClosedConvoys = closedConvoys
			mu.Unlock()
			markSuccess("closed_convoys")
		}
	}()

	go func() {
		defer wg.Done()
		issues, err := l.LoadIssues(ctx)
		if err != nil {
			addError("issues", "bd list --json --limit 0", err)
		} else {
			mu.Lock()
			snap.Issues = issues
			mu.Unlock()
			markSuccess("issues")
		}
	}()

	go func() {
		defer wg.Done()
		mail, err := l.LoadMail(ctx)
		if err != nil {
			addError("mail", "gt mail inbox --json", err)
		} else {
			mu.Lock()
			snap.Mail = mail
			mu.Unlock()
			markSuccess("mail")
		}
	}()

	go func() {
		defer wg.Done()
		// Load both hooked and in_progress issues as active work
		hooked, err := l.LoadHookedIssues(ctx)
		if err != nil {
			addError("hooked_issues", "bd list --json --status hooked --limit 0", err)
		} else {
			mu.Lock()
			snap.HookedIssues = hooked
			snap.HookedLoaded = true
			mu.Unlock()
			markSuccess("hooked_issues")
		}
	}()

	go func() {
		defer wg.Done()
		lifecycle, err := l.LoadLifecycleLog(ctx, 100) // Load last 100 events
		if err != nil {
			addError("lifecycle", "$GT_ROOT/logs/town.log", err)
		} else {
			mu.Lock()
			snap.Lifecycle = lifecycle
			mu.Unlock()
			markSuccess("lifecycle")
		}
	}()

	go func() {
		defer wg.Done()
		doctor, err := l.LoadDoctorReport(ctx)
		if err != nil {
			addError("doctor", "gt doctor", err)
		} else {
			mu.Lock()
			snap.DoctorReport = doctor
			mu.Unlock()
			markSuccess("doctor")
		}
	}()

	wg.Wait()

	// Load operational state (requires town status)
	snap.OperationalState = l.LoadOperationalState(ctx, snap.Town)

	// Load convoy statuses (requires convoys to be loaded)
	if len(snap.Convoys) > 0 {
		statuses, err := l.LoadAllConvoyStatuses(ctx, snap.Convoys)
		if err != nil {
			addError("convoy_statuses", "gt convoy status --json", err)
		} else {
			snap.ConvoyStatuses = statuses
			markSuccess("convoy_statuses")
		}
	}

	// Load MQ for each rig (requires town status)
	if snap.Town != nil {
		rigNames := make([]string, len(snap.Town.Rigs))
		for i, rig := range snap.Town.Rigs {
			rigNames[i] = rig.Name
			mrs, err := l.LoadMergeQueue(ctx, rig.Name)
			if err != nil {
				addError("merge_queue", fmt.Sprintf("gt mq list %s --json", rig.Name), err)
			} else {
				mu.Lock()
				snap.MergeQueues[rig.Name] = mrs
				mu.Unlock()
				markSuccess("merge_queue_" + rig.Name)
			}
		}

		// Load worktrees (requires rig names)
		worktrees, err := l.LoadWorktrees(ctx, rigNames)
		if err != nil {
			addError("worktrees", "filesystem scan of crew directories", err)
		} else {
			snap.Worktrees = worktrees
			markSuccess("worktrees")
		}

		// Load plugins (requires rig names)
		plugins, err := l.LoadPlugins(ctx, rigNames)
		if err != nil {
			addError("plugins", "scan of plugin directories", err)
		} else {
			snap.Plugins = plugins
			markSuccess("plugins")
		}
	}

	// Load beads routing table (fast file read)
	routes, err := l.LoadRoutes()
	if err != nil {
		addError("routes", "read ~/.gt/.beads/routes.jsonl", err)
	} else {
		snap.Routes = routes
		markSuccess("routes")
	}

	// Load identity (requires town status and issues)
	var overseer *Overseer
	if snap.Town != nil {
		overseer = &snap.Town.Overseer
	}
	snap.Identity = l.LoadIdentity(ctx, overseer, snap.Issues)

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

// HooksDataStale returns true if hook counts may be stale due to:
// - Watchdog/deacon being down (can't refresh agent states)
// - HookedIssues failing to load (fallback to gt status value)
func (s *Snapshot) HooksDataStale() bool {
	// If operational state shows watchdog unhealthy, data may be stale
	if s.OperationalState != nil && !s.OperationalState.WatchdogHealthy {
		return true
	}
	// If we failed to load hooked issues, we're using stale data from gt status
	if !s.HookedLoaded {
		return true
	}
	return false
}

// HooksCountStale returns true if the hooks count itself is unreliable.
// This is only true when hooked issues failed to load (fallback to gt status value).
// When watchdog is down but hooked issues loaded successfully, the count is accurate
// (from beads DB), even though agent runtime states may be stale.
func (s *Snapshot) HooksCountStale() bool {
	// If we failed to load hooked issues, we're using stale data from gt status
	// gt status only checks handoff beads, not hooked issues, so it's incomplete
	return !s.HookedLoaded
}

// EnrichWithHookedBeads reconciles bead-based hook state with town status.
// This updates:
// - Summary.ActiveHooks to reflect actual hooked beads
// - Agent.HasWork, Agent.FirstSubject, and hook details based on bead assignees
// - Rig.Hooks to reflect which agents have hooked work
func (s *Snapshot) EnrichWithHookedBeads() {
	if s.Town == nil {
		return
	}

	// Always update ActiveHooks when hooked issues loaded successfully
	// This ensures the count matches reality even when 0
	// Exclude ephemeral issues and message-type issues from the count
	if s.HookedLoaded {
		activeCount := 0
		for _, issue := range s.HookedIssues {
			if !issue.Ephemeral && issue.IssueType != "message" {
				activeCount++
			}
		}
		s.Town.Summary.ActiveHooks = activeCount
	}
	// If HookedLoaded is false (load failed), keep the existing value from gt status

	if len(s.HookedIssues) == 0 {
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

	// Update agents at the town level
	for i := range s.Town.Agents {
		agent := &s.Town.Agents[i]
		if issue, ok := hookedByAssignee[agent.Address]; ok {
			enrichAgentWithHook(agent, issue)
		}
	}

	// Update agents and hooks within each rig
	for i := range s.Town.Rigs {
		rig := &s.Town.Rigs[i]
		activeCount := 0

		// Update rig-level agents
		for j := range rig.Agents {
			agent := &rig.Agents[j]
			if issue, ok := hookedByAssignee[agent.Address]; ok {
				enrichAgentWithHook(agent, issue)
			}
		}

		// Update rig hooks
		for j := range rig.Hooks {
			hook := &rig.Hooks[j]
			// Hook agent format: "perch/ace" -> try "perch/polecats/ace"
			// Check both formats
			if _, ok := hookedByAssignee[hook.Agent]; ok {
				hook.HasWork = true
				activeCount++
			} else {
				// Try polecat format: rig/polecats/name
				parts := splitAgentAddress(hook.Agent)
				if len(parts) == 2 {
					polecatAddr := parts[0] + "/polecats/" + parts[1]
					if _, ok := hookedByAssignee[polecatAddr]; ok {
						hook.HasWork = true
						activeCount++
					}
				}
			}
		}

		// Also count hooked issues assigned to agents not in Hooks array
		// by checking all hooked issues whose assignee starts with this rig name
		// Exclude ephemeral issues and message-type issues
		rigPrefix := rig.Name + "/"
		for _, issue := range s.HookedIssues {
			if !issue.Ephemeral && issue.IssueType != "message" && strings.HasPrefix(issue.Assignee, rigPrefix) {
				// Check if this assignee was already counted via Hooks
				alreadyCounted := false
				for _, hook := range rig.Hooks {
					if hook.Agent == issue.Assignee {
						alreadyCounted = true
						break
					}
					// Also check polecat format conversion
					parts := splitAgentAddress(hook.Agent)
					if len(parts) == 2 {
						polecatAddr := parts[0] + "/polecats/" + parts[1]
						if polecatAddr == issue.Assignee {
							alreadyCounted = true
							break
						}
					}
				}
				if !alreadyCounted {
					activeCount++
				}
			}
		}

		rig.ActiveHooks = activeCount
	}
}

// enrichAgentWithHook populates agent hook fields from a hooked issue.
func enrichAgentWithHook(agent *Agent, issue *Issue) {
	agent.HasWork = true
	agent.FirstSubject = issue.Title
	agent.HookedBeadID = issue.ID
	agent.HookedStatus = issue.Status
	agent.HookedAt = issue.UpdatedAt
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

// rigsRegistry represents the structure of rigs.json
type rigsRegistry struct {
	Version int                    `json:"version"`
	Rigs    map[string]rigRegistry `json:"rigs"`
}

type rigRegistry struct {
	GitURL  string    `json:"git_url"`
	AddedAt time.Time `json:"added_at"`
	Beads   struct {
		Prefix string `json:"prefix"`
	} `json:"beads"`
}

// rigConfig represents the structure of <rig>/settings/config.json
type rigConfig struct {
	Theme      string           `json:"theme,omitempty"`
	MaxWorkers int              `json:"max_workers,omitempty"`
	MergeQueue MergeQueueConfig `json:"merge_queue"`
}

// LoadRigSettings loads settings for a specific rig.
// It combines data from rigs.json and <rig>/mayor/rig/settings/config.json
func (l *Loader) LoadRigSettings(ctx context.Context, rigName string) (*RigSettings, error) {
	settings := &RigSettings{
		Name: rigName,
		MergeQueue: MergeQueueConfig{
			Enabled:     true,
			RunTests:    true,
			TestCommand: "go test ./...",
		},
	}

	// Load from rigs.json
	rigsPath := filepath.Join(l.TownRoot, "mayor", "rigs.json")
	rigsData, err := os.ReadFile(rigsPath)
	if err == nil {
		var registry rigsRegistry
		if err := json.Unmarshal(rigsData, &registry); err == nil {
			if rig, ok := registry.Rigs[rigName]; ok {
				settings.GitURL = rig.GitURL
				settings.Prefix = rig.Beads.Prefix
			}
		}
	}

	// Load from <rig>/mayor/rig/settings/config.json
	configPath := filepath.Join(l.TownRoot, rigName, "mayor", "rig", "settings", "config.json")
	configData, err := os.ReadFile(configPath)
	if err == nil {
		var config rigConfig
		if err := json.Unmarshal(configData, &config); err == nil {
			settings.Theme = config.Theme
			settings.MaxWorkers = config.MaxWorkers
			settings.MergeQueue = config.MergeQueue
		}
	}

	return settings, nil
}

// SaveRigSettings saves settings for a specific rig.
// It updates both rigs.json (for prefix) and <rig>/mayor/rig/settings/config.json
func (l *Loader) SaveRigSettings(ctx context.Context, settings *RigSettings) error {
	if err := settings.Validate(); err != nil {
		return err
	}

	// Update rigs.json (only the prefix, preserve other fields)
	rigsPath := filepath.Join(l.TownRoot, "mayor", "rigs.json")
	rigsData, err := os.ReadFile(rigsPath)
	if err != nil {
		return fmt.Errorf("reading rigs.json: %w", err)
	}

	var registry rigsRegistry
	if err := json.Unmarshal(rigsData, &registry); err != nil {
		return fmt.Errorf("parsing rigs.json: %w", err)
	}

	if rig, ok := registry.Rigs[settings.Name]; ok {
		rig.Beads.Prefix = settings.Prefix
		registry.Rigs[settings.Name] = rig

		updatedData, err := json.MarshalIndent(registry, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling rigs.json: %w", err)
		}

		if err := os.WriteFile(rigsPath, updatedData, 0644); err != nil {
			return fmt.Errorf("writing rigs.json: %w", err)
		}
	}

	// Update <rig>/mayor/rig/settings/config.json
	configPath := filepath.Join(l.TownRoot, settings.Name, "mayor", "rig", "settings", "config.json")

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating settings directory: %w", err)
	}

	config := rigConfig{
		Theme:      settings.Theme,
		MaxWorkers: settings.MaxWorkers,
		MergeQueue: settings.MergeQueue,
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// LoadRoutes loads the beads routing table from ~/gt/.beads/routes.jsonl.
// Each line is a JSON object with "prefix", "location", and optional "rig" fields.
// Returns empty routes if the file doesn't exist (not an error).
func (l *Loader) LoadRoutes() (*Routes, error) {
	routesPath := filepath.Join(l.TownRoot, ".beads", "routes.jsonl")

	data, err := os.ReadFile(routesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No routes file yet - return empty routes
			return &Routes{Entries: make(map[string]BeadRoute)}, nil
		}
		return nil, fmt.Errorf("reading routes: %w", err)
	}

	routes := &Routes{Entries: make(map[string]BeadRoute)}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var route BeadRoute
		if err := json.Unmarshal(line, &route); err != nil {
			// Skip invalid lines
			continue
		}
		if route.Prefix != "" {
			routes.Entries[route.Prefix] = route
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parsing routes: %w", err)
	}

	return routes, nil
}
