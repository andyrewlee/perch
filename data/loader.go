package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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

// LoadMail loads mail messages from the inbox.
func (l *Loader) LoadMail(ctx context.Context) ([]MailMessage, error) {
	var mail []MailMessage
	if err := l.execJSON(ctx, &mail, "gt", "mail", "inbox", "--json"); err != nil {
		return nil, fmt.Errorf("loading mail: %w", err)
	}
	return mail, nil
}

// Snapshot represents a complete snapshot of town data at a point in time.
type Snapshot struct {
	Town        *TownStatus
	Polecats    []Polecat
	Convoys     []Convoy
	MergeQueues map[string][]MergeRequest
	Issues      []Issue
	Mail        []MailMessage
	LoadedAt    time.Time
	Errors      []error
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
	wg.Add(4)

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

	wg.Wait()

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
