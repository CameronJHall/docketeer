package migrator

import (
	"fmt"
	"strings"
)

// PostgresDialect implements Dialect for PostgreSQL.
type PostgresDialect struct {
	BaseDialect
}

func NewPostgresDialect() Dialect {
	d := &PostgresDialect{}
	d.dialect = d
	d.driverName = Postgres
	return d
}

func (d *PostgresDialect) Quote(name string) string {
	return "\"" + name + "\""
}

func (d *PostgresDialect) BindVar(n int) string {
	return "$" + fmt.Sprintf("%d", n)
}

func (d *PostgresDialect) SQLType(col *Column) string {
	switch col.Type {
	case DB_BigInt:
		if col.IsAutoIncrement {
			return "BIGSERIAL"
		}
		return "BIGINT"
	case DB_Integer:
		return "INTEGER"
	case DB_Varchar:
		if col.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", col.Length)
		}
		return "VARCHAR"
	case DB_Text:
		return "TEXT"
	case DB_DateTime:
		return "TIMESTAMPTZ"
	default:
		return col.Type
	}
}

func (d *PostgresDialect) CreateIndexSQL(tableName string, idx *Index) string {
	quote := d.Quote
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

func (d *PostgresDialect) DropTable(tableName string) string {
	return d.BaseDialect.DropTable(tableName)
}

func (d *PostgresDialect) DropIndexSQL(tableName string, idx *Index) string {
	return d.BaseDialect.DropIndexSQL(tableName, idx)
}

func (d *PostgresDialect) ColString(col *Column) string {
	sql := d.Quote(col.Name) + " " + d.SQLType(col) + " "

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

func (d *PostgresDialect) ColStringNoPk(col *Column) string {
	sql := d.Quote(col.Name) + " " + d.SQLType(col) + " "

	if !col.Nullable {
		sql += "NOT NULL "
	}

	if col.Default != "" {
		sql += "DEFAULT " + col.Default + " "
	}

	return strings.TrimSpace(sql)
}
