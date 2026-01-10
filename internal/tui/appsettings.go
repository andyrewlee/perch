package tui

import (
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AppSettingsForm manages the app settings form state
type AppSettingsForm struct {
	inputs         []textinput.Model
	toggles        []bool // For boolean options
	focusIndex     int
	submitted      bool
	cancelled      bool
	validationErr  string
	currentInterval time.Duration
}

// Input field indices
const (
	appSettingsInputRefresh = iota
	appSettingsFieldCount // Number of text inputs
)

// Toggle indices (after text inputs)
const (
	appSettingsToggleAutoRefresh = iota
	appSettingsToggleCount
)

// NewAppSettingsForm creates a new app settings form with loaded values
func NewAppSettingsForm(refreshInterval time.Duration, autoRefresh bool) *AppSettingsForm {
	inputs := make([]textinput.Model, appSettingsFieldCount)

	// Refresh interval field (in seconds)
	inputs[appSettingsInputRefresh] = textinput.New()
	inputs[appSettingsInputRefresh].Placeholder = "5"
	inputs[appSettingsInputRefresh].CharLimit = 8
	inputs[appSettingsInputRefresh].Width = 30
	inputs[appSettingsInputRefresh].Prompt = ""
	if refreshInterval > 0 {
		inputs[appSettingsInputRefresh].SetValue(strconv.Itoa(int(refreshInterval.Seconds())))
	}
	inputs[appSettingsInputRefresh].Focus()

	// Initialize toggles from settings
	toggles := make([]bool, appSettingsToggleCount)
	toggles[appSettingsToggleAutoRefresh] = autoRefresh

	return &AppSettingsForm{
		inputs:         inputs,
		toggles:        toggles,
		focusIndex:     0,
		currentInterval: refreshInterval,
	}
}

// RefreshInterval returns the entered refresh interval as duration
func (f *AppSettingsForm) RefreshInterval() time.Duration {
	val := f.inputs[appSettingsInputRefresh].Value()
	if val == "" {
		return 5 * time.Second // default
	}
	n, _ := strconv.Atoi(val)
	if n <= 0 {
		return 5 * time.Second
	}
	return time.Duration(n) * time.Second
}

// AutoRefresh returns whether auto-refresh is enabled
func (f *AppSettingsForm) AutoRefresh() bool {
	return f.toggles[appSettingsToggleAutoRefresh]
}

// IsValid returns true if required fields are filled and valid
func (f *AppSettingsForm) IsValid() bool {
	// Refresh interval must be a valid positive integer
	refreshStr := f.inputs[appSettingsInputRefresh].Value()
	if refreshStr != "" {
		n, err := strconv.Atoi(refreshStr)
		if err != nil || n < 1 || n > 300 {
			f.validationErr = "Refresh interval must be between 1 and 300 seconds"
			return false
		}
	}

	f.validationErr = ""
	return true
}

// ValidationError returns the current validation error
func (f *AppSettingsForm) ValidationError() string {
	return f.validationErr
}

// IsSubmitted returns true if the form was submitted
func (f *AppSettingsForm) IsSubmitted() bool {
	return f.submitted
}

// IsCancelled returns true if the form was cancelled
func (f *AppSettingsForm) IsCancelled() bool {
	return f.cancelled
}

// totalFields returns total navigable fields (inputs + toggles)
func (f *AppSettingsForm) totalFields() int {
	return appSettingsFieldCount + appSettingsToggleCount
}

// isOnToggle returns true if focus is on a toggle field
func (f *AppSettingsForm) isOnToggle() bool {
	return f.focusIndex >= appSettingsFieldCount
}

// toggleIndex returns the toggle index when focus is on a toggle
func (f *AppSettingsForm) toggleIndex() int {
	return f.focusIndex - appSettingsFieldCount
}

// Update handles input events for the form
func (f *AppSettingsForm) Update(msg tea.KeyMsg) tea.Cmd {
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

func (f *AppSettingsForm) nextInput() tea.Cmd {
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

func (f *AppSettingsForm) prevInput() tea.Cmd {
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

func (f *AppSettingsForm) updateInputs(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	f.inputs[f.focusIndex], cmd = f.inputs[f.focusIndex].Update(msg)
	return cmd
}

// View renders the form as an overlay
func (f *AppSettingsForm) View(width, height int) string {
	// Calculate overlay dimensions
	overlayWidth := 60
	overlayHeight := 20
	if overlayWidth > width-4 {
		overlayWidth = width - 4
	}
	if overlayHeight > height-4 {
		overlayHeight = height - 4
	}

	innerWidth := overlayWidth - 4

	// Title
	title := formTitleStyle.Render("App Settings")

	// Field labels and inputs
	var fields []string

	// Refresh interval field
	refreshLabel := "Refresh Interval (seconds)"
	if f.focusIndex == appSettingsInputRefresh {
		refreshLabel = formLabelFocusedStyle.Render(refreshLabel)
	} else {
		refreshLabel = formLabelStyle.Render(refreshLabel)
	}
	fields = append(fields, refreshLabel)
	fields = append(fields, f.renderInput(appSettingsInputRefresh, innerWidth))
	fields = append(fields, "")

	// Auto refresh toggle
	autoRefreshLabel := "Auto-refresh enabled"
	autoRefreshFocus := f.focusIndex == appSettingsFieldCount+appSettingsToggleAutoRefresh
	fields = append(fields, f.renderToggle(autoRefreshLabel, f.toggles[appSettingsToggleAutoRefresh], autoRefreshFocus))
	fields = append(fields, "")

	// Current interval display
	if f.currentInterval > 0 {
		currentVal := strconv.Itoa(int(f.currentInterval.Seconds()))
		fields = append(fields, mutedStyle.Render("Current: "+currentVal+"s"))
	}
	fields = append(fields, "")

	// Validation message
	f.IsValid() // Update validation error
	if f.validationErr != "" {
		fields = append(fields, formErrorStyle.Render(f.validationErr))
	} else {
		fields = append(fields, "")
	}

	// Help text
	help := mutedStyle.Render("Tab: next | Space: toggle | Enter: save | Esc: cancel")
	fields = append(fields, help)

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", lipgloss.JoinVertical(lipgloss.Left, fields...))

	overlay := formOverlayStyle.
		Width(innerWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

func (f *AppSettingsForm) renderInput(index int, width int) string {
	input := f.inputs[index]
	style := formInputStyle
	if f.focusIndex == index {
		style = formInputFocusedStyle
	}
	return style.Width(width - 4).Render(input.View())
}

func (f *AppSettingsForm) renderToggle(label string, checked bool, focused bool) string {
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
