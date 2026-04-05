package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/CameronJHall/docketeer/internal/task"
)

// ListView manages the left panel: grouped task list + ideas section.
type ListView struct {
	groups      []task.ItemGroup
	cols        ColumnFlags
	cursor      int // flat index into all visible items
	offset      int // scroll offset (top item index)
	width       int
	height      int
	sortMode    task.SortMode
	filterText  string // committed filter text (empty = all)
	filterInput string // rendered textinput view when filter is active (empty = inactive)
}

// NewListView creates a new ListView.
func NewListView(width, height int) ListView {
	return ListView{width: width, height: height}
}

// SetFilterText sets the committed filter text for display.
func (l *ListView) SetFilterText(text string) {
	l.filterText = text
}

// SetFilterInput sets the rendered textinput view for display when the filter
// input is active. Pass empty string when the input is not active.
func (l *ListView) SetFilterInput(view string) {
	l.filterInput = view
}

// SetItems updates the item list and re-groups/sorts them.
func (l *ListView) SetItems(items []task.Item, mode task.SortMode) {
	l.sortMode = mode
	l.groups = task.GroupItems(items, mode)
	l.cols = ColumnFlagsFor(l.groups)
	total := l.TotalItems()
	if l.cursor >= total && total > 0 {
		l.cursor = total - 1
	}
	l.clampOffset()
}

// SetSize updates the panel dimensions.
func (l *ListView) SetSize(width, height int) {
	l.width = width
	l.height = height
	l.clampOffset()
}

// MoveUp moves the cursor up by one.
func (l *ListView) MoveUp() {
	if l.cursor > 0 {
		l.cursor--
		l.clampOffset()
	}
}

// MoveDown moves the cursor down by one.
func (l *ListView) MoveDown() {
	if l.cursor < l.TotalItems()-1 {
		l.cursor++
		l.clampOffset()
	}
}

// TotalItems returns the total number of visible items across all groups.
func (l *ListView) TotalItems() int {
	n := 0
	for _, g := range l.groups {
		n += len(g.Items)
	}
	return n
}

// SelectedItem returns the currently selected item, or nil if empty.
func (l *ListView) SelectedItem() *task.Item {
	idx := 0
	for _, g := range l.groups {
		for i := range g.Items {
			if idx == l.cursor {
				return new(g.Items[i])
			}
			idx++
		}
	}
	return nil
}

// SetCursorByID moves the cursor to the item with the given ID.
func (l *ListView) SetCursorByID(id int64) {
	idx := 0
	for _, g := range l.groups {
		for _, item := range g.Items {
			if item.ID == id {
				l.cursor = idx
				l.clampOffset()
				return
			}
			idx++
		}
	}
}

// View renders the list panel.
func (l *ListView) View() string {
	if l.width <= 0 || l.height <= 0 {
		return ""
	}

	var lines []string

	// Sort indicator
	if l.sortMode != task.SortByPriority {
		indicator := StyleMuted.Render(fmt.Sprintf("sorted by: %s", l.sortMode.String()))
		lines = append(lines, indicator)
	}

	// Filter indicator
	if l.filterInput != "" {
		// Active filter input — show the rendered textinput.
		lines = append(lines, l.filterInput)
	} else if l.filterText != "" {
		// Committed filter — show static indicator.
		indicator := StyleMuted.Render("filter: ") + StyleAccent.Render(l.filterText) + StyleMuted.Render("  (esc clear)")
		lines = append(lines, indicator)
	}

	if l.TotalItems() == 0 {
		empty := StyleMuted.Render("no items — press n to create a task")
		lines = append(lines, "", empty)
	} else {
		flatIdx := 0
		for gi, g := range l.groups {
			// Group header — first group gets column labels right-aligned.
			headerLeft := StyleGroupHeader.Render(fmt.Sprintf("%s (%d)", g.Label(), len(g.Items)))
			if gi == 0 {
				if colLabels := l.cols.ColumnLabels(l.width - 1); colLabels != "" {
					leftW := lipgloss.Width(headerLeft)
					colW := lipgloss.Width(colLabels)
					gap := l.width - 1 - leftW - colW
					if gap < 1 {
						gap = 1
					}
					headerLeft = headerLeft + strings.Repeat(" ", gap) + colLabels
				}
			}
			lines = append(lines, headerLeft)
			hr := StyleMuted.Render(strings.Repeat("─", l.width))
			lines = append(lines, hr)

			for _, item := range g.Items {
				selected := flatIdx == l.cursor
				line := RenderTaskLine(item, l.width-1, selected, l.cols)
				lines = append(lines, line)
				flatIdx++
			}
			lines = append(lines, "") // spacer between groups
		}
	}

	// Apply scroll window
	visible := l.applyScroll(lines)

	// Pad to height
	for len(visible) < l.height {
		visible = append(visible, "")
	}

	style := lipgloss.NewStyle().Width(l.width).Height(l.height)
	return style.Render(strings.Join(visible, "\n"))
}

// clampOffset ensures the cursor is within the visible scroll window.
func (l *ListView) clampOffset() {
	if l.height <= 0 {
		return
	}
	// Count rendered lines up to cursor to find its line position.
	// Simple approach: keep offset so cursor is visible.
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+l.height {
		l.offset = l.cursor - l.height + 1
	}
}

// applyScroll returns a window of lines starting at the appropriate offset.
func (l *ListView) applyScroll(lines []string) []string {
	if len(lines) <= l.height {
		return lines
	}
	// Find which line the cursor item appears on.
	// Rather than re-computing, just scroll to keep the cursor visible.
	// We use a simplified approach: treat each line independently.
	start := l.offset
	if start >= len(lines) {
		start = 0
	}
	end := start + l.height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[start:end]
}
