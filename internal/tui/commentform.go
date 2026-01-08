package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CommentForm manages the add comment form state
type CommentForm struct {
	issueID   string      // ID of bead we're commenting on
	input     textarea.Model
	submitted bool
	cancelled bool
}

// NewCommentForm creates a new form for adding a comment
func NewCommentForm(issueID string) *CommentForm {
	input := textarea.New()
	input.Placeholder = "Enter comment text..."
	input.Focus()
	input.CharLimit = 1000
	input.SetWidth(60)
	input.SetHeight(8)
	input.ShowLineNumbers = false
	input.Prompt = ""

	return &CommentForm{
		issueID: issueID,
		input:   input,
	}
}

// IssueID returns the ID of the bead being commented on
func (f *CommentForm) IssueID() string {
	return f.issueID
}

// Content returns the comment content
func (f *CommentForm) Content() string {
	return f.input.Value()
}

// IsValid returns true if required fields are filled
func (f *CommentForm) IsValid() bool {
	return f.Content() != ""
}

// IsSubmitted returns true if the form was submitted
func (f *CommentForm) IsSubmitted() bool {
	return f.submitted
}

// IsCancelled returns true if the form was cancelled
func (f *CommentForm) IsCancelled() bool {
	return f.cancelled
}

// Update handles input events for the form
func (f *CommentForm) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		f.cancelled = true
		return nil

	case "ctrl+enter", "enter":
		if f.IsValid() {
			f.submitted = true
		}
		return nil
	}

	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	return cmd
}

// View renders the form as an overlay
func (f *CommentForm) View(width, height int) string {
	overlayWidth := 70
	overlayHeight := 20
	if overlayWidth > width-4 {
		overlayWidth = width - 4
	}
	if overlayHeight > height-4 {
		overlayHeight = height - 4
	}

	innerWidth := overlayWidth - 4

	title := formTitleStyle.Render(fmt.Sprintf("Add Comment: %s", f.issueID))

	// Content input label
	contentLabel := formLabelFocusedStyle.Render("Comment (Ctrl+Enter to save, Esc to cancel)")

	// Validation message
	var validationMsg string
	if !f.IsValid() && f.input.Value() == "" {
		validationMsg = "" // Only show error if they tried to submit with empty
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title, "",
		contentLabel, "",
		f.input.View(), "",
		validationMsg, "",
		mutedStyle.Render("Ctrl+Enter: save | Esc: cancel"),
	)

	overlay := formOverlayStyle.
		Width(innerWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}
