package task

import "time"

// ItemKind distinguishes tasks from ideas.
type ItemKind string

const (
	KindTask ItemKind = "task"
	KindIdea ItemKind = "idea"
)

// Status represents the lifecycle state of a task.
type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusBlocked    Status = "blocked"
)

// Priority represents the urgency of a task.
type Priority int

const (
	PriorityLow      Priority = 1
	PriorityMedium   Priority = 2
	PriorityHigh     Priority = 3
	PriorityCritical Priority = 4
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Label returns a short priority label where p1 is highest urgency.
func (p Priority) Label() string {
	switch p {
	case PriorityCritical:
		return "p1"
	case PriorityHigh:
		return "p2"
	case PriorityMedium:
		return "p3"
	case PriorityLow:
		return "p4"
	default:
		return "p?"
	}
}

// SortMode controls how items are ordered within status groups.
type SortMode int

const (
	SortByPriority SortMode = iota
	SortByDueDate
	SortByCreated
	SortByUpdated
)

func (s SortMode) String() string {
	switch s {
	case SortByPriority:
		return "priority"
	case SortByDueDate:
		return "due date"
	case SortByCreated:
		return "created"
	case SortByUpdated:
		return "updated"
	default:
		return "unknown"
	}
}

// DecayLevel describes how visually degraded a task line should appear.
type DecayLevel int

const (
	DecayNone     DecayLevel = iota // 0-2 days
	DecaySubtle                     // 3-7 days
	DecayModerate                   // 8-14 days
	DecayWarning                    // 15-29 days
	DecayAlert                      // 30+ days
)

// Item represents either a task or an idea.
type Item struct {
	ID          int64
	Kind        ItemKind
	Title       string
	Description string
	// Priority is nil for ideas.
	Priority *Priority
	// Status is nil for ideas.
	Status    *Status
	Project   string
	DueDate   *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Note is a timestamped append-only note attached to an item.
type Note struct {
	ID        int64
	ItemID    int64
	Content   string
	CreatedAt time.Time
}

// IsTask returns true if this item is a task.
func (i *Item) IsTask() bool {
	return i.Kind == KindTask
}

// IsIdea returns true if this item is an idea.
func (i *Item) IsIdea() bool {
	return i.Kind == KindIdea
}

// StaleDays returns the number of whole days since the item was last updated.
func (i *Item) StaleDays() int {
	return int(time.Since(i.UpdatedAt).Hours() / 24)
}

// DecayLevel returns the visual decay level based on how many days stale the item is.
// Done tasks and ideas always return DecayNone.
func (i *Item) DecayLevel() DecayLevel {
	if i.IsIdea() {
		return DecayNone
	}
	if i.Status != nil && *i.Status == StatusDone {
		return DecayNone
	}
	days := i.StaleDays()
	switch {
	case days >= 30:
		return DecayAlert
	case days >= 15:
		return DecayWarning
	case days >= 8:
		return DecayModerate
	case days >= 3:
		return DecaySubtle
	default:
		return DecayNone
	}
}

// Advance moves a task to the next logical status.
// todo -> in_progress -> done; blocked -> in_progress.
// Returns false if the status cannot be advanced (already done, or item is an idea).
func (i *Item) Advance() bool {
	if i.IsIdea() || i.Status == nil {
		return false
	}
	switch *i.Status {
	case StatusTodo:
		i.Status = new(StatusInProgress)
	case StatusInProgress:
		i.Status = new(StatusDone)
	case StatusBlocked:
		i.Status = new(StatusInProgress)
	case StatusDone:
		return false
	}
	i.UpdatedAt = time.Now()
	return true
}

// Reverse moves a task to the previous logical status.
// done -> in_progress -> todo; blocked -> todo.
// Returns false if the status cannot be reversed (already todo, or item is an idea).
func (i *Item) Reverse() bool {
	if i.IsIdea() || i.Status == nil {
		return false
	}
	switch *i.Status {
	case StatusDone:
		i.Status = new(StatusInProgress)
	case StatusInProgress:
		i.Status = new(StatusTodo)
	case StatusBlocked:
		i.Status = new(StatusTodo)
	case StatusTodo:
		return false
	}
	i.UpdatedAt = time.Now()
	return true
}

// Revive resets the decay clock by touching UpdatedAt. Returns false if not applicable.
func (i *Item) Revive() bool {
	if i.IsIdea() {
		return false
	}
	if i.Status != nil && *i.Status == StatusDone {
		return false
	}
	if i.StaleDays() < 3 {
		return false
	}
	i.UpdatedAt = time.Now()
	return true
}

// PromoteToTask converts an idea into a task with default priority and todo status.
// Returns false if the item is already a task.
func (i *Item) PromoteToTask() bool {
	if i.IsTask() {
		return false
	}
	i.Kind = KindTask
	i.Status = new(StatusTodo)
	i.Priority = new(PriorityLow)
	i.UpdatedAt = time.Now()
	return true
}

// IsOverdue returns true if the item has a due date in the past and is not done.
func (i *Item) IsOverdue() bool {
	if i.DueDate == nil {
		return false
	}
	if i.Status != nil && *i.Status == StatusDone {
		return false
	}
	return time.Now().After(*i.DueDate)
}
