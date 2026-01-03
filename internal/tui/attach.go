package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// AttachDialog represents the dialog for attaching to an existing town.
type AttachDialog struct {
	input       textinput.Model
	error       string
	validPath   bool
	validTown   bool
	townName    string
	rigCount    int
	suggestions []string
}

// TownJSON represents the structure of mayor/town.json
type TownJSON struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	Name    string `json:"name"`
}

// NewAttachDialog creates a new attach town dialog.
func NewAttachDialog() *AttachDialog {
	ti := textinput.New()
	ti.Placeholder = "Enter town root path (e.g., ~/gt)"
	ti.CharLimit = 256
	ti.Width = 50
	ti.Focus()

	// Start with home directory as default
	home, _ := os.UserHomeDir()
	if home != "" {
		ti.SetValue(filepath.Join(home, "gt"))
	}

	d := &AttachDialog{
		input: ti,
	}
	d.validate()
	return d
}

// Value returns the current input value.
func (d *AttachDialog) Value() string {
	return d.input.Value()
}

// SetValue sets the input value.
func (d *AttachDialog) SetValue(path string) {
	d.input.SetValue(path)
	d.validate()
}

// Update handles input updates.
func (d *AttachDialog) Update(msg interface{}) {
	var cmd interface{}
	d.input, cmd = d.input.Update(msg)
	_ = cmd // Ignore command for now

	d.validate()
}

// validate checks if the current path is a valid town.
func (d *AttachDialog) validate() {
	d.error = ""
	d.validPath = false
	d.validTown = false
	d.townName = ""
	d.rigCount = 0
	d.suggestions = nil

	path := expandPath(d.input.Value())
	if path == "" {
		d.error = "Enter a path"
		return
	}

	// Check if directory exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		d.error = "Directory does not exist"
		d.suggestions = findSuggestions(path)
		return
	}
	if err != nil {
		d.error = "Error accessing path: " + err.Error()
		return
	}
	if !info.IsDir() {
		d.error = "Path is not a directory"
		return
	}
	d.validPath = true

	// Check for mayor/town.json
	townJSONPath := filepath.Join(path, "mayor", "town.json")
	data, err := os.ReadFile(townJSONPath)
	if os.IsNotExist(err) {
		d.error = "Not a Gas Town: mayor/town.json not found"
		return
	}
	if err != nil {
		d.error = "Error reading town.json: " + err.Error()
		return
	}

	// Parse and validate town.json
	var town TownJSON
	if err := json.Unmarshal(data, &town); err != nil {
		d.error = "Invalid town.json: " + err.Error()
		return
	}
	if town.Type != "town" {
		d.error = "Invalid town.json: type is not 'town'"
		return
	}

	d.validTown = true
	d.townName = town.Name
	d.rigCount = countRigs(path)
}

// expandPath expands ~ to home directory and cleans the path.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		if home != "" {
			path = filepath.Join(home, path[1:])
		}
	}
	return filepath.Clean(path)
}

// countRigs counts the number of rig directories in the town.
func countRigs(townPath string) int {
	entries, err := os.ReadDir(townPath)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip mayor and hidden directories
		name := entry.Name()
		if name == "mayor" || strings.HasPrefix(name, ".") {
			continue
		}
		// Check if it has a rig.json or at least polecats dir
		rigPath := filepath.Join(townPath, name)
		if _, err := os.Stat(filepath.Join(rigPath, "polecats")); err == nil {
			count++
		} else if _, err := os.Stat(filepath.Join(rigPath, "rig.json")); err == nil {
			count++
		}
	}
	return count
}

// findSuggestions finds directory suggestions based on the current path.
func findSuggestions(path string) []string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var suggestions []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(base)) {
			suggestions = append(suggestions, filepath.Join(dir, name))
			if len(suggestions) >= 3 {
				break
			}
		}
	}
	return suggestions
}

// IsValid returns whether the current path is a valid town.
func (d *AttachDialog) IsValid() bool {
	return d.validTown
}

// ExpandedPath returns the fully expanded path.
func (d *AttachDialog) ExpandedPath() string {
	return expandPath(d.input.Value())
}

// Render renders the attach dialog.
func (d *AttachDialog) Render(width, height int) string {
	// Dialog dimensions
	dialogWidth := width * 70 / 100
	if dialogWidth < 60 {
		dialogWidth = min(60, width-4)
	}
	if dialogWidth > 80 {
		dialogWidth = 80
	}
	dialogHeight := 16
	if dialogHeight > height-4 {
		dialogHeight = height - 4
	}

	// Title
	title := helpTitleStyle.Render("Attach to Existing Town")

	// Instructions
	instructions := mutedStyle.Render("Enter the path to a Gas Town root directory.")

	// Input field
	inputLabel := headerStyle.Render("Town Root:")
	inputField := d.input.View()

	// Validation status
	var status string
	if d.validTown {
		status = statusStyle.Render("  Valid town: " + d.townName)
		if d.rigCount > 0 {
			status += mutedStyle.Render(" (" + itoa(d.rigCount) + " rigs)")
		}
	} else if d.validPath {
		status = statusErrorStyle.Render("  " + d.error)
	} else if d.error != "" {
		status = statusErrorStyle.Render("  " + d.error)
	}

	// Suggestions
	var suggestionsStr string
	if len(d.suggestions) > 0 {
		suggestionsStr = mutedStyle.Render("\n  Suggestions:")
		for _, s := range d.suggestions {
			suggestionsStr += "\n    " + mutedStyle.Render(s)
		}
	}

	// Help text
	helpText := mutedStyle.Render("\nEnter: Attach  |  Esc: Cancel  |  Tab: Autocomplete")

	// Build content
	content := strings.Join([]string{
		title,
		"",
		instructions,
		"",
		inputLabel,
		inputField,
		status,
		suggestionsStr,
		"",
		helpText,
	}, "\n")

	// Inner dimensions
	innerWidth := dialogWidth - 4
	innerHeight := dialogHeight - 2

	// Truncate if needed
	lines := strings.Split(content, "\n")
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	content = strings.Join(lines, "\n")

	// Render the dialog box
	dialogStyle := helpOverlayStyle.
		Width(innerWidth).
		Height(innerHeight)

	dialog := dialogStyle.Render(content)

	// Center the dialog
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, dialog)
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
