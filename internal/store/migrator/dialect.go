package migrator

import (
	"fmt"
	"strings"
)

// Driver names.
const (
	SQLite   = "sqlite3"
	Postgres = "postgres"
)

// Dialect interface for database-specific SQL generation.
type Dialect interface {
	DriverName() string
	Quote(name string) string
	BindVar(n int) string
	SQLType(col *Column) string
	CreateTableSQL(table *Table) string
	CreateIndexSQL(tableName string, idx *Index) string
	AddColumnSQL(tableName string, col *Column) string
	DropTable(tableName string) string
	DropIndexSQL(tableName string, idx *Index) string
	ColString(col *Column) string
	ColStringNoPk(col *Column) string
}

// BaseDialect provides default implementations for common dialect methods.
type BaseDialect struct {
	dialect    Dialect
	driverName string
}

func (b *BaseDialect) DriverName() string {
	return b.driverName
}

func (b *BaseDialect) CreateTableSQL(table *Table) string {
	sql := "CREATE TABLE " + b.dialect.Quote(table.Name) + " (\n"

	for i, col := range table.Columns {
		if col.IsPrimaryKey {
			sql += "   " + b.dialect.ColString(col)
		} else {
			sql += "   " + b.dialect.ColStringNoPk(col)
		}

		if i < len(table.Columns)-1 {
			sql += ","
		}
		sql += "\n"
	}

	sql += ")"
	return sql
}

func (b *BaseDialect) CreateIndexSQL(tableName string, idx *Index) string {
	quote := b.dialect.Quote
	var unique string
	if idx.Type == UniqueIndex {
		unique = " UNIQUE"
	}

	idxName := idx.XName(tableName)
	quotedCols := make([]string, 0, len(idx.Cols))
	for _, col := range idx.Cols {
		quotedCols = append(quotedCols, quote(col))
	}

	return fmt.Sprintf("CREATE%s INDEX %s ON %s (%s);",
		unique, quote(idxName), quote(tableName), join(quotedCols, ", "))
}

func (b *BaseDialect) DropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", b.dialect.Quote(tableName))
}

func (b *BaseDialect) DropIndexSQL(tableName string, idx *Index) string {
	quote := b.dialect.Quote
	idxName := idx.XName(tableName)
	return fmt.Sprintf("DROP INDEX IF EXISTS %s;", quote(idxName))
}

func (b *BaseDialect) AddColumnSQL(tableName string, col *Column) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", b.dialect.Quote(tableName), col.String(b.dialect))
}

func (b *BaseDialect) ColString(col *Column) string {
	sql := b.dialect.Quote(col.Name) + " "
	sql += b.dialect.SQLType(col) + " "

	if col.IsPrimaryKey {
		sql += "PRIMARY KEY "
	}

	if !col.Nullable && !col.IsPrimaryKey {
		sql += "NOT NULL "
	}

	if col.Default != "" {
		sql += "DEFAULT " + col.Default + " "
	}

	return strings.TrimSpace(sql)
}

func (b *BaseDialect) ColStringNoPk(col *Column) string {
	sql := b.dialect.Quote(col.Name) + " "
	sql += b.dialect.SQLType(col) + " "

	if !col.Nullable {
		sql += "NOT NULL "
	}

	if col.Default != "" {
		sql += "DEFAULT " + col.Default + " "
	}

	return strings.TrimSpace(sql)
}

// PrimaryKeys returns the names of primary key columns for this table.
func (t *Table) PrimaryKeys() []string {
	var pkList []string
	for _, col := range t.Columns {
		if col.IsPrimaryKey {
			pkList = append(pkList, col.Name)
		}
	}
	return pkList
}

// String returns the column definition with or without primary key info.
func (col *Column) String(d Dialect) string {
	if col.IsPrimaryKey {
		return d.ColString(col)
	}
	return d.ColStringNoPk(col)
}
