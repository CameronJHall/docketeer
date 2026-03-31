package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/CameronJHall/docketeer/internal/store"
	"github.com/CameronJHall/docketeer/internal/task"
)

// activeView identifies which full-screen view (or overlay) is visible.
type activeView int

const (
	viewList    activeView = iota
	viewForm               // create or edit item
	viewConfirm            // delete confirmation
	viewNote               // add note overlay
)

// loadItemsMsg is sent when item data is fetched from the store.
type loadItemsMsg struct {
	items       []task.Item
	completions []int
	err         error
}

// statusExpiredMsg clears the ephemeral status message.
type statusExpiredMsg struct{}

// App is the root Bubble Tea model.
type App struct {
	store       store.Store
	keys        KeyMap
	items       []task.Item
	completions []int

	sortMode    task.SortMode
	showMetrics bool

	// Project filter (vim-style / search)
	filterText   string          // committed filter text (empty = show all)
	filterActive bool            // true when text input is focused
	filterInput  textinput.Model // live text input for filter

	// lastSelectedID tracks the selected item across reloads for cursor preservation.
	lastSelectedID int64

	width  int
	height int

	view        activeView
	formView    *FormView
	confirmView *ConfirmView
	noteInput   *NoteInput

	listView   ListView
	detailView DetailView
	helpBar    HelpBar

	statusMsg string
}

// New creates a new App model backed by the given store.
func New(s store.Store) *App {
	return &App{
		store:       s,
		keys:        DefaultKeyMap(),
		sortMode:    task.SortByPriority,
		showMetrics: true,
		listView:    NewListView(0, 0),
		detailView:  NewDetailView(0, 0),
		helpBar:     NewHelpBar(0),
	}
}

// Init loads items from the store on startup.
func (a *App) Init() tea.Cmd {
	return a.loadItems()
}

