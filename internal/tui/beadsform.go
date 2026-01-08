package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BeadsFormField represents the current field being edited
type BeadsFormField int

const (
	BeadsFieldTitle BeadsFormField = iota
	BeadsFieldDescription
	BeadsFieldType
	BeadsFieldPriority
)

// BeadsFormMode is either create or edit mode
type BeadsFormMode int

const (
	BeadsModeCreate BeadsFormMode = iota
	BeadsModeEdit
)

// BeadsForm manages the create/edit beads form state
type BeadsForm struct {
	mode      BeadsFormMode
	editID    string       // ID of bead being edited (for edit mode)
	field     BeadsFormField

	// Inputs
	titleInput       textinput.Model
	descriptionInput textinput.Model

	// Selections
	issueType IssueType
	priority  int // 0-4

	submitted bool
	cancelled bool

	// Pending data for town-level confirmation (stored while showing dialog)
	pendingID          string
	pendingTitle       string
	pendingDescription string
	pendingType        string
	pendingPriority    int
}

// NewBeadsFormCreate creates a new form for creating a bead
func NewBeadsFormCreate() *BeadsForm {
	titleInput := textinput.New()
	titleInput.Placeholder = "Enter issue title..."
	titleInput.Focus()
	titleInput.CharLimit = 128
	titleInput.Width = 50
	titleInput.Prompt = ""

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "Enter description (optional)..."
	descriptionInput.CharLimit = 500
	descriptionInput.Width = 50
	descriptionInput.Prompt = ""

	return &BeadsForm{
		mode:             BeadsModeCreate,
		field:            BeadsFieldTitle,
		titleInput:       titleInput,
		descriptionInput: descriptionInput,
		issueType:        IssueTypeTask,
		priority:         2, // Default to P2 (medium)
	}
}

// NewBeadsFormEdit creates a new form for editing an existing bead
func NewBeadsFormEdit(id, title, description, issueType string, priority int) *BeadsForm {
	form := NewBeadsFormCreate()
	form.mode = BeadsModeEdit
	form.editID = id

	form.titleInput.SetValue(title)
	form.titleInput.Blur()

	form.descriptionInput.SetValue(description)
	form.descriptionInput.Blur()

	// Parse issue type
	switch issueType {
	case "bug":
		form.issueType = IssueTypeBug
	case "feature":
		form.issueType = IssueTypeFeature
	default:
		form.issueType = IssueTypeTask
	}

	// Validate priority range
	if priority < 0 || priority > 4 {
		priority = 2
	}
	form.priority = priority

	form.field = BeadsFieldTitle
	form.titleInput.Focus()

	return form
}

// Mode returns the form mode (create or edit)
func (f *BeadsForm) Mode() BeadsFormMode {
	return f.mode
}

// EditID returns the ID of the bead being edited
func (f *BeadsForm) EditID() string {
	return f.editID
}

// Title returns the issue title
func (f *BeadsForm) Title() string {
	return strings.TrimSpace(f.titleInput.Value())
}

// Description returns the issue description
func (f *BeadsForm) Description() string {
	return strings.TrimSpace(f.descriptionInput.Value())
}

// Type returns the issue type
func (f *BeadsForm) Type() IssueType {
	return f.issueType
}

// Priority returns the priority (0-4)
func (f *BeadsForm) Priority() int {
	return f.priority
}

// IsValid returns true if required fields are filled
func (f *BeadsForm) IsValid() bool {
	return f.Title() != ""
}

// IsSubmitted returns true if the form was submitted
func (f *BeadsForm) IsSubmitted() bool {
	return f.submitted
}

// IsCancelled returns true if the form was cancelled
func (f *BeadsForm) IsCancelled() bool {
	return f.cancelled
}

// Update handles input events for the form
func (f *BeadsForm) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		f.cancelled = true
		return nil

	case "enter":
		if f.IsValid() {
			f.submitted = true
		}
		return nil

	case "tab", "down":
		return f.nextField()

	case "shift+tab", "up":
		return f.prevField()

	case "left", "h":
		if f.field == BeadsFieldType {
			f.cycleType(-1)
		} else if f.field == BeadsFieldPriority {
			f.cyclePriority(-1)
		}
		return nil

	case "right", "l":
		if f.field == BeadsFieldType {
			f.cycleType(1)
		} else if f.field == BeadsFieldPriority {
			f.cyclePriority(1)
		}
		return nil

	case "1", "2", "3", "4", "5":
		if f.field == BeadsFieldPriority {
			f.priority = int(msg.String()[0] - '1')
			return nil
		}
	}

	// Handle text input
	switch f.field {
	case BeadsFieldTitle:
		var cmd tea.Cmd
		f.titleInput, cmd = f.titleInput.Update(msg)
		return cmd
	case BeadsFieldDescription:
		var cmd tea.Cmd
		f.descriptionInput, cmd = f.descriptionInput.Update(msg)
		return cmd
	}

	return nil
}

