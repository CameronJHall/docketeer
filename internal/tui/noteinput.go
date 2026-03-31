package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/CameronJHall/docketeer/internal/task"
)

// NoteInputDoneMsg is sent when the note input overlay is dismissed.
type NoteInputDoneMsg struct {
	Saved   bool
	ItemID  int64
	Content string
}

// noteInputHeight is the number of terminal rows the note section occupies
// inside the detail panel (divider + label + textarea rows + hint).
const noteInputHeight = 7

// NoteInput is an inline textarea anchored to the bottom of the detail panel.
// It implements tea.Model so App can delegate Update calls to it.
type NoteInput struct {
	item  task.Item
	ta    textarea.Model
	width int
}

// NewNoteInput creates a NoteInput for the given item sized to fit inside the
// detail panel at the given content width.
func NewNoteInput(item task.Item, width int) NoteInput {
	ta := textarea.New()
	ta.Placeholder = "Write a note…"
	ta.ShowLineNumbers = false
	ta.Prompt = " |"
	ta.DynamicHeight = false
	ta.SetHeight(3)
	ta.SetWidth(width - 4)

	taStyles := ta.Styles()
	taStyles.Blurred.Prompt = fg(colorMuted)
	taStyles.Focused.Prompt = fg(colorAccent)
	ta.SetStyles(taStyles)

	_ = ta.Focus()

	return NoteInput{item: item, ta: ta, width: width}
}

// SetWidth resizes the textarea to match a new content width.
func (n *NoteInput) SetWidth(width int) {
	n.width = width
	n.ta.SetWidth(width - 4)
}

// Init satisfies tea.Model.
func (n *NoteInput) Init() tea.Cmd { return nil }

// Update handles key events for the note input.
func (n *NoteInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return n, tea.Quit

		case "esc":
			return n, func() tea.Msg {
				return NoteInputDoneMsg{Saved: false}
			}

		case "ctrl+s", "ctrl+enter":
			content := strings.TrimSpace(n.ta.Value())
			if content == "" {
				return n, func() tea.Msg {
					return NoteInputDoneMsg{Saved: false}
				}
			}
			return n, func() tea.Msg {
				return NoteInputDoneMsg{Saved: true, ItemID: n.item.ID, Content: content}
			}
		}
	}

	var cmd tea.Cmd
	n.ta, cmd = n.ta.Update(msg)
	return n, cmd
}

// RenderInline renders the note input as a plain string to be embedded at the
// bottom of the detail panel. Width is the available content width.
func (n *NoteInput) RenderInline() string {
	var sb strings.Builder
	sb.WriteString(StyleMuted.Render(strings.Repeat("─", n.width)))
	sb.WriteString("\n")
	sb.WriteString(StyleFormFocusedLabel.Render("add note"))
	sb.WriteString("\n")
	sb.WriteString(n.ta.View())
	sb.WriteString("\n")
	sb.WriteString(StyleMuted.Render("ctrl+s save  •  esc cancel"))
	return sb.String()
}

// View satisfies tea.Model; rendering is done via RenderInline when embedded
// inside the detail panel, so this path is never reached in practice.
func (n *NoteInput) View() tea.View {
	return tea.NewView(n.RenderInline())
}
