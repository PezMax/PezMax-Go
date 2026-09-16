package codegen

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

var (
	tableIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	// systemTablePrefixes are framework tables that must never be imported.
	systemTablePrefixes = []string{"goadmin_", "kadmin_"}
)

var errTableNotFound = errors.New("table not found")

// introspector reads PostgreSQL schema metadata through the db connection.
type introspector struct {
	conn db.Connection
}

func newIntrospector(conn db.Connection) *introspector {
	return &introspector{conn: conn}
}

func (i *introspector) validTableName(name string) bool {
	return tableIdentifierPattern.MatchString(name)
}

func isSystemTable(name string) bool {
	for _, prefix := range systemTablePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// listCandidateTables lists user tables in the public schema, excluding
// framework tables.
func (i *introspector) listCandidateTables(keyword string) ([]CandidateTable, error) {
	rows, err := i.conn.Query(`SELECT tablename FROM pg_catalog.pg_tables
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY tablename`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	candidates := make([]CandidateTable, 0, len(rows))
	for _, row := range rows {
		name := textOf(row["tablename"])
		if name == "" || isSystemTable(name) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(keyword)) {
			continue
		}
		candidates = append(candidates, CandidateTable{Name: name})
	}
	return candidates, nil
}

type introspectedColumn struct {
	Name          string
	DataType      string
	UDTName       string
	Nullable      bool
	ColumnDefault string
	MaxLength     int64
	Comment       string
	PrimaryKey    bool
}

func (i *introspector) describeColumns(table string) ([]introspectedColumn, error) {
	if !i.validTableName(table) {
		return nil, fmt.Errorf("invalid table name %q", table)
	}
	rows, err := i.conn.Query(`SELECT column_name, data_type, udt_name, is_nullable, column_default, character_maximum_length
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = ?
		ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, fmt.Errorf("describe columns of %s: %w", table, err)
	}
	if len(rows) == 0 {
		return nil, errTableNotFound
	}
	pkNames, err := i.primaryKeyColumns(table)
	if err != nil {
		return nil, err
	}
	comments, err := i.columnComments(table)
	if err != nil {
		return nil, err
	}
	columns := make([]introspectedColumn, 0, len(rows))
	for _, row := range rows {
		name := textOf(row["column_name"])
		columns = append(columns, introspectedColumn{
			Name:          name,
			DataType:      textOf(row["data_type"]),
			UDTName:       textOf(row["udt_name"]),
			Nullable:      textOf(row["is_nullable"]) == "YES",
			ColumnDefault: textOf(row["column_default"]),
			MaxLength:     intOf(row["character_maximum_length"]),
			Comment:       comments[name],
			PrimaryKey:    pkNames[name],
		})
	}
	return columns, nil
}

func (i *introspector) primaryKeyColumns(table string) (map[string]bool, error) {
	rows, err := i.conn.Query(`SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema = 'public' AND tc.table_name = ? AND tc.constraint_type = 'PRIMARY KEY'`, table)
	if err != nil {
		return nil, fmt.Errorf("describe primary key of %s: %w", table, err)
	}
	names := make(map[string]bool, len(rows))
	for _, row := range rows {
		names[textOf(row["column_name"])] = true
	}
	return names, nil
}

func (i *introspector) columnComments(table string) (map[string]string, error) {
	rows, err := i.conn.Query(`SELECT a.attname AS column_name, col_description(a.attrelid, a.attnum) AS comment
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class cls ON cls.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = cls.relnamespace
		WHERE n.nspname = 'public' AND cls.relname = ? AND a.attnum > 0 AND NOT a.attisdropped`, table)
	if err != nil {
		return nil, fmt.Errorf("describe comments of %s: %w", table, err)
	}
	comments := make(map[string]string, len(rows))
	for _, row := range rows {
		comments[textOf(row["column_name"])] = textOf(row["comment"])
	}
	return comments, nil
}
