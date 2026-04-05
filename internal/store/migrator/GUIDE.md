# Docketeer Migration Framework

A Grafana-inspired migration system for Docketeer that generates SQL automatically using Go code definitions. This eliminates the need to write raw SQL while supporting PostgreSQL and SQLite.

## Philosophy

Instead of writing SQL migrations that must be rewritten for each database, define your schema in Go and let the framework generate the appropriate SQL for each database type.

```go
// You write this (database-agnostic):
AddTableMigration{
    Table: DocketsTable,
}

// Framework generates for SQLite:
CREATE TABLE dockets (id INTEGER PRIMARY KEY, ...);

// Framework generates for PostgreSQL:
CREATE TABLE dockets (id BIGSERIAL PRIMARY KEY, ...);
```

## Architecture

### Components

- **Dialect Interface**: `internal/store/migrator/dialect.go`
  - Abstracts database-specific SQL generation
  - Each database (PostgreSQL, SQLite) implements its own dialect

- **Type System**: `internal/store/migrator/types.go`
  - Database-agnostic types: `DB_BigInt`, `DB_Text`, `DB_Varchar`, `DB_DateTime`
  - Maps correctly to each database type:
    - SQLite: INTEGER → BIGINT, TEXT → TEXT
    - PostgreSQL: BIGINT → BIGINT, VARCHAR → VARCHAR

- **Migration Types**: `internal/store/migrator/migrations.go`
  - `AddTableMigration` - Create new tables
  - `AddIndexMigration` - Add indexes
  - `DropIndexMigration` - Remove indexes  
  - `DropTableMigration` - Remove tables
  - `AddColumnMigration` - Add columns to existing tables
  - `AlterTableMigration` - Raw ALTER TABLE
  - `RawSQLMigration` - Database-specific SQL

- **Migrator**: `internal/store/migrator/migrator.go`
  - Executes migrations in order
  - Logs executed migrations
  - Handles idempotency

- **Schema Versions**: `internal/store/migrator/schema_vX.go`
  - One file per version with migration array
  - Defines complete schema or incremental changes

### How It Works

1. Migration starts, checks `schema_migrations` table exists
2. Loads all schema versions (v1, v2, v3, ...)
3. For each version, executes migrations in order
4. After each migration, logs to `schema_migrations` table
5. Skips already-executed migrations (idempotent)

## Usage

### Adding a New Version

Create a new version file in `internal/store/migrator/schema_vX.go` where X is the next version number.

**Example: Adding tags feature (schema_v2.go)**

```go
package migrator

import (
    "fmt"
    "internal/store" // Adjust import path as needed
)

// SchemaV2 represents the version 2 schema
func SchemaV2() []Migration {
    return []Migration{
        // 1. Create tags table
        AddTableMigration{
            Table: Table{
                Name: "tags",
                Columns: []Column{
                    {Name: "id", Type: DB_BigInt},
                    {Name: "name", Type: DB_Varchar, Length: 255, Unique: true},
                    {Name: "created_at", Type: DB_BigInt, Nullable: true},
                },
            },
            Options: []MigrationField{
                {Field: PrimaryKeyField, Value: "id"},
                {Field: UniqueField, Value: "name"},
            },
        },

        // 2. Create taggings junction table
        AddTableMigration{
            Table: Table{
                Name: "taggings",
                Columns: []Column{
                    {Name: "id", Type: DB_BigInt},
                    {Name: "docket_id", Type: DB_BigInt, ForeignKey: "dockets (id)"},
                    {Name: "tag_id", Type: DB_BigInt, ForeignKey: "tags (id)"},
                    {Name: "created_at", Type: DB_BigInt},
                },
            },
            Options: []MigrationField{
                {Field: PrimaryKeyField, Value: "id"},
                {Field: ForeignKeyField, Value: "docket_id REFERENCES dockets (id) ON DELETE CASCADE"},
                {Field: ForeignKeyField, Value: "tag_id REFERENCES tags (id) ON DELETE CASCADE"},
            },
        },

        // 3. Add indexes for performance
        AddIndexMigration{
            Index: Index{
                TableName: "taggings",
                IndexName: "idx_taggings_docket_id",
                IsUnique:  false,
                Columns:   []Column{{Name: "docket_id", Type: DB_BigInt}},
            },
            Options: []MigrationField{
                {Field: WhereField, Value: fmt.Sprintf(`(docket_id IN (SELECT id FROM "dockets" WHERE "is_deleted" = 0))`)},
            },
        },

        AddIndexMigration{
            Index: Index{
                TableName: "taggings",
                IndexName: "idx_taggings_tag_id",
                IsUnique:  false,
                Columns:   []Column{{Name: "tag_id", Type: DB_BigInt}},
            },
            Options: []MigrationField{
                {Field: WhereField, Value: fmt.Sprintf(`(tag_id IN (SELECT id FROM "tags" WHERE id IN (SELECT DISTINCT tag_id FROM "taggings" WHERE docket_id IS NOT NULL)))`)},
            },
        },
    }
}
```

