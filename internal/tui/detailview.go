package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/CameronJHall/docketeer/internal/task"
)

// DetailView manages the right panel showing full item details.
type DetailView struct {
	viewport  viewport.Model
	item      *task.Item
	notes     []task.Note
	allItems  []task.Item // all items, used for project stats
	noteInput *NoteInput
	width     int
	height    int

	// Pre-computed project stats footer (rendered once in refresh).
	projectStatsStr   string
	projectStatsLines int
}

// NewDetailView creates a new DetailView.
func NewDetailView(width, height int) DetailView {
	d := DetailView{width: width, height: height}
	vp := viewport.New(viewport.WithWidth(d.contentWidth()), viewport.WithHeight(d.viewportHeight()))
	vp.SoftWrap = true
	d.viewport = vp
	return d
}

// contentWidth returns the usable text width inside the panel, accounting for
// the left border (1) and left padding (1) applied by StyleRightPanel.
func (d *DetailView) contentWidth() int {
	w := d.width - 2
	if w < 1 {
		w = 1
	}
	return w
}

// viewportHeight returns the height available for the scrollable viewport,
// shrunk to make room for the fixed footer area: either the note input
// (when active) or the project stats section (mutually exclusive).
func (d *DetailView) viewportHeight() int {
	h := d.height
	if d.noteInput != nil {
		h -= noteInputHeight
	} else if d.projectStatsLines > 0 {
		h -= d.projectStatsLines
	}
	if h < 1 {
		h = 1
	}
	return h
}

// SetSize updates the panel dimensions.
func (d *DetailView) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.viewport.SetWidth(d.contentWidth())
	d.viewport.SetHeight(d.viewportHeight())
	d.refresh()
}

// SetNoteInput attaches (or clears) an inline note input at the bottom of the
// panel. Pass nil to remove it.
func (d *DetailView) SetNoteInput(ni *NoteInput) {
	d.noteInput = ni
	if ni != nil {
		ni.SetWidth(d.contentWidth())
	}
	d.viewport.SetHeight(d.viewportHeight())
	d.refresh()
}

// SetItem updates the displayed item and its notes.
func (d *DetailView) SetItem(item *task.Item, notes []task.Note) {
	d.item = item
	d.notes = notes
	d.viewport.SetYOffset(0)
	d.refresh()
}

// SetAllItems stores a reference to all items for computing project-level stats.
func (d *DetailView) SetAllItems(items []task.Item) {
	d.allItems = items
	d.refresh()
}

// Update passes messages to the viewport (for scrolling).
func (d *DetailView) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return cmd
}

// View renders the detail panel.
func (d *DetailView) View() string {
	style := StyleRightPanel

	inner := d.viewport.View()

	// Append scroll indicator when content overflows the viewport.
	if !d.viewport.AtTop() || !d.viewport.AtBottom() {
		pct := int(d.viewport.ScrollPercent() * 100)
		indicator := StyleMuted.Render(fmt.Sprintf("%d%%", pct))
		inner += "\n" + indicator
	}

	// Fixed footer area: note input takes precedence over project stats.
	if d.noteInput != nil {
		inner += "\n" + d.noteInput.RenderInline()
	} else if d.projectStatsStr != "" {
		inner += "\n" + d.projectStatsStr
	}

	return style.Render(inner)
}

