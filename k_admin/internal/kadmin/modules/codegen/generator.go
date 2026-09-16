package codegen

import (
	"embed"
	"fmt"
	"go/format"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// reservedRoutePrefixes are KAdmin API group prefixes that generated modules
// must not collide with.
var reservedRoutePrefixes = map[string]bool{
	"users": true, "rbac": true, "admin-menus": true, "dictionaries": true,
	"system": true, "files": true, "jobs": true, "job-logs": true,
	"login-audits": true, "menu": true, "logs": true, "monitor": true,
	"load-ranking": true, "auth": true, "codegen": true, "departments": true,
	"roles": true, "permissions": true,
}

var (
	moduleNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	classNamePattern    = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)
	routePrefixPattern  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	businessNamePattern = regexp.MustCompile(`^\S+$`)
	reservedModuleNames = map[string]bool{"main": true, "generated": true, "codegen": true}
)

func validateConfig(config TableConfig) error {
	if !moduleNamePattern.MatchString(config.ModuleName) || reservedModuleNames[config.ModuleName] || goReservedWords[config.ModuleName] {
		return fmt.Errorf("invalid module name %q: lowercase identifier required", config.ModuleName)
	}
	if !classNamePattern.MatchString(config.ClassName) {
		return fmt.Errorf("invalid class name %q: PascalCase identifier required", config.ClassName)
	}
	if !routePrefixPattern.MatchString(config.RoutePrefix) || reservedRoutePrefixes[config.RoutePrefix] {
		return fmt.Errorf("invalid route prefix %q: lowercase dash identifier required and must not collide with built-in routes", config.RoutePrefix)
	}
	if !businessNamePattern.MatchString(strings.TrimSpace(config.BusinessName)) {
		return fmt.Errorf("business name must not be empty")
	}
	if len(config.Columns) == 0 {
		return fmt.Errorf("table %s has no columns", config.TableName)
	}
	listed, writable := 0, 0
	seen := make(map[string]bool, len(config.Columns))
	for _, column := range config.Columns {
		if seen[column.Name] {
			return fmt.Errorf("duplicate column %q", column.Name)
		}
		seen[column.Name] = true
		if column.Listed {
			listed++
		}
		if column.Creatable && column.Editable {
			writable++
		}
	}
	if listed == 0 {
		return fmt.Errorf("at least one column must be listed")
	}
	if writable == 0 {
		return fmt.Errorf("at least one column must be creatable and editable")
	}
	return nil
}

// deriveImportNames fills import defaults from the table name.
func deriveImportNames(table string) (module, className, routePrefix, business string) {
	module = strings.ToLower(strings.TrimSpace(table))
	module = strings.TrimPrefix(module, "kadmin_")
	module = strings.TrimPrefix(module, "goadmin_")
	if strings.HasSuffix(module, "s") && len(module) > 1 {
		module = strings.TrimSuffix(module, "s")
	}
	module = strings.ReplaceAll(module, "-", "_")
	if !moduleNamePattern.MatchString(module) || goReservedWords[module] || reservedModuleNames[module] {
		module = "generated_module"
	}
	return module, pascalCase(module), strings.ReplaceAll(module, "_", "-"), humanLabel(table)
}

// templateColumn is a column rendered into the templates.
type templateColumn struct {
	Name      string
	GoName    string
	JSONName  string
	GoType    string
	TSType    string
	Control   string
	Label     string
	Listed    bool
	Queryable bool
	Creatable bool
	Editable  bool
	Required  bool
	IsPK      bool
	Convert   string
	Default   string
	First     bool
}

type registryModule struct {
	Module string
}

type templateModel struct {
	Table                 string
	Module                string
	Class                 string
	Prefix                string
	Business              string
	Columns               []templateColumn
	ListedColumns         []templateColumn
	QueryColumns          []templateColumn
	WritableColumns       []templateColumn
	RequiredColumns       []templateColumn
	RequiredStringColumns []templateColumn
	HasKeyword            bool
	SelectList            string
	InsertColumns         string
	InsertPlaceholders    string
	InsertArgs            string
	UpdateSet             string
	UpdateArgs            string
	KeywordClause         string
	KeywordArgs           string
	Modules               []registryModule
}