// Update handles incoming messages and updates the model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Always handle window resize and status expiry regardless of active view.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.layout()
		// Resize sub-views if open
		if a.formView != nil {
			projects, _ := a.store.ListProjects()
			fv := NewFormView(a.formView.item.Kind, a.width, a.height, projects)
			fv.item = a.formView.item
			fv.isNew = a.formView.isNew
			a.formView = &fv
		}
		if a.confirmView != nil {
			cv := NewConfirmView(a.confirmView.item, a.width, a.height)
			a.confirmView = &cv
		}
		if a.noteInput != nil {
			ni := NewNoteInput(a.noteInput.item, a.detailView.contentWidth())
			a.noteInput = &ni
			a.detailView.SetNoteInput(a.noteInput)
		}
		return a, nil

	case statusExpiredMsg:
		a.helpBar.ClearStatus()
		a.statusMsg = ""
		return a, nil
	}

	// Delegate to active sub-view if not in list mode.
	switch a.view {
	case viewForm:
		return a.updateForm(msg)
	case viewConfirm:
		return a.updateConfirm(msg)
	case viewNote:
		return a.updateNote(msg)
	}

	// --- List view input ---
	switch msg := msg.(type) {
	case loadItemsMsg:
		if msg.err == nil {
			a.items = msg.items
			a.completions = msg.completions
			a.syncList()
		}

	case tea.KeyPressMsg:
		// When the filter text input is active, intercept all keys.
		if a.filterActive {
			switch msg.String() {
			case "enter":
				// Commit the current filter text and exit input mode.
				a.filterText = a.filterInput.Value()
				a.filterActive = false
				a.syncList()
			case "esc":
				// Cancel — revert to the previously committed filter.
				a.filterActive = false
				a.syncList()
			case "ctrl+c":
				return a, tea.Quit
			default:
				// Delegate to the textinput for normal typing.
				var cmd tea.Cmd
				a.filterInput, cmd = a.filterInput.Update(msg)
				// Live-filter as the user types.
				a.filterText = a.filterInput.Value()
				a.syncList()
				return a, cmd
			}
			return a, nil
		}

		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit

		case key.Matches(msg, a.keys.Help):
			a.helpBar.ToggleShowAll()
			a.layout()
			return a, nil

		// Esc clears a committed filter when not in input mode.
		case msg.String() == "esc":
			if a.filterText != "" {
				a.filterText = ""
				a.syncList()
			}

		case key.Matches(msg, a.keys.Up):
			a.listView.MoveUp()
			a.syncDetail()

		case key.Matches(msg, a.keys.Down):
			a.listView.MoveDown()
			a.syncDetail()

		case key.Matches(msg, a.keys.SortCycle):
			a.sortMode = task.NextSortMode(a.sortMode)
			a.syncList()

		case key.Matches(msg, a.keys.ToggleMetrics):
			a.showMetrics = !a.showMetrics

		case key.Matches(msg, a.keys.Filter):
			a.filterActive = true
			ti := textinput.New()
			ti.Prompt = "/"
			ti.SetValue(a.filterText)
			ti.Focus()
			a.filterInput = ti
			a.syncList()

		case key.Matches(msg, a.keys.Reload):
			cmds = append(cmds, a.loadItems())
			a.setStatus("reloaded")
			cmds = append(cmds, statusExpiredCmd())

		case key.Matches(msg, a.keys.Create):
			projects, _ := a.store.ListProjects()
			fv := NewFormView(task.KindTask, a.width, a.height, projects)
			a.formView = &fv
			a.view = viewForm

		case key.Matches(msg, a.keys.CreateIdea):
			projects, _ := a.store.ListProjects()
			fv := NewFormView(task.KindIdea, a.width, a.height, projects)
			a.formView = &fv
			a.view = viewForm

		case key.Matches(msg, a.keys.Edit):
			if item := a.listView.SelectedItem(); item != nil {
				projects, _ := a.store.ListProjects()
				fv := EditFormView(*item, a.width, a.height, projects)
				a.formView = &fv
				a.view = viewForm
			}

		case key.Matches(msg, a.keys.Delete):
			if item := a.listView.SelectedItem(); item != nil {
				cv := NewConfirmView(*item, a.width, a.height)
				a.confirmView = &cv
				a.view = viewConfirm
			}

		case key.Matches(msg, a.keys.Advance):
			if item := a.listView.SelectedItem(); item != nil && item.IsTask() {
				prevStatus := item.Status
				if item.Advance() {
					if err := a.store.UpdateItem(item); err == nil {
						cmds = append(cmds, a.loadItems())
						a.setStatus(advanceVerb(prevStatus))
						cmds = append(cmds, statusExpiredCmd())
					}
				}
			}

		case key.Matches(msg, a.keys.Reverse):
			if item := a.listView.SelectedItem(); item != nil && item.IsTask() {
				prevStatus := item.Status
				if item.Reverse() {
					if err := a.store.UpdateItem(item); err == nil {
						cmds = append(cmds, a.loadItems())
						a.setStatus(reverseVerb(prevStatus))
						cmds = append(cmds, statusExpiredCmd())
					}
				}
			}

		case key.Matches(msg, a.keys.Promote):
			if item := a.listView.SelectedItem(); item != nil && item.IsIdea() {
				if item.PromoteToTask() {
					if err := a.store.UpdateItem(item); err == nil {
						cmds = append(cmds, a.loadItems())
						a.setStatus("promoted to task")
						cmds = append(cmds, statusExpiredCmd())
					}
				}
			}

		case key.Matches(msg, a.keys.Revive):
			if item := a.listView.SelectedItem(); item != nil && item.IsTask() {
				if item.Revive() {
					if err := a.store.UpdateItem(item); err == nil {
						cmds = append(cmds, a.loadItems())
						a.setStatus("revived")
						cmds = append(cmds, statusExpiredCmd())
					}
				}
			}

		case key.Matches(msg, a.keys.AddNote):
			if item := a.listView.SelectedItem(); item != nil {
				ni := NewNoteInput(*item, a.detailView.contentWidth())
				a.noteInput = &ni
				a.detailView.SetNoteInput(a.noteInput)
				a.view = viewNote
			}
		}
	}

	return a, tea.Batch(cmds...)
}

// updateForm delegates messages to the form view and handles its done message.
func (a *App) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for form completion first
	if msg, ok := msg.(FormDoneMsg); ok {
		a.view = viewList
		a.formView = nil
		if msg.Saved {
			var err error
			item := msg.Item
			if item.ID == 0 {
				err = a.store.CreateItem(&item)
			} else {
				err = a.store.UpdateItem(&item)
			}
			if err != nil {
				a.setStatus("error: " + err.Error())
				return a, statusExpiredCmd()
			}
			verb := "created"
			if msg.Item.ID != 0 {
				verb = "saved"
			}
			a.setStatus(verb + ": " + msg.Item.Title)
			return a, tea.Batch(a.loadItems(), statusExpiredCmd())
		}
		return a, nil
	}

	if a.formView == nil {
		return a, nil
	}
	m, cmd := a.formView.Update(msg)
	if fv, ok := m.(*FormView); ok {
		a.formView = fv
	}
	return a, cmd
}

