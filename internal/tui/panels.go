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
	SectionWorktrees
	SectionPlugins
)

// SectionCount is the total number of sidebar sections
const SectionCount = 8

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
	case SectionWorktrees:
		return "Worktrees"
	case SectionPlugins:
		return "Plugins"
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
	label := fmt.Sprintf("%s %s", badge, a.a.Name)
	// Append hooked bead ID if present
	if a.a.HookedBeadID != "" {
		label += " " + mutedStyle.Render("["+a.a.HookedBeadID+"]")
	}
	return label
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

// worktreeItem wraps data.Worktree for selection
type worktreeItem struct {
	wt data.Worktree
}

func (w worktreeItem) ID() string { return w.wt.Path }
func (w worktreeItem) Label() string {
	indicator := ""
	if !w.wt.Clean {
		indicator = "!"
	}
	if indicator != "" {
		return fmt.Sprintf("[%s]%s %s-%s", w.wt.Rig, indicator, w.wt.SourceRig, w.wt.SourceName)
	}
	return fmt.Sprintf("[%s] %s-%s", w.wt.Rig, w.wt.SourceRig, w.wt.SourceName)
}
func (w worktreeItem) Status() string { return w.wt.Status }

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
	// Read status badge
	readBadge := mailReadStyle.Render("○")
	if !m.m.Read {
		readBadge = mailUnreadStyle.Render("●")
	}

	// Type badge (short indicator for message type)
	typeBadge := mailTypeBadge(m.m.Type)

	// Truncate subject for display
	subject := m.m.Subject
	maxLen := 24
	if typeBadge != "" {
		maxLen = 20 // Make room for type badge
	}
	if len(subject) > maxLen {
		subject = subject[:maxLen-3] + "..."
	}

	if typeBadge != "" {
		return fmt.Sprintf("%s %s %s", readBadge, typeBadge, subject)
	}
	return fmt.Sprintf("%s %s", readBadge, subject)
}
func (m mailItem) Status() string {
	if !m.m.Read {
		return "unread"
	}
	return "read"
}

// mailTypeBadge returns a styled short badge for mail type.
func mailTypeBadge(mailType string) string {
	switch mailType {
	case "MERGE_READY":
		return mailTypeMergeReadyStyle.Render("[MR]")
	case "MERGED":
		return mailTypeMergedStyle.Render("[OK]")
	case "REWORK_REQUEST", "REWORK":
		return mailTypeReworkStyle.Render("[RW]")
	case "HANDOFF":
		return mailTypeHandoffStyle.Render("[HO]")
	case "NUDGE":
		return mailTypeNudgeStyle.Render("[NU]")
	default:
		return ""
	}
}

