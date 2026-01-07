// Perch is the terminal UI for Gas Town.
//
// It provides a dashboard view of rigs, polecats, merge queues, convoys,
// and issues. The TUI uses the Bubble Tea framework and operates on the
// town root (~/gt by default, or GT_ROOT env var).
//
// Usage:
//
//	perch
//
// Environment Variables:
//
//	GT_ROOT - Path to the Gas Town workspace (default: ~/gt)
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/andyrewlee/perch/internal/tui"
)

// main is the entry point for the perch TUI.
// It initializes the town root, creates the Bubble Tea program,
// and runs it with an alternate screen.
func main() {
	// Default to ~/gt as town root, can be overridden via GT_ROOT env
	townRoot := os.Getenv("GT_ROOT")
	if townRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home dir: %v\n", err)
			os.Exit(1)
		}
		townRoot = filepath.Join(home, "gt")
	}

	p := tea.NewProgram(tui.NewWithTownRoot(townRoot), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
