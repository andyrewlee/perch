package testutil

import (
	"encoding/json"
	"time"
)

// Fixtures provides pre-built test data for gt/bd command mocking.
// Types here mirror data.* types to avoid import cycles.
type Fixtures struct{}

// NewFixtures creates a new Fixtures instance.
func NewFixtures() *Fixtures {
	return &Fixtures{}
}

// TownStatus returns a sample gt status --json response as a map.
// Returns raw JSON-compatible structure to avoid data package dependency.
func (f *Fixtures) TownStatus() map[string]any {
	return map[string]any{
		"name":     "test-town",
		"location": "/tmp/test-town",
		"overseer": map[string]any{
			"name":        "Test User",
			"email":       "test@example.com",
			"username":    "testuser",
			"source":      "git",
			"unread_mail": 2,
		},
		"agents": []map[string]any{
			{
				"name":        "mayor",
				"address":     "mayor/",
				"session":     "mayor-12345",
				"role":        "mayor",
				"running":     true,
				"has_work":    true,
				"unread_mail": 1,
			},
		},
		"rigs": []map[string]any{
			{
				"name":          "perch",
				"polecats":      []string{"able", "baker"},
				"polecat_count": 2,
				"crews":         []string{"alpha"},
				"crew_count":    1,
				"has_witness":   true,
				"has_refinery":  true,
				"agents": []map[string]any{
					{"name": "witness", "address": "perch/witness", "role": "witness", "running": true, "has_work": false},
					{"name": "refinery", "address": "perch/refinery", "role": "refinery", "running": true, "has_work": true},
					{"name": "able", "address": "perch/polecats/able", "role": "polecat", "running": true, "has_work": true},
					{"name": "baker", "address": "perch/polecats/baker", "role": "polecat", "running": false, "has_work": false},
				},
			},
			{
				"name":          "sidekick",
				"polecats":      []string{"charlie"},
				"polecat_count": 1,
				"has_witness":   true,
				"has_refinery":  false,
				"agents": []map[string]any{
					{"name": "witness", "address": "sidekick/witness", "role": "witness", "running": true, "has_work": false},
					{"name": "charlie", "address": "sidekick/polecats/charlie", "role": "polecat", "running": true, "has_work": true},
				},
			},
		},
		"summary": map[string]any{
			"rig_count":      2,
			"polecat_count":  3,
			"crew_count":     1,
			"witness_count":  2,
			"refinery_count": 1,
			"active_hooks":   3,
		},
	}
}

// TownStatusJSON returns the JSON encoding of TownStatus.
func (f *Fixtures) TownStatusJSON() []byte {
	b, _ := json.Marshal(f.TownStatus())
	return b
}

// Polecats returns sample gt polecat list --all --json response.
func (f *Fixtures) Polecats() []map[string]any {
	return []map[string]any{
		{"rig": "perch", "name": "able", "state": "idle", "session_running": true},
		{"rig": "perch", "name": "baker", "state": "idle", "session_running": false},
		{"rig": "sidekick", "name": "charlie", "state": "working", "session_running": true},
	}
}

// PolecatsJSON returns the JSON encoding of Polecats.
func (f *Fixtures) PolecatsJSON() []byte {
	b, _ := json.Marshal(f.Polecats())
	return b
}

// Convoys returns sample gt convoy list --json response.
func (f *Fixtures) Convoys() []map[string]any {
	return []map[string]any{
		{"id": "convoy-001", "title": "Feature: Auth", "status": "in_progress", "created_at": time.Now().Add(-24 * time.Hour).Format(time.RFC3339)},
		{"id": "convoy-002", "title": "Bug: Fix login", "status": "complete", "created_at": time.Now().Add(-48 * time.Hour).Format(time.RFC3339)},
	}
}

// ConvoysJSON returns the JSON encoding of Convoys.
func (f *Fixtures) ConvoysJSON() []byte {
	b, _ := json.Marshal(f.Convoys())
	return b
}

// MergeQueue returns sample gt mq list <rig> --json response.
func (f *Fixtures) MergeQueue(rig string) []map[string]any {
	switch rig {
	case "perch":
		return []map[string]any{
			{"id": "mr-001", "title": "Add auth", "status": "pending", "worker": "able", "branch": "feat/auth", "priority": 1},
			{"id": "mr-002", "title": "Fix bug", "status": "merging", "worker": "baker", "branch": "fix/bug", "priority": 2},
		}
	case "sidekick":
		return []map[string]any{
			{"id": "mr-003", "title": "Update docs", "status": "pending", "worker": "charlie", "branch": "docs/update", "priority": 3},
		}
	default:
		return nil
	}
}

// MergeQueueJSON returns the JSON encoding of MergeQueue for a rig.
func (f *Fixtures) MergeQueueJSON(rig string) []byte {
	b, _ := json.Marshal(f.MergeQueue(rig))
	return b
}

// Issues returns sample bd list --json response.
func (f *Fixtures) Issues() []map[string]any {
	now := time.Now()
	return []map[string]any{
		{
			"id":               "gt-001",
			"title":            "Implement auth",
			"description":      "Add user authentication",
			"status":           "in_progress",
			"priority":         1,
			"issue_type":       "feature",
			"created_at":       now.Add(-72 * time.Hour).Format(time.RFC3339),
			"created_by":       "testuser",
			"updated_at":       now.Add(-1 * time.Hour).Format(time.RFC3339),
			"labels":           []string{"auth", "security"},
			"dependency_count": 0,
			"dependent_count":  0,
		},
		{
			"id":               "gt-002",
			"title":            "Fix login bug",
			"description":      "Login fails on mobile",
			"status":           "open",
			"priority":         0,
			"issue_type":       "bug",
			"created_at":       now.Add(-24 * time.Hour).Format(time.RFC3339),
			"created_by":       "testuser",
			"updated_at":       now.Add(-2 * time.Hour).Format(time.RFC3339),
			"labels":           []string{"bug", "mobile"},
			"dependency_count": 0,
			"dependent_count":  0,
		},
		{
			"id":               "gt-003",
			"title":            "Update docs",
			"description":      "README needs update",
			"status":           "closed",
			"priority":         3,
			"issue_type":       "task",
			"created_at":       now.Add(-96 * time.Hour).Format(time.RFC3339),
			"created_by":       "testuser",
			"updated_at":       now.Add(-48 * time.Hour).Format(time.RFC3339),
			"labels":           []string{"docs"},
			"dependency_count": 0,
			"dependent_count":  0,
		},
	}
}

// IssuesJSON returns the JSON encoding of Issues.
func (f *Fixtures) IssuesJSON() []byte {
	b, _ := json.Marshal(f.Issues())
	return b
}

// OpenIssues returns only open issues.
func (f *Fixtures) OpenIssues() []map[string]any {
	var open []map[string]any
	for _, issue := range f.Issues() {
		if issue["status"] == "open" {
			open = append(open, issue)
		}
	}
	return open
}

// OpenIssuesJSON returns the JSON encoding of OpenIssues.
func (f *Fixtures) OpenIssuesJSON() []byte {
	b, _ := json.Marshal(f.OpenIssues())
	return b
}

// EmptyResponse returns empty JSON array (for empty list responses).
func (f *Fixtures) EmptyResponse() []byte {
	return []byte("[]")
}

// NullResponse returns JSON null.
func (f *Fixtures) NullResponse() []byte {
	return []byte("null")
}
