package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/andyrewlee/perch/data"
	"github.com/charmbracelet/lipgloss"
)

// SidebarSection represents which list is active in the sidebar
type SidebarSection int

const (
	SectionRigs SidebarSection = iota
	SectionConvoys
	SectionMergeQueue
	SectionAgents
	SectionMail
	SectionLifecycle
)

// SectionCount is the total number of sidebar sections
const SectionCount = 6

func (s SidebarSection) String() string {
	switch s {
	case SectionRigs:
		return "Rigs"
	case SectionConvoys:
		return "Convoys"
	case SectionMergeQueue:
		return "Merge Queue"
	case SectionAgents:
		return "Agents"
	case SectionMail:
		return "Mail"
	case SectionLifecycle:
		return "Lifecycle"
	default:
		return "Unknown"
	}
}

// SelectableItem represents an item that can be selected in the sidebar
type SelectableItem interface {
	ID() string
	Label() string
	Status() string
}

// convoyItem wraps data.Convoy for selection
type convoyItem struct {
	c data.Convoy
}

func (c convoyItem) ID() string     { return c.c.ID }
func (c convoyItem) Label() string  { return c.c.Title }
func (c convoyItem) Status() string { return c.c.Status }

// mrItem wraps data.MergeRequest for selection
type mrItem struct {
	mr  data.MergeRequest
	rig string
}

func (m mrItem) ID() string { return m.mr.ID }
func (m mrItem) Label() string {
	indicator := ""
	if m.mr.HasConflicts {
		indicator = "!"
	} else if m.mr.NeedsRebase {
		indicator = "~"
	}
	if indicator != "" {
		return fmt.Sprintf("[%s]%s %s", m.rig, indicator, m.mr.Title)
	}
	return fmt.Sprintf("[%s] %s", m.rig, m.mr.Title)
}
func (m mrItem) Status() string { return m.mr.Status }

// agentItem wraps data.Agent for selection
type agentItem struct {
	a data.Agent
}

func (a agentItem) ID() string { return a.a.Address }
func (a agentItem) Label() string {
	badge := agentStatusBadge(a.a.Running, a.a.HasWork, a.a.UnreadMail)
	return fmt.Sprintf("%s %s", badge, a.a.Name)
}
func (a agentItem) Status() string {
	return agentStatusText(a.a.Running, a.a.HasWork, a.a.UnreadMail)
}

// agentStatusBadge returns a colored badge for agent status
func agentStatusBadge(running, hasWork bool, unreadMail int) string {
	if !running {
		return stoppedStyle.Render("◌")
	}
	if unreadMail > 0 {
		return attentionStyle.Render("!")
	}
	if hasWork {
		return workingStyle.Render("●")
	}
	return idleStyle.Render("○")
}

// agentStatusText returns status text for agent
func agentStatusText(running, hasWork bool, unreadMail int) string {
	if !running {
		return "stopped"
	}
	if unreadMail > 0 {
		return "attention"
	}
	if hasWork {
		return "working"
	}
	return "idle"
}

// rigItem wraps data.Rig for selection with aggregated counts
type rigItem struct {
	r       data.Rig
	mrCount int // merge request count for this rig
}

func (r rigItem) ID() string { return r.r.Name }
func (r rigItem) Label() string {
	// Show rig name with summary counts: polecats, active/total hooks
	activeHooks := 0
	for _, h := range r.r.Hooks {
		if h.HasWork {
			activeHooks++
		}
	}
	totalHooks := len(r.r.Hooks)
	return fmt.Sprintf("%s (%dpol %d/%dhk)", r.r.Name, r.r.PolecatCount, activeHooks, totalHooks)
}
func (r rigItem) Status() string {
	// Count running agents
	running := 0
	for _, a := range r.r.Agents {
		if a.Running {
			running++
		}
	}
	if running > 0 {
		return "active"
	}
	return "idle"
}

// mailItem wraps data.MailMessage for selection
type mailItem struct {
	m data.MailMessage
}

func (m mailItem) ID() string { return m.m.ID }
func (m mailItem) Label() string {
	indicator := ""
	if !m.m.Read {
		indicator = "* "
	}
	// Truncate subject for display
	subject := m.m.Subject
	if len(subject) > 30 {
		subject = subject[:27] + "..."
	}
	return fmt.Sprintf("%s%s", indicator, subject)
}
func (m mailItem) Status() string {
	if !m.m.Read {
		return "unread"
	}
	return "read"
}

