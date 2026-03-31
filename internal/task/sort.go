package task

import (
	"sort"
	"time"
)

// statusGroupOrder defines the display order for status groups.
var statusGroupOrder = map[Status]int{
	StatusInProgress: 0,
	StatusBlocked:    1,
	StatusTodo:       2,
	StatusDone:       3,
}

// NextSortMode cycles to the next sort mode.
func NextSortMode(current SortMode) SortMode {
	return (current + 1) % 4
}

// GroupItems returns tasks grouped by status in display order, followed by ideas.
// Within each group items are sorted by mode.
func GroupItems(items []Item, mode SortMode) (groups []ItemGroup) {
	var tasks []Item
	var ideas []Item

	for _, item := range items {
		if item.IsTask() {
			tasks = append(tasks, item)
		} else {
			ideas = append(ideas, item)
		}
	}

	// Group tasks by status.
	byStatus := make(map[Status][]Item)
	for _, t := range tasks {
		if t.Status == nil {
			continue
		}
		byStatus[*t.Status] = append(byStatus[*t.Status], t)
	}

	// Sort each status group and append in display order.
	orderedStatuses := []Status{StatusInProgress, StatusBlocked, StatusTodo, StatusDone}
	for _, s := range orderedStatuses {
		group := byStatus[s]
		if len(group) == 0 {
			continue
		}
		sortByMode(group, mode)
		groups = append(groups, ItemGroup{Status: &s, Items: group})
	}

	// Ideas section last, sorted by mode (ignoring status).
	if len(ideas) > 0 {
		sortByMode(ideas, mode)
		groups = append(groups, ItemGroup{Status: nil, Items: ideas})
	}

	return groups
}

// SortItems returns a flat sorted slice suitable for cursor-index operations.
// Tasks are sorted by status group order then by mode; ideas come last.
func SortItems(items []Item, mode SortMode) []Item {
	result := make([]Item, len(items))
	copy(result, items)

	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]

		// Ideas always after tasks.
		if a.IsTask() != b.IsTask() {
			return a.IsTask()
		}

		// Both tasks: sort by status group order first.
		if a.IsTask() && b.IsTask() {
			aOrder := statusOrder(a)
			bOrder := statusOrder(b)
			if aOrder != bOrder {
				return aOrder < bOrder
			}
		}

		// Within same group/kind: sort by mode.
		return modeLess(a, b, mode)
	})

	return result
}

// ItemGroup holds a status label and the items belonging to it.
// Status is nil for the ideas group.
type ItemGroup struct {
	Status *Status
	Items  []Item
}

// Label returns a display-friendly group label.
func (g ItemGroup) Label() string {
	if g.Status == nil {
		return "Ideas"
	}
	switch *g.Status {
	case StatusInProgress:
		return "In Progress"
	case StatusBlocked:
		return "Blocked"
	case StatusTodo:
		return "Todo"
	case StatusDone:
		return "Done"
	default:
		return string(*g.Status)
	}
}

// statusOrder returns the display-order index for a task item.
func statusOrder(item Item) int {
	if item.Status == nil {
		return 99
	}
	if o, ok := statusGroupOrder[*item.Status]; ok {
		return o
	}
	return 99
}

// sortByMode sorts a slice of items in-place by the given mode.
func sortByMode(items []Item, mode SortMode) {
	sort.SliceStable(items, func(i, j int) bool {
		return modeLess(items[i], items[j], mode)
	})
}

// modeLess returns true if a should appear before b given the sort mode.
func modeLess(a, b Item, mode SortMode) bool {
	switch mode {
	case SortByPriority:
		ap := priorityVal(a)
		bp := priorityVal(b)
		if ap != bp {
			return ap > bp // higher priority value = higher urgency = first
		}
		return a.CreatedAt.Before(b.CreatedAt)

	case SortByDueDate:
		// Items with no due date go last.
		if a.DueDate == nil && b.DueDate == nil {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		if a.DueDate == nil {
			return false
		}
		if b.DueDate == nil {
			return true
		}
		if !a.DueDate.Equal(*b.DueDate) {
			return a.DueDate.Before(*b.DueDate)
		}
		return a.CreatedAt.Before(b.CreatedAt)

	case SortByCreated:
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.After(b.CreatedAt) // newest first
		}
		return a.ID < b.ID

	case SortByUpdated:
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt) // recently touched first
		}
		return a.ID < b.ID

	default:
		return a.ID < b.ID
	}
}

// priorityVal returns a numeric priority value for sorting (higher = more urgent).
func priorityVal(item Item) int {
	if item.Priority == nil {
		return 0
	}
	return int(*item.Priority)
}

// BacklogAge returns the cumulative number of days all open (non-done, non-idea)
// items have been open since creation. Used for the backlog age metric.
func BacklogAge(items []Item) int {
	now := time.Now()
	total := 0
	for _, item := range items {
		if item.IsIdea() {
			continue
		}
		if item.Status != nil && *item.Status == StatusDone {
			continue
		}
		days := int(now.Sub(item.CreatedAt).Hours() / 24)
		total += days
	}
	return total
}
