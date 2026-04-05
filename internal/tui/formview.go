package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	datepicker "github.com/CameronJHall/bubble-datepicker/v2"

	"github.com/CameronJHall/docketeer/internal/task"
)

// FormDoneMsg is sent when the form is submitted or cancelled.
type FormDoneMsg struct {
	Saved bool
	Item  task.Item // only valid when Saved == true
}

// formField identifies each editable field.
type formField int

const (
	fieldTitle formField = iota
	fieldDescription
	fieldStatus
	fieldPriority
	fieldDueDate
	fieldProject
	fieldCount // sentinel
)

// ideaFieldCount is the number of fields shown for ideas (no status/priority/due date).
const ideaFieldCount = 2 // title, description, project → 3 fields

// FormView is a full-screen create/edit form. It implements tea.Model so the
// root App can delegate Update/View calls to it directly.
type FormView struct {
	item   task.Item // working copy; ID==0 means new
	isNew  bool
	width  int
	height int

	// focused is the field the cursor is currently sitting on.
	focused formField

	// descActive is true when the description textarea has been explicitly
	// activated (enter) and is accepting free-form text input.
	descActive bool

	// dateActive is true when the datepicker calendar has been explicitly
	// activated (enter) and is accepting navigation input.
	dateActive bool

	titleInput    textinput.Model
	descInput     textarea.Model
	projectInput  textinput.Model
	projects      []string // known project names for suggestions + color matching
	dueDatePicker datepicker.Model
	dueDateSet    bool // whether a due date has been selected

	// Cycling selectors
	statusIdx   int
	priorityIdx int

	err string // validation error
}

var allStatuses = []task.Status{
	task.StatusTodo,
	task.StatusInProgress,
	task.StatusBlocked,
	task.StatusDone,
}

var allPriorities = []task.Priority{
	task.PriorityLow,
	task.PriorityMedium,
	task.PriorityHigh,
	task.PriorityCritical,
}

// NewFormView creates a blank form for a new item.
func NewFormView(kind task.ItemKind, width, height int, projects []string) FormView {
	item := task.Item{
		Kind:     kind,
		Status:   new(task.StatusTodo),
		Priority: new(task.PriorityMedium),
	}
	return newFormView(item, true, width, height, projects)
}

// EditFormView creates a form pre-filled with an existing item.
func EditFormView(item task.Item, width, height int, projects []string) FormView {
	return newFormView(item, false, width, height, projects)
}

func newFormView(item task.Item, isNew bool, width, height int, projects []string) FormView {
	// Title input — always active (single-line, no ambiguous navigation).
	ti := textinput.New()
	ti.Placeholder = "Title"
	ti.CharLimit = 200
	ti.SetWidth(width - 6)
	ti.SetValue(item.Title)

	// Description textarea — starts blurred; activated explicitly with enter.
	ta := textarea.New()
	ta.Placeholder = "Description (optional)"
	ta.ShowLineNumbers = true
	ta.Prompt = " |"
	ta.DynamicHeight = true
	ta.MaxContentHeight = 6
	ta.MinHeight = 2
	ta.SetWidth(width - 8) // narrower to account for the 2-char prompt
	ta.SetValue(item.Description)
	taStyles := ta.Styles()
	taStyles.Blurred.Prompt = fg(colorMuted)
	taStyles.Focused.Prompt = fg(colorAccent)
	ta.SetStyles(taStyles)

	// Project input
	pi := textinput.New()
	pi.Placeholder = "Project (optional)"
	pi.CharLimit = 100
	pi.SetWidth(width - 6)
	pi.SetValue(item.Project)
	if len(projects) > 0 {
		pi.SetSuggestions(projects)
		pi.ShowSuggestions = true
		km := pi.KeyMap
		km.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
		pi.KeyMap = km
	}

	// Due date picker — always kept at FocusCalendar and always Selected so
	// the cursor date is always rendered with FocusedText (pink+bold).
	// dueDateSet tracks whether the user actually intends a due date; if false
	// the picker is cosmetically active but submit ignores the time value.
	seed := time.Now()
	dueDateSet := false
	if item.DueDate != nil {
		seed = *item.DueDate
		dueDateSet = true
	}
	dp := datepicker.New(seed)
	dp.SetFocus(datepicker.FocusCalendar)
	dp.SelectDate() // always on so the cursor cell is always highlighted

	// Find current indices
	statusIdx := 0
	if item.Status != nil {
		for i, s := range allStatuses {
			if s == *item.Status {
				statusIdx = i
				break
			}
		}
	}
	priorityIdx := 1 // default medium
	if item.Priority != nil {
		for i, p := range allPriorities {
			if p == *item.Priority {
				priorityIdx = i
				break
			}
		}
	}

	f := FormView{
		item:          item,
		isNew:         isNew,
		width:         width,
		height:        height,
		focused:       fieldTitle,
		titleInput:    ti,
		descInput:     ta,
		projectInput:  pi,
		projects:      projects,
		dueDatePicker: dp,
		dueDateSet:    dueDateSet,
		statusIdx:     statusIdx,
		priorityIdx:   priorityIdx,
	}
	_ = f.titleInput.Focus()
	f.syncProjectColor()
	return f
}

