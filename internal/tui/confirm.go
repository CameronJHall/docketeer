package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/CameronJHall/docketeer/internal/task"
)

// ConfirmDeleteMsg is sent when the user confirms (Confirmed==true) or cancels deletion.
type ConfirmDeleteMsg struct {
	Confirmed bool
	Item      task.Item
}

// ConfirmView is a minimal y/n overlay for delete confirmation.
// It implements tea.Model.
type ConfirmView struct {
	item   task.Item
	width  int
	height int
}

// NewConfirmView creates a confirm view for the given item.
func NewConfirmView(item task.Item, width, height int) ConfirmView {
	return ConfirmView{item: item, width: width, height: height}
}

// Init satisfies tea.Model.
func (c *ConfirmView) Init() tea.Cmd { return nil }

// Update handles y/n/esc key presses.
func (c *ConfirmView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "y", "Y":
			return c, func() tea.Msg { return ConfirmDeleteMsg{Confirmed: true, Item: c.item} }
		case "n", "N", "esc", "q":
			return c, func() tea.Msg { return ConfirmDeleteMsg{Confirmed: false} }
		case "ctrl+c":
			return c, tea.Quit
		}
	}
	return c, nil
}

// View renders the confirmation overlay centred on screen.
func (c *ConfirmView) View() tea.View {
	kind := "task"
	if c.item.IsIdea() {
		kind = "idea"
	}

	title := truncate(c.item.Title, 50)

	line1 := fmt.Sprintf("Delete %s: %q?", kind, title)
	line2 := StyleMuted.Render("y") + " yes   " + StyleMuted.Render("n/esc") + " cancel"

	box := StyleConfirmBox.Render(line1 + "\n\n" + line2)

	v := tea.NewView(centerInScreen(box, c.width, c.height))
	v.AltScreen = true
	return v
}
