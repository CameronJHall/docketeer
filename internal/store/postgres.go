package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/CameronJHall/docketeer/internal/store/migrator"
	"github.com/CameronJHall/docketeer/internal/task"
	_ "github.com/lib/pq"
)

// PostgresStore is a Store backed by a PostgreSQL database.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore opens a PostgreSQL database at connectionString and runs schema migrations.
func NewPostgresStore(connectionString string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	dialect := migrator.NewPostgresDialect()
	m := migrator.NewMigrator(db, dialect, "schema_migrations")
	migrator.AddVersion1Migrations(m)

	if err := m.Run(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// Close releases the database connection.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// ExecRaw executes a raw SQL statement. Used by tooling (e.g. seed) that needs
// to bypass the store's automatic timestamp behaviour.
func (s *PostgresStore) ExecRaw(query string, args ...any) error {
	_, err := s.db.Exec(query, args...)
	return err
}

// CreateItem inserts a new item and sets its ID and timestamps.
func (s *PostgresStore) CreateItem(item *task.Item) error {
	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now

	err := s.db.QueryRow(`
         INSERT INTO items (kind, title, description, priority, status, project, due_date, created_at, updated_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
         RETURNING id`,
		string(item.Kind),
		item.Title,
		item.Description,
		priorityToSQL(item.Priority),
		statusToSQL(item.Status),
		item.Project,
		timeToSQL(item.DueDate),
		item.CreatedAt.Unix(),
		item.UpdatedAt.Unix(),
	).Scan(&item.ID)

	if err != nil {
		return fmt.Errorf("create item: %w", err)
	}
	return nil
}

// UpdateItem updates all mutable fields of an existing item.
func (s *PostgresStore) UpdateItem(item *task.Item) error {
	item.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
         UPDATE items SET
            kind        = $1,
            title       = $2,
            description = $3,
            priority    = $4,
            status      = $5,
            project     = $6,
            due_date    = $7,
            updated_at  = $8
         WHERE id = $9`,
		string(item.Kind),
		item.Title,
		item.Description,
		priorityToSQL(item.Priority),
		statusToSQL(item.Status),
		item.Project,
		timeToSQL(item.DueDate),
		item.UpdatedAt.Unix(),
		item.ID,
	)
	if err != nil {
		return fmt.Errorf("update item %d: %w", item.ID, err)
	}
	return nil
}

// DeleteItem removes an item by ID. Associated notes are deleted via CASCADE.
func (s *PostgresStore) DeleteItem(id int64) error {
	_, err := s.db.Exec(`DELETE FROM items WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete item %d: %w", id, err)
	}
	return nil
}

// GetItem returns a single item by ID.
func (s *PostgresStore) GetItem(id int64) (*task.Item, error) {
	row := s.db.QueryRow(`
         SELECT id, kind, title, description, priority, status, project, due_date, created_at, updated_at
         FROM items WHERE id = $1`, id)
	item, err := scanItem(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("item %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get item %d: %w", id, err)
	}
	return item, nil
}

// ListItems returns all items ordered by created_at descending.
func (s *PostgresStore) ListItems() ([]task.Item, error) {
	rows, err := s.db.Query(`
         SELECT id, kind, title, description, priority, status, project, due_date, created_at, updated_at
         FROM items ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	var items []task.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// AddNote inserts a new note for an item and sets its ID and timestamp.
func (s *PostgresStore) AddNote(note *task.Note) error {
	note.CreatedAt = time.Now()

	err := s.db.QueryRow(`
         INSERT INTO notes (item_id, content, created_at)
         VALUES ($1, $2, $3)
         RETURNING id`,
		note.ItemID,
		note.Content,
		note.CreatedAt.Unix(),
	).Scan(&note.ID)

	if err != nil {
		return fmt.Errorf("add note: %w", err)
	}
	return nil
}

// ListNotes returns all notes for an item ordered by created_at ascending.
func (s *PostgresStore) ListNotes(itemID int64) ([]task.Note, error) {
	rows, err := s.db.Query(`
         SELECT id, item_id, content, created_at
         FROM notes WHERE item_id = $1 ORDER BY created_at ASC`, itemID)
	if err != nil {
		return nil, fmt.Errorf("list notes for item %d: %w", itemID, err)
	}
	defer rows.Close()

	var notes []task.Note
	for rows.Next() {
		var n task.Note
		var createdUnix int64
		if err := rows.Scan(&n.ID, &n.ItemID, &n.Content, &createdUnix); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		n.CreatedAt = time.Unix(createdUnix, 0)
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

// ListProjects returns all distinct non-empty project names, sorted alphabetically.
func (s *PostgresStore) ListProjects() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT project FROM items WHERE project != '' ORDER BY project`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// CompletionsLast7Days returns the count of tasks marked done on each of the past
// 7 days. Index 0 is 6 days ago; index 6 is today.
func (s *PostgresStore) CompletionsLast7Days() ([]int, error) {
	counts := make([]int, 7)

	now := time.Now()
	// Start of today (local time).
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for i := 0; i < 7; i++ {
		dayStart := today.AddDate(0, 0, -(6 - i))
		dayEnd := dayStart.Add(24 * time.Hour)

		var count int
		err := s.db.QueryRow(`
            SELECT COUNT(*) FROM items
            WHERE kind = 'task'
              AND status = 'done'
              AND updated_at >= $1
              AND updated_at < $2`,
			dayStart.Unix(),
			dayEnd.Unix(),
		).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("completions day %d: %w", i, err)
		}
		counts[i] = count
	}
	return counts, nil
}
