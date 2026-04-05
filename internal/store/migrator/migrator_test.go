package migrator

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigratorSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	dialect := NewSQLiteDialect()
	m := NewMigrator(db, dialect, "schema_migrations")
	AddVersion1Migrations(m)

	if err := m.Run(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify tables exist
	var count int
	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='items'").Scan(&count)
	if count != 1 {
		t.Error("expected items table to exist")
	}

	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='notes'").Scan(&count)
	if count != 1 {
		t.Error("expected notes table to exist")
	}

	// Verify indexes exist
	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='IDX_items_updated_at'").Scan(&count)
	if count != 1 {
		t.Error("expected items updated_at index to exist")
	}

	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='IDX_notes_item_id'").Scan(&count)
	if count != 1 {
		t.Error("expected notes item_id index to exist")
	}

	// Verify migration log table exists
	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&count)
	if count != 1 {
		t.Error("expected schema_migrations table to exist")
	}

	// Verify idempotency - running again should not fail
	if err := m.Run(); err != nil {
		t.Errorf("second migration run should be idempotent: %v", err)
	}
}

func TestMigratorDialects(t *testing.T) {
	t.Run("SQLite dialect", func(t *testing.T) {
		d := NewSQLiteDialect()
		if d.DriverName() != "sqlite3" {
			t.Errorf("expected driver name 'sqlite3', got %s", d.DriverName())
		}

		if d.Quote("table") != "`table`" {
			t.Errorf("expected quoted name `table`, got %s", d.Quote("table"))
		}

		if d.BindVar(1) != "?" {
			t.Errorf("expected bind var '?', got %s", d.BindVar(1))
		}

		col := &Column{Name: "id", Type: DB_BigInt, IsAutoIncrement: true, IsPrimaryKey: true}
		sqlType := d.SQLType(col)
		if sqlType != "INTEGER" {
			t.Errorf("expected INTEGER type, got %s", sqlType)
		}
	})

	t.Run("Postgres dialect", func(t *testing.T) {
		d := NewPostgresDialect()
		if d.DriverName() != "postgres" {
			t.Errorf("expected driver name 'postgres', got %s", d.DriverName())
		}

		if d.Quote("table") != `"table"` {
			t.Errorf("expected quoted name \"table\", got %s", d.Quote("table"))
		}

		if d.BindVar(1) != "$1" {
			t.Errorf("expected bind var '$1', got %s", d.BindVar(1))
		}

		col := &Column{Name: "id", Type: DB_BigInt, IsAutoIncrement: true, IsPrimaryKey: true}
		sqlType := d.SQLType(col)
		if sqlType != "BIGSERIAL" {
			t.Errorf("expected BIGSERIAL type, got %s", sqlType)
		}

		col2 := &Column{Name: "name", Type: DB_Varchar, Length: 255}
		sqlType2 := d.SQLType(col2)
		if sqlType2 != "VARCHAR(255)" {
			t.Errorf("expected VARCHAR(255) type, got %s", sqlType2)
		}
	})
}

func TestMigratorRawSQLMigration(t *testing.T) {
	mig := NewRawSQLMigration()

	mig.SQLite("SQLite SQL")
	mig.Postgres("Postgres SQL")

	if mig.SQL(NewSQLiteDialect()) != "SQLite SQL" {
		t.Error("expected SQLite SQL")
	}

	if mig.SQL(NewPostgresDialect()) != "Postgres SQL" {
		t.Error("expected Postgres SQL")
	}
}

func TestMigratorAddTableMigration(t *testing.T) {
	table := Table{
		Name: "test_table",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "name", Type: DB_Text, Nullable: false},
		},
	}

	mig := NewAddTableMigration(table)

	sql := mig.SQL(NewSQLiteDialect())
	if sql == "" {
		t.Error("expected SQL to be generated")
	}

	if sql[:13] != "CREATE TABLE " {
		t.Errorf("expected CREATE TABLE statement, got: %s", sql)
	}
}

func TestMigratorAddIndexMigration(t *testing.T) {
	table := Table{
		Name: "test_table",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "name", Type: DB_Text, Nullable: false},
		},
	}

	index := &Index{
		Cols: []string{"name"},
		Type: NormalIndex,
	}

	mig := NewAddIndexMigration(table, index)
	sql := mig.SQL(NewSQLiteDialect())

	if sql == "" {
		t.Error("expected SQL to be generated")
	}

	if sql[:13] != "CREATE INDEX " {
		t.Errorf("expected CREATE INDEX statement, got: %s", sql)
	}
}
