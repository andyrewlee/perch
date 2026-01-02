package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/andyrewlee/perch/internal/tui"
)

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