// lifecycleEventItem wraps data.LifecycleEvent for selection
type lifecycleEventItem struct {
	e data.LifecycleEvent
}

func (l lifecycleEventItem) ID() string { return l.e.Timestamp.Format("150405") + l.e.Agent }
func (l lifecycleEventItem) Label() string {
	badge := lifecycleEventBadge(l.e.EventType)
	timeStr := l.e.Timestamp.Format("15:04")
	agent := l.e.Agent
	if len(agent) > 15 {
		agent = agent[:12] + "..."
	}
	return fmt.Sprintf("%s %s %s", badge, timeStr, agent)
}
func (l lifecycleEventItem) Status() string { return string(l.e.EventType) }

// lifecycleEventBadge returns a colored badge for the event type
func lifecycleEventBadge(eventType data.LifecycleEventType) string {
	switch eventType {
	case data.EventSpawn:
		return spawnStyle.Render("+")
	case data.EventWake:
		return wakeStyle.Render("~")
	case data.EventNudge:
		return nudgeStyle.Render("!")
	case data.EventHandoff:
		return handoffStyle.Render(">")
	case data.EventDone:
		return doneStyle.Render("✓")
	case data.EventCrash:
		return crashStyle.Render("✗")
	case data.EventKill:
		return killStyle.Render("×")
	default:
		return mutedStyle.Render("?")
	}
}

// SidebarState manages sidebar list selection
type SidebarState struct {
	Section   SidebarSection
	Selection int // Index within current section

	// Cached items for each section
	Rigs            []rigItem
	Convoys         []convoyItem
	MRs             []mrItem
	Agents          []agentItem
	Mail            []mailItem
	LifecycleEvents []lifecycleEventItem

	// Lifecycle filters
	LifecycleFilter      data.LifecycleEventType // Empty = show all
	LifecycleAgentFilter string                  // Empty = show all

	// Loading/error state for agents panel
	AgentsLastRefresh time.Time // Last successful agent data refresh
	AgentsLoadError   error     // Error from last agent load attempt (nil if successful)
	AgentsLoading     bool      // True during initial load
}

// NewSidebarState creates a new sidebar state
func NewSidebarState() *SidebarState {
	return &SidebarState{
		Section:       SectionRigs,
		Selection:     0,
		AgentsLoading: true, // Start in loading state until first successful refresh
	}
}

// UpdateFromSnapshot refreshes the sidebar data from a snapshot
func (s *SidebarState) UpdateFromSnapshot(snap *data.Snapshot) {
	if snap == nil {
		return
	}

	// Build MR count per rig first (needed for rig items)
	mrCounts := make(map[string]int)
	for rig, mrs := range snap.MergeQueues {
		mrCounts[rig] = len(mrs)
	}

	// Update rigs
	if snap.Town != nil {
		s.Rigs = make([]rigItem, len(snap.Town.Rigs))
		for i, r := range snap.Town.Rigs {
			s.Rigs[i] = rigItem{r: r, mrCount: mrCounts[r.Name]}
		}
	}

	// Update convoys
	s.Convoys = make([]convoyItem, len(snap.Convoys))
	for i, c := range snap.Convoys {
		s.Convoys[i] = convoyItem{c}
	}

	// Update merge requests (flatten all rigs)
	s.MRs = nil
	for rig, mrs := range snap.MergeQueues {
		for _, mr := range mrs {
			s.MRs = append(s.MRs, mrItem{mr, rig})
		}
	}

	// Update agents with loading/error tracking
	// Preserve last-known agents when Town fails to load
	if snap.Town != nil {
		s.Agents = make([]agentItem, len(snap.Town.Agents))
		for i, a := range snap.Town.Agents {
			s.Agents[i] = agentItem{a}
		}
		s.AgentsLastRefresh = snap.LoadedAt
		s.AgentsLoadError = nil
		s.AgentsLoading = false
	} else {
		// Town failed to load - find the error if any
		s.AgentsLoading = false
		for _, err := range snap.Errors {
			if err != nil {
				s.AgentsLoadError = err
				break
			}
		}
		// Preserve s.Agents (last-known value) - don't clear it
	}

	// Update mail
	s.Mail = make([]mailItem, len(snap.Mail))
	for i, m := range snap.Mail {
		s.Mail[i] = mailItem{m}
	}

	// Update lifecycle events (with filtering)
	s.LifecycleEvents = nil
	if snap.Lifecycle != nil {
		for _, e := range snap.Lifecycle.Events {
			// Apply type filter
			if s.LifecycleFilter != "" && e.EventType != s.LifecycleFilter {
				continue
			}
			// Apply agent filter
			if s.LifecycleAgentFilter != "" && e.Agent != s.LifecycleAgentFilter {
				continue
			}
			s.LifecycleEvents = append(s.LifecycleEvents, lifecycleEventItem{e})
		}
	}

	// Clamp selection to valid range
	s.clampSelection()
}

