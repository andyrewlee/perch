package tui

import (
	"testing"
	"time"

	"github.com/andyrewlee/perch/data"
)

// TestGolden_SidebarPanels tests golden renders of sidebar panels in various states.
func TestGolden_SidebarPanels(t *testing.T) {
	tests := []struct {
		name   string
		golden string
		state  func() *SidebarState
		snap   *data.Snapshot
		width  int
		height int
	}{
		{
			name:   "identity_section",
			golden: "sidebar_identity",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionIdentity
				s.Identity = []identityItem{
					{id: "actor", label: "testuser", kind: "actor"},
					{id: "abc123", label: "abc123 Add initial commit", kind: "commit"},
					{id: "gt-001", label: "Implement feature X", kind: "bead"},
				}
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "rigs_section",
			golden: "sidebar_rigs",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionRigs
				s.Selection = 0
				s.Rigs = []rigItem{
					{
						r: data.Rig{
							Name:         "perch",
							PolecatCount: 3,
							ActiveHooks:  2,
							Hooks: []data.Hook{
								{Agent: "able", HasWork: true},
								{Agent: "baker", HasWork: true},
								{Agent: "charlie", HasWork: false},
							},
							HasWitness:  true,
							HasRefinery: true,
						},
					},
					{
						r: data.Rig{
							Name:         "sidekick",
							PolecatCount: 1,
							ActiveHooks:  0,
							Hooks:        []data.Hook{},
							HasWitness:   true,
							HasRefinery: false,
						},
					},
				}
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "agents_section_running",
			golden: "sidebar_agents_running",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionAgents
				s.AgentsLoading = false
				s.Selection = 0
				s.Agents = []agentItem{
					{a: data.Agent{Name: "witness", Address: "perch/witness", Running: true, HasWork: false, UnreadMail: 0}},
					{a: data.Agent{Name: "refinery", Address: "perch/refinery", Running: true, HasWork: true, UnreadMail: 0}},
					{a: data.Agent{Name: "able", Address: "perch/polecats/able", Running: true, HasWork: true, HookedBeadID: "gt-001", HookedStatus: "in_progress", UnreadMail: 0}},
					{a: data.Agent{Name: "baker", Address: "perch/polecats/baker", Running: false, HasWork: false, UnreadMail: 0}},
				}
				return s
			},
			snap:   &data.Snapshot{Town: &data.TownStatus{}, OperationalState: &data.OperationalState{WatchdogHealthy: true}},
			width:  40,
			height: 20,
		},
		{
			name:   "agents_section_loading",
			golden: "sidebar_agents_loading",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionAgents
				s.AgentsLoading = true
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "agents_section_error",
			golden: "sidebar_agents_error",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionAgents
				s.AgentsLoading = false
				s.AgentsLoadError = &testError{"connection failed"}
				// Use fixed time for deterministic golden output
				s.AgentsLastRefresh = time.Date(2026, 1, 6, 13, 42, 0, 0, time.UTC)
				s.Agents = []agentItem{
					{a: data.Agent{Name: "witness", Address: "perch/witness", Running: true}},
				}
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "agents_section_services_stopped",
			golden: "sidebar_agents_stopped",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionAgents
				s.AgentsLoading = false
				s.Agents = []agentItem{
					{a: data.Agent{Name: "witness", Address: "perch/witness", Running: false}},
					{a: data.Agent{Name: "refinery", Address: "perch/refinery", Running: false}},
				}
				return s
			},
			snap:   &data.Snapshot{Town: &data.TownStatus{}, OperationalState: &data.OperationalState{WatchdogHealthy: true}},
			width:  40,
			height: 20,
		},
		{
			name:   "agents_section_with_hq_bead_id",
			golden: "sidebar_agents_hq_bead_id",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionAgents
				s.AgentsLoading = false
				s.Selection = 0
				s.Agents = []agentItem{
					{a: data.Agent{Name: "mayor", Address: "mayor", Running: true, HasWork: true, HookedBeadID: "hq-mayor", HookedStatus: "in_progress", UnreadMail: 0}},
					{a: data.Agent{Name: "deacon", Address: "deacon", Running: true, HasWork: true, HookedBeadID: "hq-deacon", HookedStatus: "hooked", UnreadMail: 0}},
					{a: data.Agent{Name: "able", Address: "perch/polecats/able", Running: true, HasWork: true, HookedBeadID: "pe-001", HookedStatus: "in_progress", UnreadMail: 0}},
				}
				return s
			},
			snap:   &data.Snapshot{Town: &data.TownStatus{}, OperationalState: &data.OperationalState{WatchdogHealthy: true}},
			width:  50,
			height: 20,
		},
		{
			name:   "mergequeue_section_with_items",
			golden: "sidebar_mergequeue_items",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionMergeQueue
				s.MQsLoading = false
				s.Selection = 0
				s.MRs = []mrItem{
					{mr: data.MergeRequest{ID: "mr-001", Title: "Add auth feature", Branch: "feat/auth", HasConflicts: false}, rig: "perch"},
					{mr: data.MergeRequest{ID: "mr-002", Title: "Fix login bug", Branch: "fix/login", NeedsRebase: true}, rig: "perch"},
					{mr: data.MergeRequest{ID: "mr-003", Title: "Update docs", Branch: "docs/update", HasConflicts: true}, rig: "perch"},
				}
				return s
			},
			snap:   &data.Snapshot{Town: &data.TownStatus{Agents: []data.Agent{{Name: "refinery", Role: "refinery", Running: true}}}},
			width:  40,
			height: 20,
		},
		{
			name:   "mergequeue_section_empty",
			golden: "sidebar_mergequeue_empty",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionMergeQueue
				s.MQsLoading = false
				s.MRs = nil
				return s
			},
			snap:   &data.Snapshot{Town: &data.TownStatus{Agents: []data.Agent{{Name: "refinery", Role: "refinery", Running: true}}}},
			width:  40,
			height: 20,
		},
		{
			name:   "mergequeue_section_refinery_stopped",
			golden: "sidebar_mergequeue_refinery_stopped",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionMergeQueue
				s.MQsLoading = false
				s.MRs = nil
				return s
			},
			snap:   &data.Snapshot{Town: &data.TownStatus{Agents: []data.Agent{{Name: "refinery", Role: "refinery", Running: false}}}, OperationalState: &data.OperationalState{WatchdogHealthy: true}},
			width:  40,
			height: 20,
		},
		{
			name:   "convoys_section_active",
			golden: "sidebar_convoys_active",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionConvoys
				s.ConvoysLoading = false
				s.Selection = 0
				s.Convoys = []convoyItem{
					{c: data.Convoy{ID: "convoy-001", Title: "Feature: Auth system", Status: "open"}},
					{c: data.Convoy{ID: "convoy-002", Title: "Bug: Fix memory leak", Status: "in_progress"}},
				}
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "convoys_section_loading",
			golden: "sidebar_convoys_loading",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionConvoys
				s.ConvoysLoading = true
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "beads_section_open_issues",
			golden: "sidebar_beads_open",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionBeads
				s.BeadsLoading = false
				s.BeadsScope = BeadsScopeRig
				s.BeadsTotalCount = 4
				s.Selection = 0
				s.Beads = []beadItem{
					{issue: data.Issue{ID: "pe-001", Title: "Implement TUI panels", Status: "in_progress", Priority: 1}},
					{issue: data.Issue{ID: "pe-002", Title: "Add golden tests", Status: "open", Priority: 2}},
					{issue: data.Issue{ID: "pe-003", Title: "Fix rendering bug", Status: "open", Priority: 0}},
				}
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "beads_section_all_closed",
			golden: "sidebar_beads_all_closed",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionBeads
				s.BeadsLoading = false
				s.BeadsScope = BeadsScopeRig
				s.BeadsTotalCount = 3
				s.Beads = nil // All filtered out (closed)
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "beads_section_town_scope_with_hq_prefix",
			golden: "sidebar_beads_town_scope",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionBeads
				s.BeadsLoading = false
				s.BeadsScope = BeadsScopeTown
				s.BeadsTotalCount = 3
				s.Selection = 0
				s.Beads = []beadItem{
					{issue: data.Issue{ID: "hq-mayor", Title: "Mayor agent bead", Status: "in_progress", Priority: 0}},
					{issue: data.Issue{ID: "hq-deacon", Title: "Deacon agent bead", Status: "open", Priority: 1}},
				}
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "mail_section_with_messages",
			golden: "sidebar_mail_messages",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionMail
				s.MailLoading = false
				s.Selection = 0
				s.Mail = []mailItem{
					{m: data.MailMessage{ID: "mail-001", From: "mayor", To: "perch/witness", Subject: "New work assigned", Read: false, Type: "MERGE_READY", Timestamp: time.Now()}},
					{m: data.MailMessage{ID: "mail-002", From: "refinery", To: "perch/polecats/able", Subject: "Merge complete", Read: true, Type: "MERGED", Timestamp: time.Now().Add(-1 * time.Hour)}},
					{m: data.MailMessage{ID: "mail-003", From: "witness", To: "perch/polecats/baker", Subject: "Rework requested", Read: false, Type: "REWORK_REQUEST", Timestamp: time.Now().Add(-2 * time.Hour)}},
				}
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "mail_section_empty",
			golden: "sidebar_mail_empty",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionMail
				s.MailLoading = false
				s.Mail = nil
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "alerts_section_with_errors",
			golden: "sidebar_alerts_errors",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionAlerts
				s.Alerts = []alertItem{
					{e: data.LoadError{Source: "Town Status", Command: "gt status", Error: "connection refused", OccurredAt: time.Now().Add(-5 * time.Minute)}},
					{e: data.LoadError{Source: "Convoy", Command: "gt convoy list", Error: "timeout", OccurredAt: time.Now().Add(-2 * time.Minute)}},
				}
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
		{
			name:   "alerts_section_empty",
			golden: "sidebar_alerts_empty",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionAlerts
				s.Alerts = nil
				return s
			},
			snap:   nil,
			width:  40,
			height: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.state()
			rendered := RenderSidebar(state, tt.snap, tt.width, tt.height, true, nil)
			GoldenTest(t, tt.golden, rendered)
		})
	}
}