func (f *BeadsForm) nextField() tea.Cmd {
	f.blurAll()
	f.field = (f.field + 1) % 4
	return f.focusCurrent()
}

func (f *BeadsForm) prevField() tea.Cmd {
	f.blurAll()
	f.field = (f.field - 1 + 4) % 4
	return f.focusCurrent()
}

func (f *BeadsForm) blurAll() {
	f.titleInput.Blur()
	f.descriptionInput.Blur()
}

func (f *BeadsForm) focusCurrent() tea.Cmd {
	switch f.field {
	case BeadsFieldTitle:
		return f.titleInput.Focus()
	case BeadsFieldDescription:
		return f.descriptionInput.Focus()
	}
	return nil
}

func (f *BeadsForm) cycleType(dir int) {
	types := []IssueType{IssueTypeTask, IssueTypeBug, IssueTypeFeature}
	for i, t := range types {
		if t == f.issueType {
			f.issueType = types[(i+dir+len(types))%len(types)]
			return
		}
	}
}

func (f *BeadsForm) cyclePriority(dir int) {
	f.priority = (f.priority + dir + 5) % 5
}

// View renders the form as an overlay
func (f *BeadsForm) View(width, height int) string {
	overlayWidth := 65
	overlayHeight := 22
	if overlayWidth > width-4 {
		overlayWidth = width - 4
	}
	if overlayHeight > height-4 {
		overlayHeight = height - 4
	}

	innerWidth := overlayWidth - 4

	var titleText string
	if f.mode == BeadsModeEdit {
		titleText = fmt.Sprintf("Edit Bead: %s", f.editID)
	} else {
		titleText = "Create New Bead"
	}

	title := formTitleStyle.Render(titleText)

	// Title field
	titleLabel := "Title"
	if f.field == BeadsFieldTitle {
		titleLabel = formLabelFocusedStyle.Render(titleLabel)
	} else {
		titleLabel = formLabelStyle.Render(titleLabel)
	}
	titleInputStyle := formInputStyle
	if f.field == BeadsFieldTitle {
		titleInputStyle = formInputFocusedStyle
	}
	titleField := titleInputStyle.Width(innerWidth - 4).Render(f.titleInput.View())

	// Description field
	descLabel := "Description (optional)"
	if f.field == BeadsFieldDescription {
		descLabel = formLabelFocusedStyle.Render(descLabel)
	} else {
		descLabel = formLabelStyle.Render(descLabel)
	}
	descInputStyle := formInputStyle
	if f.field == BeadsFieldDescription {
		descInputStyle = formInputFocusedStyle
	}
	descField := descInputStyle.Width(innerWidth - 4).Render(f.descriptionInput.View())

	// Type selector
	typeLabel := "Type (h/l to change, or Tab)"
	if f.field == BeadsFieldType {
		typeLabel = formLabelFocusedStyle.Render(typeLabel)
	} else {
		typeLabel = formLabelStyle.Render(typeLabel)
	}
	typeOptions := f.renderTypeSelector()

	// Priority selector
	priorityLabel := "Priority (h/l or 1-5 to change, or Tab)"
	if f.field == BeadsFieldPriority {
		priorityLabel = formLabelFocusedStyle.Render(priorityLabel)
	} else {
		priorityLabel = formLabelStyle.Render(priorityLabel)
	}
	priorityOptions := f.renderPrioritySelector()

	// Validation message
	var validationMsg string
	if !f.IsValid() && f.Title() == "" && f.descriptionInput.Value() != "" {
		validationMsg = formErrorStyle.Render("Title is required")
	}

	// Help text
	help := mutedStyle.Render("Tab: next field | Enter: save | Esc: cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title, "",
		titleLabel, titleField, "",
		descLabel, descField, "",
		typeLabel, typeOptions, "",
		priorityLabel, priorityOptions, "",
		validationMsg, "",
		help,
	)

	overlay := formOverlayStyle.
		Width(innerWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

func (f *BeadsForm) renderTypeSelector() string {
	types := []IssueType{IssueTypeTask, IssueTypeBug, IssueTypeFeature}
	var parts []string

	for _, t := range types {
		label := string(t)
		if t == f.issueType {
			parts = append(parts, formLabelFocusedStyle.Render("["+label+"]"))
		} else {
			parts = append(parts, mutedStyle.Render(" "+label+" "))
		}
	}

	return strings.Join(parts, "  ")
}

func (f *BeadsForm) renderPrioritySelector() string {
	var parts []string

	for i := 0; i <= 4; i++ {
		label := fmt.Sprintf("P%d", i)
		if i == f.priority {
			parts = append(parts, formLabelFocusedStyle.Render("["+label+"]"))
		} else {
			parts = append(parts, mutedStyle.Render(" "+label+" "))
		}
	}

	return strings.Join(parts, " ")
}
