package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AddRigForm manages the add rig form state
type AddRigForm struct {
	inputs     []textinput.Model
	focusIndex int
	submitted  bool
	cancelled  bool
}

const (
	addRigInputName = iota
	addRigInputURL
	addRigInputPrefix
)

// NewAddRigForm creates a new add rig form
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

// Name returns the entered rig name
func (f *AddRigForm) Name() string {
	return f.inputs[addRigInputName].Value()
}

// URL returns the entered git URL
func (f *AddRigForm) URL() string {
	return f.inputs[addRigInputURL].Value()
}

// Prefix returns the entered prefix (may be empty)
func (f *AddRigForm) Prefix() string {
	return f.inputs[addRigInputPrefix].Value()
}

// IsValid returns true if required fields are filled
func (f *AddRigForm) IsValid() bool {
	return f.Name() != "" && f.URL() != ""
}

// IsSubmitted returns true if the form was submitted
func (f *AddRigForm) IsSubmitted() bool {
	return f.submitted
}

// IsCancelled returns true if the form was cancelled
func (f *AddRigForm) IsCancelled() bool {
	return f.cancelled
}

// Update handles input events for the form
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

// View renders the form as an overlay
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
