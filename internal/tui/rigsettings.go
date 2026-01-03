package tui

import (
	"strconv"

	"github.com/andyrewlee/perch/data"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RigSettingsForm manages the rig settings form state
type RigSettingsForm struct {
	rigName    string
	inputs     []textinput.Model
	toggles    []bool // For boolean options
	focusIndex int
	submitted  bool
	cancelled  bool
	validationErr string
}

// Input field indices
const (
	settingsInputPrefix = iota
	settingsInputTheme
	settingsInputMaxWorkers
	settingsInputTestCommand
	settingsFieldCount // Number of text inputs
)

// Toggle indices (after text inputs)
const (
	settingsToggleMQEnabled = iota
	settingsToggleRunTests
	settingsToggleCount
)

// NewRigSettingsForm creates a new rig settings form with loaded values
func NewRigSettingsForm(rigName string, settings *data.RigSettings) *RigSettingsForm {
	inputs := make([]textinput.Model, settingsFieldCount)

	// Prefix field
	inputs[settingsInputPrefix] = textinput.New()
	inputs[settingsInputPrefix].Placeholder = "gt"
	inputs[settingsInputPrefix].CharLimit = 16
	inputs[settingsInputPrefix].Width = 30
	inputs[settingsInputPrefix].Prompt = ""
	if settings != nil && settings.Prefix != "" {
		inputs[settingsInputPrefix].SetValue(settings.Prefix)
	}

	// Theme field
	inputs[settingsInputTheme] = textinput.New()
	inputs[settingsInputTheme].Placeholder = "(optional)"
	inputs[settingsInputTheme].CharLimit = 32
	inputs[settingsInputTheme].Width = 30
	inputs[settingsInputTheme].Prompt = ""
	if settings != nil && settings.Theme != "" {
		inputs[settingsInputTheme].SetValue(settings.Theme)
	}

	// Max workers field
	inputs[settingsInputMaxWorkers] = textinput.New()
	inputs[settingsInputMaxWorkers].Placeholder = "0 (unlimited)"
	inputs[settingsInputMaxWorkers].CharLimit = 4
	inputs[settingsInputMaxWorkers].Width = 30
	inputs[settingsInputMaxWorkers].Prompt = ""
	if settings != nil && settings.MaxWorkers > 0 {
		inputs[settingsInputMaxWorkers].SetValue(strconv.Itoa(settings.MaxWorkers))
	}

	// Test command field
	inputs[settingsInputTestCommand] = textinput.New()
	inputs[settingsInputTestCommand].Placeholder = "go test ./..."
	inputs[settingsInputTestCommand].CharLimit = 128
	inputs[settingsInputTestCommand].Width = 30
	inputs[settingsInputTestCommand].Prompt = ""
	if settings != nil && settings.MergeQueue.TestCommand != "" {
		inputs[settingsInputTestCommand].SetValue(settings.MergeQueue.TestCommand)
	}

	// Focus first input
	inputs[settingsInputPrefix].Focus()

	// Initialize toggles from settings
	toggles := make([]bool, settingsToggleCount)
	if settings != nil {
		toggles[settingsToggleMQEnabled] = settings.MergeQueue.Enabled
		toggles[settingsToggleRunTests] = settings.MergeQueue.RunTests
	} else {
		// Defaults
		toggles[settingsToggleMQEnabled] = true
		toggles[settingsToggleRunTests] = true
	}

	return &RigSettingsForm{
		rigName:    rigName,
		inputs:     inputs,
		toggles:    toggles,
		focusIndex: 0,
	}
}

// RigName returns the rig name being edited
func (f *RigSettingsForm) RigName() string {
	return f.rigName
}

// Prefix returns the entered prefix
func (f *RigSettingsForm) Prefix() string {
	return f.inputs[settingsInputPrefix].Value()
}

// Theme returns the entered theme
func (f *RigSettingsForm) Theme() string {
	return f.inputs[settingsInputTheme].Value()
}

// MaxWorkers returns the entered max workers as int
func (f *RigSettingsForm) MaxWorkers() int {
	val := f.inputs[settingsInputMaxWorkers].Value()
	if val == "" {
		return 0
	}
	n, _ := strconv.Atoi(val)
	return n
}

// TestCommand returns the entered test command
func (f *RigSettingsForm) TestCommand() string {
	return f.inputs[settingsInputTestCommand].Value()
}

// MQEnabled returns whether merge queue is enabled
func (f *RigSettingsForm) MQEnabled() bool {
	return f.toggles[settingsToggleMQEnabled]
}

// RunTests returns whether tests should run
func (f *RigSettingsForm) RunTests() bool {
	return f.toggles[settingsToggleRunTests]
}

// ToSettings converts form values to RigSettings
func (f *RigSettingsForm) ToSettings() *data.RigSettings {
	return &data.RigSettings{
		Name:       f.rigName,
		Prefix:     f.Prefix(),
		Theme:      f.Theme(),
		MaxWorkers: f.MaxWorkers(),
		MergeQueue: data.MergeQueueConfig{
			Enabled:     f.MQEnabled(),
			RunTests:    f.RunTests(),
			TestCommand: f.TestCommand(),
		},
	}
}

// IsValid returns true if required fields are filled and valid
func (f *RigSettingsForm) IsValid() bool {
	// Prefix is required
	if f.Prefix() == "" {
		f.validationErr = "Prefix is required"
		return false
	}

	// MaxWorkers must be a valid non-negative integer
	maxWorkersStr := f.inputs[settingsInputMaxWorkers].Value()
	if maxWorkersStr != "" {
		n, err := strconv.Atoi(maxWorkersStr)
		if err != nil || n < 0 {
			f.validationErr = "Max workers must be a non-negative number"
			return false
		}
	}

	// If run tests is enabled, test command is required
	if f.RunTests() && f.TestCommand() == "" {
		f.validationErr = "Test command is required when run tests is enabled"
		return false
	}

	f.validationErr = ""
	return true
}

// ValidationError returns the current validation error
func (f *RigSettingsForm) ValidationError() string {
	return f.validationErr
}

// IsSubmitted returns true if the form was submitted
func (f *RigSettingsForm) IsSubmitted() bool {
	return f.submitted
}

// IsCancelled returns true if the form was cancelled
func (f *RigSettingsForm) IsCancelled() bool {
	return f.cancelled
}

// totalFields returns total navigable fields (inputs + toggles)
func (f *RigSettingsForm) totalFields() int {
	return settingsFieldCount + settingsToggleCount
}

// isOnToggle returns true if focus is on a toggle field
func (f *RigSettingsForm) isOnToggle() bool {
	return f.focusIndex >= settingsFieldCount
}

// toggleIndex returns the toggle index when focus is on a toggle
func (f *RigSettingsForm) toggleIndex() int {
	return f.focusIndex - settingsFieldCount
}

// Update handles input events for the form
func (f *RigSettingsForm) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		f.cancelled = true
		return nil

	case "enter":
		if f.focusIndex == f.totalFields()-1 {
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

	case " ":
		// Space toggles checkboxes
		if f.isOnToggle() {
			idx := f.toggleIndex()
			f.toggles[idx] = !f.toggles[idx]
			return nil
		}
	}

	// Update focused input (only if on text input)
	if !f.isOnToggle() {
		cmd := f.updateInputs(msg)
		return cmd
	}

	return nil
}

