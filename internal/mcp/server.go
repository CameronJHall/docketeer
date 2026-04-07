package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/CameronJHall/docketeer/internal/store"
	"github.com/CameronJHall/docketeer/internal/task"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates a new MCP server for docketeer.
func NewServer(s store.Store) *mcpsdk.Server {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "docketeer",
		Version: "1.0.0",
	}, nil)

	// Tools
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "list_items",
		Description: `List tasks and ideas. Returns all items by default; use filters to narrow results.

Items have two kinds:
  - "task": actionable item with status (todo|in_progress|done|blocked) and priority (low|medium|high|critical)
  - "idea": unstructured note or future consideration — no status or priority

Call list_projects first to discover valid project name strings when filtering by project.`,
	}, listItemsHandler(s))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "get_item",
		Description: `Get full details of a task or idea by ID, including all attached notes.

Prefer this over list_items when you need note history or complete item details for a specific item.`,
	}, getItemHandler(s))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "create_task",
		Description: `Create a new task. Tasks start with status "todo" and default priority "medium".

Valid priorities: low, medium, high, critical.
Use create_idea instead for items that don't need status tracking or completion.`,
	}, createTaskHandler(s))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "create_idea",
		Description: `Create a new idea. Ideas have no status or priority — they represent unactionable or future considerations.

Use create_task if the item needs to be tracked and completed.
Use promote_idea to convert an existing idea into a task later.`,
	}, createIdeaHandler(s))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "update_item",
		Description: `Update fields of an existing task or idea. Only provided fields are changed; omitted fields are left unchanged (patch semantics).

Valid statuses: todo, in_progress, done, blocked.
Valid priorities: low, medium, high, critical.
To clear due_date, pass an empty string "".
Prefer advance_task for simple forward status progression (todo→in_progress→done).
Use this tool directly when setting status to "blocked" or making other field changes alongside a status change.`,
	}, updateItemHandler(s))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "delete_item",
		Description: "Permanently delete a task or idea by ID. This action is irreversible.",
	}, deleteItemHandler(s))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "advance_task",
		Description: `Advance a task's status one step forward along its lifecycle.

Transitions: todo → in_progress → done, and blocked → in_progress.
Only works on tasks (not ideas). Fails if the task is already "done".
Prefer this over update_item when simply moving a task forward without changing other fields.`,
	}, advanceTaskHandler(s))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "add_note",
		Description: `Append a timestamped note to a task or idea. Notes are append-only and cannot be edited or deleted.

Use for progress updates, decisions, blockers, or observations about the item.
Retrieve notes by calling get_item.`,
	}, addNoteHandler(s))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "list_projects",
		Description: `List all distinct project names that currently have at least one item.

Call this before filtering list_items by project to ensure you use the exact project name string (names are case-sensitive and free-form).`,
	}, listProjectsHandler(s))

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "promote_idea",
		Description: `Convert an existing idea into a task. Sets status to "todo" and priority to "low".

Use update_item afterwards to adjust priority or other fields.
Fails if the item is already a task.`,
	}, promoteIdeaHandler(s))

	// Resources
	server.AddResourceTemplate(
		&mcpsdk.ResourceTemplate{
			URITemplate: "docketeer://item/{item_id}",
			Name:        "item",
			Description: "A task or idea by ID",
			MIMEType:    "application/json",
		},
		itemResourceHandler(s),
	)

	server.AddResourceTemplate(
		&mcpsdk.ResourceTemplate{
			URITemplate: "docketeer://project/{project_name}",
			Name:        "project",
			Description: "All items in a project",
			MIMEType:    "application/json",
		},
		projectResourceHandler(s),
	)

	return server
}

// ItemListParams represents parameters for listing items.
type ItemListParams struct {
	Project *string `json:"project,omitempty" jsonschema:"exact project name to filter by; call list_projects to discover valid names"`
	Status  *string `json:"status,omitempty" jsonschema:"filter to items with this status; one of: todo, in_progress, done, blocked — only tasks have status, ideas are never returned when this filter is set"`
	Limit   *int    `json:"limit,omitempty" jsonschema:"cap the number of items returned; omit to return all matching items"`
}

