package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings for the TUI.
type KeyMap struct {
	// Left moves to the previous panel.
	Left key.Binding
	// Right moves to the next panel.
	Right key.Binding
	// Up moves up within the current panel.
	Up key.Binding
	// Down moves down within the current panel.
	Down key.Binding
	// NextPanel cycles to the next panel (tab).
	NextPanel key.Binding
	// PrevPanel cycles to the previous panel (shift+tab).
	PrevPanel key.Binding

	// Select activates the selected item.
	Select key.Binding

	// Refresh reloads the current view.
	Refresh key.Binding
	// Help toggles the help screen.
	Help key.Binding
	// Quit exits the application.
	Quit key.Binding
}

// DefaultKeyMap returns the default keybindings configured for the TUI.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "panel left"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/→", "panel right"),
		),
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "move down"),
		),
		NextPanel: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		PrevPanel: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("S-tab", "prev panel"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp returns keybindings to show in the mini help view.
// It implements the help.KeyMap interface.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view.
// It implements the help.KeyMap interface.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.NextPanel, k.PrevPanel, k.Select},
		{k.Refresh, k.Help, k.Quit},
	}
}
