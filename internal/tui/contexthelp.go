package tui

// ContextHelp provides inline explanations for Gas Town concepts.
// These help users understand what they're seeing without exposing CLI commands.

// SectionHelp returns a brief description for a sidebar section.
func SectionHelp(section SidebarSection) string {
	switch section {
	case SectionIdentity:
		return "Who you are"
	case SectionRigs:
		return "Project workspaces"
	case SectionConvoys:
		return "Batched work"
	case SectionMergeQueue:
		return "Pending merges"
	case SectionAgents:
		return "All workers"
	case SectionMail:
		return "Messages"
	case SectionLifecycle:
		return "Agent events"
	case SectionWorktrees:
		return "Cross-rig worktrees"
	case SectionPlugins:
		return "Town extensions"
	case SectionErrors:
		return "Load errors"
	default:
		return ""
	}
}

// RoleHelp returns a description for an agent role.
func RoleHelp(role string) string {
	switch role {
	case "witness":
		return "Manages polecat lifecycle (start, nudge, cleanup)"
	case "refinery":
		return "Processes merge queue (rebase, merge to main)"
	case "polecat":
		return "Autonomous worker (witness-managed)"
	case "crew":
		return "Human-managed worker (you control lifecycle)"
	default:
		return ""
	}
}

// StatusHelp returns a description for an agent status.
func StatusHelp(running, hasWork bool, unreadMail int) string {
	if !running {
		return "Session not running"
	}
	if unreadMail > 0 {
		return "Has messages that may need attention"
	}
	if hasWork {
		return "Actively executing assigned work"
	}
	return "Running but waiting for work"
}

// RigComponentHelp provides descriptions for rig components.
var RigComponentHelp = struct {
	Polecats  string
	Crews     string
	Witness   string
	Refinery  string
	MergeQueue string
	Hooks     string
}{
	Polecats:   "Autonomous workers started by the witness",
	Crews:      "Human-managed workers (you run sessions)",
	Witness:    "Manages worker lifecycle automatically",
	Refinery:   "Merges completed work into main branch",
	MergeQueue: "Work waiting to be merged",
	Hooks:      "Work assignment mechanism",
}

// ConvoyHelp provides context about convoys.
var ConvoyHelp = struct {
	Description string
	Statuses    map[string]string
}{
	Description: "A convoy groups related work items for coordinated delivery",
	Statuses: map[string]string{
		"pending":     "Work not yet started",
		"in_progress": "Workers actively processing",
		"completed":   "All work finished and merged",
		"blocked":     "Waiting on dependencies",
	},
}

// MergeQueueHelp provides context about merge requests.
var MergeQueueHelp = struct {
	Description  string
	ConflictHelp string
	RebaseHelp   string
}{
	Description:  "Completed work waiting to be merged into main",
	ConflictHelp: "Files changed in both this branch and main",
	RebaseHelp:   "Branch has fallen behind main, needs update",
}

// BadgeLegend returns the legend for agent status badges.
func BadgeLegend() []struct{ Badge, Status, Desc string } {
	return []struct{ Badge, Status, Desc string }{
		{"●", "working", "Agent running with hooked work"},
		{"○", "idle", "Agent running, waiting for work"},
		{"!", "attention", "Has unread mail"},
		{"◌", "stopped", "Session not running"},
	}
}

// RoleLegend returns the legend for agent role abbreviations.
func RoleLegend() []struct{ Abbrev, Role, Desc string } {
	return []struct{ Abbrev, Role, Desc string }{
		{"P", "polecat", "Autonomous worker"},
		{"W", "witness", "Lifecycle manager"},
		{"R", "refinery", "Merge processor"},
		{"C", "crew", "Human-managed worker"},
	}
}
