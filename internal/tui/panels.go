package tui

import (
	"fmt"
	"strings"

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
)

// SectionCount is the total number of sidebar sections
const SectionCount = 4

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

func (m mrItem) ID() string     { return m.mr.ID }
func (m mrItem) Label() string  { return fmt.Sprintf("[%s] %s", m.rig, m.mr.Title) }
func (m mrItem) Status() string { return m.mr.Status }

// agentItem wraps data.Agent for selection
type agentItem struct {
	a data.Agent
}

func (a agentItem) ID() string { return a.a.Address }
func (a agentItem) Label() string {
	status := "stopped"
	if a.a.Running {
		status = "running"
	}
	return fmt.Sprintf("%s (%s)", a.a.Name, status)
}
func (a agentItem) Status() string {
	if a.a.Running {
		return "running"
	}
	return "stopped"
}

// rigItem wraps data.Rig for selection with aggregated counts
type rigItem struct {
	r       data.Rig
	mrCount int // merge request count for this rig
}

func (r rigItem) ID() string { return r.r.Name }
func (r rigItem) Label() string {
	// Show rig name with summary counts: polecats, active hooks
	activeHooks := 0
	for _, h := range r.r.Hooks {
		if h.HasWork {
			activeHooks++
		}
	}
	return fmt.Sprintf("%s (%dP %dH)", r.r.Name, r.r.PolecatCount, activeHooks)
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

// SidebarState manages sidebar list selection
type SidebarState struct {
	Section   SidebarSection
	Selection int // Index within current section

	// Cached items for each section
	Rigs    []rigItem
	Convoys []convoyItem
	MRs     []mrItem
	Agents  []agentItem
}

// NewSidebarState creates a new sidebar state
func NewSidebarState() *SidebarState {
	return &SidebarState{
		Section:   SectionRigs,
		Selection: 0,
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

	// Update agents
	if snap.Town != nil {
		s.Agents = make([]agentItem, len(snap.Town.Agents))
		for i, a := range snap.Town.Agents {
			s.Agents[i] = agentItem{a}
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
func RenderSidebar(state *SidebarState, width, height int, focused bool) string {
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
	for sec := SectionRigs; sec <= SectionAgents; sec++ {
		isActive := state.Section == sec
		header := renderSectionHeader(sec.String(), isActive)
		items := getSectionItems(state, sec)
		list := renderItemList(items, state.Selection, isActive, innerWidth, sectionHeight)
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

func renderSectionHeader(name string, active bool) string {
	style := headerStyle
	if active {
		style = style.Foreground(highlight)
	}
	return style.Render(name)
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
	}
	return nil
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
	}

	return mutedStyle.Render("Select an item to see details")
}

func renderConvoyDetails(c data.Convoy, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Convoy"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("ID:      %s", c.ID))
	lines = append(lines, fmt.Sprintf("Title:   %s", c.Title))
	lines = append(lines, fmt.Sprintf("Status:  %s", c.Status))
	lines = append(lines, fmt.Sprintf("Created: %s", c.CreatedAt.Format("2006-01-02 15:04")))
	return strings.Join(lines, "\n")
}

func renderMRDetails(mr data.MergeRequest, rig string, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Merge Request"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("ID:       %s", mr.ID))
	lines = append(lines, fmt.Sprintf("Rig:      %s", rig))
	lines = append(lines, fmt.Sprintf("Title:    %s", mr.Title))
	lines = append(lines, fmt.Sprintf("Status:   %s", mr.Status))
	lines = append(lines, fmt.Sprintf("Worker:   %s", mr.Worker))
	lines = append(lines, fmt.Sprintf("Branch:   %s", mr.Branch))
	lines = append(lines, fmt.Sprintf("Priority: P%d", mr.Priority))
	return strings.Join(lines, "\n")
}

func renderAgentDetails(a data.Agent, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Agent"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Name:    %s", a.Name))
	lines = append(lines, fmt.Sprintf("Address: %s", a.Address))
	lines = append(lines, fmt.Sprintf("Role:    %s", a.Role))
	lines = append(lines, fmt.Sprintf("Session: %s", a.Session))

	status := "Stopped"
	if a.Running {
		status = "Running"
	}
	lines = append(lines, fmt.Sprintf("Status:  %s", status))

	if a.HasWork {
		lines = append(lines, fmt.Sprintf("Work:    %s", a.FirstSubject))
	}
	if a.UnreadMail > 0 {
		lines = append(lines, fmt.Sprintf("Mail:    %d unread", a.UnreadMail))
	}

	return strings.Join(lines, "\n")
}

func renderRigDetails(r rigItem, width int) string {
	var lines []string
	lines = append(lines, headerStyle.Render("Rig"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Name:       %s", r.r.Name))
	lines = append(lines, "")

	// Worker counts
	lines = append(lines, headerStyle.Render("Workers"))
	lines = append(lines, fmt.Sprintf("Polecats:   %d", r.r.PolecatCount))
	lines = append(lines, fmt.Sprintf("Crews:      %d", r.r.CrewCount))
	lines = append(lines, "")

	// Infrastructure status
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
	lines = append(lines, fmt.Sprintf("Refinery:   %s", refineryStatus))
	lines = append(lines, fmt.Sprintf("Merge Queue: %d items", r.mrCount))
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
