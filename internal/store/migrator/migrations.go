package migrator

// Migration interface for all migration types.
type Migration interface {
	SQL(dialect Dialect) string
	Id() string
	SetId(id string)
}

// MigrationBase provides common migration functionality.
type MigrationBase struct {
	id string
}

func (m *MigrationBase) Id() string {
	return m.id
}

func (m *MigrationBase) SetId(id string) {
	m.id = id
}

// AddTableMigration creates a new table.
type AddTableMigration struct {
	MigrationBase
	table Table
}

func NewAddTableMigration(table Table) *AddTableMigration {
	return &AddTableMigration{table: table}
}

func (m *AddTableMigration) SQL(d Dialect) string {
	return d.CreateTableSQL(&m.table)
}

// AddIndexMigration adds an index to an existing table.
type AddIndexMigration struct {
	MigrationBase
	tableName string
	index     *Index
}

func NewAddIndexMigration(table Table, index *Index) *AddIndexMigration {
	return &AddIndexMigration{tableName: table.Name, index: index}
}

func (m *AddIndexMigration) SQL(d Dialect) string {
	return d.CreateIndexSQL(m.tableName, m.index)
}

// DropIndexMigration removes an index from a table.
type DropIndexMigration struct {
	MigrationBase
	tableName string
	index     *Index
}

func NewDropIndexMigration(table Table, index *Index) *DropIndexMigration {
	return &DropIndexMigration{tableName: table.Name, index: index}
}

func (m *DropIndexMigration) SQL(d Dialect) string {
	return d.DropIndexSQL(m.tableName, m.index)
}

// DropTableMigration removes a table.
type DropTableMigration struct {
	MigrationBase
	tableName string
}

func NewDropTableMigration(tableName string) *DropTableMigration {
	return &DropTableMigration{tableName: tableName}
}

func (m *DropTableMigration) SQL(d Dialect) string {
	return d.DropTable(m.tableName)
}

// AddColumnMigration adds a column to an existing table.
type AddColumnMigration struct {
	MigrationBase
	tableName string
	column    *Column
}

func NewAddColumnMigration(tableName string, col *Column) *AddColumnMigration {
	return &AddColumnMigration{tableName: tableName, column: col}
}

func (m *AddColumnMigration) SQL(d Dialect) string {
	return d.AddColumnSQL(m.tableName, m.column)
}

// RawSQLMigration allows dialect-specific SQL for complex operations.
type RawSQLMigration struct {
	MigrationBase
	sql map[string]string
}

func NewRawSQLMigration() *RawSQLMigration {
	return &RawSQLMigration{sql: make(map[string]string)}
}

func (m *RawSQLMigration) SQL(d Dialect) string {
	if sql, ok := m.sql[d.DriverName()]; ok {
		return sql
	}
	return ""
}

func (m *RawSQLMigration) SQLite(sql string) *RawSQLMigration {
	m.sql[SQLite] = sql
	return m
}

func (m *RawSQLMigration) Postgres(sql string) *RawSQLMigration {
	m.sql[Postgres] = sql
	return m
}

func (m *RawSQLMigration) IfPostgres(sql string) *RawSQLMigration {
	return m.Postgres(sql)
}

func (m *RawSQLMigration) IfSQLite(sql string) *RawSQLMigration {
	return m.SQLite(sql)
}

// AlterTableMigration allows custom table alterations.
type AlterTableMigration struct {
	RawSQLMigration
	tableName string
}

func NewAlterTableMigration(tableName string) *AlterTableMigration {
	return &AlterTableMigration{
		RawSQLMigration: *NewRawSQLMigration(),
		tableName:       tableName,
	}
}

func (m *AlterTableMigration) Table(tableName string) *AlterTableMigration {
	m.tableName = tableName
	return m
}

func (m *AlterTableMigration) Name(name string) *AlterTableMigration {
	if m.id == "" {
		m.id = name
	}
	return m
}