// ItemListResult represents the result of listing items.
type ItemListResult struct {
	Items []taskItem `json:"items"`
	Total int        `json:"total"`
}

// TaskCreateParams represents parameters for creating a task.
type TaskCreateParams struct {
	Title       string  `json:"title" jsonschema:"short descriptive title for the task"`
	Description *string `json:"description,omitempty" jsonschema:"optional longer description or acceptance criteria"`
	Project     *string `json:"project,omitempty" jsonschema:"optional project name for grouping; use consistent names across items — names are case-sensitive free-form strings"`
	Priority    *string `json:"priority,omitempty" jsonschema:"one of: low, medium, high, critical; defaults to medium if omitted"`
	DueDate     *string `json:"due_date,omitempty" jsonschema:"optional deadline in YYYY-MM-DD format"`
}

// TaskCreateResult represents the result of creating a task.
type TaskCreateResult struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// IdeaCreateParams represents parameters for creating an idea.
type IdeaCreateParams struct {
	Title       string  `json:"title" jsonschema:"short descriptive title for the idea"`
	Description *string `json:"description,omitempty" jsonschema:"optional longer description or context"`
	Project     *string `json:"project,omitempty" jsonschema:"optional project name for grouping; use consistent names across items"`
}

// IdeaCreateResult represents the result of creating an idea.
type IdeaCreateResult struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// ItemUpdateParams represents parameters for updating an item.
type ItemUpdateParams struct {
	ID          int64   `json:"id" jsonschema:"ID of the item to update"`
	Title       *string `json:"title,omitempty" jsonschema:"new title; omit to leave unchanged"`
	Description *string `json:"description,omitempty" jsonschema:"new description; omit to leave unchanged"`
	Project     *string `json:"project,omitempty" jsonschema:"new project name; omit to leave unchanged"`
	Priority    *string `json:"priority,omitempty" jsonschema:"new priority — one of: low, medium, high, critical; omit to leave unchanged"`
	Status      *string `json:"status,omitempty" jsonschema:"new status — one of: todo, in_progress, done, blocked; omit to leave unchanged; use advance_task for simple forward progression instead"`
	DueDate     *string `json:"due_date,omitempty" jsonschema:"new due date in YYYY-MM-DD format; pass empty string to clear the due date; omit to leave unchanged"`
}

// ItemUpdateResult represents the result of updating an item.
type ItemUpdateResult struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// ItemGetParams represents parameters for getting an item.
type ItemGetParams struct {
	ID int64 `json:"id" jsonschema:"ID of the item to retrieve"`
}

// ItemDeleteParams represents parameters for deleting an item.
type ItemDeleteParams struct {
	ID int64 `json:"id" jsonschema:"ID of the item to permanently delete"`
}

// ItemDeleteResult represents the result of deleting an item.
type ItemDeleteResult struct {
	Deleted bool `json:"deleted"`
}

// AdvanceTaskParams represents parameters for advancing a task.
type AdvanceTaskParams struct {
	ID int64 `json:"id" jsonschema:"ID of the task to advance one step forward (todo→in_progress→done, or blocked→in_progress)"`
}

// AdvanceTaskResult represents the result of advancing a task.
type AdvanceTaskResult struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	NewStatus string `json:"new_status"`
}

// NoteCreateParams represents parameters for adding a note.
type NoteCreateParams struct {
	ItemID  int64  `json:"item_id" jsonschema:"ID of the item to attach this note to"`
	Content string `json:"content" jsonschema:"note text; notes are append-only and cannot be edited or deleted after creation"`
}

// NoteCreateResult represents the result of adding a note.
type NoteCreateResult struct {
	ID      int64  `json:"id"`
	ItemID  int64  `json:"item_id"`
	Created string `json:"created"`
}

// PromoteIdeaParams represents parameters for promoting an idea to a task.
type PromoteIdeaParams struct {
	ID int64 `json:"id" jsonschema:"ID of the idea to convert into a task; fails if the item is already a task"`
}

// PromoteIdeaResult represents the result of promoting an idea to a task.
type PromoteIdeaResult struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

