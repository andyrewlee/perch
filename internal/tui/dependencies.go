package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/andyrewlee/perch/data"
	"github.com/charmbracelet/lipgloss"
)

// Dependency represents a required tool for Gas Town.
type Dependency struct {
	// Name is the command name (e.g., "gt", "bd", "tmux")
	Name string
	// Description is a user-friendly description of what the tool does
	Description string
	// InstallHint provides guidance on how to install the tool
	InstallHint string
}

// requiredDependencies lists all tools needed to run Gas Town.
var requiredDependencies = []Dependency{
	{
		Name:        "tmux",
		Description: "Manages terminal sessions for agents",
		InstallHint: "brew install tmux (macOS) or apt install tmux (Linux)",
	},
	{
		Name:        "gt",
		Description: "Gas Town command-line interface",
		InstallHint: "See https://github.com/anthropics/gas-town for installation",
	},
	{
		Name:        "bd",
		Description: "Beads issue tracker",
		InstallHint: "Installed with Gas Town (gt)",
	},
}

// DependencyStatus represents the check result for a single dependency.
type DependencyStatus struct {
	Dependency Dependency
	Found      bool
}

// DependencyCheckResult contains the results of checking all dependencies.
type DependencyCheckResult struct {
	Statuses []DependencyStatus
	AllFound bool
}

// Missing returns only the missing dependencies.
func (r *DependencyCheckResult) Missing() []Dependency {
	var missing []Dependency
	for _, s := range r.Statuses {
		if !s.Found {
			missing = append(missing, s.Dependency)
		}
	}
	return missing
}

// CheckDependencies checks if all required tools are available.
// Uses the provided command runner, or creates a real one if nil.
func CheckDependencies(runner data.CommandRunner, workDir string) *DependencyCheckResult {
	if runner == nil {
		runner = &realDependencyRunner{}
	}

	result := &DependencyCheckResult{
		AllFound: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, dep := range requiredDependencies {
		status := DependencyStatus{
			Dependency: dep,
			Found:      commandExists(ctx, runner, workDir, dep.Name),
		}
		result.Statuses = append(result.Statuses, status)
		if !status.Found {
			result.AllFound = false
		}
	}

	return result
}

// commandExists checks if a command is available in PATH.
func commandExists(ctx context.Context, runner data.CommandRunner, workDir, cmd string) bool {
	_, _, err := runner.Exec(ctx, workDir, "which", cmd)
	return err == nil
}

// realDependencyRunner executes commands using os/exec.
type realDependencyRunner struct{}

func (r *realDependencyRunner) Exec(ctx context.Context, workDir string, args ...string) ([]byte, []byte, error) {
	// Reuse the actionRunner implementation
	ar := &actionRunner{}
	return ar.Exec(ctx, workDir, args...)
}

// FormatDependencyError creates a user-friendly error message for missing dependencies.
func FormatDependencyError(result *DependencyCheckResult) string {
	if result.AllFound {
		return ""
	}

	missing := result.Missing()
	if len(missing) == 0 {
		return ""
	}

	var names []string
	for _, dep := range missing {
		names = append(names, dep.Name)
	}

	return fmt.Sprintf("missing required tools: %s", strings.Join(names, ", "))
}

// RenderDependencyStatus renders a visual status of all dependencies.
// Used in setup wizard to show what's available and what's missing.
func RenderDependencyStatus(result *DependencyCheckResult) string {
	var lines []string

	for _, status := range result.Statuses {
		var indicator string
		var style lipgloss.Style

		if status.Found {
			indicator = "✓"
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
		} else {
			indicator = "✗"
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6666"))
		}

		name := lipgloss.NewStyle().Bold(true).Render(status.Dependency.Name)
		desc := mutedStyle.Render(" - " + status.Dependency.Description)

		lines = append(lines, fmt.Sprintf("  %s %s%s", style.Render(indicator), name, desc))
	}

	return strings.Join(lines, "\n")
}

// RenderInstallGuidance renders install instructions for missing dependencies.
func RenderInstallGuidance(result *DependencyCheckResult) string {
	missing := result.Missing()
	if len(missing) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, mutedStyle.Render("To install missing tools:"))

	for _, dep := range missing {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("  • %s: %s", dep.Name, dep.InstallHint)))
	}

	return strings.Join(lines, "\n")
}