### Migration Types Reference

#### AddTableMigration

Creates a new table.

```go
AddTableMigration{
    Table: Table{
        Name: "dockets",
        Columns: []Column{
            {Name: "id", Type: DB_BigInt},
            {Name: "title", Type: DB_Varchar, Length: 255},
            {Name: "priority", Type: DB_BigInt},
            {Name: "created_at", Type: DB_BigInt},
            {Name: "deleted_at", Type: DB_BigInt, Nullable: true},
            {Name: "is_deleted", Type: DB_BigInt, Default: "0"},
        },
    },
    Options: []MigrationField{
        {Field: PrimaryKeyField, Value: "id"},
        {Field: UniqueField, Value: "title"},
    },
}
```

#### AddColumnMigration

Adds a column to an existing table. **Important**: Must use string table name (not `store.DocketsTable`).

```go
AddColumnMigration{
    TableName: "dockets", // String name, not Table reference
    Column: Column{
        Name:    "due_date",
        Type:    DB_BigInt,
        Nullable: true,
    },
}
```

Use `AlterTableMigration` or `RawSQLMigration` for more complex column additions:

```go
AlterTableMigration{
    Sql: "ALTER TABLE dockets ADD COLUMN due_date INTEGER DEFAULT 0", // SQLite
}
```

#### AddIndexMigration

Creates an index on one or more columns.

```go
AddIndexMigration{
    Index: Index{
        TableName: "dockets",
        IndexName: "idx_dockets_deleted",
        IsUnique:  false,
        Columns:   []Column{{Name: "is_deleted", Type: DB_BigInt}},
    },
    Options: []MigrationField{
        {Field: WhereField, Value: "(is_deleted = 0)"},
    },
}
```

For PostgreSQL without WHERE clause:

```go
AddIndexMigration{
    Index: Index{
        TableName: "dockets",
        IndexName: "idx_dockets_deleted",
        IsUnique:  false,
        Columns:   []Column{{Name: "is_deleted", Type: DB_BigInt}},
    },
}
```

#### DropIndexMigration

Removes an index.

```go
DropIndexMigration{
    Index: Index{
        TableName: "dockets",
        IndexName: "idx_dockets_deleted",
    },
}
```

#### DropTableMigration

Removes a table.

```go
DropTableMigration{
    TableName: "old_table",
}
```

#### RawSQLMigration

Uses raw, database-specific SQL. **Required** for PostgreSQL CHECK constraints.

```go
// Only works on SQLite
RawSQLMigration{
    Sql:    "ALTER TABLE dockets RENAME TO dockets_old",
    Dialect: "sqlite", // Optional, defaults to current
}

// PostgreSQL CHECK constraint (PostgreSQL only)
RawSQLMigration{
    Sql: `ALTER TABLE dockets ADD CONSTRAINT chk_priority CHECK (priority BETWEEN 0 AND 4)`,
    Dialect: "postgres",
}
```

#### AlterTableMigration

Generic ALTER TABLE for database-specific operations.

```go
AlterTableMigration{
    Sql: "ALTER TABLE dockets RENAME COLUMN 'title' TO 'name'", // SQLite
}
```

### Registering Migrations

Add your version to the migrator in `internal/store/migrator/migrations.go`:

```go
// AllSchemaVersions is the complete list of schema versions
var AllSchemaVersions = []func() []Migration{
    SchemaV1,
    SchemaV2, // Add your version here
    // SchemaV3,
}
```

### Timestamp Handling

**Always use `DB_BigInt` for timestamps.** Store as Unix timestamps.

```go
// ✅ Correct - works on both databases
Column{
    Name: "created_at",
    Type: DB_BigInt, // Unix timestamp as integer
}

// ❌ Wrong - won't work across databases
Column{
    Name: "created_at",
    Type: DB_DateTime, // PostgreSQL only
}
```

The migrator will:
- SQLite: Store as INTEGER
- PostgreSQL: Store as BIGINT

Application code receives timestamps as integers and converts to `time.Time` as needed.

