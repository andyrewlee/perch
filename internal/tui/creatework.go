package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CreateWorkStep represents the current step in the wizard
type CreateWorkStep int

const (
	StepIssueDetails CreateWorkStep = iota
	StepSelectRig
	StepSelectTarget
	StepConfirm
)

// IssueType represents the type of issue to create
type IssueType string

const (
	IssueTypeTask    IssueType = "task"
	IssueTypeBug     IssueType = "bug"
	IssueTypeFeature IssueType = "feature"
)

// CreateWorkForm manages the create work wizard state
type CreateWorkForm struct {
	step CreateWorkStep

	// Issue details (step 1)
	titleInput textinput.Model
	issueType  IssueType
	priority   int // 0-4

	// Rig selection (step 2)
	rigs        []string
	rigIndex    int
	skipSling   bool

	// Target selection (step 3)
	targets     []string // polecats or "new"
	targetIndex int

	// State
	submitted bool
	cancelled bool
}

// NewCreateWorkForm creates a new create work form
func NewCreateWorkForm(rigs []string) *CreateWorkForm {
	titleInput := textinput.New()
	titleInput.Placeholder = "Enter issue title..."
	titleInput.Focus()
	titleInput.CharLimit = 128
	titleInput.Width = 50
	titleInput.Prompt = ""

	return &CreateWorkForm{
		step:       StepIssueDetails,
		titleInput: titleInput,
		issueType:  IssueTypeTask,
		priority:   2,
		rigs:       rigs,
		rigIndex:   0,
	}
}

// Step returns the current step
func (f *CreateWorkForm) Step() CreateWorkStep {
	return f.step
}

// Title returns the issue title
func (f *CreateWorkForm) Title() string {
	return strings.TrimSpace(f.titleInput.Value())
}

// Type returns the issue type
func (f *CreateWorkForm) Type() IssueType {
	return f.issueType
}

// Priority returns the priority (0-4)
func (f *CreateWorkForm) Priority() int {
	return f.priority
}

// SelectedRig returns the selected rig name
func (f *CreateWorkForm) SelectedRig() string {
	if f.skipSling || len(f.rigs) == 0 || f.rigIndex >= len(f.rigs) {
		return ""
	}
	return f.rigs[f.rigIndex]
}

// SelectedTarget returns the selected polecat name (or "new" for new polecat)
func (f *CreateWorkForm) SelectedTarget() string {
	if f.skipSling || len(f.targets) == 0 || f.targetIndex >= len(f.targets) {
		return ""
	}
	return f.targets[f.targetIndex]
}

// SkipSling returns whether slinging was skipped
func (f *CreateWorkForm) SkipSling() bool {
	return f.skipSling
}

// IsValid returns true if the current step is valid
func (f *CreateWorkForm) IsValid() bool {
	switch f.step {
	case StepIssueDetails:
		return f.Title() != ""
	case StepSelectRig:
		return len(f.rigs) > 0 || f.skipSling
	case StepSelectTarget:
		return len(f.targets) > 0 || f.skipSling
	case StepConfirm:
		return true
	}
	return false
}

// IsSubmitted returns true if the form was submitted
func (f *CreateWorkForm) IsSubmitted() bool {
	return f.submitted
}

// IsCancelled returns true if the form was cancelled
func (f *CreateWorkForm) IsCancelled() bool {
	return f.cancelled
}

// SetTargets sets the available targets for the selected rig
func (f *CreateWorkForm) SetTargets(targets []string) {
	// Always include "new polecat" option
	f.targets = append([]string{"(new polecat)"}, targets...)
	f.targetIndex = 0
}

// Update handles input events for the form
func (f *CreateWorkForm) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		if f.step > StepIssueDetails {
			// Go back a step
			f.step--
			return nil
		}
		f.cancelled = true
		return nil

	case "enter":
		return f.handleEnter()

	case "tab", "down", "j":
		return f.handleDown()

	case "shift+tab", "up", "k":
		return f.handleUp()

	case "left", "h":
		return f.handleLeft()

	case "right", "l":
		return f.handleRight()

	case "s":
		// Skip sling option (only in rig selection)
		if f.step == StepSelectRig {
			f.skipSling = true
			f.step = StepConfirm
		}
		return nil
	}

	// Pass to text input if in issue details step
	if f.step == StepIssueDetails {
		var cmd tea.Cmd
		f.titleInput, cmd = f.titleInput.Update(msg)
		return cmd
	}

	return nil
}