func buildTemplateModel(config TableConfig, registryModules []registryModule) (templateModel, error) {
	if err := validateConfig(config); err != nil {
		return templateModel{}, err
	}
	model := templateModel{
		Table:    config.TableName,
		Module:   config.ModuleName,
		Class:    config.ClassName,
		Prefix:   config.RoutePrefix,
		Business: config.BusinessName,
		Modules:  registryModules,
	}
	columns := make([]templateColumn, 0, len(config.Columns))
	listedNames := make([]string, 0, len(config.Columns))
	for _, column := range config.Columns {
		rendered := templateColumn{
			Name:      column.Name,
			GoName:    goFieldName(column.Name),
			JSONName:  jsonFieldName(column.Name),
			GoType:    column.GoType,
			TSType:    column.TSType,
			Control:   column.Control,
			Label:     column.Label,
			Listed:    column.Listed,
			Queryable: column.Queryable,
			Creatable: column.Creatable,
			Editable:  column.Editable,
			Required:  column.Required,
			IsPK:      column.IsPK,
			Convert:   convertFunc(column.Control, column.GoType),
			Default:   defaultLiteral(column.TSType),
		}
		columns = append(columns, rendered)
		if rendered.Listed {
			listedNames = append(listedNames, rendered.Name)
			model.ListedColumns = append(model.ListedColumns, rendered)
		}
		if rendered.Queryable {
			model.QueryColumns = append(model.QueryColumns, rendered)
		}
		if rendered.Creatable && rendered.Editable {
			model.WritableColumns = append(model.WritableColumns, rendered)
		}
		if rendered.Required && rendered.Creatable && rendered.Editable {
			model.RequiredColumns = append(model.RequiredColumns, rendered)
			if rendered.GoType == "string" {
				model.RequiredStringColumns = append(model.RequiredStringColumns, rendered)
			}
		}
		if rendered.GoType == "string" && !rendered.IsPK {
			model.HasKeyword = true
			model.KeywordClause = joinKeywordClause(model.KeywordClause, rendered.Name)
			model.KeywordArgs = joinKeywordArg(model.KeywordArgs)
		}
	}
	if len(model.ListedColumns) > 0 {
		model.ListedColumns[0].First = true
	}
	model.Columns = columns
	model.SelectList = strings.Join(allColumnNames(columns), ", ")
	writableNames := make([]string, 0, len(model.WritableColumns))
	insertPlaceholders := make([]string, 0, len(model.WritableColumns))
	insertArgs := make([]string, 0, len(model.WritableColumns))
	updateSet := make([]string, 0, len(model.WritableColumns))
	updateArgs := make([]string, 0, len(model.WritableColumns))
	for _, column := range model.WritableColumns {
		writableNames = append(writableNames, column.Name)
		insertPlaceholders = append(insertPlaceholders, "?")
		insertArgs = append(insertArgs, payloadArg(column))
		updateSet = append(updateSet, column.Name+" = ?")
		updateArgs = append(updateArgs, payloadArg(column))
	}
	model.InsertColumns = strings.Join(writableNames, ", ")
	model.InsertPlaceholders = strings.Join(insertPlaceholders, ", ")
	model.InsertArgs = strings.Join(insertArgs, ", ")
	model.UpdateSet = strings.Join(updateSet, ", ")
	model.UpdateArgs = strings.Join(append(updateArgs, "id"), ", ")
	return model, nil
}

// payloadArg renders the payload accessor for a writable column; date and
// time columns map empty strings to NULL so nullable columns survive CRUD.
func payloadArg(column templateColumn) string {
	switch column.Control {
	case "date", "datetime", "time":
		return "optionalTime(payload." + column.GoName + ")"
	default:
		return "payload." + column.GoName
	}
}

func allColumnNames(columns []templateColumn) []string {
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name)
	}
	return names
}

func joinKeywordClause(clause, column string) string {
	part := "CAST(" + column + " AS TEXT) ILIKE ?"
	if clause == "" {
		return part
	}
	return clause + " OR " + part
}

func joinKeywordArg(args string) string {
	part := `"%"+filter.Keyword+"%"`
	if args == "" {
		return part
	}
	return args + ", " + part
}

func convertFunc(control, goType string) string {
	switch control {
	case "datetime", "date", "time":
		return "timeText"
	}
	switch goType {
	case "int64":
		return "toInt64"
	case "float64":
		return "toFloat64"
	case "bool":
		return "toBool"
	default:
		return "toString"
	}
}

func defaultLiteral(tsType string) string {
	switch tsType {
	case "number":
		return "undefined"
	case "boolean":
		return "false"
	default:
		return "''"
	}
}

// registryModulesFor returns the union of already generated modules plus the
// current config, ordered by module name.
func registryModulesFor(existing []TableConfig, current TableConfig) []registryModule {
	modules := make([]registryModule, 0, len(existing)+1)
	seen := make(map[string]bool, len(existing)+1)
	for _, config := range existing {
		if seen[config.ModuleName] {
			continue
		}
		seen[config.ModuleName] = true
		modules = append(modules, registryModule{Module: config.ModuleName})
	}
	if !seen[current.ModuleName] {
		modules = append(modules, registryModule{Module: current.ModuleName})
	}
	return modules
}

// templateSet renders every artifact template for a table config.
func renderArtifacts(config TableConfig, modules []registryModule) ([]Artifact, error) {
	model, err := buildTemplateModel(config, modules)
	if err != nil {
		return nil, err
	}
	files := []struct {
		name   string
		path   string
		delims [2]string
	}{
		{name: "go_types.tmpl", path: fmt.Sprintf("internal/kadmin/generated/%s/types.go", model.Module)},
		{name: "go_repository.tmpl", path: fmt.Sprintf("internal/kadmin/generated/%s/repository.go", model.Module)},
		{name: "go_routes.tmpl", path: fmt.Sprintf("internal/kadmin/generated/%s/routes.go", model.Module)},
		{name: "registry.tmpl", path: "internal/kadmin/generated/registry.go"},
		{name: "ts_api.tmpl", path: fmt.Sprintf("admin-web/apps/web-antd/src/api/kadmin/generated/%s.ts", model.Module)},
		{name: "vue_list.tmpl", path: fmt.Sprintf("admin-web/apps/web-antd/src/views/kadmin/generated/%s/%sListView.vue", model.Module, model.Class), delims: [2]string{"[[", "]]"}},
	}
	artifacts := make([]Artifact, 0, len(files))
	for _, file := range files {
		renderer := template.New(file.name).Delims(file.delims[0], file.delims[1])
		parsed, err := renderer.ParseFS(templateFS, "templates/"+file.name)
		if err != nil {
			return nil, fmt.Errorf("parse codegen template %s: %w", file.name, err)
		}
		var buffer strings.Builder
		if err := parsed.Execute(&buffer, model); err != nil {
			return nil, fmt.Errorf("render %s: %w", file.name, err)
		}
		content := buffer.String()
		if strings.HasSuffix(file.path, ".go") {
			formatted, err := format.Source([]byte(content))
			if err != nil {
				return nil, fmt.Errorf("format %s: %w", file.name, err)
			}
			content = string(formatted)
		}
		artifacts = append(artifacts, Artifact{Path: file.path, Content: content})
	}
	return artifacts, nil
}
