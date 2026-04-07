package migrator

import (
	"strings"
)

// SQLiteDialect implements Dialect for SQLite.
type SQLiteDialect struct {
	BaseDialect
}

func NewSQLiteDialect() Dialect {
	d := &SQLiteDialect{}
	d.dialect = d
	d.driverName = SQLite
	return d
}

func (d *SQLiteDialect) Quote(name string) string {
	return "`" + name + "`"
}

func (d *SQLiteDialect) BindVar(_ int) string {
	return "?"
}

func (d *SQLiteDialect) SQLType(col *Column) string {
	switch col.Type {
	case DB_BigInt, DB_Integer:
		return "INTEGER"
	case DB_Varchar, DB_Text:
		return "TEXT"
	case DB_DateTime:
		return "DATETIME"
	default:
		return col.Type
	}
}

func (d *SQLiteDialect) CreateTableSQL(table *Table) string {
	sql := d.BaseDialect.CreateTableSQL(table)

	// Remove duplicate AUTOINCREMENT if present
	sql = strings.Replace(sql, "AUTOINCREMENT AUTOINCREMENT", "AUTOINCREMENT", -1)

	return sql
}

func (d *SQLiteDialect) ColString(col *Column) string {
	sql := d.Quote(col.Name) + " " + d.SQLType(col) + " "

	if col.IsPrimaryKey {
		sql += "PRIMARY KEY "
		if col.IsAutoIncrement {
			sql += "AUTOINCREMENT "
		}
	}

	if !col.Nullable && !col.IsPrimaryKey {
		sql += "NOT NULL "
	}

	if col.Default != "" {
		sql += "DEFAULT " + col.Default + " "
	}

	return strings.TrimSpace(sql)
}
