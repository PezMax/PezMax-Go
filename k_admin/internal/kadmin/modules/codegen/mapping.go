package codegen

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// textOf converts a driver value to string.
func textOf(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

// intOf converts a driver value to int64.
func intOf(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case nil:
		return 0
	default:
		return 0
	}
}

// boolOf converts a driver value to bool.
func boolOf(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(typed)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseBool(string(typed))
		return parsed
	default:
		return false
	}
}

// timeText formats a driver time value as a local datetime string.
func timeText(value interface{}) string {
	if value == nil {
		return ""
	}
	if parsed, ok := value.(time.Time); ok {
		return parsed.In(time.Local).Format("2006-01-02 15:04:05")
	}
	text := strings.TrimSpace(textOf(value))
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.In(time.Local).Format("2006-01-02 15:04:05")
		}
	}
	return text
}

// typeMapping maps a PostgreSQL type to Go, TypeScript and form control.
type typeMapping struct {
	GoType  string
	TSType  string
	Control string
}

var postgresTypeMappings = map[string]typeMapping{
	"int2":        {GoType: "int64", TSType: "number", Control: "number"},
	"int4":        {GoType: "int64", TSType: "number", Control: "number"},
	"int8":        {GoType: "int64", TSType: "number", Control: "number"},
	"float4":      {GoType: "float64", TSType: "number", Control: "number"},
	"float8":      {GoType: "float64", TSType: "number", Control: "number"},
	"numeric":     {GoType: "float64", TSType: "number", Control: "number"},
	"bool":        {GoType: "bool", TSType: "boolean", Control: "switch"},
	"varchar":     {GoType: "string", TSType: "string", Control: "input"},
	"bpchar":      {GoType: "string", TSType: "string", Control: "input"},
	"text":        {GoType: "string", TSType: "string", Control: "textarea"},
	"uuid":        {GoType: "string", TSType: "string", Control: "input"},
	"timestamptz": {GoType: "string", TSType: "string", Control: "datetime"},
	"timestamp":   {GoType: "string", TSType: "string", Control: "datetime"},
	"date":        {GoType: "string", TSType: "string", Control: "date"},
	"time":        {GoType: "string", TSType: "string", Control: "time"},
	"timetz":      {GoType: "string", TSType: "string", Control: "time"},
	"json":        {GoType: "string", TSType: "string", Control: "textarea"},
	"jsonb":       {GoType: "string", TSType: "string", Control: "textarea"},
	"bytea":       {GoType: "string", TSType: "string", Control: "input"},
}

func mappingFor(udtName, dataType string, maxLength int64) typeMapping {
	if mapping, ok := postgresTypeMappings[udtName]; ok {
		if udtName == "varchar" && maxLength > 255 {
			mapping.Control = "textarea"
		}
		return mapping
	}
	return typeMapping{GoType: "string", TSType: "string", Control: "input"}
}

var goReservedWords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// goFieldName converts a snake_case column name to a PascalCase Go field name.
func goFieldName(column string) string {
	parts := strings.Split(column, "_")
	name := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		name += string(runes)
	}
	if goReservedWords[strings.ToLower(name)] {
		return name + "Field"
	}
	return name
}

// jsonFieldName converts a snake_case column name to camelCase.
func jsonFieldName(column string) string {
	field := goFieldName(column)
	if field == "" {
		return ""
	}
	runes := []rune(field)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// humanLabel builds a default UI label from a column name.
func humanLabel(column string) string {
	words := strings.Fields(strings.ReplaceAll(column, "_", " "))
	if len(words) == 0 {
		return column
	}
	for index, word := range words {
		runes := []rune(word)
		if len(runes) > 0 && unicode.IsLower(runes[0]) {
			runes[0] = unicode.ToUpper(runes[0])
			words[index] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

// pascalCase converts a snake_case identifier to PascalCase.
func pascalCase(value string) string {
	return goFieldName(value)
}

// inferColumnConfigs derives editable column defaults from introspection.
func inferColumnConfigs(columns []introspectedColumn) []ColumnConfig {
	configs := make([]ColumnConfig, 0, len(columns))
	for _, column := range columns {
		mapping := mappingFor(column.UDTName, column.DataType, column.MaxLength)
		label := strings.TrimSpace(column.Comment)
		if label == "" {
			label = humanLabel(column.Name)
		}
		isTimestamp := column.Name == "created_at" || column.Name == "updated_at"
		config := ColumnConfig{
			Name:      column.Name,
			Label:     label,
			GoType:    mapping.GoType,
			TSType:    mapping.TSType,
			Control:   mapping.Control,
			Listed:    true,
			Queryable: mapping.TSType == "string" && !column.PrimaryKey && !isTimestamp,
			Creatable: !column.PrimaryKey && !isTimestamp,
			Editable:  !column.PrimaryKey && !isTimestamp,
			Required:  !column.Nullable && column.ColumnDefault == "" && !column.PrimaryKey && !isTimestamp,
			IsPK:      column.PrimaryKey,
		}
		configs = append(configs, config)
	}
	return configs
}
