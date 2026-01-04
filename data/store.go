package data

import (
	"context"
	"sync"
	"time"
)

// Store maintains a cached snapshot of town data with refresh capability.
type Store struct {
	loader   *Loader
	mu       sync.RWMutex
	snapshot *Snapshot

	// RefreshInterval is how often to auto-refresh. Zero disables auto-refresh.
	RefreshInterval time.Duration

	// OnRefresh is called after each refresh with the new snapshot.
	OnRefresh func(*Snapshot)

	// OnError is called when refresh fails entirely (no partial data).
	OnError func(error)

	cancelFunc context.CancelFunc
}

// NewStore creates a new data store.
func NewStore(townRoot string) *Store {
	return &Store{
		loader: NewLoader(townRoot),
	}
}

// NewStoreWithLoader creates a new data store with a custom loader.
// Useful for testing with mock loaders.
func NewStoreWithLoader(loader *Loader) *Store {
	return &Store{
		loader: loader,
	}
}

// Snapshot returns the current cached snapshot.
// Returns nil if no data has been loaded.
func (s *Store) Snapshot() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

// Refresh loads fresh data from all sources.
func (s *Store) Refresh(ctx context.Context) *Snapshot {
	snap := s.loader.LoadAll(ctx)

	s.mu.Lock()
	s.snapshot = snap
	s.mu.Unlock()

	if s.OnRefresh != nil {
		s.OnRefresh(snap)
	}

	return snap
}

// StartAutoRefresh begins periodic background refreshes.
// Call Stop() to stop the refresh loop.
func (s *Store) StartAutoRefresh(ctx context.Context) {
	if s.RefreshInterval <= 0 {
		return
	}

	ctx, s.cancelFunc = context.WithCancel(ctx)

	// Initial load
	s.Refresh(ctx)

	go func() {
		ticker := time.NewTicker(s.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Refresh(ctx)
			}
		}
	}()
}

// Stop halts auto-refresh.
func (s *Store) Stop() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

// Town returns the town status from the cached snapshot.
func (s *Store) Town() *TownStatus {
	snap := s.Snapshot()
	if snap == nil {
		return nil
	}
	return snap.Town
}

// Rigs returns all rigs from the cached snapshot.
func (s *Store) Rigs() []Rig {
	town := s.Town()
	if town == nil {
		return nil
	}
	return town.Rigs
}

// Rig returns a specific rig by name.
func (s *Store) Rig(name string) *Rig {
	for _, r := range s.Rigs() {
		if r.Name == name {
			return &r
		}
	}
	return nil
}

// Polecats returns all polecats from the cached snapshot.
func (s *Store) Polecats() []Polecat {
	snap := s.Snapshot()
	if snap == nil {
		return nil
	}
	return snap.Polecats
}

// Convoys returns all convoys from the cached snapshot.
func (s *Store) Convoys() []Convoy {
	snap := s.Snapshot()
	if snap == nil {
		return nil
	}
	return snap.Convoys
}

// MergeQueue returns the merge queue for a specific rig.
func (s *Store) MergeQueue(rig string) []MergeRequest {
	snap := s.Snapshot()
	if snap == nil {
		return nil
	}
	return snap.MergeQueues[rig]
}

// AllMergeQueues returns merge queues for all rigs.
func (s *Store) AllMergeQueues() map[string][]MergeRequest {
	snap := s.Snapshot()
	if snap == nil {
		return nil
	}
	return snap.MergeQueues
}

// Issues returns all issues from the cached snapshot.
func (s *Store) Issues() []Issue {
	snap := s.Snapshot()
	if snap == nil {
		return nil
	}
	return snap.Issues
}

// IssuesByStatus returns issues filtered by status.
func (s *Store) IssuesByStatus(status string) []Issue {
	all := s.Issues()
	if all == nil {
		return nil
	}
	var filtered []Issue
	for _, issue := range all {
		if issue.Status == status {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// OpenIssues returns all open issues.
func (s *Store) OpenIssues() []Issue {
	return s.IssuesByStatus("open")
}

// InProgressIssues returns all in-progress issues.
func (s *Store) InProgressIssues() []Issue {
	return s.IssuesByStatus("in_progress")
}

// Summary returns the town summary from the cached snapshot.
func (s *Store) Summary() *Summary {
	town := s.Town()
	if town == nil {
		return nil
	}
	return &town.Summary
}

// LastRefresh returns when data was last loaded.
func (s *Store) LastRefresh() time.Time {
	snap := s.Snapshot()
	if snap == nil {
		return time.Time{}
	}
	return snap.LoadedAt
}

// Errors returns any errors from the last refresh.
func (s *Store) Errors() []error {
	snap := s.Snapshot()
	if snap == nil {
		return nil
	}
	return snap.Errors
}

// Loader returns the underlying data loader for direct queries.
func (s *Store) Loader() *Loader {
	return s.loader
}
