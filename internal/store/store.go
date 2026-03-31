package store

import "github.com/CameronJHall/docketeer/internal/task"

// Store is the persistence interface for docketeer.
// All methods are safe to call concurrently.
type Store interface {
	// Item operations
	CreateItem(item *task.Item) error
	UpdateItem(item *task.Item) error
	DeleteItem(id int64) error
	GetItem(id int64) (*task.Item, error)
	ListItems() ([]task.Item, error)

	// Note operations
	AddNote(note *task.Note) error
	ListNotes(itemID int64) ([]task.Note, error)

	// Projects
	// ListProjects returns all distinct non-empty project names, sorted alphabetically.
	ListProjects() ([]string, error)

	// Metrics
	// CompletionsLast7Days returns the count of tasks marked done each day
	// for the past 7 days, index 0 = oldest day, index 6 = today.
	CompletionsLast7Days() ([]int, error)

	// Close releases any resources held by the store.
	Close() error
}
