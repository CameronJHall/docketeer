package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// contextKeyMap implements help.KeyMap with a flat slice of bindings.
// ShortHelp returns the primary (short) bindings; FullHelp groups them into
// one column alongside the global static bindings.
type contextKeyMap struct {
	bindings []key.Binding
	static   KeyMap
}

func (c contextKeyMap) ShortHelp() []key.Binding { return c.bindings }
func (c contextKeyMap) FullHelp() [][]key.Binding {
	if len(c.bindings) == 0 {
		return c.static.FullHelp()
	}
	// Column 0: context-specific primary bindings
	// Remaining columns: full static map
	cols := [][]key.Binding{c.bindings}
	cols = append(cols, c.static.FullHelp()...)
	return cols
}

// HelpBar wraps the bubbles help component with docketeer's key map.
type HelpBar struct {
	help       help.Model
	keys       KeyMap
	contextMap *contextKeyMap
	width      int
	status     string // ephemeral status message
}

// NewHelpBar creates a new HelpBar with default key map.
func NewHelpBar(width int) HelpBar {
	h := help.New()
	h.Styles = help.DefaultDarkStyles()
	return HelpBar{
		help:  h,
		keys:  DefaultKeyMap(),
		width: width,
	}
}

// SetWidth updates the available width.
func (b *HelpBar) SetWidth(width int) {
	b.width = width
	b.help.SetWidth(width)
}

// SetStatus sets an ephemeral status message shown above the key hints.
func (b *HelpBar) SetStatus(msg string) {
	b.status = msg
}

// ClearStatus clears any ephemeral status message.
func (b *HelpBar) ClearStatus() {
	b.status = ""
}

// SetContextKeys sets a dynamic set of key bindings for the current context.
// Pass nil to fall back to the default key map.
func (b *HelpBar) SetContextKeys(bindings []key.Binding) {
	if bindings == nil {
		b.contextMap = nil
		return
	}
	b.contextMap = &contextKeyMap{bindings: bindings, static: b.keys}
}

// ToggleShowAll flips between short and full help view.
func (b *HelpBar) ToggleShowAll() {
	b.help.ShowAll = !b.help.ShowAll
}

// ShowAll returns whether the full help is currently shown.
func (b *HelpBar) ShowAll() bool {
	return b.help.ShowAll
}

// FullHelpHeight returns the number of lines the full help view will occupy
// (border + tallest column). Used by app.go to size the body correctly.
func (b *HelpBar) FullHelpHeight() int {
	var km help.KeyMap = b.keys
	if b.contextMap != nil {
		km = b.contextMap
	}
	groups := km.FullHelp()
	max := 0
	for _, g := range groups {
		enabled := 0
		for _, binding := range g {
			if binding.Enabled() {
				enabled++
			}
		}
		if enabled > max {
			max = enabled
		}
	}
	if max == 0 {
		return 2 // border + 1 empty line
	}
	return max + 1 // +1 for top border
}

// View renders the help bar.
func (b *HelpBar) View() string {
	var km help.KeyMap = b.keys
	if b.contextMap != nil {
		km = b.contextMap
	}
	helpStr := b.help.View(km)

	bar := lipgloss.NewStyle().
		Width(b.width).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorDivider)

	if b.status != "" {
		statusLine := StyleStatusMsg.Render(b.status)
		return bar.Render(statusLine + "\n" + helpStr)
	}
	return bar.Render(helpStr)
}