// CurrentItems returns the items for the current section
func (s *SidebarState) CurrentItems() []SelectableItem {
	switch s.Section {
	case SectionRigs:
		items := make([]SelectableItem, len(s.Rigs))
		for i, r := range s.Rigs {
			items[i] = r
		}
		return items
	case SectionConvoys:
		items := make([]SelectableItem, len(s.Convoys))
		for i, c := range s.Convoys {
			items[i] = c
		}
		return items
	case SectionMergeQueue:
		items := make([]SelectableItem, len(s.MRs))
		for i, m := range s.MRs {
			items[i] = m
		}
		return items
	case SectionAgents:
		items := make([]SelectableItem, len(s.Agents))
		for i, a := range s.Agents {
			items[i] = a
		}
		return items
	case SectionMail:
		items := make([]SelectableItem, len(s.Mail))
		for i, m := range s.Mail {
			items[i] = m
		}
		return items
	case SectionLifecycle:
		items := make([]SelectableItem, len(s.LifecycleEvents))
		for i, e := range s.LifecycleEvents {
			items[i] = e
		}
		return items
	}
	return nil
}

// SelectedItem returns the currently selected item, or nil
func (s *SidebarState) SelectedItem() SelectableItem {
	items := s.CurrentItems()
	if s.Selection >= 0 && s.Selection < len(items) {
		return items[s.Selection]
	}
	return nil
}

// NextSection moves to the next section
func (s *SidebarState) NextSection() {
	s.Section = (s.Section + 1) % SectionCount
	s.Selection = 0
	s.clampSelection()
}

// PrevSection moves to the previous section
func (s *SidebarState) PrevSection() {
	s.Section = (s.Section + SectionCount - 1) % SectionCount
	s.Selection = 0
	s.clampSelection()
}

// SelectNext moves selection down
func (s *SidebarState) SelectNext() {
	items := s.CurrentItems()
	if len(items) > 0 {
		s.Selection = (s.Selection + 1) % len(items)
	}
}

// SelectPrev moves selection up
func (s *SidebarState) SelectPrev() {
	items := s.CurrentItems()
	if len(items) > 0 {
		s.Selection = (s.Selection + len(items) - 1) % len(items)
	}
}

func (s *SidebarState) clampSelection() {
	items := s.CurrentItems()
	if s.Selection >= len(items) {
		s.Selection = len(items) - 1
	}
	if s.Selection < 0 {
		s.Selection = 0
	}
}