func (f *CreateWorkForm) handleEnter() tea.Cmd {
	if !f.IsValid() {
		return nil
	}

	switch f.step {
	case StepIssueDetails:
		if len(f.rigs) > 0 {
			f.step = StepSelectRig
		} else {
			// No rigs, skip to confirm
			f.skipSling = true
			f.step = StepConfirm
		}
	case StepSelectRig:
		f.step = StepSelectTarget
		// Targets will be set by the model when it detects step change
	case StepSelectTarget:
		f.step = StepConfirm
	case StepConfirm:
		f.submitted = true
	}
	return nil
}

func (f *CreateWorkForm) handleDown() tea.Cmd {
	switch f.step {
	case StepSelectRig:
		if len(f.rigs) > 0 {
			f.rigIndex = (f.rigIndex + 1) % len(f.rigs)
		}
	case StepSelectTarget:
		if len(f.targets) > 0 {
			f.targetIndex = (f.targetIndex + 1) % len(f.targets)
		}
	}
	return nil
}

func (f *CreateWorkForm) handleUp() tea.Cmd {
	switch f.step {
	case StepSelectRig:
		if len(f.rigs) > 0 {
			f.rigIndex = (f.rigIndex - 1 + len(f.rigs)) % len(f.rigs)
		}
	case StepSelectTarget:
		if len(f.targets) > 0 {
			f.targetIndex = (f.targetIndex - 1 + len(f.targets)) % len(f.targets)
		}
	}
	return nil
}

func (f *CreateWorkForm) handleLeft() tea.Cmd {
	switch f.step {
	case StepIssueDetails:
		// Cycle type left
		types := []IssueType{IssueTypeTask, IssueTypeBug, IssueTypeFeature}
		for i, t := range types {
			if t == f.issueType {
				f.issueType = types[(i-1+len(types))%len(types)]
				break
			}
		}
	}
	return nil
}

func (f *CreateWorkForm) handleRight() tea.Cmd {
	switch f.step {
	case StepIssueDetails:
		// Cycle type right
		types := []IssueType{IssueTypeTask, IssueTypeBug, IssueTypeFeature}
		for i, t := range types {
			if t == f.issueType {
				f.issueType = types[(i+1)%len(types)]
				break
			}
		}
	}
	return nil
}