// TestGolden_DetailsPanels tests golden renders of details panels.
func TestGolden_DetailsPanels(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		golden string
		state  func() *SidebarState
		snap   *data.Snapshot
		audit  *AuditTimelineState
		width  int
		height int
	}{
		{
			name:   "identity_details",
			golden: "details_identity",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionIdentity
				return s
			},
			snap: &data.Snapshot{
				Identity: &data.Identity{
					Name:     "Test User",
					Username: "testuser",
					Email:    "test@example.com",
					Source:   "git",
					LastCommits: []data.CommitInfo{
						{Hash: "abc123", Subject: "Add initial implementation"},
						{Hash: "def456", Subject: "Fix rendering bug"},
					},
					LastBeads: []data.BeadInfo{
						{ID: "gt-001", Title: "Implement auth", Status: "in_progress"},
						{ID: "gt-002", Title: "Fix bug", Status: "open"},
					},
				},
				Mail: []data.MailMessage{
					{ID: "mail-001", From: "mayor", Subject: "Work assigned", Read: false},
				},
			},
			width:  50,
			height: 20,
		},
		{
			name:   "identity_details_with_hq_bead_ids",
			golden: "details_identity_hq_beads",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionIdentity
				return s
			},
			snap: &data.Snapshot{
				Identity: &data.Identity{
					Name:     "Test User",
					Username: "testuser",
					Email:    "test@example.com",
					Source:   "git",
					LastCommits: []data.CommitInfo{
						{Hash: "abc123", Subject: "Add initial implementation"},
					},
					LastBeads: []data.BeadInfo{
						{ID: "hq-mayor", Title: "Mayor agent bead", Status: "in_progress"},
						{ID: "hq-deacon", Title: "Deacon agent bead", Status: "open"},
						{ID: "pe-001", Title: "Implement TUI panels", Status: "closed"},
					},
				},
				Mail: []data.MailMessage{
					{ID: "mail-001", From: "mayor", Subject: "Work assigned", Read: false},
				},
				Routes: &data.Routes{
					Entries: map[string]data.BeadRoute{
						"hq-": {Prefix: "hq-", Location: "/Users/andrewlee/gt"},
						"pe-": {Prefix: "pe-", Location: "/Users/andrewlee/gt/perch", Rig: "perch"},
					},
				},
			},
			width:  50,
			height: 30,
		},
		{
			name:   "rig_details",
			golden: "details_rig",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionRigs
				s.Selection = 0
				s.Rigs = []rigItem{
					{
						r: data.Rig{
							Name:         "perch",
							PolecatCount: 3,
							CrewCount:    1,
							ActiveHooks:  2,
							Hooks:        []data.Hook{{Agent: "able", HasWork: true}, {Agent: "baker", HasWork: true}},
							HasWitness:   true,
							HasRefinery:  true,
							Agents: []data.Agent{
								{Name: "witness", Running: true},
								{Name: "refinery", Running: true},
								{Name: "able", Running: true},
							},
						},
						mrCount: 2,
					},
				}
				return s
			},
			snap:   nil,
			width:  50,
			height: 20,
		},
		{
			name:   "agent_details_working",
			golden: "details_agent_working",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionAgents
				s.Selection = 0
				s.Agents = []agentItem{
					{a: data.Agent{
						Name:          "able",
						Address:       "perch/polecats/able",
						Role:          "polecat",
						Running:       true,
						HasWork:       true,
						UnreadMail:    0,
						Session:       "able-session-123",
						HookedBeadID:  "gt-001",
						FirstSubject:  "Implement auth feature",
						HookedStatus:  "in_progress",
						HookedAt:      now.Add(-2 * time.Hour),
					}},
				}
				return s
			},
			audit: &AuditTimelineState{
				Entries: []data.AuditEntry{
					{Type: "session_start", Timestamp: now.Add(-3 * time.Hour), Summary: "Session started"},
					{Type: "sling", Timestamp: now.Add(-2 * time.Hour), Summary: "Work assigned: gt-001"},
					{Type: "commit", Timestamp: now.Add(-1 * time.Hour), Summary: "Commit: feat/auth"},
				},
			},
			width:  50,
			height: 20,
		},
		{
			name:   "agent_details_stopped",
			golden: "details_agent_stopped",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionAgents
				s.Selection = 0
				s.Agents = []agentItem{
					{a: data.Agent{
						Name:        "baker",
						Address:     "perch/polecats/baker",
						Role:        "polecat",
						Running:     false,
						HasWork:     false,
						UnreadMail:  0,
						Session:     "baker-session-456",
					}},
				}
				return s
			},
			audit: &AuditTimelineState{
				Entries: []data.AuditEntry{
					{Type: "kill", Timestamp: now.Add(-1 * time.Hour), Summary: "Session killed"},
				},
			},
			width:  50,
			height: 20,
		},
		{
			name:   "convoy_details_in_progress",
			golden: "details_convoy_progress",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionConvoys
				s.Selection = 0
				s.Convoys = []convoyItem{
					// Use fixed time for deterministic golden output
					{c: data.Convoy{ID: "convoy-001", Title: "Feature: Auth system", Status: "open", CreatedAt: time.Date(2026, 1, 6, 13, 47, 0, 0, time.UTC)}},
				}
				return s
			},
			snap: &data.Snapshot{
				ConvoyStatuses: map[string]*data.ConvoyStatus{
					"convoy-001": {
						ID:        "convoy-001",
						Completed: 1,
						Total:     3,
						Tracked: []data.TrackedIssue{
							{ID: "gt-001", Title: "Design auth flow", Status: "closed", Worker: "able", WorkerAge: "5m"},
							{ID: "gt-002", Title: "Implement auth", Status: "in_progress", Worker: "baker", WorkerAge: "15m"},
							{ID: "gt-003", Title: "Add tests", Status: "open", Assignee: "charlie"},
						},
					},
				},
			},
			width:  50,
			height: 20,
		},
		{
			name:   "merge_request_details_with_conflicts",
			golden: "details_mr_conflicts",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionMergeQueue
				s.Selection = 0
				s.MRs = []mrItem{
					{mr: data.MergeRequest{
						ID:           "mr-001",
						Title:        "Add auth feature",
						Status:       "pending",
						Worker:       "able",
						Branch:       "feat/auth",
						Priority:     1,
						HasConflicts: true,
						ConflictInfo: "src/auth.go: merge conflict",
						LastChecked:  "10m ago",
					}, rig: "perch"},
				}
				return s
			},
			snap:   nil,
			width:  50,
			height: 20,
		},
		{
			name:   "bead_details_with_dependencies",
			golden: "details_bead_deps",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionBeads
				s.BeadsScope = BeadsScopeRig
				s.Selection = 0
				s.Beads = []beadItem{
					{issue: data.Issue{
						ID:             "pe-001",
						Title:          "Implement TUI panels",
						IssueType:      "feature",
						Priority:       1,
						Status:         "in_progress",
						Assignee:       "able",
						CreatedAt:      now.Add(-48 * time.Hour),
						CreatedBy:      "testuser",
						UpdatedAt:      now.Add(-2 * time.Hour),
						DependencyCount: 1,
						DependentCount:  2,
						Labels:         []string{"tui", "feature"},
						Description:    "Implement the sidebar and details panels for the TUI interface.",
					}},
				}
				return s
			},
			snap:   nil,
			width:  50,
			height: 20,
		},
		{
			name:   "mail_details_unread",
			golden: "details_mail_unread",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionMail
				s.Selection = 0
				s.Mail = []mailItem{
					{m: data.MailMessage{
						ID:        "mail-001",
						From:      "mayor",
						To:        "perch/witness",
						Subject:   "Urgent: Fix production bug",
						Body:      "A critical bug has been reported in production. Please investigate and fix ASAP.\n\nBug details:\n- Error: panic in auth handler\n- Impact: users cannot login\n- Priority: P0",
						Read:      false,
						Type:      "MERGE_READY",
						Priority:  "high",
						Timestamp: now.Add(-30 * time.Minute),
						ThreadID:  "thread-001",
					}},
				}
				return s
			},
			snap:   nil,
			width:  50,
			height: 20,
		},
		{
			name:   "beads_routing_view",
			golden: "details_beads_routing",
			state: func() *SidebarState {
				s := NewSidebarState()
				s.Section = SectionBeads
				s.BeadsScope = BeadsScopeRig
				s.Beads = []beadItem{} // No beads selected, shows routing view
				return s
			},
			snap: &data.Snapshot{
				Routes: &data.Routes{
					Entries: map[string]data.BeadRoute{
						"hq-": {
							Prefix:   "hq-",
							Location: "/Users/test/.beads/",
							Rig:      "",
						},
						"pe-": {
							Prefix:   "pe-",
							Location: "perch/mayor/rig",
							Rig:      "perch",
						},
						"gt-": {
							Prefix:   "gt-",
							Location: "gastown/rig",
							Rig:      "gastown",
						},
					},
				},
			},
			width:  60,
			height: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.state()
			rendered := RenderDetails(state, tt.snap, tt.audit, tt.width, tt.height, true, nil, nil)
			GoldenTest(t, tt.golden, rendered)
		})
	}
}