// taskItem is a JSON-serializable representation of a task.Item.
type taskItem struct {
	ID          int64   `json:"id"`
	Kind        string  `json:"kind"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    *string `json:"priority,omitempty"`
	Status      *string `json:"status,omitempty"`
	Project     string  `json:"project,omitempty"`
	DueDate     *string `json:"due_date,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func itemToList(item task.Item) taskItem {
	result := taskItem{
		ID:          item.ID,
		Kind:        string(item.Kind),
		Title:       item.Title,
		Description: item.Description,
		Project:     item.Project,
		CreatedAt:   item.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   item.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if item.Priority != nil {
		p := item.Priority.String()
		result.Priority = &p
	}

	if item.Status != nil {
		s := string(*item.Status)
		result.Status = &s
	}

	if item.DueDate != nil {
		d := item.DueDate.Format("2006-01-02")
		result.DueDate = &d
	}

	return result
}

func filterItems(items []task.Item, params ItemListParams) []task.Item {
	if params.Project == nil && params.Status == nil {
		return items
	}

	filtered := make([]task.Item, 0, len(items))
	for _, item := range items {
		match := true

		if params.Project != nil && item.Project != *params.Project {
			match = false
		}

		if params.Status != nil {
			if item.Status == nil || string(*item.Status) != *params.Status {
				match = false
			}
		}

		if match {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func applyLimit(items []task.Item, limit *int) []task.Item {
	if limit == nil || *limit <= 0 || *limit >= len(items) {
		return items
	}
	return items[:*limit]
}

func parsePriority(s string) (*task.Priority, error) {
	if s == "" {
		return nil, nil
	}
	var p task.Priority
	switch s {
	case "low":
		p = task.PriorityLow
	case "medium":
		p = task.PriorityMedium
	case "high":
		p = task.PriorityHigh
	case "critical":
		p = task.PriorityCritical
	default:
		return nil, fmt.Errorf("invalid priority: %s", s)
	}
	return &p, nil
}

func parseStatus(s string) (*task.Status, error) {
	if s == "" {
		return nil, nil
	}
	st := task.Status(s)
	switch st {
	case task.StatusTodo, task.StatusInProgress, task.StatusDone, task.StatusBlocked:
		return &st, nil
	default:
		return nil, fmt.Errorf("invalid status: %s", s)
	}
}

func defaultPriority() *task.Priority {
	p := task.PriorityMedium
	return &p
}

func parseDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, errors.New("invalid date format, use YYYY-MM-DD")
}

type taskNote struct {
	ID      int64  `json:"id"`
	ItemID  int64  `json:"item_id"`
	Content string `json:"content"`
	Created string `json:"created"`
}

func listItemsHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, ItemListParams) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, params ItemListParams) (*mcpsdk.CallToolResult, any, error) {
		items, err := s.ListItems()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list items: %w", err)
		}

		filtered := filterItems(items, params)
		limited := applyLimit(filtered, params.Limit)

		taskItems := make([]taskItem, 0, len(limited))
		for _, item := range limited {
			taskItems = append(taskItems, itemToList(item))
		}

		result := ItemListResult{
			Items: taskItems,
			Total: len(filtered),
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(data)},
			},
		}, result, nil
	}
}

func getItemHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, ItemGetParams) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, params ItemGetParams) (*mcpsdk.CallToolResult, any, error) {
		item, err := s.GetItem(params.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, fmt.Errorf("item %d not found", params.ID)
			}
			return nil, nil, fmt.Errorf("failed to get item: %w", err)
		}

		notes, err := s.ListNotes(params.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get notes: %w", err)
		}

		result := struct {
			Item  taskItem   `json:"item"`
			Notes []taskNote `json:"notes"`
		}{
			Item:  itemToList(*item),
			Notes: make([]taskNote, 0, len(notes)),
		}

		for _, note := range notes {
			result.Notes = append(result.Notes, taskNote{
				ID:      note.ID,
				ItemID:  note.ItemID,
				Content: note.Content,
				Created: note.CreatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(data)},
			},
		}, result, nil
	}
}

func createTaskHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, TaskCreateParams) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, params TaskCreateParams) (*mcpsdk.CallToolResult, any, error) {
		if params.Title == "" {
			return nil, nil, errors.New("title is required")
		}

		status := task.StatusTodo
		item := &task.Item{
			Kind:     task.KindTask,
			Title:    params.Title,
			Status:   &status,
			Priority: defaultPriority(),
		}

		if params.Description != nil {
			item.Description = *params.Description
		}

		if params.Project != nil {
			item.Project = *params.Project
		}

		if params.Priority != nil {
			p, err := parsePriority(*params.Priority)
			if err != nil {
				return nil, nil, err
			}
			item.Priority = p
		}

		if params.DueDate != nil && *params.DueDate != "" {
			t, err := parseDate(*params.DueDate)
			if err != nil {
				return nil, nil, err
			}
			item.DueDate = &t
		}

		if err := s.CreateItem(item); err != nil {
			return nil, nil, fmt.Errorf("failed to create task: %w", err)
		}

		result := TaskCreateResult{ID: item.ID, Title: item.Title}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(data)},
			},
		}, result, nil
	}
}

func createIdeaHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, IdeaCreateParams) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, params IdeaCreateParams) (*mcpsdk.CallToolResult, any, error) {
		if params.Title == "" {
			return nil, nil, errors.New("title is required")
		}

		item := &task.Item{
			Kind:  task.KindIdea,
			Title: params.Title,
		}

		if params.Description != nil {
			item.Description = *params.Description
		}

		if params.Project != nil {
			item.Project = *params.Project
		}

		if err := s.CreateItem(item); err != nil {
			return nil, nil, fmt.Errorf("failed to create idea: %w", err)
		}

		result := IdeaCreateResult{ID: item.ID, Title: item.Title}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(data)},
			},
		}, result, nil
	}
}

func updateItemHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, ItemUpdateParams) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, params ItemUpdateParams) (*mcpsdk.CallToolResult, any, error) {
		item, err := s.GetItem(params.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, fmt.Errorf("item %d not found", params.ID)
			}
			return nil, nil, fmt.Errorf("failed to get item: %w", err)
		}

		if params.Title != nil {
			item.Title = *params.Title
		}
		if params.Description != nil {
			item.Description = *params.Description
		}
		if params.Project != nil {
			item.Project = *params.Project
		}
		if params.Priority != nil {
			p, err := parsePriority(*params.Priority)
			if err != nil {
				return nil, nil, err
			}
			item.Priority = p
		}
		if params.Status != nil {
			st, err := parseStatus(*params.Status)
			if err != nil {
				return nil, nil, err
			}
			item.Status = st
		}
		if params.DueDate != nil {
			if *params.DueDate == "" {
				item.DueDate = nil
			} else {
				t, err := parseDate(*params.DueDate)
				if err != nil {
					return nil, nil, err
				}
				item.DueDate = &t
			}
		}

		if err := s.UpdateItem(item); err != nil {
			return nil, nil, fmt.Errorf("failed to update item: %w", err)
		}

		result := ItemUpdateResult{ID: item.ID, Title: item.Title}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(data)},
			},
		}, result, nil
	}
}

func deleteItemHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, ItemDeleteParams) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, params ItemDeleteParams) (*mcpsdk.CallToolResult, any, error) {
		err := s.DeleteItem(params.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, fmt.Errorf("item %d not found", params.ID)
			}
			return nil, nil, fmt.Errorf("failed to delete item: %w", err)
		}

		result := ItemDeleteResult{Deleted: true}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(data)},
			},
		}, result, nil
	}
}

func advanceTaskHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, AdvanceTaskParams) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, params AdvanceTaskParams) (*mcpsdk.CallToolResult, any, error) {
		item, err := s.GetItem(params.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, fmt.Errorf("item %d not found", params.ID)
			}
			return nil, nil, fmt.Errorf("failed to get item: %w", err)
		}

		if !item.IsTask() {
			return nil, nil, fmt.Errorf("item %d is not a task", params.ID)
		}

		if item.Status == nil || *item.Status == task.StatusDone {
			return nil, nil, fmt.Errorf("item %d cannot be advanced", params.ID)
		}

		if !item.Advance() {
			return nil, nil, fmt.Errorf("failed to advance item %d", params.ID)
		}

		if err := s.UpdateItem(item); err != nil {
			return nil, nil, fmt.Errorf("failed to update item: %w", err)
		}

		result := AdvanceTaskResult{
			ID:        item.ID,
			Title:     item.Title,
			NewStatus: string(*item.Status),
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(data)},
			},
		}, result, nil
	}
}

func addNoteHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, NoteCreateParams) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, params NoteCreateParams) (*mcpsdk.CallToolResult, any, error) {
		if params.Content == "" {
			return nil, nil, errors.New("note content is required")
		}

		_, err := s.GetItem(params.ItemID)
		if err != nil {
			return nil, nil, fmt.Errorf("item %d not found", params.ItemID)
		}

		note := &task.Note{
			ItemID:  params.ItemID,
			Content: params.Content,
		}

		if err := s.AddNote(note); err != nil {
			return nil, nil, fmt.Errorf("failed to add note: %w", err)
		}

		result := NoteCreateResult{
			ID:      note.ID,
			ItemID:  note.ItemID,
			Created: note.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: fmt.Sprintf("Added note %d to item %d", note.ID, note.ItemID)},
			},
		}, result, nil
	}
}

func listProjectsHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, any) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, _ any) (*mcpsdk.CallToolResult, any, error) {
		projects, err := s.ListProjects()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list projects: %w", err)
		}

		type ProjectResult struct {
			Projects []string `json:"projects"`
			Total    int      `json:"total"`
		}

		result := ProjectResult{
			Projects: projects,
			Total:    len(projects),
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: fmt.Sprintf("Found %d projects", len(projects))},
			},
		}, result, nil
	}
}

func itemResourceHandler(s store.Store) func(context.Context, *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	return func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		var itemID int64
		if _, err := fmt.Sscanf(req.Params.URI, "docketeer://item/%d", &itemID); err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}

		item, err := s.GetItem(itemID)
		if err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}

		data, err := json.Marshal(itemToList(*item))
		if err != nil {
			return nil, fmt.Errorf("failed to marshal item: %w", err)
		}

		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				},
			},
		}, nil
	}
}

func projectResourceHandler(s store.Store) func(context.Context, *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	return func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		var projectName string
		if _, err := fmt.Sscanf(req.Params.URI, "docketeer://project/%s", &projectName); err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}

		items, err := s.ListItems()
		if err != nil {
			return nil, fmt.Errorf("failed to list items: %w", err)
		}

		filtered := make([]task.Item, 0)
		for _, item := range items {
			if item.Project == projectName {
				filtered = append(filtered, item)
			}
		}

		if len(filtered) == 0 {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}

		taskItems := make([]taskItem, 0, len(filtered))
		for _, item := range filtered {
			taskItems = append(taskItems, itemToList(item))
		}

		data, err := json.Marshal(taskItems)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal items: %w", err)
		}

		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				},
			},
		}, nil
	}
}

func promoteIdeaHandler(s store.Store) func(context.Context, *mcpsdk.CallToolRequest, PromoteIdeaParams) (*mcpsdk.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, params PromoteIdeaParams) (*mcpsdk.CallToolResult, any, error) {
		item, err := s.GetItem(params.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, fmt.Errorf("item %d not found", params.ID)
			}
			return nil, nil, fmt.Errorf("failed to get item: %w", err)
		}

		if !item.PromoteToTask() {
			return nil, nil, fmt.Errorf("item %d is already a task", params.ID)
		}

		if err := s.UpdateItem(item); err != nil {
			return nil, nil, fmt.Errorf("failed to promote idea: %w", err)
		}

		result := PromoteIdeaResult{
			ID:       item.ID,
			Title:    item.Title,
			Status:   string(*item.Status),
			Priority: item.Priority.String(),
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: fmt.Sprintf("Promoted idea %d to task: %s (status: %s)", item.ID, item.Title, *item.Status)},
			},
		}, result, nil
	}
}
