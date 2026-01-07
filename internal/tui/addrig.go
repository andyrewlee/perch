// Package tui provides terminal UI components for the Gas Town dashboard.
//
// This file implements the AddRigForm, an interactive form for adding new rigs
// to the Gas Town workspace.
package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AddRigForm manages the state and rendering of an interactive form for adding
// a new rig to the Gas Town workspace.
//
// The form collects three pieces of information:
//   - Name: The rig name (required)
//   - URL: The git repository URL (required)
//   - Prefix: The beads issue prefix (optional, derived from name if not provided)
//
// The form supports keyboard navigation:
//   - Tab/Down: Move to next field
//   - Shift+Tab/Up: Move to previous field
//   - Enter: Submit form (when on last field)
//   - Esc: Cancel form
//
// Example usage:
//
//	form := NewAddRigForm()
//	// In Update method:
//	if _, ok := msg.(tea.KeyMsg); ok {
//	    cmd := form.Update(msg)
//	    if form.IsSubmitted() {
//	        name, url, prefix := form.Name(), form.URL(), form.Prefix()
//	        // Proceed with rig creation
//	    }
//	}
//	// In View method:
//	return form.View(width, height)
type AddRigForm struct {
	inputs     []textinput.Model
	focusIndex int
	submitted  bool
	cancelled  bool
}

const (
	// addRigInputName is the index for the name input field.
	addRigInputName = iota
	// addRigInputURL is the index for the git URL input field.
	addRigInputURL
	// addRigInputPrefix is the index for the prefix input field.
	addRigInputPrefix
)

// NewAddRigForm creates and initializes a new AddRigForm with three input fields:
// name, git URL, and optional prefix. The name field is focused by default.
func NewAddRigForm() *AddRigForm {
	inputs := make([]textinput.Model, 3)

	// Name field
	inputs[addRigInputName] = textinput.New()
	inputs[addRigInputName].Placeholder = "my-project"
	inputs[addRigInputName].Focus()
	inputs[addRigInputName].CharLimit = 64
	inputs[addRigInputName].Width = 40
	inputs[addRigInputName].Prompt = ""

	// Git URL field
	inputs[addRigInputURL] = textinput.New()
	inputs[addRigInputURL].Placeholder = "https://github.com/user/repo.git"
	inputs[addRigInputURL].CharLimit = 256
	inputs[addRigInputURL].Width = 40
	inputs[addRigInputURL].Prompt = ""

	// Prefix field (optional)
	inputs[addRigInputPrefix] = textinput.New()
	inputs[addRigInputPrefix].Placeholder = "(optional, derived from name)"
	inputs[addRigInputPrefix].CharLimit = 16
	inputs[addRigInputPrefix].Width = 40
	inputs[addRigInputPrefix].Prompt = ""

	return &AddRigForm{
		inputs:     inputs,
		focusIndex: 0,
	}
}

// Name returns the value of the name input field.
func (f *AddRigForm) Name() string {
	return f.inputs[addRigInputName].Value()
}

// URL returns the value of the git URL input field.
func (f *AddRigForm) URL() string {
	return f.inputs[addRigInputURL].Value()
}

// Prefix returns the value of the prefix input field.
// May be empty if the user chose not to specify a prefix.
func (f *AddRigForm) Prefix() string {
	return f.inputs[addRigInputPrefix].Value()
}

// IsValid returns true if all required fields (name and URL) are filled.
// The prefix field is optional and does not affect validation.
func (f *AddRigForm) IsValid() bool {
	return f.Name() != "" && f.URL() != ""
}

// IsSubmitted returns true if the form was submitted via Enter key.
// When submitted, the form can be considered complete and values retrieved.
func (f *AddRigForm) IsSubmitted() bool {
	return f.submitted
}

// IsCancelled returns true if the form was cancelled via Escape key.
// When cancelled, the form should be dismissed without processing values.
func (f *AddRigForm) IsCancelled() bool {
	return f.cancelled
}

// Update handles keyboard input events for the form.
//
// It supports the following keybindings:
//   - Esc: Cancel the form (sets IsCancelled to true)
//   - Enter: Submit the form if on the last field and valid (sets IsSubmitted to true)
//   - Tab/Down: Move focus to the next field
//   - Shift+Tab/Up: Move focus to the previous field
//
// Returns a Bubbletea command (typically a Focus/Blur command for input fields).
func (f *AddRigForm) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		f.cancelled = true
		return nil

	case "enter":
		if f.focusIndex == len(f.inputs)-1 {
			// On last field, submit
			if f.IsValid() {
				f.submitted = true
			}
			return nil
		}
		// Move to next field
		return f.nextInput()

	case "tab", "down":
		return f.nextInput()

	case "shift+tab", "up":
		return f.prevInput()
	}

	// Update focused input
	cmd := f.updateInputs(msg)
	return cmd
}