// TestGolden_HelpOverlay tests golden renders of the help overlay.
func TestGolden_HelpOverlay(t *testing.T) {
	tests := []struct {
		name   string
		golden string
		width  int
		height int
	}{
		{
			name:   "help_overlay_standard",
			golden: "help_overlay_standard",
			width:  80,
			height: 24,
		},
		{
			name:   "help_overlay_small",
			golden: "help_overlay_small",
			width:  60,
			height: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km := DefaultKeyMap()
			help := NewHelpOverlay(km)
			rendered := help.Render(tt.width, tt.height)
			GoldenTest(t, tt.golden, rendered)
		})
	}
}

// TestGolden_FooterHUD tests golden renders of the footer HUD in various states.
func TestGolden_FooterHUD(t *testing.T) {
	tests := []struct {
		name   string
		golden string
		model  func(*testing.T) Model
	}{
		{
			name:   "hud_disconnected",
			golden: "hud_disconnected",
			model: func(t *testing.T) Model {
				t.Helper()
				m := NewTestModel(t)
				m.ready = true
				m.width = 80
				return m
			},
		},
		{
			name:   "hud_refreshing",
			golden: "hud_refreshing",
			model: func(t *testing.T) Model {
				t.Helper()
				m := NewTestModel(t)
				m.ready = true
				m.width = 80
				m.isRefreshing = true
				return m
			},
		},
		{
			name:   "hud_connected",
			golden: "hud_connected",
			model: func(t *testing.T) Model {
				t.Helper()
				m := NewTestModel(t)
				m.ready = true
				m.width = 80
				m.lastRefresh = time.Now()
				return m
			},
		},
		{
			name:   "hud_with_errors",
			golden: "hud_with_errors",
			model: func(t *testing.T) Model {
				t.Helper()
				m := NewTestModel(t)
				m.ready = true
				m.width = 80
				m.errorCount = 3
				m.lastRefresh = time.Now().Add(-1 * time.Minute)
				return m
			},
		},
		{
			name:   "hud_with_status_message",
			golden: "hud_with_status",
			model: func(t *testing.T) Model {
				t.Helper()
				m := NewTestModel(t)
				m.ready = true
				m.width = 80
				m.lastRefresh = time.Now()
				m.statusMessage = &StatusMessage{
					Text:      "Work submitted successfully",
					IsError:   false,
					ExpiresAt: time.Now().Add(5 * time.Second),
				}
				return m
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tt.model(t)
			rendered := model.renderFooter()
			GoldenTest(t, tt.golden, rendered)
		})
	}
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e testError) Error() string {
	return e.msg
}