// refresh rebuilds the viewport content from the current item.
func (d *DetailView) refresh() {
	if d.item == nil {
		d.projectStatsStr = ""
		d.projectStatsLines = 0
		d.viewport.SetHeight(d.viewportHeight())
		d.viewport.SetContent(StyleMuted.Render("select an item to preview"))
		return
	}

	// Pre-compute project stats footer so we can size the viewport correctly.
	d.projectStatsStr = ""
	d.projectStatsLines = 0
	if d.item.Project != "" {
		d.buildProjectStatsFooter(d.item.Project)
	}
	d.viewport.SetHeight(d.viewportHeight())

	var sb strings.Builder

	// Title
	sb.WriteString(StyleDetailTitle.Render(d.item.Title))
	sb.WriteString("\n")
	sb.WriteString(StyleMuted.Render(strings.Repeat("─", d.contentWidth())))
	sb.WriteString("\n\n")

	// Metadata
	if d.item.IsTask() {
		if d.item.Status != nil {
			dot := fg(StatusColor(*d.item.Status)).Render(StatusDot(*d.item.Status))
			statusLabel := fg(StatusColor(*d.item.Status)).Render(string(*d.item.Status))
			sb.WriteString(StyleDetailMeta.Render("status  ") + dot + " " + statusLabel + "\n")
		}
		if d.item.Priority != nil {
			pLabel := fg(PriorityColor(*d.item.Priority)).Render(d.item.Priority.Label())
			sb.WriteString(StyleDetailMeta.Render("priority") + " " + pLabel + "\n")
		}
		if d.item.DueDate != nil {
			dateStr := d.item.DueDate.Format("Mon, Jan 02 2006")
			dateStyle := StyleDetailMeta
			if d.item.IsOverdue() {
				dateStyle = StyleOverdue
				dateStr += " (overdue)"
			}
			sb.WriteString(StyleDetailMeta.Render("due     ") + " " + dateStyle.Render(dateStr) + "\n")
		}
	} else {
		sb.WriteString(StyleIdea.Render(ideaIcon+" idea") + "\n")
	}

	if d.item.Project != "" {
		projColor := fg(ProjectColor(d.item.Project)).Render(d.item.Project)
		sb.WriteString(StyleDetailMeta.Render("project ") + " " + projColor + "\n")
	}

	// Staleness warning
	if d.item.IsTask() {
		decay := d.item.DecayLevel()
		if decay > task.DecayNone {
			days := d.item.StaleDays()
			var warning string
			switch decay {
			case task.DecaySubtle, task.DecayModerate:
				warning = fmt.Sprintf("%dd untouched", days)
			case task.DecayWarning:
				warning = fmt.Sprintf("%dd untouched — still planning to do this?", days)
			case task.DecayAlert:
				warning = fmt.Sprintf("%dd untouched — buried. revive or delete.", days)
			}
			sb.WriteString("\n" + fg(DecayColor(decay)).Render(warning) + "\n")
		}
	}

	// Description
	if d.item.Description != "" {
		sb.WriteString("\n")
		sb.WriteString(d.item.Description)
		sb.WriteString("\n")
	}

	// Notes
	if len(d.notes) > 0 {
		sb.WriteString("\n")
		sb.WriteString(StyleDetailMeta.Render("notes"))
		sb.WriteString("\n")
		sb.WriteString(StyleMuted.Render(strings.Repeat("─", d.contentWidth())))
		sb.WriteString("\n")
		for _, n := range d.notes {
			ts := StyleMuted.Render(n.CreatedAt.Format("Jan 02 15:04"))
			sb.WriteString(ts + "\n")
			sb.WriteString(StyleDetailNote.Render(n.Content) + "\n\n")
		}
	}

	// Timestamps
	sb.WriteString("\n")
	sb.WriteString(StyleMuted.Render(fmt.Sprintf("created  %s", d.item.CreatedAt.Format("Jan 02 2006 15:04"))) + "\n")
	sb.WriteString(StyleMuted.Render(fmt.Sprintf("updated  %s", d.item.UpdatedAt.Format("Jan 02 2006 15:04"))) + "\n")

	d.viewport.SetContent(sb.String())
}

// buildProjectStatsFooter pre-computes the project stats footer string and
// its line count, storing the result in d.projectStatsStr / d.projectStatsLines.
func (d *DetailView) buildProjectStatsFooter(project string) {
	if len(d.allItems) == 0 {
		return
	}

	var inProgress, blocked, todo, done, ideas int
	for _, item := range d.allItems {
		if item.Project != project {
			continue
		}
		if item.IsIdea() {
			ideas++
			continue
		}
		if item.Status == nil {
			continue
		}
		switch *item.Status {
		case task.StatusInProgress:
			inProgress++
		case task.StatusBlocked:
			blocked++
		case task.StatusTodo:
			todo++
		case task.StatusDone:
			done++
		}
	}

	taskTotal := inProgress + blocked + todo + done
	total := taskTotal + ideas
	if total <= 1 {
		// No point showing stats for a single-item project.
		return
	}

	var sb strings.Builder

	// Divider + styled header
	sb.WriteString(StyleMuted.Render(strings.Repeat("─", d.contentWidth())))
	sb.WriteString("\n")
	projC := ProjectColor(project)
	sb.WriteString(fg(projC).Bold(true).Render("project progress"))
	sb.WriteString("\n")

	// Progress bar via bubbles progress component with gradient.
	barWidth := d.contentWidth()
	if barWidth > 40 {
		barWidth = 40
	}
	if barWidth < 5 {
		barWidth = 5
	}

	var pct float64
	if taskTotal > 0 {
		pct = float64(done) / float64(taskTotal)
	}

	// Build a gradient from the project color to a lighter tint.
	cf, _ := colorful.MakeColor(projC)
	light := cf.BlendLab(colorful.Color{R: 1, G: 1, B: 1}, 0.45)

	bar := progress.New(
		progress.WithWidth(barWidth),
		progress.WithColors(projC, lipgloss.Color(light.Hex())),
		progress.WithoutPercentage(),
		progress.WithScaled(true),
	)
	label := StyleMuted.Render(fmt.Sprintf(" %d/%d done", done, taskTotal))
	sb.WriteString(bar.ViewAs(pct) + label + "\n")

	// Status breakdown
	var parts []string
	if inProgress > 0 {
		parts = append(parts, fg(StatusColor(task.StatusInProgress)).Render(fmt.Sprintf("%d in progress", inProgress)))
	}
	if blocked > 0 {
		parts = append(parts, fg(StatusColor(task.StatusBlocked)).Render(fmt.Sprintf("%d blocked", blocked)))
	}
	if todo > 0 {
		parts = append(parts, fg(StatusColor(task.StatusTodo)).Render(fmt.Sprintf("%d todo", todo)))
	}
	if done > 0 {
		parts = append(parts, fg(StatusColor(task.StatusDone)).Render(fmt.Sprintf("%d done", done)))
	}
	if ideas > 0 {
		parts = append(parts, StyleIdea.Render(fmt.Sprintf("%d ideas", ideas)))
	}
	if len(parts) > 0 {
		sb.WriteString(StyleMuted.Render("  ") + strings.Join(parts, StyleMuted.Render(" · ")))
	}

	d.projectStatsStr = sb.String()
	d.projectStatsLines = strings.Count(d.projectStatsStr, "\n") + 1
}
