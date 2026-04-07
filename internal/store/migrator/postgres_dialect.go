package migrator

import (
	"fmt"
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
