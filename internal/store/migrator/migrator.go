package migrator

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Migrator manages and executes database migrations.
type Migrator struct {
	db                    *sql.DB
	dialect               Dialect
	migrations            []migrationEntry
	migrationLogTableName string
}

type migrationEntry struct {
	name      string
	migration Migration
}

// NewMigrator creates a new migrator for the given database and dialect.
func NewMigrator(db *sql.DB, dialect Dialect, migrationLogTableName string) *Migrator {
	m := &Migrator{
		db:                    db,
		dialect:               dialect,
		migrations:            make([]migrationEntry, 0),
		migrationLogTableName: migrationLogTableName,
	}
	return m
}

// initMigrationLogTable initializes the migration log table.
// This must be called before running migrations.
func (m *Migrator) initMigrationLogTable() error {
	table := Table{
		Name: m.migrationLogTableName,
		Columns: []*Column{
			{Name: "id", Type: DB_BigInt, IsPrimaryKey: true, IsAutoIncrement: true},
			{Name: "migration_id", Type: DB_Varchar, Length: 255, Nullable: false},
			{Name: "sql", Type: DB_Text, Nullable: true},
			{Name: "success", Type: DB_BigInt, Nullable: false, Default: "0"},
			{Name: "error", Type: DB_Text, Nullable: true},
			{Name: "timestamp", Type: DB_BigInt, Nullable: false},
		},
		Indices: []*Index{{Cols: []string{"migration_id"}, Type: UniqueIndex}},
	}

	createSQL := m.dialect.CreateTableSQL(&table)
	if _, err := m.db.Exec(createSQL); err != nil {
		return fmt.Errorf("create migration log table: %w", err)
	}

	indexSQL := m.dialect.CreateIndexSQL(m.migrationLogTableName, table.Indices[0])
	if _, err := m.db.Exec(indexSQL); err != nil {
		return fmt.Errorf("create migration log index: %w", err)
	}

	return nil
}

// AddMigration registers a migration to be executed.
func (m *Migrator) AddMigration(name string, migration Migration) {
	migration.SetId(name)
	m.migrations = append(m.migrations, migrationEntry{name: name, migration: migration})
}

// Run executes all registered migrations that haven't been run yet.
func (m *Migrator) Run() error {
	if err := m.initMigrationLogTable(); err != nil {
		return err
	}

	for _, entry := range m.migrations {
		if err := m.runMigration(entry); err != nil {
			return fmt.Errorf("migration %s: %w", entry.name, err)
		}
	}
	return nil
}

func (m *Migrator) runMigration(entry migrationEntry) error {
	alreadyRun, err := m.isMigrationRun(entry.migration.Id())
	if err != nil {
		return fmt.Errorf("check migration: %w", err)
	}
	if alreadyRun {
		return nil
	}

	sql := entry.migration.SQL(m.dialect)
	if sql != "" {
		if err := m.executeSQL(sql); err != nil {
			return err
		}
	}

	return m.logMigration(entry.migration.Id(), sql, true, "")
}

func (m *Migrator) executeSQL(sql string) error {
	statements := strings.Split(sql, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		stmt += ";"

		_, err := m.db.Exec(stmt)
		if err != nil {
			return fmt.Errorf("executing SQL: %w", err)
		}
	}
	return nil
}

func (m *Migrator) isMigrationRun(migrationID string) (bool, error) {
	quoted := m.dialect.Quote
	querySQL := fmt.Sprintf("SELECT 1 FROM %s WHERE %s = %s",
		quoted(m.migrationLogTableName),
		quoted("migration_id"),
		m.dialect.BindVar(1))

	var exists int
	err := m.db.QueryRow(querySQL, migrationID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (m *Migrator) logMigration(migrationID, sqlStr string, success bool, errMsg string) error {
	quoted := m.dialect.Quote
	table := quoted(m.migrationLogTableName)

	successInt := int64(0)
	if success {
		successInt = 1
	}

	var sqlArg, errArg any
	if sqlStr == "" {
		sqlArg = nil
	} else {
		sqlArg = sqlStr
	}
	if errMsg == "" {
		errArg = nil
	}

	var sqlStmt string
	var args []any

	switch m.dialect.DriverName() {
	case SQLite:
		sqlStmt = fmt.Sprintf(`INSERT INTO %s (%s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?)`,
			table,
			quoted("migration_id"),
			quoted("sql"),
			quoted("success"),
			quoted("error"),
			quoted("timestamp"))
		args = []any{migrationID, sqlArg, successInt, errArg, time.Now().Unix()}
	case Postgres:
		sqlStmt = fmt.Sprintf(`INSERT INTO %s (%s, %s, %s, %s, %s) VALUES ($1, $2, $3, $4, $5)`,
			table,
			quoted("migration_id"),
			quoted("sql"),
			quoted("success"),
			quoted("error"),
			quoted("timestamp"))
		args = []any{migrationID, sqlArg, successInt, errArg, time.Now().Unix()}
	default:
		return fmt.Errorf("unsupported dialect: %s", m.dialect.DriverName())
	}

	_, err := m.db.Exec(sqlStmt, args...)
	return err
}
