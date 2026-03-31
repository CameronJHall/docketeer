// Package seed populates a store with realistic fixture data for manual testing.
package seed

import (
	"fmt"
	"time"

	"github.com/CameronJHall/docketeer/internal/store"
	"github.com/CameronJHall/docketeer/internal/task"
)

// Run inserts fixture items and notes into s. Safe to call on an empty database only —
// it does not check for existing data.
func Run(s store.Store) error {
	now := time.Now()

	// helpers
	p := func(v task.Priority) *task.Priority { return &v }
	st := func(v task.Status) *task.Status { return &v }
	daysAgo := func(n int) time.Time { return now.AddDate(0, 0, -n) }
	future := func(n int) *time.Time { t := now.AddDate(0, 0, n); return &t }
	past := func(n int) *time.Time { t := now.AddDate(0, 0, -n); return &t }

	type itemSpec struct {
		item  task.Item
		notes []string
	}

	specs := []itemSpec{
		// --- in_progress ---
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Migrate auth service to JWT",
				Description: "Replace session cookies with stateless JWT tokens. Need to update middleware and all client SDKs.",
				Priority:    p(task.PriorityCritical), Status: st(task.StatusInProgress),
				Project: "auth", CreatedAt: daysAgo(12), UpdatedAt: daysAgo(1),
			},
			notes: []string{
				"Middleware updated, working on SDK wrappers now.",
				"iOS SDK done. Android next.",
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Add pagination to /api/items endpoint",
				Description: "Cursor-based pagination. Client sends `after` param.",
				Priority:    p(task.PriorityHigh), Status: st(task.StatusInProgress),
				Project: "api", CreatedAt: daysAgo(5), UpdatedAt: daysAgo(0),
			},
			notes: []string{"Drafted schema, needs review."},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Refactor database connection pool",
				Priority: p(task.PriorityMedium), Status: st(task.StatusInProgress),
				Project: "infra", CreatedAt: daysAgo(20), UpdatedAt: daysAgo(9),
			},
		},

		// --- blocked ---
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Deploy staging environment",
				Description: "Waiting on DevOps to provision the new VPC. Ticket #4821.",
				Priority:    p(task.PriorityCritical), Status: st(task.StatusBlocked),
				Project: "infra", CreatedAt: daysAgo(8), UpdatedAt: daysAgo(3),
			},
			notes: []string{"Blocked on network team. Following up Thursday."},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Integrate Stripe billing webhooks",
				Description: "Need product sign-off on the cancellation flow before proceeding.",
				Priority:    p(task.PriorityHigh), Status: st(task.StatusBlocked),
				Project: "billing", CreatedAt: daysAgo(14), UpdatedAt: daysAgo(6),
			},
		},

		// --- todo, various priorities and decay ---
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Write OpenAPI spec for v2 endpoints",
				Priority: p(task.PriorityCritical), Status: st(task.StatusTodo),
				Project: "api", DueDate: future(3),
				CreatedAt: daysAgo(2), UpdatedAt: daysAgo(2),
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Fix memory leak in websocket handler",
				Description: "Goroutines not cleaned up on disconnect. Tracked in profiling run from last sprint.",
				Priority:    p(task.PriorityCritical), Status: st(task.StatusTodo),
				Project: "api", DueDate: past(1), // overdue
				CreatedAt: daysAgo(10), UpdatedAt: daysAgo(4), // subtle decay
			},
			notes: []string{"Reproduced locally. Goroutine count climbs ~200/hr under load."},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Add rate limiting to public endpoints",
				Priority: p(task.PriorityHigh), Status: st(task.StatusTodo),
				Project: "api", DueDate: future(14),
				CreatedAt: daysAgo(3), UpdatedAt: daysAgo(3),
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Set up error alerting in Datadog",
				Priority: p(task.PriorityHigh), Status: st(task.StatusTodo),
				Project: "infra", CreatedAt: daysAgo(18), UpdatedAt: daysAgo(11), // moderate decay
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Update dependencies to latest stable",
				Priority: p(task.PriorityMedium), Status: st(task.StatusTodo),
				Project: "infra", CreatedAt: daysAgo(22), UpdatedAt: daysAgo(17), // warning decay
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Document onboarding flow for new engineers",
				Priority: p(task.PriorityMedium), Status: st(task.StatusTodo),
				Project: "docs", CreatedAt: daysAgo(45), UpdatedAt: daysAgo(35), // alert decay
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Add search to admin dashboard",
				Priority: p(task.PriorityMedium), Status: st(task.StatusTodo),
				Project: "dashboard", DueDate: future(21),
				CreatedAt: daysAgo(1), UpdatedAt: daysAgo(1),
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Audit S3 bucket permissions",
				Priority: p(task.PriorityLow), Status: st(task.StatusTodo),
				Project: "infra", CreatedAt: daysAgo(60), UpdatedAt: daysAgo(42), // alert decay
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Clean up unused feature flags",
				Priority: p(task.PriorityLow), Status: st(task.StatusTodo),
				Project: "api", CreatedAt: daysAgo(5), UpdatedAt: daysAgo(5),
			},
		},

		// --- done, spread across last 7 days for sparkline ---
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Fix login redirect bug on mobile",
				Priority: p(task.PriorityCritical), Status: st(task.StatusDone),
				Project: "auth", CreatedAt: daysAgo(9), UpdatedAt: daysAgo(0),
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Increase test coverage for billing module",
				Priority: p(task.PriorityMedium), Status: st(task.StatusDone),
				Project: "billing", CreatedAt: daysAgo(12), UpdatedAt: daysAgo(0),
			},
			notes: []string{"Got to 84% coverage. Good enough for now."},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Switch CI to GitHub Actions",
				Priority: p(task.PriorityHigh), Status: st(task.StatusDone),
				Project: "infra", CreatedAt: daysAgo(15), UpdatedAt: daysAgo(1),
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Add 2FA support to admin accounts",
				Priority: p(task.PriorityHigh), Status: st(task.StatusDone),
				Project: "auth", CreatedAt: daysAgo(20), UpdatedAt: daysAgo(1),
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Optimise slow dashboard query",
				Priority: p(task.PriorityHigh), Status: st(task.StatusDone),
				Project: "dashboard", CreatedAt: daysAgo(8), UpdatedAt: daysAgo(2),
			},
			notes: []string{"Added composite index on (user_id, created_at). Query went from 4s → 40ms."},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Write migration script for users table",
				Priority: p(task.PriorityMedium), Status: st(task.StatusDone),
				Project: "api", CreatedAt: daysAgo(11), UpdatedAt: daysAgo(3),
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Set up log aggregation with Loki",
				Priority: p(task.PriorityMedium), Status: st(task.StatusDone),
				Project: "infra", CreatedAt: daysAgo(18), UpdatedAt: daysAgo(4),
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Remove deprecated v1 API routes",
				Priority: p(task.PriorityLow), Status: st(task.StatusDone),
				Project: "api", CreatedAt: daysAgo(30), UpdatedAt: daysAgo(5),
			},
		},
		{
			item: task.Item{
				Kind: task.KindTask, Title: "Update privacy policy page",
				Priority: p(task.PriorityLow), Status: st(task.StatusDone),
				Project: "docs", CreatedAt: daysAgo(22), UpdatedAt: daysAgo(6),
			},
		},

		// --- ideas ---
		{
			item: task.Item{
				Kind: task.KindIdea, Title: "Offline mode with local-first sync",
				Description: "Use CRDTs or operational transforms so the app works without a network connection and syncs when reconnected.",
				Project:     "api", CreatedAt: daysAgo(7), UpdatedAt: daysAgo(7),
			},
		},
		{
			item: task.Item{
				Kind: task.KindIdea, Title: "Per-user feature flag overrides in admin",
				Description: "Let support staff enable experimental features for specific accounts without a deploy.",
				Project:     "dashboard", CreatedAt: daysAgo(3), UpdatedAt: daysAgo(3),
			},
		},
		{
			item: task.Item{
				Kind: task.KindIdea, Title: "Webhook delivery retry with exponential backoff",
				Project: "billing", CreatedAt: daysAgo(14), UpdatedAt: daysAgo(14),
			},
		},
	}

	for i := range specs {
		item := &specs[i].item
		if err := insertItemWithTimestamps(s, item); err != nil {
			return fmt.Errorf("seed item %q: %w", item.Title, err)
		}
		for _, content := range specs[i].notes {
			note := &task.Note{
				ItemID:  item.ID,
				Content: content,
			}
			if err := s.AddNote(note); err != nil {
				return fmt.Errorf("seed note for %q: %w", item.Title, err)
			}
		}
	}

	return nil
}

