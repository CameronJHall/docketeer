package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CameronJHall/docketeer/internal/task"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS items (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	kind        TEXT    NOT NULL CHECK(kind IN ('task', 'idea')),
	title       TEXT    NOT NULL,
	description TEXT    NOT NULL DEFAULT '',
	priority    INTEGER,
	status      TEXT    CHECK(status IN ('todo', 'in_progress', 'done', 'blocked')),
	project     TEXT    NOT NULL DEFAULT '',
	due_date    INTEGER,
	created_at  INTEGER NOT NULL,
	updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS notes (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	item_id    INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	content    TEXT    NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notes_item_id ON notes(item_id);
CREATE INDEX IF NOT EXISTS idx_items_updated_at ON items(updated_at);
PRAGMA foreign_keys = ON;
`

// SQLiteStore is a Store backed by a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// DefaultDBPath returns the XDG data dir path for the database.
func DefaultDBPath() (string, error) {
	dataDir, err := xdgDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "docketeer", "docketeer.db"), nil
}

func xdgDataDir() (string, error) {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

// NewSQLiteStore opens (or creates) a SQLite database at dbPath and runs schema migrations.
// Pass ":memory:" for an in-memory database (useful in tests).
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode and foreign keys.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set pragma (%s): %w", p, err)
		}
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Close releases the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// ExecRaw executes a raw SQL statement. Used by tooling (e.g. seed) that needs
// to bypass the store's automatic timestamp behaviour.
func (s *SQLiteStore) ExecRaw(query string, args ...any) error {
	_, err := s.db.Exec(query, args...)
	return err
}

// CreateItem inserts a new item and sets its ID and timestamps.
func (s *SQLiteStore) CreateItem(item *task.Item) error {
	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now

	res, err := s.db.Exec(`
		INSERT INTO items (kind, title, description, priority, status, project, due_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(item.Kind),
		item.Title,
		item.Description,
		priorityToSQL(item.Priority),
		statusToSQL(item.Status),
		item.Project,
		timeToSQL(item.DueDate),
		item.CreatedAt.Unix(),
		item.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("create item: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	item.ID = id
	return nil
}

// UpdateItem updates all mutable fields of an existing item.
func (s *SQLiteStore) UpdateItem(item *task.Item) error {
	item.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		UPDATE items SET
			kind        = ?,
			title       = ?,
			description = ?,
			priority    = ?,
			status      = ?,
			project     = ?,
			due_date    = ?,
			updated_at  = ?
		WHERE id = ?`,
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
func (s *SQLiteStore) DeleteItem(id int64) error {
	_, err := s.db.Exec(`DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete item %d: %w", id, err)
	}
	return nil
}

// GetItem returns a single item by ID.
func (s *SQLiteStore) GetItem(id int64) (*task.Item, error) {
	row := s.db.QueryRow(`
		SELECT id, kind, title, description, priority, status, project, due_date, created_at, updated_at
		FROM items WHERE id = ?`, id)
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
func (s *SQLiteStore) ListItems() ([]task.Item, error) {
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
func (s *SQLiteStore) AddNote(note *task.Note) error {
	note.CreatedAt = time.Now()

	res, err := s.db.Exec(`
		INSERT INTO notes (item_id, content, created_at)
		VALUES (?, ?, ?)`,
		note.ItemID,
		note.Content,
		note.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("add note: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get note last insert id: %w", err)
	}
	note.ID = id
	return nil
}

// ListNotes returns all notes for an item ordered by created_at ascending.
func (s *SQLiteStore) ListNotes(itemID int64) ([]task.Note, error) {
	rows, err := s.db.Query(`
		SELECT id, item_id, content, created_at
		FROM notes WHERE item_id = ? ORDER BY created_at ASC`, itemID)
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
func (s *SQLiteStore) ListProjects() ([]string, error) {
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
func (s *SQLiteStore) CompletionsLast7Days() ([]int, error) {
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
			  AND updated_at >= ?
			  AND updated_at < ?`,
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

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func scanItem(s scanner) (*task.Item, error) {
	var item task.Item
	var kindStr string
	var prioritySQL sql.NullInt64
	var statusSQL sql.NullString
	var dueDateSQL sql.NullInt64
	var createdUnix, updatedUnix int64

	err := s.Scan(
		&item.ID,
		&kindStr,
		&item.Title,
		&item.Description,
		&prioritySQL,
		&statusSQL,
		&item.Project,
		&dueDateSQL,
		&createdUnix,
		&updatedUnix,
	)
	if err != nil {
		return nil, err
	}

	item.Kind = task.ItemKind(kindStr)
	item.CreatedAt = time.Unix(createdUnix, 0)
	item.UpdatedAt = time.Unix(updatedUnix, 0)

	if prioritySQL.Valid {
		p := task.Priority(prioritySQL.Int64)
		item.Priority = &p
	}
	if statusSQL.Valid {
		st := task.Status(statusSQL.String)
		item.Status = &st
	}
	if dueDateSQL.Valid {
		t := time.Unix(dueDateSQL.Int64, 0)
		item.DueDate = &t
	}

	return &item, nil
}

func priorityToSQL(p *task.Priority) any {
	if p == nil {
		return nil
	}
	return int(*p)
}

func statusToSQL(s *task.Status) any {
	if s == nil {
		return nil
	}
	return string(*s)
}

func timeToSQL(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}