// RenderSidebar renders the sidebar with all sections
func RenderSidebar(state *SidebarState, snap *data.Snapshot, width, height int, focused bool) string {
	innerWidth := width - 4
	innerHeight := height - 2

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	// Calculate height per section (4 sections)
	sectionHeight := (innerHeight - SectionCount) / SectionCount // -SectionCount for headers
	if sectionHeight < 2 {
		sectionHeight = 2
	}

	var sections []string

	// Render each section
	for sec := SectionRigs; sec <= SectionLifecycle; sec++ {
		isActive := state.Section == sec
		header := renderSectionHeader(sec.String(), sec, isActive)
		items := getSectionItems(state, sec)

		var list string
		if sec == SectionMergeQueue && len(items) == 0 {
			// Special empty state for merge queue with context
			list = renderMQEmptyState(snap, innerWidth)
		} else if sec == SectionAgents {
			// Special handling for agents section with loading/error states
			list = renderAgentsList(state, items, isActive, innerWidth, sectionHeight)
		} else {
			list = renderItemList(items, state.Selection, isActive, innerWidth, sectionHeight)
		}
		sections = append(sections, header, list)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Pad to fill height
	lines := strings.Split(content, "\n")
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	content = strings.Join(lines, "\n")

	style := sidebarStyle.
		Width(innerWidth).
		Height(innerHeight)

	if focused {
		style = style.BorderForeground(highlight)
	}

	return style.Render(content)
}

func renderSectionHeader(name string, section SidebarSection, active bool) string {
	style := headerStyle
	if active {
		style = style.Foreground(highlight)
	}
	header := style.Render(name)

	// Add section description when active
	if active {
		if help := SectionHelp(section); help != "" {
			header += " " + mutedStyle.Render("("+help+")")
		}
	}
	return header
}

func getSectionItems(state *SidebarState, sec SidebarSection) []SelectableItem {
	switch sec {
	case SectionRigs:
		items := make([]SelectableItem, len(state.Rigs))
		for i, r := range state.Rigs {
			items[i] = r
		}
		return items
	case SectionConvoys:
		items := make([]SelectableItem, len(state.Convoys))
		for i, c := range state.Convoys {
			items[i] = c
		}
		return items
	case SectionMergeQueue:
		items := make([]SelectableItem, len(state.MRs))
		for i, m := range state.MRs {
			items[i] = m
		}
		return items
	case SectionAgents:
		items := make([]SelectableItem, len(state.Agents))
		for i, a := range state.Agents {
			items[i] = a
		}
		return items
	case SectionMail:
		items := make([]SelectableItem, len(state.Mail))
		for i, m := range state.Mail {
			items[i] = m
		}
		return items
	case SectionLifecycle:
		items := make([]SelectableItem, len(state.LifecycleEvents))
		for i, e := range state.LifecycleEvents {
			items[i] = e
		}
		return items
	}
	return nil
}

// renderMQEmptyState renders the merge queue empty state with refinery context
func renderMQEmptyState(snap *data.Snapshot, width int) string {
	var lines []string

	// Find refinery agents and their status
	refineryRunning := false
	refineryCount := 0
	if snap != nil && snap.Town != nil {
		for _, agent := range snap.Town.Agents {
			if agent.Role == "refinery" {
				refineryCount++
				if agent.Running {
					refineryRunning = true
				}
			}
		}
	}

	// Show refinery status
	if refineryCount > 0 {
		if refineryRunning {
			lines = append(lines, idleStyle.Render("  ○ Refinery idle"))
		} else {
			lines = append(lines, stoppedStyle.Render("  ◌ Refinery stopped"))
		}
	} else {
		lines = append(lines, mutedStyle.Render("  No refinery configured"))
	}

	// Healthy empty hint
	lines = append(lines, mutedStyle.Render("  Queue clear - work landing"))

	return strings.Join(lines, "\n")
}

func renderItemList(items []SelectableItem, selection int, isActiveSection bool, width, maxLines int) string {
	if len(items) == 0 {
		return mutedStyle.Render("  (empty)")
	}

	var lines []string
	for i, item := range items {
		if len(lines) >= maxLines {
			break
		}

		label := item.Label()
		if len(label) > width-4 {
			label = label[:width-7] + "..."
		}

		if isActiveSection && i == selection {
			lines = append(lines, selectedItemStyle.Render("> "+label))
		} else {
			lines = append(lines, itemStyle.Render("  "+label))
		}
	}

	return strings.Join(lines, "\n")
}

// renderAgentsList renders the agents list with loading/error state indicators.
// Per acceptance criteria: always show last-known agents; if loading, show explicit
// loading state; if error, show error + last refresh time.
func renderAgentsList(state *SidebarState, items []SelectableItem, isActiveSection bool, width, maxLines int) string {
	var lines []string

	// Show loading indicator during initial load
	if state.AgentsLoading {
		lines = append(lines, mutedStyle.Render("  Loading agents..."))
		return strings.Join(lines, "\n")
	}

	// Show error indicator if agents failed to load (but still show last-known)
	if state.AgentsLoadError != nil {
		errLine := statusErrorStyle.Render("  ! Load error")
		if !state.AgentsLastRefresh.IsZero() {
			errLine += mutedStyle.Render(" (last: " + state.AgentsLastRefresh.Format("15:04") + ")")
		}
		lines = append(lines, errLine)
	}

	// Show last-known agents (or empty state if none)
	if len(items) == 0 {
		if state.AgentsLoadError != nil {
			lines = append(lines, mutedStyle.Render("  (no cached agents)"))
		} else {
			lines = append(lines, mutedStyle.Render("  (empty)"))
		}
		return strings.Join(lines, "\n")
	}

	// Calculate remaining lines for agent list
	remainingLines := maxLines - len(lines)
	if remainingLines < 1 {
		remainingLines = 1
	}

	// Render agent items
	for i, item := range items {
		if len(lines) >= maxLines {
			break
		}

		label := item.Label()
		if len(label) > width-4 {
			label = label[:width-7] + "..."
		}

		if isActiveSection && i == state.Selection {
			lines = append(lines, selectedItemStyle.Render("> "+label))
		} else {
			lines = append(lines, itemStyle.Render("  "+label))
		}
	}

	return strings.Join(lines, "\n")
}

// RenderDetails renders the details panel for the selected item
func RenderDetails(state *SidebarState, snap *data.Snapshot, width, height int, focused bool) string {
	innerWidth := width - 4
	innerHeight := height - 2

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	title := titleStyle.Render("Details")
	content := renderSelectedDetails(state, snap, innerWidth)

	// Pad content to fill space
	inner := lipgloss.JoinVertical(lipgloss.Left, title, content)
	lines := strings.Split(inner, "\n")
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	inner = strings.Join(lines, "\n")

	style := detailsStyle.
		Width(innerWidth).
		Height(innerHeight)

	if focused {
		style = style.BorderForeground(highlight)
	}

	return style.Render(inner)
}

func renderSelectedDetails(state *SidebarState, snap *data.Snapshot, width int) string {
	if state == nil || snap == nil {
		return mutedStyle.Render("No data loaded")
	}

	switch state.Section {
	case SectionRigs:
		if state.Selection >= 0 && state.Selection < len(state.Rigs) {
			return renderRigDetails(state.Rigs[state.Selection], width)
		}
	case SectionConvoys:
		if state.Selection >= 0 && state.Selection < len(state.Convoys) {
			return renderConvoyDetails(state.Convoys[state.Selection].c, width)
		}
	case SectionMergeQueue:
		if state.Selection >= 0 && state.Selection < len(state.MRs) {
			mr := state.MRs[state.Selection]
			return renderMRDetails(mr.mr, mr.rig, width)
		}
	case SectionAgents:
		if state.Selection >= 0 && state.Selection < len(state.Agents) {
			return renderAgentDetails(state.Agents[state.Selection].a, width)
		}
	case SectionMail:
		if state.Selection >= 0 && state.Selection < len(state.Mail) {
			return renderMailDetails(state.Mail[state.Selection].m, width)
		}
	case SectionLifecycle:
		if state.Selection >= 0 && state.Selection < len(state.LifecycleEvents) {
			return renderLifecycleDetails(state.LifecycleEvents[state.Selection].e, state, width)
		}
	}

	return mutedStyle.Render("Select an item to see details")
}

func renderConvoyDetails(c data.Convoy, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Convoy"))
	lines = append(lines, mutedStyle.Render(ConvoyHelp.Description))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("ID:      %s", c.ID))
	lines = append(lines, fmt.Sprintf("Title:   %s", c.Title))
	lines = append(lines, fmt.Sprintf("Status:  %s", c.Status))
	if statusHelp, ok := ConvoyHelp.Statuses[c.Status]; ok {
		lines = append(lines, mutedStyle.Render("         "+statusHelp))
	}
	lines = append(lines, fmt.Sprintf("Created: %s", c.CreatedAt.Format("2006-01-02 15:04")))
	return strings.Join(lines, "\n")
}

