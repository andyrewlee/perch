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
	case SectionBeads:
		return "Browse issues"
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

// BeadStatusHelp returns a description for a bead status.
func BeadStatusHelp(status string) string {
	switch status {
	case "hooked":
		return "Work assigned to agent - executing immediately"
	case "in_progress":
		return "Agent actively working on this issue"
	case "open":
		return "Not yet started - available to be assigned"
	case "closed":
		return "Completed and resolved"
	default:
		return ""
	}
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

// GlossaryEntry represents a single term and its definition.
type GlossaryEntry struct {
	Term       string
	Definition string
	Extended   string // Additional context (optional)
}

// GlossaryEntries returns all glossary entries for the help system.
func GlossaryEntries() []GlossaryEntry {
	return []GlossaryEntry{
		{
			Term:       "Rig",
			Definition: "A project workspace container with its own agents and issue tracking.",
			Extended:   "Rigs isolate work by project. Each rig has polecats (workers), a witness (lifecycle manager), and a refinery (merge processor).",
		},
		{
			Term:       "Convoy",
			Definition: "A group of related work items (beads) delivered together.",
			Extended:   "Convoys batch multiple tasks for coordinated completion. Statuses: pending → in_progress → completed or blocked.",
		},
		{
			Term:       "Hook",
			Definition: "The work assignment mechanism. Hooked work triggers immediate execution.",
			Extended:   "When work is 'hooked' to an agent, that agent executes autonomously without waiting for confirmation. This is the Gas Town propulsion principle.",
		},
		{
			Term:       "Polecat",
			Definition: "An autonomous worker agent that executes assigned tasks.",
			Extended:   "Polecats are witness-managed. They run in isolated git worktrees, auto-cleanup when idle, and signal completion via the merge queue.",
		},
		{
			Term:       "Witness",
			Definition: "The rig's lifecycle manager for polecat workers.",
			Extended:   "The witness starts polecats when work is assigned, nudges stuck agents, and cleans up idle sessions. Runs automatically in each rig.",
		},
		{
			Term:       "Refinery",
			Definition: "The merge queue processor that lands completed work.",
			Extended:   "The refinery monitors the merge queue, rebases branches when needed, handles conflicts, and merges completed work to main branch.",
		},
		{
			Term:       "Bead",
			Definition: "An issue tracked in the beads system (task, bug, feature, or epic).",
			Extended:   "Beads track work across rigs. They support dependencies, assignees, priorities (P0-P4), and lifecycle states (open, in_progress, hooked, closed).",
		},
		{
			Term:       "Crew",
			Definition: "A human-managed worker where you control the lifecycle.",
			Extended:   "Unlike polecats, crew agents require you to start/stop sessions manually. Useful for interactive development and debugging.",
		},
	}
}

// TermTooltip returns a brief tooltip for a given term.
// Returns empty string if term is not recognized.
func TermTooltip(term string) string {
	entries := GlossaryEntries()
	for _, e := range entries {
		if e.Term == term {
			return e.Definition
		}
	}
	return ""
}

// TermWithTooltip appends a tooltip hint after a term for inline help.
func TermWithTooltip(term, display string) string {
	if tooltip := TermTooltip(term); tooltip != "" {
		return tooltip
	}
	return display
}