// Init satisfies tea.Model; the form has no startup commands.
func (f *FormView) Init() tea.Cmd { return nil }

// Update handles key events for the form.
func (f *FormView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// ctrl+c is always global.
		if msg.String() == "ctrl+c" {
			return f, tea.Quit
		}

		// ── Description is active: it owns all keys except esc ────────────
		if f.descActive {
			switch msg.String() {
			case "esc":
				// Deactivate — leave the cursor on the description field.
				f.descInput.Blur()
				f.descActive = false
				return f, nil
			case "tab":
				f.descInput.Blur()
				f.descActive = false
				return f, f.nextField()
			case "shift+tab":
				f.descInput.Blur()
				f.descActive = false
				return f, f.prevField()
			default:
				var cmd tea.Cmd
				f.descInput, cmd = f.descInput.Update(msg)
				return f, cmd
			}
		}

		// ── Datepicker is active: own all navigation directly ──────────────
		// We bypass the datepicker's internal tab/shift-tab sub-focus model
		// entirely and drive it with a simpler, flatter key scheme:
		//   ←/→        prev/next day
		//   ↑/↓        prev/next week
		//   [/]        prev/next month
		//   enter      select / clear
		//   esc        deactivate
		if f.dateActive {
			switch msg.String() {
			case "esc":
				f.dateActive = false
				return f, nil
			case "tab":
				f.dateActive = false
				return f, f.nextField()
			case "shift+tab":
				f.dateActive = false
				return f, f.prevField()
			case "left":
				f.dueDatePicker.Yesterday()
				return f, nil
			case "right":
				f.dueDatePicker.Tomorrow()
				return f, nil
			case "up":
				f.dueDatePicker.LastWeek()
				return f, nil
			case "down":
				f.dueDatePicker.NextWeek()
				return f, nil
			case "[":
				f.dueDatePicker.LastMonth()
				return f, nil
			case "]":
				f.dueDatePicker.NextMonth()
				return f, nil
			}
			return f, nil
		}

		// ── Normal navigation (no widget is active) ────────────────────────
		switch msg.String() {
		case "ctrl+s", "ctrl+enter":
			return f.submit()

		case "esc":
			return f, func() tea.Msg { return FormDoneMsg{Saved: false} }

		case "tab", "down":
			cmds = append(cmds, f.nextField())

		case "shift+tab", "up":
			cmds = append(cmds, f.prevField())

		case "enter", " ":
			switch f.focused {
			case fieldStatus, fieldPriority:
				f.cycleField(1)
			case fieldDescription:
				f.descActive = true
				cmds = append(cmds, f.descInput.Focus())
			case fieldDueDate:
				// Activate and immediately mark a date as set — navigating
				// the calendar means you intend to pick a date.
				f.dateActive = true
				f.dueDateSet = true
			default:
				cmds = append(cmds, f.nextField())
			}

		case "d":
			// Clear the due date when the field is highlighted but not active.
			if f.focused == fieldDueDate {
				f.dueDateSet = false
				f.dueDatePicker.SetTime(time.Now())
			}

		case "left":
			if f.focused == fieldStatus || f.focused == fieldPriority {
				f.cycleField(-1)
			}
		case "right":
			if f.focused == fieldStatus || f.focused == fieldPriority {
				f.cycleField(1)
			}
		}
	}

	// Delegate passive updates to active text inputs.
	var cmd tea.Cmd
	switch f.focused {
	case fieldTitle:
		f.titleInput, cmd = f.titleInput.Update(msg)
	case fieldProject:
		f.projectInput, cmd = f.projectInput.Update(msg)
		f.syncProjectColor()
	}
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return f, tea.Batch(cmds...)
}