func renderMRDetails(mr data.MergeRequest, rig string, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Merge Request"))
	lines = append(lines, mutedStyle.Render(MergeQueueHelp.Description))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("ID:       %s", mr.ID))
	lines = append(lines, fmt.Sprintf("Rig:      %s", rig))
	lines = append(lines, fmt.Sprintf("Title:    %s", mr.Title))
	lines = append(lines, fmt.Sprintf("Status:   %s", mr.Status))
	lines = append(lines, fmt.Sprintf("Worker:   %s", mr.Worker))
	lines = append(lines, fmt.Sprintf("Branch:   %s", mr.Branch))
	lines = append(lines, fmt.Sprintf("Priority: P%d", mr.Priority))

	// Conflict/Rebase status section
	if mr.HasConflicts || mr.NeedsRebase {
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Issues"))

		if mr.HasConflicts {
			lines = append(lines, conflictStyle.Render("! Merge conflicts detected"))
			if mr.ConflictInfo != "" {
				lines = append(lines, fmt.Sprintf("  %s", mr.ConflictInfo))
			}
		}

		if mr.NeedsRebase {
			lines = append(lines, rebaseStyle.Render("~ Branch needs rebase"))
		}

		// Guidance section
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Resolution"))
		if mr.HasConflicts {
			lines = append(lines, "1. Fetch latest main: git fetch origin main")
			lines = append(lines, fmt.Sprintf("2. Checkout branch:   git checkout %s", mr.Branch))
			lines = append(lines, "3. Rebase on main:    git rebase origin/main")
			lines = append(lines, "4. Fix conflicts in each file")
			lines = append(lines, "5. Stage fixes:       git add <files>")
			lines = append(lines, "6. Continue rebase:   git rebase --continue")
			lines = append(lines, "7. Force push:        git push --force-with-lease")
		} else if mr.NeedsRebase {
			lines = append(lines, "1. Fetch latest main: git fetch origin main")
			lines = append(lines, fmt.Sprintf("2. Checkout branch:   git checkout %s", mr.Branch))
			lines = append(lines, "3. Rebase on main:    git rebase origin/main")
			lines = append(lines, "4. Force push:        git push --force-with-lease")
		}

		// Action hint
		lines = append(lines, "")
		lines = append(lines, mutedStyle.Render("Press 'n' to nudge polecat to resolve"))
	} else {
		// Show last checked if available
		if mr.LastChecked != "" {
			lines = append(lines, "")
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("Checked: %s", mr.LastChecked)))
		}
	}

	return strings.Join(lines, "\n")
}