func (f *AddRigForm) nextInput() tea.Cmd {
	f.inputs[f.focusIndex].Blur()
	f.focusIndex = (f.focusIndex + 1) % len(f.inputs)
	return f.inputs[f.focusIndex].Focus()
}

func (f *AddRigForm) prevInput() tea.Cmd {
	f.inputs[f.focusIndex].Blur()
	f.focusIndex = (f.focusIndex - 1 + len(f.inputs)) % len(f.inputs)
	return f.inputs[f.focusIndex].Focus()
}

func (f *AddRigForm) updateInputs(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	f.inputs[f.focusIndex], cmd = f.inputs[f.focusIndex].Update(msg)
	return cmd
}

// View renders the form as a centered overlay dialog.
//
// The overlay displays:
//   - A title "Add New Rig"
//   - Three labeled input fields (Name, Git URL, Prefix)
//   - Validation messages for incomplete required fields
//   - Help text showing keyboard shortcuts
//
// The overlay is automatically centered within the provided dimensions
// and adjusts its size if the viewport is too small.
//
// Parameters:
//   - width: The total width of the viewport
//   - height: The total height of the viewport
//
// Returns the rendered form as a string suitable for display in the TUI.
func (f *AddRigForm) View(width, height int) string {
	// Calculate overlay dimensions
	overlayWidth := 60
	overlayHeight := 16
	if overlayWidth > width-4 {
		overlayWidth = width - 4
	}
	if overlayHeight > height-4 {
		overlayHeight = height - 4
	}

	innerWidth := overlayWidth - 4

	// Title
	title := formTitleStyle.Render("Add New Rig")

	// Field labels and inputs
	var fields []string

	// Name field
	nameLabel := "Name"
	if f.focusIndex == addRigInputName {
		nameLabel = formLabelFocusedStyle.Render(nameLabel)
	} else {
		nameLabel = formLabelStyle.Render(nameLabel)
	}
	fields = append(fields, nameLabel)
	fields = append(fields, f.renderInput(addRigInputName, innerWidth))
	fields = append(fields, "")

	// URL field
	urlLabel := "Git URL"
	if f.focusIndex == addRigInputURL {
		urlLabel = formLabelFocusedStyle.Render(urlLabel)
	} else {
		urlLabel = formLabelStyle.Render(urlLabel)
	}
	fields = append(fields, urlLabel)
	fields = append(fields, f.renderInput(addRigInputURL, innerWidth))
	fields = append(fields, "")

	// Prefix field
	prefixLabel := "Prefix (optional)"
	if f.focusIndex == addRigInputPrefix {
		prefixLabel = formLabelFocusedStyle.Render(prefixLabel)
	} else {
		prefixLabel = formLabelStyle.Render(prefixLabel)
	}
	fields = append(fields, prefixLabel)
	fields = append(fields, f.renderInput(addRigInputPrefix, innerWidth))
	fields = append(fields, "")

	// Validation message
	var validationMsg string
	if !f.IsValid() && (f.Name() != "" || f.URL() != "") {
		if f.Name() == "" {
			validationMsg = formErrorStyle.Render("Name is required")
		} else if f.URL() == "" {
			validationMsg = formErrorStyle.Render("Git URL is required")
		}
	}
	fields = append(fields, validationMsg)

	// Help text
	help := mutedStyle.Render("Tab: next field | Enter: submit | Esc: cancel")
	fields = append(fields, help)

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", lipgloss.JoinVertical(lipgloss.Left, fields...))

	overlay := formOverlayStyle.
		Width(innerWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

func (f *AddRigForm) renderInput(index int, width int) string {
	input := f.inputs[index]
	style := formInputStyle
	if f.focusIndex == index {
		style = formInputFocusedStyle
	}
	return style.Width(width - 4).Render(input.View())
}

// Form-specific styles
var (
	formOverlayStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(highlight).
				Padding(1, 2).
				Background(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1A1A1A"})

	formTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginBottom(1)

	formLabelStyle = lipgloss.NewStyle().
			Foreground(muted)

	formLabelFocusedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(highlight)

	formInputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(subtle).
			Padding(0, 1)

	formInputFocusedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(highlight).
				Padding(0, 1)

	formErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6666"))
)
