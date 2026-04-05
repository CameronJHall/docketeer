package store

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/CameronJHall/docketeer/internal/task"
)

func newTestPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()

	connStr := os.Getenv("POSTGRES_TEST_URL")
	if connStr == "" {
		connStr = "postgresql://testuser:testpass@localhost:5433/docketeer_test?sslmode=disable"
	}

	// Drop existing tables for a clean slate
	dropDB(connStr)

	db, err := NewPostgresStore(connStr)
	if err != nil {
		t.Skipf("skipping postgres integration test: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		cleanupTestDB(db)
	})

	return db
}

func dropDB(connStr string) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS schema_migrations")
	db.Exec("DROP TABLE IF EXISTS notes")
	db.Exec("DROP TABLE IF EXISTS items")
}

func cleanupTestDB(s *PostgresStore) {
	s.db.Exec("DELETE FROM notes")
	s.db.Exec("DELETE FROM items")
}

func TestPostgresStore_CreateAndGetItem(t *testing.T) {
	s := newTestPostgresStore(t)

	item := &task.Item{
		Kind:   task.KindTask,
		Title:  "Test Task",
		Status: ptr(task.StatusTodo),
	}

	if err := s.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	if item.ID == 0 {
		t.Error("expected item to have ID set after creation")
	}

	if item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		t.Error("expected timestamps to be set")
	}

	found, err := s.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if found.ID != item.ID {
		t.Errorf("expected ID %d, got %d", item.ID, found.ID)
	}
	if found.Title != item.Title {
		t.Errorf("expected title %q, got %q", item.Title, found.Title)
	}
}

func TestPostgresStore_UpdateItem(t *testing.T) {
	s := newTestPostgresStore(t)

	item := &task.Item{
		Kind:        task.KindTask,
		Title:       "Original Title",
		Description: "Original desc",
		Status:      ptr(task.StatusTodo),
	}

	if err := s.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	item.Title = "Updated Title"
	item.Description = "Updated desc"
	item.Status = new(task.StatusInProgress)

	if err := s.UpdateItem(item); err != nil {
		t.Fatalf("failed to update item: %v", err)
	}

	found, err := s.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if found.Title != "Updated Title" {
		t.Errorf("expected title %q, got %q", "Updated Title", found.Title)
	}
	if found.Description != "Updated desc" {
		t.Errorf("expected description %q, got %q", "Updated desc", found.Description)
	}
	if found.Status == nil || *found.Status != task.StatusInProgress {
		t.Errorf("expected status %q, got %v", task.StatusInProgress, found.Status)
	}
}

func TestPostgresStore_DeleteItem(t *testing.T) {
	s := newTestPostgresStore(t)

	item := &task.Item{
		Kind:  task.KindIdea,
		Title: "Test Idea",
	}

	if err := s.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	if err := s.DeleteItem(item.ID); err != nil {
		t.Fatalf("failed to delete item: %v", err)
	}

	_, err := s.GetItem(item.ID)
	if err == nil {
		t.Error("expected error when getting deleted item")
	}
}

func TestPostgresStore_ListItems(t *testing.T) {
	s := newTestPostgresStore(t)

	items := []*task.Item{
		{Kind: task.KindTask, Title: "Task 1"},
		{Kind: task.KindIdea, Title: "Idea 1"},
		{Kind: task.KindTask, Title: "Task 2"},
	}

	for _, item := range items {
		if err := s.CreateItem(item); err != nil {
			t.Fatalf("failed to create item: %v", err)
		}
	}

	list, err := s.ListItems()
	if err != nil {
		t.Fatalf("failed to list items: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("expected 3 items, got %d", len(list))
	}
}

func TestPostgresStore_AddAndListNotes(t *testing.T) {
	s := newTestPostgresStore(t)

	item := &task.Item{
		Kind:  task.KindTask,
		Title: "Task with notes",
	}

	if err := s.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	note1 := &task.Note{
		ItemID:  item.ID,
		Content: "First note",
	}
	note2 := &task.Note{
		ItemID:  item.ID,
		Content: "Second note",
	}

	if err := s.AddNote(note1); err != nil {
		t.Fatalf("failed to add note1: %v", err)
	}
	if err := s.AddNote(note2); err != nil {
		t.Fatalf("failed to add note2: %v", err)
	}

	if note1.ID == 0 || note2.ID == 0 {
		t.Error("expected notes to have IDs set after creation")
	}

	notes, err := s.ListNotes(item.ID)
	if err != nil {
		t.Fatalf("failed to list notes: %v", err)
	}

	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}

	if notes[0].Content != "First note" {
		t.Errorf("expected first note content %q, got %q", "First note", notes[0].Content)
	}
}

