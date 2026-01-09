package tui

import (
	"os"
	"testing"
	"time"
)

// NewTestModel creates a Model with a temporary town root for testing.
// This ensures tests don't depend on the host filesystem having a real town.
func NewTestModel(t *testing.T) Model {
	t.Helper()
	resetNow := setNow(time.Date(2026, 1, 8, 17, 0, 0, 0, time.UTC))
	t.Cleanup(resetNow)
	tmpDir := t.TempDir()
	// Create mayor/ directory to make TownExists return true
	if err := os.MkdirAll(tmpDir+"/mayor", 0755); err != nil {
		t.Fatalf("failed to create test town: %v", err)
	}
	return NewWithTownRoot(tmpDir)
}