func renderAgentDetails(a data.Agent, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Agent"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Name:    %s", a.Name))
	lines = append(lines, fmt.Sprintf("Address: %s", a.Address))
	lines = append(lines, fmt.Sprintf("Role:    %s", a.Role))
	if roleHelp := RoleHelp(a.Role); roleHelp != "" {
		lines = append(lines, mutedStyle.Render("         "+roleHelp))
	}
	lines = append(lines, fmt.Sprintf("Session: %s", a.Session))

	// Status with badge and explanation
	badge := agentStatusBadge(a.Running, a.HasWork, a.UnreadMail)
	statusText := agentStatusText(a.Running, a.HasWork, a.UnreadMail)
	lines = append(lines, fmt.Sprintf("Status:  %s %s", badge, statusText))
	statusHelp := StatusHelp(a.Running, a.HasWork, a.UnreadMail)
	lines = append(lines, mutedStyle.Render("         "+statusHelp))

	if a.HasWork {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Work:    %s", a.FirstSubject))
	}
	if a.UnreadMail > 0 {
		lines = append(lines, fmt.Sprintf("Mail:    %d unread", a.UnreadMail))
	}

	// Action hints based on state
	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Press 'o' to open logs"))

	return strings.Join(lines, "\n")
}

func renderRigDetails(r rigItem, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Rig"))
	lines = append(lines, mutedStyle.Render("A project workspace with its own agents and merge queue"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Name:       %s", r.r.Name))
	lines = append(lines, "")

	// Worker counts with explanations
	lines = append(lines, headerStyle.Render("Workers"))
	lines = append(lines, fmt.Sprintf("Polecats:   %d", r.r.PolecatCount))
	lines = append(lines, mutedStyle.Render("            "+RigComponentHelp.Polecats))
	lines = append(lines, fmt.Sprintf("Crews:      %d", r.r.CrewCount))
	lines = append(lines, mutedStyle.Render("            "+RigComponentHelp.Crews))
	lines = append(lines, "")

	// Infrastructure status with explanations
	lines = append(lines, headerStyle.Render("Infrastructure"))
	witnessStatus := "No"
	if r.r.HasWitness {
		witnessStatus = "Yes"
	}
	refineryStatus := "No"
	if r.r.HasRefinery {
		refineryStatus = "Yes"
	}
	lines = append(lines, fmt.Sprintf("Witness:    %s", witnessStatus))
	lines = append(lines, mutedStyle.Render("            "+RigComponentHelp.Witness))
	lines = append(lines, fmt.Sprintf("Refinery:   %s", refineryStatus))
	lines = append(lines, mutedStyle.Render("            "+RigComponentHelp.Refinery))
	lines = append(lines, fmt.Sprintf("Merge Queue: %d items", r.mrCount))
	lines = append(lines, mutedStyle.Render("            "+RigComponentHelp.MergeQueue))
	lines = append(lines, "")

	// Hooks status
	activeHooks := 0
	for _, h := range r.r.Hooks {
		if h.HasWork {
			activeHooks++
		}
	}
	lines = append(lines, headerStyle.Render("Activity"))
	lines = append(lines, fmt.Sprintf("Hooks:      %d total, %d active", len(r.r.Hooks), activeHooks))
	lines = append(lines, mutedStyle.Render("            "+RigComponentHelp.Hooks))

	// Running agents in this rig
	running := 0
	for _, a := range r.r.Agents {
		if a.Running {
			running++
		}
	}
	lines = append(lines, fmt.Sprintf("Agents:     %d running", running))

	return strings.Join(lines, "\n")
}

func renderMailDetails(m data.MailMessage, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Mail"))
	lines = append(lines, "")

	// Read status badge
	readBadge := mailReadStyle.Render("●")
	readText := "Read"
	if !m.Read {
		readBadge = mailUnreadStyle.Render("●")
		readText = "Unread"
	}

	lines = append(lines, fmt.Sprintf("Status:  %s %s", readBadge, readText))
	lines = append(lines, fmt.Sprintf("ID:      %s", m.ID))
	lines = append(lines, fmt.Sprintf("From:    %s", m.From))
	lines = append(lines, fmt.Sprintf("To:      %s", m.To))
	lines = append(lines, fmt.Sprintf("Date:    %s", m.Timestamp.Format("2006-01-02 15:04")))
	if m.ThreadID != "" {
		lines = append(lines, fmt.Sprintf("Thread:  %s", m.ThreadID))
	}
	lines = append(lines, "")

	// Subject
	lines = append(lines, headerStyle.Render("Subject"))
	lines = append(lines, m.Subject)
	lines = append(lines, "")

	// Body (wrap long lines)
	lines = append(lines, headerStyle.Render("Body"))
	bodyLines := strings.Split(m.Body, "\n")
	for _, line := range bodyLines {
		// Wrap long lines
		if len(line) > width-4 {
			for len(line) > width-4 {
				lines = append(lines, line[:width-4])
				line = line[width-4:]
			}
		}
		lines = append(lines, line)
	}

	// Quick actions hint
	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Press 'm' to mark read/unread, 'd' to delete"))

	return strings.Join(lines, "\n")
}

func renderLifecycleDetails(e data.LifecycleEvent, state *SidebarState, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Lifecycle Event"))
	lines = append(lines, "")

	// Event type with badge
	badge := lifecycleEventBadge(e.EventType)
	lines = append(lines, fmt.Sprintf("Type:      %s %s", badge, string(e.EventType)))
	lines = append(lines, fmt.Sprintf("Timestamp: %s", e.Timestamp.Format("2006-01-02 15:04:05")))
	lines = append(lines, fmt.Sprintf("Agent:     %s", e.Agent))
	lines = append(lines, "")

	// Message
	lines = append(lines, headerStyle.Render("Message"))
	// Wrap long lines
	msg := e.Message
	if len(msg) > width-4 {
		for len(msg) > width-4 {
			lines = append(lines, msg[:width-4])
			msg = msg[width-4:]
		}
	}
	if msg != "" {
		lines = append(lines, msg)
	}

	// Current filters section
	lines = append(lines, "")
	lines = append(lines, headerStyle.Render("Filters"))
	if state.LifecycleFilter != "" {
		lines = append(lines, fmt.Sprintf("Type: %s", string(state.LifecycleFilter)))
	} else {
		lines = append(lines, mutedStyle.Render("Type: (all)"))
	}
	if state.LifecycleAgentFilter != "" {
		lines = append(lines, fmt.Sprintf("Agent: %s", state.LifecycleAgentFilter))
	} else {
		lines = append(lines, mutedStyle.Render("Agent: (all)"))
	}

	// Quick actions hint
	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("e: cycle type filter | g: filter by this agent | x: clear filters"))

	return strings.Join(lines, "\n")
}
