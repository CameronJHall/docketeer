package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/CameronJHall/docketeer/internal/task"
)

const (
	cursorIndicator   = "▶"
	cursorPlaceholder = " "
	ideaIcon          = "✦"
	ellipsis          = "…"
)

// Fixed column widths (visual characters, not bytes).
const (
	colCursor  = 2  // "▶ " or "  "
	colIcon    = 2  // "✦ " for ideas (tasks have no icon, same width as prio)
	colPrio    = 3  // "p1 " … "p4 "
	colStale   = 5  // "  9d" … " 99d" — always 3 visible + 2 leading spaces, or 5 spaces
	colDue     = 8  // "  Jan 02" — always 6 visible + 2 leading spaces, or 8 spaces
	colGap     = 2  // gap before each right-side column
	colProjMax = 10 // max project badge chars (+ colGap leading = 12)
)

// ColumnFlags controls which optional columns are rendered in task lines.
// A column is hidden when no item in the visible list uses it.
type ColumnFlags struct {
	ShowProject   bool
	ShowDueDate   bool
	ShowStaleness bool
	// ProjWidth is the widest project name in the list (capped at colProjMax).
	ProjWidth int
}

// ColumnFlagsFor scans items and returns flags for which columns to show.
func ColumnFlagsFor(groups []task.ItemGroup) ColumnFlags {
	var f ColumnFlags
	for _, g := range groups {
		for _, item := range g.Items {
			if item.Project != "" {
				f.ShowProject = true
				if w := len([]rune(item.Project)); w > f.ProjWidth {
					f.ProjWidth = w
				}
			}
			if item.IsTask() && item.DueDate != nil {
				f.ShowDueDate = true
			}
			if item.IsTask() && item.DecayLevel() > task.DecayNone {
				f.ShowStaleness = true
			}
		}
	}
	if f.ProjWidth > colProjMax {
		f.ProjWidth = colProjMax
	}
	return f
}

// rightColsWidth returns the total fixed width consumed by the right-side columns.
func (c ColumnFlags) rightColsWidth() int {
	w := 0
	if c.ShowProject {
		w += c.ProjWidth + colGap
	}
	if c.ShowDueDate {
		w += colDue
	}
	if c.ShowStaleness {
		w += colStale
	}
	return w
}

// ColumnLabels returns right-aligned muted column header labels matching the
// layout of RenderTaskLine. width is the total available line width (same value
// passed to RenderTaskLine). The returned string is meant to be placed on the
// same line as the group header text.
func (c ColumnFlags) ColumnLabels(width int) string {
	if !c.ShowProject && !c.ShowDueDate && !c.ShowStaleness {
		return ""
	}
	var parts string
	if c.ShowProject {
		label := padRight("project", c.ProjWidth)
		parts += "  " + StyleDetailMeta.Render(label)
	}
	if c.ShowDueDate {
		parts += "  " + StyleDetailMeta.Render("   due")
	}
	if c.ShowStaleness {
		parts += "  " + StyleDetailMeta.Render("age")
	}
	return parts
}

// RenderTaskLine renders a single task item row within the list panel.
// width is the available width for the whole line.
// selected indicates whether this line has the cursor.
// cols controls which optional columns are visible.
func RenderTaskLine(item task.Item, width int, selected bool, cols ColumnFlags) string {
	// --- Left fixed columns ---

	// Cursor (2 chars)
	var cursorStr string
	if selected {
		cursorStr = fg(colorAccent).Render(cursorIndicator) + " "
	} else {
		cursorStr = "  "
	}

	// Icon / priority (3 chars for tasks, 2+1 for ideas)
	var iconStr string
	if item.IsIdea() {
		iconStr = StyleIdea.Render(ideaIcon) + " "
	} else if item.Priority != nil {
		label := (*item.Priority).Label()
		iconStr = fg(PriorityColor(*item.Priority)).Render(label) + " "
	} else {
		iconStr = "    " // 3 chars + space
	}

	leftFixed := colCursor + colPrio // 2 + 3 = 5

	// --- Right fixed columns ---

	// Staleness badge — fixed width colStale (5 chars: "  9d" style)
	var staleStr string
	if cols.ShowStaleness && item.IsTask() {
		decay := item.DecayLevel()
		if decay > task.DecayNone {
			days := item.StaleDays()
			// Format to exactly 3 visible chars: right-justify e.g. " 9d", "34d"
			raw := fmt.Sprintf("%3s", fmt.Sprintf("%dd", days))
			staleStr = "  " + fg(DecayColor(decay)).Render(raw)
		} else {
			staleStr = "     " // 2 leading + 3 badge
		}
	}

	// Due date (8 chars, or empty)
	var dueStr string
	if cols.ShowDueDate && item.IsTask() {
		if item.DueDate != nil {
			dateStr := item.DueDate.Format("Jan 02")
			dueStyle := StyleMuted
			if item.IsOverdue() {
				dueStyle = StyleOverdue
			}
			dueStr = "  " + dueStyle.Render(dateStr)
		} else {
			dueStr = "        " // 2 leading + 6 date
		}
	}

	// Project badge (ProjWidth chars right-padded to uniform width, or empty)
	var projStr string
	if cols.ShowProject {
		proj := truncate(item.Project, cols.ProjWidth)
		if proj != "" {
			padded := padRight(proj, cols.ProjWidth)
			projStr = "  " + fg(ProjectColor(item.Project)).Render(padded)
		} else {
			projStr = "  " + fmt.Sprintf("%*s", cols.ProjWidth, "")
		}
	}

	// --- Title fills remaining space ---
	titleWidth := width - leftFixed - cols.rightColsWidth()
	if titleWidth < 4 {
		titleWidth = 4
	}
	title := truncate(item.Title, titleWidth)

	var titleStyle lipgloss.Style
	if item.IsTask() {
		switch item.DecayLevel() {
		case task.DecayWarning, task.DecayAlert:
			titleStyle = fg(colorDecayDim)
		default:
			titleStyle = lipgloss.NewStyle()
		}
	} else {
		titleStyle = StyleIdea
	}
	titleStr := titleStyle.Render(padRight(title, titleWidth))

	line := cursorStr + iconStr + titleStr + projStr + dueStr + staleStr

	if selected {
		// PlaceHorizontal pads to `width` using the selection background on
		// whitespace only — it does not re-render the existing ANSI sequences,
		// avoiding color bleed from wrapping already-colored strings.
		selBg := lipgloss.NewStyle().Background(lipgloss.Color("237"))
		line = lipgloss.PlaceHorizontal(width, lipgloss.Left, line,
			lipgloss.WithWhitespaceStyle(selBg))
	}

	return line
}