### Type Mapping Reference

| Go Type      | SQLite       | PostgreSQL   | Use Case            |
|--------------|--------------|--------------|---------------------|
| `DB_BigInt`  | INTEGER      | BIGINT       | IDs, timestamps     |
| `DB_Text`    | TEXT         | TEXT         | Long text, notes    |
| `DB_Varchar` | TEXT         | VARCHAR(n)   | String with max len |
| `DB_DateTime`| INTEGER      | TIMESTAMPTZ  | Avoid, use BigInt   |
| `DB_Bool`    | INTEGER      | INTEGER      | 0/1 flags           |

## Best Practices

### 1. One Version File Per Schema Change

- **Good**: `schema_v2.go` for tags feature
- **Bad**: `schema_v2.go` with both tags and calendar

### 2. Use Database-Specific SQL Only When Necessary

- **Good**: Use `AddTableMigration` for table creation
- **Bad**: Write custom SQL for every operation

Use `RawSQLMigration` only when:
- CHECK constraints (PostgreSQL only)
- Complex ALTER TABLE (SQLite doesn't support type changes)
- Database-specific optimizations

### 3. Idempotency is Automatic

The framework tracks executed migrations in `schema_migrations` table. Running migrations multiple times is safe.

```bash
# Run this multiple times - only new migrations execute
go run internal/store/migrator/main.go sqlite file.db
```

### 4. PostgreSQL CHECK Constraints

PostgreSQL supports CHECK constraints, SQLite does not (SQLite 3.35+ has partial support but Docketeer avoids it for simplicity).

Use `RawSQLMigration` with dialect filter:

```go
RawSQLMigration{
    Sql: `ALTER TABLE dockets ADD CONSTRAINT chk_priority CHECK (priority BETWEEN 0 AND 4)`,
    Dialect: "postgres",
}
```

This migration only runs on PostgreSQL databases.

### 5. Testing Migrations

Run the full test suite:

```bash
# All tests (SQLite + PostgreSQL)
go test -v ./internal/...

# Just migrator tests (SQLite only)
go test -v ./internal/store/migrator/

# Just store tests (SQLite + PostgreSQL)
go test -v -tags=integration ./internal/store/
```

### 6. Migration Order

Migrations **must** be added in `AllSchemaVersions` in version order:

```go
var AllSchemaVersions = []func() []Migration{
    SchemaV1, // Must be first
    SchemaV2, // Must come after V1
    SchemaV3, // Must come after V2
}
```

The migrator executes versions sequentially.

### 7. Column Default Values

Use the `Default` field for column defaults:

```go
Column{
    Name:    "is_deleted",
    Type:    DB_BigInt,
    Default: "0", // String literal stored in column
}
```

## Rollbacks

Currently, the framework only supports forward migrations. To rollback:

1. **Database-specific**: Remove version from `AllSchemaVersions`
2. **Manual**: Drop new tables/columns manually
3. **Best practice**: Start from clean database in development

Future: Add `AddDropColumnMigration` and support downgrade operations.

## Common Patterns

### Pattern 1: Foreign Key Relationship

```go
AddTableMigration{
    Table: Table{
        Name: "taggings",
        Columns: []Column{
            {Name: "docket_id", Type: DB_BigInt, ForeignKey: "dockets (id)"},
            {Name: "tag_id", Type: DB_BigInt, ForeignKey: "tags (id)"},
        },
    },
    Options: []MigrationField{
        {Field: ForeignKeyField, Value: "docket_id REFERENCES dockets (id) ON DELETE CASCADE"},
        {Field: ForeignKeyField, Value: "tag_id REFERENCES tags (id) ON DELETE CASCADE"},
    },
}
```

### Pattern 2: Partial Index (PostgreSQL Only)

```go
AddIndexMigration{
    Index: Index{
        TableName: "dockets",
        IndexName: "idx_dockets_active",
        Columns:   []Column{{Name: "is_deleted", Type: DB_BigInt}},
    },
    Options: []MigrationField{
        {Field: WhereField, Value: "(is_deleted = 0)"},
    },
}

// For PostgreSQL-only (use RawSQLMigration)
RawSQLMigration{
    Dialect: "postgres",
    Sql:     `CREATE INDEX idx_dockets_active ON dockets (is_deleted) WHERE is_deleted = 0`,
}
```

### Pattern 3: Conditional Migration

```go
// Only run on SQLite
RawSQLMigration{
    Dialect: "sqlite",
    Sql:     `ALTER TABLE dockets ADD COLUMN new_column INTEGER DEFAULT 0`,
}

// Only run on PostgreSQL
RawSQLMigration{
    Dialect: "postgres",
    Sql:     `ALTER TABLE dockets ADD COLUMN new_column BIGINT DEFAULT 0`,
}
```

## Troubleshooting

### Migration Already Exists

If you see "migration already exists" errors:

1. Check `schema_migrations` table
2. Migration was likely already run
3. Safe to re-run (idempotent)

### Missing Column After Migration

- Verify migration version is in `AllSchemaVersions`
- Check migration executed successfully (query `schema_migrations`)
- Check for syntax errors in migration definition

### Type Mismatches

- Always use `DB_BigInt` for IDs and timestamps
- Use `DB_Varchar` for bounded strings
- Use `DB_Text` for unbounded text
- Avoid `DB_DateTime`

### PostgreSQL-Specific Errors

- CHECK constraints: Use `RawSQLMigration` with `Dialect: "postgres"`
- Type names: Always use framework types (`DB_BigInt`, not `BIGINT`)
- Column length: `DB_Varchar` + `Length` field

## File Structure

```
internal/store/migrator/
├── types.go            # Type constants, Table, Column, Index structs
├── dialect.go          # Dialect interface, BaseDialect
├── sqlite_dialect.go   # SQLite implementation
├── postgres_dialect.go # PostgreSQL implementation
├── migrations.go       # Migration types, AllSchemaVersions
├── migrator.go         # Migration execution, logging
├── migrator_test.go    # Unit tests
├── schema_v1.go        # Current schema (dockets, notes)
├── schema_v2.go        # Your next version
├── schema_v3.go        # Future versions
└── GUIDE.md            # This file
```

```
internal/store/
├── store.go            # Store interface
├── sqlite.go           # SQLite implementation
├── postgres.go         # PostgreSQL implementation
├── helpers.go          # Type conversion helpers
├── sqlite_test.go      # SQLite tests
└── migrator/           # Migration framework (see above)
```

## Reusing Types Across the App

The migration framework uses database-agnostic types for schema definitions, while the application uses Go-native types. Here's how they map:

### Application Types (`internal/task`)

```go
// Internal application model
type Item struct {
    ID          int64
    Kind        ItemKind        // "task" | "idea"
    Title       string
    Description string
    Priority    *Priority       // 1-4 (nil for ideas)
    Status      *Status         // "todo" | "in_progress" | "done" | "blocked"
    Project     string
    DueDate     *time.Time      // Unix timestamp
    CreatedAt   time.Time       // Stored as int64 in DB
    UpdatedAt   time.Time       // Stored as int64 in DB
}
```

### Database Schema Types (`internal/store/migrator`)

```go
// Database schema definition
Column{
    Name: "id",
    Type: DB_BigInt,           // Maps to INTEGER (SQLite) / BIGINT (PostgreSQL)
}

Column{
    Name: "title",
    Type: DB_Varchar,
    Length: 255,               // Maps to TEXT (SQLite) / VARCHAR(255) (PostgreSQL)
}

Column{
    Name: "created_at",
    Type: DB_BigInt,           // Stored as Unix timestamp (int64)
}
```

### Type Conversion Helpers (`internal/store/helpers.go`)

The store layer handles bidirectional conversion transparently:

```go
// helpers.go - converts DB rows to task.Item
func scanItem(s scanner) (*task.Item, error) {
    var item task.Item
    prioritySQL sql.NullInt64    // Database type
    createdUnix int64           // Unix timestamp

    // Scan DB row
    s.Scan(&item.ID, &kindStr, ..., &prioritySQL, ..., &createdUnix)

    // Convert to app types
    item.CreatedAt = time.Unix(createdUnix, 0)
    if prioritySQL.Valid {
        p := task.Priority(prioritySQL.Int64)
        item.Priority = &p
    }

    return &item, nil
}

// Convert app types to DB-compatible types
func priorityToSQL(p *task.Priority) any {
    if p == nil {
        return nil
    }
    return int64(*p)  // int64 works on both SQLite and PostgreSQL
}

func timeToSQL(t *time.Time) any {
    if t == nil {
        return nil
    }
    return t.Unix()  // Convert time.Time to Unix timestamp
}
```

### How It Works Together

```
┌─────────────────────────────────────────────────────────────┐
│ Application Layer                                            │
│ └── internal/task                                           │
│     Item, Note, Priority, Status                           │
│     Uses Go native types (time.Time, int64, string)        │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Store Layer                                                  │
│ └── internal/store                                         │
│     sqlite.go, postgres.go                                 │
│     helpers.go (conversion logic)                          │
│                                                             │
│ Converts:                                                    │
│   time.Time ↔ int64 (Unix timestamp)                       │
│   task.Priority ↔ sql.NullInt64                            │
│   task.Status ↔ sql.NullString                             │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Migration Framework                                          │
│ └── internal/store/migrator                                │
│     Defines schema using DB_BigInt, DB_Varchar, etc.       │
│     Generates database-specific SQL                        │
└─────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

1. **Timestamps as Unix Epoch**: Both databases store timestamps as integers (BIGINT/INTEGER). The store layer converts to/from `time.Time`.

2. **Nullable Fields**: Use `sql.NullInt64` and `sql.NullString` for nullable columns, then convert to pointer types (`*Priority`, `*Status`).

3. **Database-Agnostic Schema**: The migration framework uses abstract types (`DB_BigInt`, `DB_Varchar`) that generate different SQL per database.

4. **Centralized Conversion**: All type conversions happen in `helpers.go`, keeping store implementations clean.

### Adding New Types

If you need a new type (e.g., a calendar feature):

1. **Define in `internal/task`**:
```go
type CalendarEvent struct {
    ID        int64
    ItemID    int64
    Title     string
    StartTime time.Time
    EndTime   time.Time
}
```

2. **Add migration in `schema_vX.go`**:
```go
AddTableMigration{
    Table: Table{
        Name: "calendar_events",
        Columns: []Column{
            {Name: "id", Type: DB_BigInt},
            {Name: "item_id", Type: DB_BigInt, ForeignKey: "items (id)"},
            {Name: "title", Type: DB_Varchar, Length: 255},
            {Name: "start_time", Type: DB_BigInt},
            {Name: "end_time", Type: DB_BigInt},
        },
    },
}
```

3. **Add helpers in `helpers.go`**:
```go
func scanCalendarEvent(s scanner) (*task.CalendarEvent, error) {
    var event task.CalendarEvent
    var startUnix, endUnix int64

    err := s.Scan(&event.ID, &event.ItemID, &event.Title, &startUnix, &endUnix)
    if err != nil {
        return nil, err
    }

    event.StartTime = time.Unix(startUnix, 0)
    event.EndTime = time.Unix(endUnix, 0)

    return &event, nil
}
```

4. **Use in store layer**:
```go
func (s *SQLiteStore) GetCalendarEvent(id int64) (*task.CalendarEvent, error) {
    row := s.db.QueryRow(`SELECT id, item_id, title, start_time, end_time FROM calendar_events WHERE id = ?`, id)
    return scanCalendarEvent(row)
}
```
internal/store/migrator/
├── types.go           # Type constants, Table, Column, Index structs
├── dialect.go         # Dialect interface, BaseDialect
├── sqlite_dialect.go  # SQLite implementation
├── postgres_dialect.go# PostgreSQL implementation
├── migrations.go      # Migration types, AllSchemaVersions
├── migrator.go        # Migration execution, logging
├── migrator_test.go   # Unit tests
├── schema_v1.go       # Current schema (dockets, notes)
├── schema_v2.go       # Your next version
└── schema_v3.go       # Future versions
```

## Adding New Database Types

If you need a new type or change mapping:

1. **Add to types.go**: Add `DB_YourType` constant
2. **Update dialects**:
   - Add to `BaseDialect` (common behavior)
   - Override in `SQLiteDialect` and `PostgresDialect` if needed
3. **Test**: Run full test suite

## FAQ

**Q: Can I migrate data between databases?**
A: Not yet. The current framework focuses on schema changes. Data migration requires manual handling (dump/restore).

**Q: How do I test PostgreSQL migrations locally?**
A: Run `docker-compose -f docker-compose.test.yml up -d` then run integration tests with PostgreSQL env vars set.

**Q: Can I add migrations out of order?**
A: Yes, but add them to `AllSchemaVersions` in version order. The framework will run them sequentially.

**Q: What about downgrades?**
A: Not supported. Remove the version from `AllSchemaVersions` and manually drop tables/columns.

**Q: How do I handle database-specific constraints?**
A: Use `RawSQLMigration` with `Dialect` field to target specific databases.

**Q: Can I modify existing columns?**
A: SQLite cannot modify column types. Use table recreation (drop/create) or `RawSQLMigration` for PostgreSQL-specific changes.

---

**Version**: 1.0  
**Last Updated**: 2026-02-05