// insertItemWithTimestamps inserts an item while preserving the CreatedAt/UpdatedAt
// values set in the spec, bypassing the store's auto-timestamp behaviour.
// It uses CreateItem (which overwrites timestamps) then updates the row directly.
func insertItemWithTimestamps(s store.Store, item *task.Item) error {
	created := item.CreatedAt
	updated := item.UpdatedAt

	// CreateItem stamps now on both timestamps; we fix them up below.
	if err := s.CreateItem(item); err != nil {
		return err
	}

	// Patch timestamps via UpdateItem by temporarily swapping them.
	// UpdateItem only sets updated_at = now, so we use direct DB access — but
	// we only have the Store interface. Workaround: call UpdateItem then fix via
	// a second pass using the same interface with a fake UpdatedAt.
	// Since we can't set UpdatedAt through the public interface, use a type assertion
	// to reach the underlying SQLite store for a direct exec.
	type rawExecer interface {
		ExecRaw(query string, args ...any) error
	}
	if re, ok := s.(rawExecer); ok {
		return re.ExecRaw(
			`UPDATE items SET created_at = ?, updated_at = ? WHERE id = ?`,
			created.Unix(), updated.Unix(), item.ID,
		)
	}

	// Fallback: ExecRaw not available (shouldn't happen with SQLiteStore).
	// Just leave timestamps as-is; data is still useful, just decay won't be accurate.
	return nil
}