// updateConfirm delegates messages to the confirm view and handles its done message.
func (a *App) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(ConfirmDeleteMsg); ok {
		a.view = viewList
		a.confirmView = nil
		if msg.Confirmed {
			if err := a.store.DeleteItem(msg.Item.ID); err != nil {
				a.setStatus("error: " + err.Error())
				return a, statusExpiredCmd()
			}
			a.setStatus("deleted: " + msg.Item.Title)
			return a, tea.Batch(a.loadItems(), statusExpiredCmd())
		}
		return a, nil
	}

	if a.confirmView == nil {
		return a, nil
	}
	m, cmd := a.confirmView.Update(msg)
	if cv, ok := m.(*ConfirmView); ok {
		a.confirmView = cv
	}
	return a, cmd
}

// updateNote delegates messages to the note input overlay and handles its done message.
func (a *App) updateNote(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(NoteInputDoneMsg); ok {
		a.view = viewList
		a.noteInput = nil
		a.detailView.SetNoteInput(nil)
		if msg.Saved {
			note := &task.Note{
				ItemID:  msg.ItemID,
				Content: msg.Content,
			}
			if err := a.store.AddNote(note); err != nil {
				a.setStatus("error: " + err.Error())
				return a, statusExpiredCmd()
			}
			a.setStatus("note added")
			a.syncDetail()
			return a, statusExpiredCmd()
		}
		return a, nil
	}

	if a.noteInput == nil {
		return a, nil
	}
	m, cmd := a.noteInput.Update(msg)
	if ni, ok := m.(*NoteInput); ok {
		a.noteInput = ni
	}
	return a, cmd
}

