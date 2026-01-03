package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SetupWizard manages the first-run town setup flow.
type SetupWizard struct {
	pathInput   textinput.Model
	state       setupState
	townRoot    string
	err         error
	statusMsg   string
	width       int
	height      int

	// Dependency check results
	depCheck *DependencyCheckResult
}

type setupState int

const (
	setupStateInput setupState = iota
	setupStateInstalling
	setupStateComplete
	setupStateError
)

// setupCompleteMsg signals that setup has completed (successfully or with error)
type setupCompleteMsg struct {
	townRoot string
	err      error
}

// NewSetupWizard creates a new setup wizard.
func NewSetupWizard() *SetupWizard {
	// Default path is ~/gt
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "gt")

	input := textinput.New()
	input.Placeholder = defaultPath
	input.SetValue(defaultPath)
	input.Focus()
	input.CharLimit = 256
	input.Width = 50
	input.Prompt = ""

	// Check dependencies upfront
	depCheck := CheckDependencies(nil, home)

	return &SetupWizard{
		pathInput: input,
		state:     setupStateInput,
		depCheck:  depCheck,
	}
}

// Init implements tea.Model.
func (w *SetupWizard) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (w *SetupWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return w.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
		return w, nil

	case setupCompleteMsg:
		if msg.err != nil {
			w.state = setupStateError
			w.err = msg.err
			return w, nil
		}
		w.state = setupStateComplete
		w.townRoot = msg.townRoot
		return w, nil
	}

	// Update text input
	var cmd tea.Cmd
	w.pathInput, cmd = w.pathInput.Update(msg)
	return w, cmd
}

func (w *SetupWizard) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch w.state {
	case setupStateInput:
		switch msg.String() {
		case "ctrl+c", "esc":
			return w, tea.Quit

		case "enter":
			// Block installation if dependencies are missing
			if w.depCheck != nil && !w.depCheck.AllFound {
				return w, nil
			}

			path := w.pathInput.Value()
			if path == "" {
				return w, nil
			}
			// Expand ~ to home directory
			if strings.HasPrefix(path, "~/") {
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, path[2:])
			}
			w.townRoot = path
			w.state = setupStateInstalling
			w.statusMsg = "Installing Gas Town..."
			return w, w.installCmd(path)

		case "r":
			// Recheck dependencies
			home, _ := os.UserHomeDir()
			w.depCheck = CheckDependencies(nil, home)
			return w, nil
		}

		// Update text input for other keys
		var cmd tea.Cmd
		w.pathInput, cmd = w.pathInput.Update(msg)
		return w, cmd

	case setupStateError:
		switch msg.String() {
		case "ctrl+c", "esc":
			return w, tea.Quit
		case "enter", "r":
			// Retry - go back to input state
			w.state = setupStateInput
			w.err = nil
			w.statusMsg = ""
			return w, nil
		}

	case setupStateComplete:
		// Any key proceeds to dashboard
		return w, nil
	}

	return w, nil
}

// installCmd runs gt install in the background.
func (w *SetupWizard) installCmd(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Run gt install
		runner := NewActionRunner(filepath.Dir(path))
		err := runner.runCommand(ctx, "gt", "install", path)

		return setupCompleteMsg{townRoot: path, err: err}
	}
}

// IsComplete returns true if setup finished successfully.
func (w *SetupWizard) IsComplete() bool {
	return w.state == setupStateComplete
}

// TownRoot returns the configured town root path.
func (w *SetupWizard) TownRoot() string {
	return w.townRoot
}

