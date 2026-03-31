package tui

import "charm.land/bubbles/v2/key"

// KeyMap holds all key bindings for docketeer.
type KeyMap struct {
	// Navigation
	Up   key.Binding
	Down key.Binding

	// Actions
	Create     key.Binding
	CreateIdea key.Binding
	Edit       key.Binding
	Delete     key.Binding
	Advance    key.Binding
	Reverse    key.Binding
	AddNote    key.Binding
	Promote    key.Binding
	Revive     key.Binding
	Reload     key.Binding

	// View controls
	SortCycle     key.Binding
	ToggleMetrics key.Binding
	Filter        key.Binding

	// Universal
	Help key.Binding
	Quit key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "down"),
		),
		Create: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new task"),
		),
		CreateIdea: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "new idea"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Advance: key.NewBinding(
			key.WithKeys(">", "."),
			key.WithHelp(">", "begin"),
		),
		Reverse: key.NewBinding(
			key.WithKeys("<", ","),
			key.WithHelp("<", "reopen"),
		),
		AddNote: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add note"),
		),
		Promote: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "promote"),
		),
		Revive: key.NewBinding(
			key.WithKeys("!"),
			key.WithHelp("!", "revive"),
		),
		Reload: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "reload"),
		),
		SortCycle: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		ToggleMetrics: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "metrics"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
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

// ShortHelp returns the minimal help shown in the footer by default.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Help, k.Quit}
}

// FullHelp returns all bindings grouped into columns, shown when ? is pressed.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Create, k.CreateIdea, k.Edit, k.Delete, k.AddNote},
		{k.Advance, k.Reverse, k.Promote, k.Revive, k.Reload},
		{k.SortCycle, k.ToggleMetrics, k.Filter, k.Help, k.Quit},
	}
}