// mailTypeLabel returns a human-readable label for mail type.
func mailTypeLabel(mailType string) string {
	switch mailType {
	case "MERGE_READY":
		return "Merge Ready"
	case "MERGED":
		return "Merged"
	case "REWORK_REQUEST", "REWORK":
		return "Rework Request"
	case "HANDOFF":
		return "Handoff"
	case "NUDGE":
		return "Nudge"
	default:
		if mailType != "" {
			return mailType
		}
		return "General"
	}
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

// pluginItem wraps data.Plugin for selection
type pluginItem struct {
	p data.Plugin
}

func (p pluginItem) ID() string { return p.p.Path }
func (p pluginItem) Label() string {
	status := "on"
	if !p.p.Enabled {
		status = "off"
	}
	if p.p.HasError {
		status = "err"
	}
	scopePrefix := ""
	if p.p.Scope != "town" {
		scopePrefix = "[" + p.p.Scope + "] "
	}
	return fmt.Sprintf("%s%s (%s)", scopePrefix, p.p.Title, status)
}
func (p pluginItem) Status() string {
	if p.p.HasError {
		return "error"
	}
	if !p.p.Enabled {
		return "disabled"
	}
	return "enabled"
}

// SidebarState manages sidebar list selection
type SidebarState struct {
	Section   SidebarSection
	Selection int // Index within current section

	// Convoy view mode: false = active, true = history (landed)
	ShowConvoyHistory bool

	// Cached items for each section
	Rigs            []rigItem
	Convoys         []convoyItem // Active convoys
	ClosedConvoys   []convoyItem // Landed convoys (history)
	MRs             []mrItem
	Agents          []agentItem
	Mail            []mailItem
	LifecycleEvents []lifecycleEventItem
	Worktrees       []worktreeItem
	Plugins         []pluginItem

	// Lifecycle filters
	LifecycleFilter      data.LifecycleEventType // Empty = show all
	LifecycleAgentFilter string                  // Empty = show all

	// Loading/error state for agents panel
	AgentsLastRefresh time.Time // Last successful agent data refresh
	AgentsLoadError   error     // Error from last agent load attempt (nil if successful)
	AgentsLoading     bool      // True during initial load

	// Loading/error state for convoys panel
	ConvoysLastRefresh time.Time // Last successful convoy data refresh
	ConvoysLoadError   error     // Error from last convoy load attempt (nil if successful)
	ConvoysLoading     bool      // True during initial load
}

// NewSidebarState creates a new sidebar state
func NewSidebarState() *SidebarState {
	return &SidebarState{
		Section:        SectionRigs,
		Selection:      0,
		AgentsLoading:  true, // Start in loading state until first successful refresh
		ConvoysLoading: true, // Start in loading state until first successful refresh
	}
}

// ToggleConvoyHistory toggles between active and history convoy view.
func (s *SidebarState) ToggleConvoyHistory() {
	s.ShowConvoyHistory = !s.ShowConvoyHistory
	s.Selection = 0
	s.clampSelection()
}

// ConvoyViewLabel returns a label describing the current convoy view.
func (s *SidebarState) ConvoyViewLabel() string {
	if s.ShowConvoyHistory {
		return "History"
	}
	return "Active"
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

	// Update convoys with loading/error tracking
	// Check if convoy data was successfully loaded (nil means error occurred)
	if snap.Convoys != nil {
		s.Convoys = make([]convoyItem, len(snap.Convoys))
		for i, c := range snap.Convoys {
			s.Convoys[i] = convoyItem{c}
		}
		s.ConvoysLastRefresh = snap.LoadedAt
		s.ConvoysLoadError = nil
		s.ConvoysLoading = false
	} else {
		// Convoys failed to load - find the convoy-specific error if any
		s.ConvoysLoading = false
		for _, err := range snap.Errors {
			if err != nil && strings.Contains(err.Error(), "convoy") {
				s.ConvoysLoadError = err
				break
			}
		}
		// Preserve s.Convoys (last-known value) - don't clear it
	}

	// Update closed/landed convoys (same pattern)
	if snap.ClosedConvoys != nil {
		s.ClosedConvoys = make([]convoyItem, len(snap.ClosedConvoys))
		for i, c := range snap.ClosedConvoys {
			s.ClosedConvoys[i] = convoyItem{c}
		}
	}
	// Note: we use the same error state for both - if active convoys fail, closed likely did too

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

	// Update worktrees
	s.Worktrees = make([]worktreeItem, len(snap.Worktrees))
	for i, wt := range snap.Worktrees {
		s.Worktrees[i] = worktreeItem{wt}
	}

	// Update plugins
	s.Plugins = make([]pluginItem, len(snap.Plugins))
	for i, p := range snap.Plugins {
		s.Plugins[i] = pluginItem{p}
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
		convoys := s.Convoys
		if s.ShowConvoyHistory {
			convoys = s.ClosedConvoys
		}
		items := make([]SelectableItem, len(convoys))
		for i, c := range convoys {
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
	case SectionWorktrees:
		items := make([]SelectableItem, len(s.Worktrees))
		for i, w := range s.Worktrees {
			items[i] = w
		}
		return items
	case SectionPlugins:
		items := make([]SelectableItem, len(s.Plugins))
		for i, p := range s.Plugins {
			items[i] = p
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

// SidebarOptions provides optional configuration for sidebar rendering.
type SidebarOptions struct {
	// LastMergeTime is used to show when the last merge occurred (for empty queue state)
	LastMergeTime time.Time
}

// RenderSidebar renders the sidebar with all sections.
// snap is used to check refinery status for the merge queue empty state.
// opts provides optional configuration like last merge time.
func RenderSidebar(state *SidebarState, snap *data.Snapshot, width, height int, focused bool, opts *SidebarOptions) string {
	innerWidth := width - 4
	innerHeight := height - 2

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	// Calculate height per section (5 sections)
	sectionHeight := (innerHeight - SectionCount) / SectionCount // -SectionCount for headers
	if sectionHeight < 2 {
		sectionHeight = 2
	}

	var sections []string

	// Render each section
	for sec := SectionRigs; sec <= SectionPlugins; sec++ {
		isActive := state.Section == sec
		headerText := sec.String()
		// For convoys, show active/history toggle state
		if sec == SectionConvoys {
			if state.ShowConvoyHistory {
				headerText = "Convoys [H]"
			} else {
				headerText = "Convoys [A]"
			}
		}
		header := renderSectionHeader(headerText, sec, isActive, state)
		items := getSectionItems(state, sec)

		var list string
		if sec == SectionMergeQueue && len(items) == 0 {
			// Special empty state for merge queue with context
			list = renderMQEmptyState(snap, opts, isActive, innerWidth)
		} else if sec == SectionAgents {
			// Special handling for agents section with loading/error states
			list = renderAgentsList(state, items, isActive, innerWidth, sectionHeight)
		} else if sec == SectionConvoys {
			// Special handling for convoys section with loading/error states
			list = renderConvoysList(state, items, isActive, innerWidth, sectionHeight)
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

func renderSectionHeader(name string, section SidebarSection, active bool, state *SidebarState) string {
	style := headerStyle
	if active {
		style = style.Foreground(highlight)
	}
	header := style.Render(name)

	// Add unread count badge for Mail section
	if section == SectionMail && state != nil {
		unread := 0
		for _, m := range state.Mail {
			if !m.m.Read {
				unread++
			}
		}
		if unread > 0 {
			header += " " + mailUnreadStyle.Render(fmt.Sprintf("(%d)", unread))
		}
	}

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
		convoys := state.Convoys
		if state.ShowConvoyHistory {
			convoys = state.ClosedConvoys
		}
		items := make([]SelectableItem, len(convoys))
		for i, c := range convoys {
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
	case SectionWorktrees:
		items := make([]SelectableItem, len(state.Worktrees))
		for i, w := range state.Worktrees {
			items[i] = w
		}
		return items
	case SectionPlugins:
		items := make([]SelectableItem, len(state.Plugins))
		for i, p := range state.Plugins {
			items[i] = p
		}
		return items
	}
	return nil
}

// renderMQEmptyState renders the merge queue empty state with refinery context.
// This helps beginners understand that an empty queue is normal and healthy.
func renderMQEmptyState(snap *data.Snapshot, opts *SidebarOptions, isActive bool, width int) string {
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

	// Last merge time (if available)
	if opts != nil && !opts.LastMergeTime.IsZero() {
		ago := formatDuration(time.Since(opts.LastMergeTime))
		lines = append(lines, mutedStyle.Render("  Last merge: "+ago+" ago"))
	}

	// Hint for beginners when section is active
	if isActive {
		lines = append(lines, mutedStyle.Render("  Run 'gt done' to submit work"))
	}

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

// renderConvoysList renders the convoys list with loading/error state indicators.
// Per acceptance criteria: always show last-known convoys; if loading, show explicit
// loading state; if error, show error + last refresh time.
func renderConvoysList(state *SidebarState, items []SelectableItem, isActiveSection bool, width, maxLines int) string {
	var lines []string

	// Show loading indicator during initial load
	if state.ConvoysLoading {
		lines = append(lines, mutedStyle.Render("  Loading convoys..."))
		return strings.Join(lines, "\n")
	}

	// Show error indicator if convoys failed to load (but still show last-known)
	if state.ConvoysLoadError != nil {
		errLine := statusErrorStyle.Render("  ! Load error")
		if !state.ConvoysLastRefresh.IsZero() {
			errLine += mutedStyle.Render(" (last: " + state.ConvoysLastRefresh.Format("15:04") + ")")
		}
		lines = append(lines, errLine)
	}

	// Show last-known convoys (or empty state if none)
	if len(items) == 0 {
		if state.ConvoysLoadError != nil {
			lines = append(lines, mutedStyle.Render("  (no cached convoys)"))
		} else {
			// Normal empty state - no active convoys
			viewType := "active"
			if state.ShowConvoyHistory {
				viewType = "landed"
			}
			lines = append(lines, mutedStyle.Render("  (no "+viewType+" convoys)"))
			if isActiveSection {
				lines = append(lines, mutedStyle.Render("  Press H to toggle history"))
			}
		}
		return strings.Join(lines, "\n")
	}

	// Calculate remaining lines for convoy list
	remainingLines := maxLines - len(lines)
	if remainingLines < 1 {
		remainingLines = 1
	}

	// Render convoy items
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

// AuditTimelineState holds the audit timeline data for the selected agent.
type AuditTimelineState struct {
	Actor   string
	Entries []data.AuditEntry
	Loading bool
}

// RenderDetails renders the details panel for the selected item
func RenderDetails(state *SidebarState, snap *data.Snapshot, audit *AuditTimelineState, width, height int, focused bool) string {
	innerWidth := width - 4
	innerHeight := height - 2

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	title := titleStyle.Render("Details")
	content := renderSelectedDetails(state, snap, audit, innerWidth)

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

func renderSelectedDetails(state *SidebarState, snap *data.Snapshot, audit *AuditTimelineState, width int) string {
	if state == nil || snap == nil {
		return mutedStyle.Render("No data loaded")
	}

	switch state.Section {
	case SectionRigs:
		if state.Selection >= 0 && state.Selection < len(state.Rigs) {
			return renderRigDetails(state.Rigs[state.Selection], width)
		}
	case SectionConvoys:
		convoys := state.Convoys
		if state.ShowConvoyHistory {
			convoys = state.ClosedConvoys
		}
		if state.Selection >= 0 && state.Selection < len(convoys) {
			convoy := convoys[state.Selection].c
			var status *data.ConvoyStatus
			if snap.ConvoyStatuses != nil {
				status = snap.ConvoyStatuses[convoy.ID]
			}
			return renderConvoyDetails(convoy, status, width, state.ShowConvoyHistory)
		}
	case SectionMergeQueue:
		if state.Selection >= 0 && state.Selection < len(state.MRs) {
			mr := state.MRs[state.Selection]
			return renderMRDetails(mr.mr, mr.rig, width)
		}
	case SectionAgents:
		if state.Selection >= 0 && state.Selection < len(state.Agents) {
			return renderAgentDetails(state.Agents[state.Selection].a, audit, width)
		}
	case SectionMail:
		if state.Selection >= 0 && state.Selection < len(state.Mail) {
			return renderMailDetails(state.Mail[state.Selection].m, width)
		}
	case SectionLifecycle:
		if state.Selection >= 0 && state.Selection < len(state.LifecycleEvents) {
			return renderLifecycleDetails(state.LifecycleEvents[state.Selection].e, state, width)
		}
	case SectionWorktrees:
		if state.Selection >= 0 && state.Selection < len(state.Worktrees) {
			return renderWorktreeDetails(state.Worktrees[state.Selection].wt, width)
		}
	case SectionPlugins:
		if state.Selection >= 0 && state.Selection < len(state.Plugins) {
			return renderPluginDetails(state.Plugins[state.Selection].p, width)
		}
	}

	return mutedStyle.Render("Select an item to see details")
}

func renderConvoyDetails(c data.Convoy, status *data.ConvoyStatus, width int, isHistory bool) string {
	var lines []string
	headerText := "Convoy"
	if isHistory {
		headerText = "Landed Convoy"
	}
	lines = append(lines, headerStyle.Render(headerText))
	lines = append(lines, mutedStyle.Render(ConvoyHelp.Description))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("ID:      %s", c.ID))
	lines = append(lines, fmt.Sprintf("Title:   %s", c.Title))

	// Status with visual indicator
	statusBadge := convoyStatusBadge(c.Status)
	lines = append(lines, fmt.Sprintf("Status:  %s %s", statusBadge, c.Status))
	if statusHelp, ok := ConvoyHelp.Statuses[c.Status]; ok {
		lines = append(lines, mutedStyle.Render("         "+statusHelp))
	}
	lines = append(lines, fmt.Sprintf("Created: %s", c.CreatedAt.Format("2006-01-02 15:04")))

	// Progress section (if we have detailed status)
	if status != nil && status.Total > 0 {
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Progress"))

		// Progress bar
		progressPct := 0
		if status.Total > 0 {
			progressPct = status.Completed * 100 / status.Total
		}
		progressBar := renderProgressBar(progressPct, 20)
		lines = append(lines, fmt.Sprintf("%s %d/%d (%d%%)", progressBar, status.Completed, status.Total, progressPct))

		// Tracked issues section
		if len(status.Tracked) > 0 {
			lines = append(lines, "")
			lines = append(lines, headerStyle.Render("Tracked Issues"))

			for _, issue := range status.Tracked {
				// Issue badge based on status
				issueBadge := issueStatusBadge(issue.Status)
				issueTitle := issue.Title
				if len(issueTitle) > width-20 {
					issueTitle = issueTitle[:width-23] + "..."
				}
				lines = append(lines, fmt.Sprintf("%s %s", issueBadge, issue.ID))
				lines = append(lines, fmt.Sprintf("  %s", issueTitle))

				// Show worker if assigned
				if issue.Worker != "" {
					workerInfo := issue.Worker
					if issue.WorkerAge != "" {
						workerInfo = fmt.Sprintf("%s (%s)", issue.Worker, issue.WorkerAge)
					}
					lines = append(lines, fmt.Sprintf("  %s %s", workingStyle.Render("→"), workerInfo))
				} else if issue.Assignee != "" {
					lines = append(lines, fmt.Sprintf("  %s %s", mutedStyle.Render("assigned:"), issue.Assignee))
				}
			}
		}
	}

	if isHistory && c.IsLanded() {
		// For landed convoys, show additional info
		lines = append(lines, "")
		lines = append(lines, mutedStyle.Render("This convoy has been completed and landed."))
	}

	return strings.Join(lines, "\n")
}

// convoyStatusBadge returns a colored badge for convoy status
func convoyStatusBadge(status string) string {
	switch status {
	case "open":
		return workingStyle.Render("●")
	case "landed":
		return completedStyle.Render("✓")
	case "closed":
		return mutedStyle.Render("○")
	default:
		return mutedStyle.Render("?")
	}
}

// issueStatusBadge returns a colored badge for issue status
func issueStatusBadge(status string) string {
	switch status {
	case "in_progress":
		return workingStyle.Render("●")
	case "open":
		return idleStyle.Render("○")
	case "closed":
		return completedStyle.Render("✓")
	default:
		return mutedStyle.Render("?")
	}
}

// renderProgressBar renders a simple ASCII progress bar
func renderProgressBar(percent int, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := width * percent / 100
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	if percent == 100 {
		return completedStyle.Render("[" + bar + "]")
	}
	return workingStyle.Render("[" + bar + "]")
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

func renderAgentDetails(a data.Agent, audit *AuditTimelineState, width int) string {
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

	// Hook section - show hooked issue details
	lines = append(lines, "")
	lines = append(lines, headerStyle.Render("Hook"))
	if a.HookedBeadID != "" {
		lines = append(lines, fmt.Sprintf("Bead:    %s", a.HookedBeadID))
		lines = append(lines, fmt.Sprintf("Title:   %s", a.FirstSubject))
		lines = append(lines, fmt.Sprintf("Status:  %s", a.HookedStatus))
		lines = append(lines, fmt.Sprintf("Age:     %s", formatAge(a.HookedAt)))
	} else {
		lines = append(lines, mutedStyle.Render("(empty)"))
	}

	if a.UnreadMail > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Mail:    %d unread", a.UnreadMail))
	}

	// Audit timeline section
	lines = append(lines, "")
	lines = append(lines, headerStyle.Render("Activity Timeline"))
	lines = append(lines, renderAuditTimeline(audit, width))

	// Action hints
	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Actions: o=logs S=sling H=handoff K=kill n=nudge m=mail"))

	return strings.Join(lines, "\n")
}

// formatAge returns a human-readable age string (e.g., "5m", "2h", "3d")
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}


func renderWorktreeDetails(wt data.Worktree, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Worktree"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Target Rig: %s", wt.Rig))
	lines = append(lines, fmt.Sprintf("Source:     %s-%s", wt.SourceRig, wt.SourceName))
	lines = append(lines, fmt.Sprintf("Path:       %s", wt.Path))
	lines = append(lines, "")
	lines = append(lines, headerStyle.Render("Git Status"))
	lines = append(lines, fmt.Sprintf("Branch:     %s", wt.Branch))

	statusStyle := idleStyle
	if !wt.Clean {
		statusStyle = conflictStyle
	}
	lines = append(lines, fmt.Sprintf("Status:     %s", statusStyle.Render(wt.Status)))

	lines = append(lines, "")
	lines = append(lines, headerStyle.Render("Actions"))
	lines = append(lines, mutedStyle.Render("Press 'x' to remove this worktree"))

	return strings.Join(lines, "\n")
}

// renderAuditTimeline renders the audit timeline entries.
func renderAuditTimeline(audit *AuditTimelineState, width int) string {
	if audit == nil {
		return mutedStyle.Render("  (no timeline)")
	}
	if audit.Loading {
		return mutedStyle.Render("  Loading...")
	}
	if len(audit.Entries) == 0 {
		return mutedStyle.Render("  (no activity)")
	}

	var lines []string
	for i, entry := range audit.Entries {
		if i >= 10 { // Limit display to 10 entries
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("  ... and %d more", len(audit.Entries)-10)))
			break
		}

		// Format timestamp as relative time or short date
		timeStr := formatRelativeTime(entry.Timestamp)

		// Format entry type with icon
		icon := auditTypeIcon(entry.Type)

		// Truncate summary if needed
		summary := entry.Summary
		maxSummaryLen := width - len(timeStr) - len(icon) - 6
		if maxSummaryLen > 0 && len(summary) > maxSummaryLen {
			summary = summary[:maxSummaryLen-3] + "..."
		}

		line := fmt.Sprintf("  %s %s %s", mutedStyle.Render(timeStr), icon, summary)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// formatRelativeTime formats a timestamp as relative time (e.g., "2m ago", "1h ago", "Jan 2").
func formatRelativeTime(t time.Time) string {
	since := time.Since(t)
	switch {
	case since < time.Minute:
		return "now"
	case since < time.Hour:
		return fmt.Sprintf("%dm", int(since.Minutes()))
	case since < 24*time.Hour:
		return fmt.Sprintf("%dh", int(since.Hours()))
	case since < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(since.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}

// auditTypeIcon returns an icon for audit entry type.
func auditTypeIcon(entryType string) string {
	switch entryType {
	case "commit":
		return "◆" // diamond for commits
	case "sling":
		return "→" // arrow for work assignment
	case "session_start":
		return "▶" // play for session start
	case "done":
		return "✓" // check for completion
	case "kill":
		return "■" // stop for kill
	case "spawn":
		return "+" // plus for spawn
	case "handoff":
		return "⤳" // handoff arrow
	default:
		return "·" // dot for unknown
	}
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

	// Message type with styled badge
	if m.Type != "" {
		typeBadge := mailTypeBadge(m.Type)
		typeLabel := mailTypeLabel(m.Type)
		lines = append(lines, fmt.Sprintf("Type:    %s %s", typeBadge, typeLabel))
	}

	lines = append(lines, fmt.Sprintf("ID:      %s", m.ID))
	lines = append(lines, fmt.Sprintf("From:    %s", m.From))
	lines = append(lines, fmt.Sprintf("To:      %s", m.To))
	lines = append(lines, fmt.Sprintf("Date:    %s", m.Timestamp.Format("2006-01-02 15:04")))
	if m.ThreadID != "" {
		lines = append(lines, fmt.Sprintf("Thread:  %s", m.ThreadID))
	}
	if m.Priority != "" && m.Priority != "normal" {
		lines = append(lines, fmt.Sprintf("Priority: %s", m.Priority))
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
	actionHint := "m: read/unread | y: acknowledge"
	lines = append(lines, mutedStyle.Render(actionHint))

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

func renderPluginDetails(p data.Plugin, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Plugin"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Name:    %s", p.Title))
	lines = append(lines, fmt.Sprintf("Scope:   %s", p.Scope))

	status := "Enabled"
	if !p.Enabled {
		status = "Disabled"
	}
	lines = append(lines, fmt.Sprintf("Status:  %s", status))

	if p.GateType != "" {
		lines = append(lines, fmt.Sprintf("Gate:    %s", p.GateType))
	}
	if p.Schedule != "" {
		lines = append(lines, fmt.Sprintf("Schedule: %s", p.Schedule))
	}

	if p.Description != "" {
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Description"))
		// Wrap description
		desc := p.Description
		if len(desc) > width-2 {
			desc = desc[:width-5] + "..."
		}
		lines = append(lines, desc)
	}

	// Last run info
	if !p.LastRun.IsZero() {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Last Run: %s", p.LastRun.Format("2006-01-02 15:04")))
	}

	// Error section
	if p.HasError {
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Error"))
		errMsg := p.LastError
		if len(errMsg) > width-2 {
			errMsg = errMsg[:width-5] + "..."
		}
		lines = append(lines, mutedStyle.Render(errMsg))
	}

	// Actions hint
	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Actions: e=toggle enabled"))

	return strings.Join(lines, "\n")
}
