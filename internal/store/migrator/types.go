package migrator

// Type constants for database-agnostic schema definitions.
const (
	DB_BigInt   = "BIGINT"
	DB_Integer  = "INTEGER"
	DB_Text     = "TEXT"
	DB_Varchar  = "VARCHAR"
	DB_DateTime = "DATETIME"
)

// Table represents a database table definition.
type Table struct {
	Name    string
	Columns []*Column
	Indices []*Index
}

// Column represents a table column definition.
type Column struct {
	Name            string
	Type            string
	Length          int
	IsPrimaryKey    bool
	IsAutoIncrement bool
	Nullable        bool
	Default         string
}

// Index represents a table index definition.
type Index struct {
	Name string
	Type int // 1=normal, 2=unique
	Cols []string
}

// IndexType constants.
const (
	NormalIndex = iota + 1
	UniqueIndex
)

// XName generates a unique index name based on table and columns.
func (idx *Index) XName(tableName string) string {
	if idx.Name != "" {
		return idx.Name
	}

	prefix := "IDX_"
	if idx.Type == UniqueIndex {
		prefix = "UQE_"
	}
	return prefix + tableName + "_" + join(idx.Cols, "_")
}

func join(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}
