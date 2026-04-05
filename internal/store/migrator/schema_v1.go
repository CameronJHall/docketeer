package migrator

// AddVersion1Migrations adds the initial schema migrations.
func AddVersion1Migrations(mg *Migrator) {
	itemsTable := Table{
		Name: "items",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "kind", Type: DB_Varchar, Length: 10, Nullable: false},
			{Name: "title", Type: DB_Text, Nullable: false},
			{Name: "description", Type: DB_Text, Default: "''", Nullable: false},
			{Name: "priority", Type: DB_Integer, Nullable: true},
			{Name: "status", Type: DB_Varchar, Length: 20, Nullable: true},
			{Name: "project", Type: DB_Text, Default: "''", Nullable: false},
			{Name: "due_date", Type: DB_BigInt, Nullable: true},
			{Name: "created_at", Type: DB_BigInt, Nullable: false},
			{Name: "updated_at", Type: DB_BigInt, Nullable: false},
		},
	}

	mg.AddMigration("create items table", NewAddTableMigration(itemsTable))

	checkConstraintSQL := NewAlterTableMigration("items").
		Name("add postgres kind check constraint").
		Postgres(`ALTER TABLE "items" ADD CONSTRAINT kind_check CHECK (kind IN ('task', 'idea'))`).
		SQLite("")

	mg.AddMigration("add postgres kind check constraint", checkConstraintSQL)

	statusCheckSQL := NewAlterTableMigration("items").
		Name("add postgres status check constraint").
		Postgres(`ALTER TABLE "items" ADD CONSTRAINT status_check CHECK (status IS NULL OR status IN ('todo', 'in_progress', 'done', 'blocked'))`).
		SQLite("")

	mg.AddMigration("add postgres status check constraint", statusCheckSQL)

	notesTable := Table{
		Name: "notes",
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "item_id", Type: DB_BigInt, Nullable: false},
			{Name: "content", Type: DB_Text, Nullable: false},
			{Name: "created_at", Type: DB_BigInt, Nullable: false},
		},
	}

	mg.AddMigration("create notes table", NewAddTableMigration(notesTable))

	itemsUpdatedAtIndex := &Index{
		Cols: []string{"updated_at"},
		Type: NormalIndex,
	}
	mg.AddMigration("create items updated_at index", NewAddIndexMigration(itemsTable, itemsUpdatedAtIndex))

	notesItemIDIndex := &Index{
		Cols: []string{"item_id"},
		Type: NormalIndex,
	}
	mg.AddMigration("create notes item_id index", NewAddIndexMigration(notesTable, notesItemIDIndex))

	addForeignKeySQL := NewAlterTableMigration("notes").
		Name("add postgres foreign key constraint").
		Postgres(`ALTER TABLE "notes" ADD CONSTRAINT notes_item_id_fkey FOREIGN KEY ("item_id") REFERENCES "items"("id") ON DELETE CASCADE`).
		SQLite("")

	mg.AddMigration("add postgres foreign key constraint", addForeignKeySQL)
}