func (f *RigSettingsForm) nextInput() tea.Cmd {
	// Blur current input if on text field
	if !f.isOnToggle() {
		f.inputs[f.focusIndex].Blur()
	}

	f.focusIndex = (f.focusIndex + 1) % f.totalFields()

	// Focus new input if on text field
	if !f.isOnToggle() {
		return f.inputs[f.focusIndex].Focus()
	}
	return nil
}

func (f *RigSettingsForm) prevInput() tea.Cmd {
	// Blur current input if on text field
	if !f.isOnToggle() {
		f.inputs[f.focusIndex].Blur()
	}

	f.focusIndex = (f.focusIndex - 1 + f.totalFields()) % f.totalFields()

	// Focus new input if on text field
	if !f.isOnToggle() {
		return f.inputs[f.focusIndex].Focus()
	}
	return nil
}

func (f *RigSettingsForm) updateInputs(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	f.inputs[f.focusIndex], cmd = f.inputs[f.focusIndex].Update(msg)
	return cmd
}

// View renders the form as an overlay
func (f *RigSettingsForm) View(width, height int) string {
	// Calculate overlay dimensions
	overlayWidth := 60
	overlayHeight := 26
	if overlayWidth > width-4 {
		overlayWidth = width - 4
	}
	if overlayHeight > height-4 {
		overlayHeight = height - 4
	}

	innerWidth := overlayWidth - 4

	// Title
	title := formTitleStyle.Render("Edit Rig: " + f.rigName)

	// Field labels and inputs
	var fields []string

	// Prefix field
	prefixLabel := "Prefix"
	if f.focusIndex == settingsInputPrefix {
		prefixLabel = formLabelFocusedStyle.Render(prefixLabel)
	} else {
		prefixLabel = formLabelStyle.Render(prefixLabel)
	}
	fields = append(fields, prefixLabel)
	fields = append(fields, f.renderInput(settingsInputPrefix, innerWidth))
	fields = append(fields, "")

	// Theme field
	themeLabel := "Theme (optional)"
	if f.focusIndex == settingsInputTheme {
		themeLabel = formLabelFocusedStyle.Render(themeLabel)
	} else {
		themeLabel = formLabelStyle.Render(themeLabel)
	}
	fields = append(fields, themeLabel)
	fields = append(fields, f.renderInput(settingsInputTheme, innerWidth))
	fields = append(fields, "")

	// Max workers field
	maxWorkersLabel := "Max Workers (0 = unlimited)"
	if f.focusIndex == settingsInputMaxWorkers {
		maxWorkersLabel = formLabelFocusedStyle.Render(maxWorkersLabel)
	} else {
		maxWorkersLabel = formLabelStyle.Render(maxWorkersLabel)
	}
	fields = append(fields, maxWorkersLabel)
	fields = append(fields, f.renderInput(settingsInputMaxWorkers, innerWidth))
	fields = append(fields, "")

	// Merge Queue section header
	fields = append(fields, settingsSectionStyle.Render("Merge Queue"))

	// MQ Enabled toggle
	mqEnabledLabel := "Enabled"
	mqEnabledFocus := f.focusIndex == settingsFieldCount+settingsToggleMQEnabled
	fields = append(fields, f.renderToggle(mqEnabledLabel, f.toggles[settingsToggleMQEnabled], mqEnabledFocus))

	// Run Tests toggle
	runTestsLabel := "Run Tests"
	runTestsFocus := f.focusIndex == settingsFieldCount+settingsToggleRunTests
	fields = append(fields, f.renderToggle(runTestsLabel, f.toggles[settingsToggleRunTests], runTestsFocus))
	fields = append(fields, "")

	// Test command field
	testCmdLabel := "Test Command"
	if f.focusIndex == settingsInputTestCommand {
		testCmdLabel = formLabelFocusedStyle.Render(testCmdLabel)
	} else {
		testCmdLabel = formLabelStyle.Render(testCmdLabel)
	}
	fields = append(fields, testCmdLabel)
	fields = append(fields, f.renderInput(settingsInputTestCommand, innerWidth))
	fields = append(fields, "")

	// Validation message
	f.IsValid() // Update validation error
	if f.validationErr != "" {
		fields = append(fields, formErrorStyle.Render(f.validationErr))
	} else {
		fields = append(fields, "")
	}

	// Help text
	help := mutedStyle.Render("Tab: next | Space: toggle | Enter: submit | Esc: cancel")
	fields = append(fields, help)

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", lipgloss.JoinVertical(lipgloss.Left, fields...))

	overlay := formOverlayStyle.
		Width(innerWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

func (f *RigSettingsForm) renderInput(index int, width int) string {
	input := f.inputs[index]
	style := formInputStyle
	if f.focusIndex == index {
		style = formInputFocusedStyle
	}
	return style.Width(width - 4).Render(input.View())
}

func (f *RigSettingsForm) renderToggle(label string, checked bool, focused bool) string {
	checkbox := "[ ]"
	if checked {
		checkbox = "[✓]"
	}

	style := formLabelStyle
	if focused {
		style = formLabelFocusedStyle
		checkbox = lipgloss.NewStyle().Foreground(highlight).Render(checkbox)
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, checkbox, " ", style.Render(label))
}

// Settings-specific styles
var settingsSectionStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(subtle).
	MarginTop(1).
	MarginBottom(1)