// View renders the form.
func (f *FormView) View() tea.View {
	var sb strings.Builder

	title := "New Task"
	if f.item.IsIdea() {
		title = "New Idea"
	}
	if !f.isNew {
		title = "Edit"
	}

	sb.WriteString(StyleAccent.Render(title) + "\n")
	sb.WriteString(StyleMuted.Render(strings.Repeat("─", f.width-2)) + "\n\n")

	f.writeField(&sb, fieldTitle, "Title *", f.titleInput.View())
	f.writeFieldRaw(&sb, fieldDescription, "Description", f.descInput.View())

	if f.item.IsTask() {
		status := allStatuses[f.statusIdx]
		statusDisplay := fg(StatusColor(status)).Render(fmt.Sprintf("◀ %s ▶", string(status)))
		f.writeField(&sb, fieldStatus, "Status", statusDisplay)

		priority := allPriorities[f.priorityIdx]
		priorityDisplay := fg(PriorityColor(priority)).Render(fmt.Sprintf("◀ %s ▶", priority.String()))
		f.writeField(&sb, fieldPriority, "Priority", priorityDisplay)

		dueDateLabel := "Due Date"
		if f.dueDateSet {
			dueDateLabel = fmt.Sprintf("Due Date (%s)", f.dueDatePicker.Time.Format("2006-01-02"))
		}
		calView := f.dueDatePicker.View()
		if f.dateActive {
			calView = StyleCalendarActive.Render(calView)
		}
		f.writeFieldRaw(&sb, fieldDueDate, dueDateLabel, calView)
	}

	f.writeField(&sb, fieldProject, "Project", f.projectInput.View())

	if f.err != "" {
		sb.WriteString("\n" + StyleFormError.Render("✗ "+f.err) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(StyleMuted.Render(f.helpText()))

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

func (f *FormView) helpText() string {
	switch {
	case f.descActive:
		return "esc done  •  tab next field  •  shift+tab prev field"
	case f.dateActive:
		return "←→ day  •  ↑↓ week  •  [/] month  •  esc done  •  tab next field  •  shift+tab prev field"
	case f.focused == fieldDueDate && f.dueDateSet:
		return "enter navigate  •  d clear  •  tab next field  •  shift+tab prev field  •  ctrl+s save  •  esc cancel"
	case f.focused == fieldDueDate:
		return "enter navigate  •  tab next field  •  shift+tab prev field  •  ctrl+s save  •  esc cancel"
	default:
		return "tab next field  •  shift+tab prev field  •  ctrl+s save  •  esc cancel"
	}
}

func (f *FormView) writeField(sb *strings.Builder, field formField, label, content string) {
	f.writeFieldRaw(sb, field, label, "  "+content)
}

// writeFieldRaw writes a field without adding extra indentation to content.
// Use this for multiline widgets that manage their own indentation (e.g. textarea, datepicker).
func (f *FormView) writeFieldRaw(sb *strings.Builder, field formField, label, content string) {
	indicator := "  "
	labelStyle := StyleMuted
	if f.focused == field {
		indicator = StyleAccent.Render("▶ ")
		labelStyle = StyleFormFocusedLabel
	}
	sb.WriteString(indicator + labelStyle.Render(label) + "\n")
	sb.WriteString(content + "\n\n")
}

func (f *FormView) nextField() tea.Cmd {
	f.blurCurrent()
	next := f.focused + 1
	if f.item.IsIdea() {
		switch next {
		case fieldStatus, fieldPriority, fieldDueDate:
			next = fieldProject
		}
	}
	if int(next) >= int(fieldCount) {
		next = fieldTitle
	}
	f.focused = next
	return f.focusCurrent()
}

func (f *FormView) prevField() tea.Cmd {
	f.blurCurrent()
	prev := f.focused - 1
	if prev < 0 {
		prev = fieldProject
	}
	if f.item.IsIdea() {
		switch prev {
		case fieldStatus, fieldPriority, fieldDueDate:
			prev = fieldDescription
		}
	}
	f.focused = prev
	return f.focusCurrent()
}

// blurCurrent deactivates any active widget on the current field before
// moving focus away.
func (f *FormView) blurCurrent() {
	switch f.focused {
	case fieldTitle:
		f.titleInput.Blur()
	case fieldDescription:
		if f.descActive {
			f.descInput.Blur()
			f.descActive = false
		}
	case fieldProject:
		f.projectInput.Blur()
	case fieldDueDate:
		f.dateActive = false
	}
}

// focusCurrent sets up the newly-focused field. For title/project the
// underlying input is immediately active (single-line, no nav ambiguity).
// For description and due date, only the field is highlighted — the widget
// itself stays inactive until the user presses enter.
func (f *FormView) focusCurrent() tea.Cmd {
	switch f.focused {
	case fieldTitle:
		return f.titleInput.Focus()
	case fieldProject:
		return f.projectInput.Focus()
	}
	// description and due date: no widget activation on arrival.
	return nil
}

func (f *FormView) cycleField(dir int) {
	switch f.focused {
	case fieldStatus:
		f.statusIdx = (f.statusIdx + dir + len(allStatuses)) % len(allStatuses)
	case fieldPriority:
		f.priorityIdx = (f.priorityIdx + dir + len(allPriorities)) % len(allPriorities)
	}
}

func (f *FormView) submit() (tea.Model, tea.Cmd) {
	f.err = ""

	title := strings.TrimSpace(f.titleInput.Value())
	if title == "" {
		f.err = "title is required"
		return f, nil
	}

	f.item.Title = title
	f.item.Description = strings.TrimSpace(f.descInput.Value())
	f.item.Project = strings.TrimSpace(f.projectInput.Value())

	if f.item.IsTask() {
		f.item.Status = new(allStatuses[f.statusIdx])
		f.item.Priority = new(allPriorities[f.priorityIdx])

		if f.dueDateSet {
			t := f.dueDatePicker.Time
			// Normalise to midnight local time. Using UTC here would shift the
			// date backward for timezones west of UTC when the Unix timestamp
			// is read back via time.Unix in local time.
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
			f.item.DueDate = &t
		} else {
			f.item.DueDate = nil
		}
	}

	return f, func() tea.Msg { return FormDoneMsg{Saved: true, Item: f.item} }
}

// syncProjectColor updates the project input text color to match the
// deterministic ProjectColor when the current value is a known project name.
func (f *FormView) syncProjectColor() {
	val := strings.TrimSpace(f.projectInput.Value())
	styles := f.projectInput.Styles()
	matched := false
	for _, p := range f.projects {
		if strings.EqualFold(p, val) {
			matched = true
			break
		}
	}
	if matched {
		c := ProjectColor(val)
		styles.Focused.Text = styles.Focused.Text.Foreground(c)
		styles.Blurred.Text = styles.Blurred.Text.Foreground(c)
	} else {
		styles.Focused.Text = styles.Focused.Text.UnsetForeground()
		styles.Blurred.Text = styles.Blurred.Text.UnsetForeground()
	}
	f.projectInput.SetStyles(styles)
}