// View implements tea.Model.
func (w *SetupWizard) View() string {
	width := w.width
	height := w.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	// Calculate overlay dimensions
	overlayWidth := 70
	if overlayWidth > width-4 {
		overlayWidth = width - 4
	}

	innerWidth := overlayWidth - 4

	var content string
	switch w.state {
	case setupStateInput:
		content = w.renderInputState(innerWidth)
	case setupStateInstalling:
		content = w.renderInstallingState(innerWidth)
	case setupStateError:
		content = w.renderErrorState(innerWidth)
	case setupStateComplete:
		content = w.renderCompleteState(innerWidth)
	}

	overlay := setupOverlayStyle.
		Width(innerWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

func (w *SetupWizard) renderInputState(width int) string {
	title := setupTitleStyle.Render("Welcome to Gas Town")

	intro := []string{
		"",
		"Gas Town is a multi-agent workspace manager that coordinates",
		"autonomous AI agents (polecats, witnesses, refineries) across",
		"your projects.",
		"",
	}

	// Dependency status section
	var depSection []string
	depSection = append(depSection, headerStyle.Render("Required Tools:"))

	if w.depCheck != nil {
		depSection = append(depSection, RenderDependencyStatus(w.depCheck))

		if !w.depCheck.AllFound {
			depSection = append(depSection, "")
			depSection = append(depSection, RenderInstallGuidance(w.depCheck))
			depSection = append(depSection, "")
			depSection = append(depSection, formErrorStyle.Render("Please install missing tools before continuing."))
			depSection = append(depSection, mutedStyle.Render("Press 'r' to recheck after installing."))
		}
	}

	// Path input section (only interactive if deps are satisfied)
	var pathSection []string
	if w.depCheck == nil || w.depCheck.AllFound {
		pathSection = append(pathSection, "")
		pathSection = append(pathSection, "Choose a location for your town root directory:")
		pathSection = append(pathSection, "")
		pathLabel := formLabelFocusedStyle.Render("Town Root")
		pathInput := formInputFocusedStyle.Width(width - 4).Render(w.pathInput.View())
		pathSection = append(pathSection, pathLabel, pathInput)
	}

	// Help section
	var help []string
	if w.depCheck == nil || w.depCheck.AllFound {
		help = []string{
			"",
			mutedStyle.Render("This directory will contain:"),
			mutedStyle.Render("  • CLAUDE.md     - Mayor configuration"),
			mutedStyle.Render("  • mayor/        - Town management"),
			mutedStyle.Render("  • .beads/       - Issue tracking"),
			"",
			mutedStyle.Render("Enter: install | Esc: quit"),
		}
	} else {
		help = []string{
			"",
			mutedStyle.Render("r: recheck | Esc: quit"),
		}
	}

	parts := []string{title}
	parts = append(parts, intro...)
	parts = append(parts, depSection...)
	parts = append(parts, pathSection...)
	parts = append(parts, help...)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (w *SetupWizard) renderInstallingState(width int) string {
	title := setupTitleStyle.Render("Setting Up Gas Town")

	content := []string{
		"",
		"Installing Gas Town at:",
		headerStyle.Render(w.townRoot),
		"",
		setupProgressStyle.Render("◐ " + w.statusMsg),
		"",
		mutedStyle.Render("This may take a moment..."),
	}

	parts := []string{title}
	parts = append(parts, content...)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (w *SetupWizard) renderErrorState(width int) string {
	title := setupTitleStyle.Render("Setup Failed")

	errMsg := "Unknown error"
	if w.err != nil {
		errMsg = w.err.Error()
	}

	content := []string{
		"",
		formErrorStyle.Render("Error: " + errMsg),
		"",
		"Please check that:",
		mutedStyle.Render("  • The path is writable"),
		mutedStyle.Render("  • The 'gt' command is installed"),
		mutedStyle.Render("  • No existing town blocks installation"),
		"",
		mutedStyle.Render("Press Enter or 'r' to retry | Esc to quit"),
	}

	parts := []string{title}
	parts = append(parts, content...)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (w *SetupWizard) renderCompleteState(width int) string {
	title := setupTitleStyle.Render("Gas Town Ready!")

	content := []string{
		"",
		"Successfully created Gas Town at:",
		headerStyle.Render(w.townRoot),
		"",
		"Next steps:",
		fmt.Sprintf("  %s Add a rig with 'a' key", setupCheckStyle.Render("✓")),
		fmt.Sprintf("  %s Boot your first rig with 'b' key", mutedStyle.Render("○")),
		fmt.Sprintf("  %s Start working with agents", mutedStyle.Render("○")),
		"",
		mutedStyle.Render("Press any key to continue to dashboard..."),
	}

	parts := []string{title}
	parts = append(parts, content...)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// Setup-specific styles
var (
	setupOverlayStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(highlight).
		Padding(1, 2).
		Background(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1A1A1A"})

	setupTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(highlight).
		MarginBottom(1)

	setupProgressStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFF00"))

	setupCheckStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00"))
)

// TownExists checks if a town exists at the given path.
func TownExists(path string) bool {
	// Check for .gt directory or CLAUDE.md file (indicators of a town)
	gtDir := filepath.Join(path, ".gt")
	if _, err := os.Stat(gtDir); err == nil {
		return true
	}

	mayorDir := filepath.Join(path, "mayor")
	if _, err := os.Stat(mayorDir); err == nil {
		return true
	}

	return false
}