func TestPostgresStore_ListProjects(t *testing.T) {
	s := newTestPostgresStore(t)

	items := []*task.Item{
		{Kind: task.KindTask, Title: "Task 1", Project: "project-a"},
		{Kind: task.KindTask, Title: "Task 2", Project: "project-b"},
		{Kind: task.KindTask, Title: "Task 3", Project: "project-a"},
		{Kind: task.KindTask, Title: "Task 4", Project: ""},
	}

	for _, item := range items {
		if err := s.CreateItem(item); err != nil {
			t.Fatalf("failed to create item: %v", err)
		}
	}

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("failed to list projects: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d: %v", len(projects), projects)
	}

	if projects[0] != "project-a" || projects[1] != "project-b" {
		t.Errorf("expected projects in sorted order, got %v", projects)
	}
}

func TestPostgresStore_CompletionsLast7Days(t *testing.T) {
	s := newTestPostgresStore(t)

	status := task.StatusDone
	now := time.Now()

	tasks := []*task.Item{
		{Kind: task.KindTask, Title: "Task today", Status: &status},
		{Kind: task.KindTask, Title: "Task yesterday", Status: &status, UpdatedAt: now.AddDate(0, 0, -1)},
		{Kind: task.KindTask, Title: "Task 6 days ago", Status: &status, UpdatedAt: now.AddDate(0, 0, -6)},
		{Kind: task.KindTask, Title: "Task 8 days ago", Status: &status, UpdatedAt: now.AddDate(0, 0, -8)},
		{Kind: task.KindIdea, Title: "Idea"},
	}

	// Create tasks. Note: CreateItem and UpdateItem both set UpdatedAt to now,
	// so the first task will have UpdatedAt in the current day.
	for i, item := range tasks {
		if err := s.CreateItem(item); err != nil {
			t.Fatalf("failed to create item: %v", err)
		}
		// Manually update the UpdatedAt to the desired time using raw SQL
		switch i {
		case 1:
			_, _ = s.db.Exec("UPDATE items SET updated_at = $1 WHERE id = $2", now.AddDate(0, 0, -1).Unix(), item.ID)
		case 2:
			_, _ = s.db.Exec("UPDATE items SET updated_at = $1 WHERE id = $2", now.AddDate(0, 0, -6).Unix(), item.ID)
		case 3:
			_, _ = s.db.Exec("UPDATE items SET updated_at = $1 WHERE id = $2", now.AddDate(0, 0, -8).Unix(), item.ID)
		}
	}

	counts, err := s.CompletionsLast7Days()
	if err != nil {
		t.Fatalf("failed to get completions: %v", err)
	}

	if len(counts) != 7 {
		t.Errorf("expected 7 days of counts, got %d", len(counts))
	}

	// Task 0 was just created (current day), tasks 1 & 2 were backdated
	// Task 3 is outside the 7-day window, task 4 is an idea (not counted)
	totalCompletions := 0
	for _, c := range counts {
		totalCompletions += c
	}

	if totalCompletions != 3 {
		t.Errorf("expected 3 total completions, got %d", totalCompletions)
	}
}

func ptr[T any](v T) *T {
	return &v
}