// View renders the form as an overlay
func (f *CreateWorkForm) View(width, height int) string {
	overlayWidth := 65
	overlayHeight := 20
	if overlayWidth > width-4 {
		overlayWidth = width - 4
	}
	if overlayHeight > height-4 {
		overlayHeight = height - 4
	}

	innerWidth := overlayWidth - 4

	var content string
	switch f.step {
	case StepIssueDetails:
		content = f.renderIssueDetails(innerWidth)
	case StepSelectRig:
		content = f.renderRigSelection(innerWidth)
	case StepSelectTarget:
		content = f.renderTargetSelection(innerWidth)
	case StepConfirm:
		content = f.renderConfirmation(innerWidth)
	}

	overlay := formOverlayStyle.
		Width(innerWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

func (f *CreateWorkForm) renderIssueDetails(width int) string {
	title := formTitleStyle.Render("Create Work - Issue Details")

	// Progress indicator
	progress := f.renderProgress()

	// Title field
	titleLabel := formLabelFocusedStyle.Render("Title")
	titleInput := formInputFocusedStyle.Width(width - 4).Render(f.titleInput.View())

	// Type selector
	typeLabel := formLabelStyle.Render("Type (h/l to change)")
	typeOptions := f.renderTypeSelector()

	// Priority selector
	priorityLabel := formLabelStyle.Render("Priority (1-5 to set)")
	priorityOptions := f.renderPrioritySelector()

	help := mutedStyle.Render("Enter: next step | h/l: change type | 1-5: set priority | Esc: cancel")

	return lipgloss.JoinVertical(lipgloss.Left,
		title, progress, "",
		titleLabel, titleInput, "",
		typeLabel, typeOptions, "",
		priorityLabel, priorityOptions, "",
		help,
	)
}

func (f *CreateWorkForm) renderRigSelection(width int) string {
	title := formTitleStyle.Render("Create Work - Select Rig")
	progress := f.renderProgress()

	var lines []string
	lines = append(lines, formLabelStyle.Render("Select a rig to sling work to:"))
	lines = append(lines, "")

	for i, rig := range f.rigs {
		prefix := "  "
		if i == f.rigIndex {
			prefix = "> "
			lines = append(lines, selectedItemStyle.Render(prefix+rig))
		} else {
			lines = append(lines, itemStyle.Render(prefix+rig))
		}
	}

	if len(f.rigs) == 0 {
		lines = append(lines, mutedStyle.Render("  No rigs available"))
	}

	lines = append(lines, "")
	help := mutedStyle.Render("j/k: select | Enter: next | s: skip slinging | Esc: back")
	lines = append(lines, help)

	return lipgloss.JoinVertical(lipgloss.Left,
		title, progress, "",
		lipgloss.JoinVertical(lipgloss.Left, lines...),
	)
}

func (f *CreateWorkForm) renderTargetSelection(width int) string {
	title := formTitleStyle.Render("Create Work - Select Target")
	progress := f.renderProgress()

	rigName := f.SelectedRig()
	subtitle := mutedStyle.Render(fmt.Sprintf("Rig: %s", rigName))

	var lines []string
	lines = append(lines, formLabelStyle.Render("Select a polecat to assign work to:"))
	lines = append(lines, "")

	for i, target := range f.targets {
		prefix := "  "
		if i == f.targetIndex {
			prefix = "> "
			lines = append(lines, selectedItemStyle.Render(prefix+target))
		} else {
			lines = append(lines, itemStyle.Render(prefix+target))
		}
	}

	if len(f.targets) == 0 {
		lines = append(lines, mutedStyle.Render("  No polecats available"))
	}

	lines = append(lines, "")
	help := mutedStyle.Render("j/k: select | Enter: next | Esc: back")
	lines = append(lines, help)

	return lipgloss.JoinVertical(lipgloss.Left,
		title, progress, "",
		subtitle, "",
		lipgloss.JoinVertical(lipgloss.Left, lines...),
	)
}

func (f *CreateWorkForm) renderConfirmation(width int) string {
	title := formTitleStyle.Render("Create Work - Confirm")
	progress := f.renderProgress()

	var lines []string
	lines = append(lines, formLabelStyle.Render("Summary:"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Title:    %s", f.Title()))
	lines = append(lines, fmt.Sprintf("  Type:     %s", f.issueType))
	lines = append(lines, fmt.Sprintf("  Priority: P%d", f.priority))

	if f.skipSling {
		lines = append(lines, "")
		lines = append(lines, mutedStyle.Render("  (Not slinging to a rig)"))
	} else {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  Rig:      %s", f.SelectedRig()))
		target := f.SelectedTarget()
		if target == "(new polecat)" {
			lines = append(lines, "  Target:   Create new polecat")
		} else {
			lines = append(lines, fmt.Sprintf("  Target:   %s", target))
		}
	}

	lines = append(lines, "")
	help := mutedStyle.Render("Enter: create | Esc: back")
	lines = append(lines, help)

	return lipgloss.JoinVertical(lipgloss.Left,
		title, progress, "",
		lipgloss.JoinVertical(lipgloss.Left, lines...),
	)
}

func (f *CreateWorkForm) renderProgress() string {
	steps := []string{"Issue", "Rig", "Target", "Confirm"}
	var parts []string

	for i, s := range steps {
		stepNum := CreateWorkStep(i)
		if stepNum < f.step {
			parts = append(parts, workingStyle.Render("✓ "+s))
		} else if stepNum == f.step {
			parts = append(parts, formLabelFocusedStyle.Render("● "+s))
		} else {
			parts = append(parts, mutedStyle.Render("○ "+s))
		}
	}

	return strings.Join(parts, "  ")
}

func (f *CreateWorkForm) renderTypeSelector() string {
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

func (f *CreateWorkForm) renderPrioritySelector() string {
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