// View renders the full screen.
func (a *App) View() tea.View {
	if a.width == 0 {
		return tea.NewView("loading…")
	}

	// Delegate to sub-views when active
	switch a.view {
	case viewForm:
		if a.formView != nil {
			return a.formView.View()
		}
	case viewConfirm:
		if a.confirmView != nil {
			return a.confirmView.View()
		}
	}

	var sb strings.Builder

	// Header
	header := RenderHeader(a.width, a.items, a.completions, a.showMetrics)
	sb.WriteString(header)
	sb.WriteString("\n")

	// Body
	bodyHeight := a.bodyHeight()
	if a.isWide() {
		leftW, rightW := a.panelWidths()
		left := a.listView.View()
		right := a.detailView.View()
		_ = leftW
		_ = rightW
		sb.WriteString(joinHorizontal(left, right))
	} else {
		a.listView.SetSize(a.width, bodyHeight)
		sb.WriteString(a.listView.View())
	}
	sb.WriteString("\n")

	// Help bar
	sb.WriteString(a.helpBar.View())

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// --- internal helpers ---

func (a *App) isWide() bool {
	return a.width >= 80
}

func (a *App) panelWidths() (left, right int) {
	left = (a.width * 55) / 100
	right = a.width - left - 1
	if right < 20 {
		right = 20
		left = a.width - right - 1
	}
	return left, right
}

func (a *App) headerHeight() int {
	return 2
}

func (a *App) helpBarHeight() int {
	base := 0
	if a.statusMsg != "" {
		base = 1 // extra line for status message
	}
	if a.helpBar.ShowAll() {
		return a.helpBar.FullHelpHeight() + base
	}
	// short help: border (1) + one line of keys (1)
	return 2 + base
}

func (a *App) bodyHeight() int {
	h := a.height - a.headerHeight() - a.helpBarHeight()
	if h < 1 {
		h = 1
	}
	return h
}

func (a *App) layout() {
	bodyH := a.bodyHeight()
	if a.isWide() {
		leftW, rightW := a.panelWidths()
		a.listView.SetSize(leftW, bodyH)
		a.detailView.SetSize(rightW, bodyH)
	} else {
		a.listView.SetSize(a.width, bodyH)
		a.detailView.SetSize(a.width, bodyH)
	}
	a.helpBar.SetWidth(a.width)
}

func (a *App) syncList() {
	// Capture current selection before re-grouping so we can restore it.
	if item := a.listView.SelectedItem(); item != nil {
		a.lastSelectedID = item.ID
	}

	// Pass filter display state to the list view.
	if a.filterActive {
		a.listView.SetFilterInput(a.filterInput.View())
	} else {
		a.listView.SetFilterInput("")
	}
	a.listView.SetFilterText(a.filterText)

	a.listView.SetItems(a.filteredItems(), a.sortMode)
	// Always pass unfiltered items so the detail panel can compute
	// project-level stats across the entire dataset.
	a.detailView.SetAllItems(a.items)
	if a.lastSelectedID != 0 {
		a.listView.SetCursorByID(a.lastSelectedID)
	}
	a.syncDetail()
}

func (a *App) syncDetail() {
	item := a.listView.SelectedItem()
	if item == nil {
		a.detailView.SetItem(nil, nil)
	} else {
		notes, _ := a.store.ListNotes(item.ID)
		a.detailView.SetItem(item, notes)
	}
	a.syncHelpBar()
}

func (a *App) loadItems() tea.Cmd {
	return func() tea.Msg {
		items, err := a.store.ListItems()
		if err != nil {
			return loadItemsMsg{err: err}
		}
		completions, _ := a.store.CompletionsLast7Days()
		return loadItemsMsg{items: items, completions: completions}
	}
}

func (a *App) setStatus(msg string) {
	a.statusMsg = msg
	a.helpBar.SetStatus(msg)
}

// syncHelpBar recomputes context-sensitive key hints and pushes them to the help bar.
// Call this whenever focus, selection, or view state changes.
func (a *App) syncHelpBar() {
	a.helpBar.SetContextKeys(a.contextKeys())
}

// contextKeys returns the relevant key bindings for the current UI state.
// Kept short (≤5 keys) — press ? to see everything.
// Advance/Reverse bindings get dynamic help text based on the selected item's status.
func (a *App) contextKeys() []key.Binding {
	k := a.keys

	item := a.listView.SelectedItem()

	// Empty list
	if item == nil {
		return []key.Binding{k.Create, k.CreateIdea, k.Help, k.Quit}
	}

	var bindings []key.Binding

	if item.IsIdea() {
		bindings = append(bindings, k.Promote, k.Edit, k.Delete)
	} else if item.Status != nil {
		switch *item.Status {
		case task.StatusTodo:
			bindings = append(bindings, advanceBinding(">", "begin"))
		case task.StatusInProgress:
			bindings = append(bindings, advanceBinding(">", "complete"))
			bindings = append(bindings, reverseBinding("<", "unpick"))
		case task.StatusBlocked:
			bindings = append(bindings, advanceBinding(">", "unblock"))
			bindings = append(bindings, reverseBinding("<", "unpick"))
		case task.StatusDone:
			bindings = append(bindings, reverseBinding("<", "reopen"))
		}
		bindings = append(bindings, k.Edit, k.Delete)
		if item.StaleDays() >= 3 && *item.Status != task.StatusDone {
			bindings = append(bindings, k.Revive)
		}
	}

	bindings = append(bindings, k.Help)
	if a.filterText != "" {
		// When a filter is active, show esc hint — handled inline, not via binding.
		bindings = append(bindings, key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")))
	}
	bindings = append(bindings, k.Filter)
	return bindings
}

// advanceBinding returns a copy of the Advance binding with custom help text.
func advanceBinding(keyStr, label string) key.Binding {
	return key.NewBinding(key.WithKeys(">", "."), key.WithHelp(keyStr, label))
}

// reverseBinding returns a copy of the Reverse binding with custom help text.
func reverseBinding(keyStr, label string) key.Binding {
	return key.NewBinding(key.WithKeys("<", ","), key.WithHelp(keyStr, label))
}

// advanceVerb returns the past-tense verb for the status transition triggered by Advance.
// prevStatus is the status *before* the transition occurred.
func advanceVerb(prevStatus *task.Status) string {
	if prevStatus == nil {
		return "advanced"
	}
	switch *prevStatus {
	case task.StatusTodo:
		return "begun"
	case task.StatusInProgress:
		return "completed"
	case task.StatusBlocked:
		return "unblocked"
	default:
		return "advanced"
	}
}

// reverseVerb returns the past-tense verb for the status transition triggered by Reverse.
// prevStatus is the status *before* the transition occurred.
func reverseVerb(prevStatus *task.Status) string {
	if prevStatus == nil {
		return "reversed"
	}
	switch *prevStatus {
	case task.StatusDone:
		return "reopened"
	case task.StatusInProgress:
		return "unpicked"
	case task.StatusBlocked:
		return "unpicked"
	default:
		return "reversed"
	}
}

// joinHorizontal places two strings side by side, line by line.
func joinHorizontal(left, right string) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	for len(leftLines) < len(rightLines) {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < len(leftLines) {
		rightLines = append(rightLines, "")
	}

	var out []string
	for i := range leftLines {
		out = append(out, leftLines[i]+rightLines[i])
	}
	return strings.Join(out, "\n")
}

// statusExpiredCmd returns a command that fires statusExpiredMsg after a delay.
func statusExpiredCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return statusExpiredMsg{}
	})
}

// filteredItems returns items matching the active filter text.
// If no filter is active, all items are returned.
// Matching is case-insensitive substring on item.Project.
func (a *App) filteredItems() []task.Item {
	if a.filterText == "" {
		return a.items
	}
	query := strings.ToLower(a.filterText)
	var filtered []task.Item
	for _, item := range a.items {
		if strings.Contains(strings.ToLower(item.Project), query) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
