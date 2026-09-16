package codegen

import (
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
)

// Permission codes for the code generator management page.
const (
	ListPermission     = "system:codegen:list"
	ImportPermission   = "system:codegen:import"
	GeneratePermission = "system:codegen:generate"
)

// Dependencies carries the KAdmin services the codegen module needs.
type Dependencies struct {
	Connection        db.Connection
	RequireAuth       gin.HandlerFunc
	RequirePermission func(...string) gin.HandlerFunc
	// RepoRoot is the repository root used to resolve output directories.
	// Defaults to the current working directory when empty.
	RepoRoot string
}

// ColumnConfig describes one column of an imported table.
type ColumnConfig struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	GoType    string `json:"goType"`
	TSType    string `json:"tsType"`
	Control   string `json:"control"`
	Listed    bool   `json:"listed"`
	Queryable bool   `json:"queryable"`
	Creatable bool   `json:"creatable"`
	Editable  bool   `json:"editable"`
	Required  bool   `json:"required"`
	IsPK      bool   `json:"isPK"`
}

// TableConfig is the persisted import configuration for one database table.
type TableConfig struct {
	ID           int64          `json:"id"`
	TableName    string         `json:"tableName"`
	ModuleName   string         `json:"moduleName"`
	ClassName    string         `json:"className"`
	BusinessName string         `json:"businessName"`
	RoutePrefix  string         `json:"routePrefix"`
	Columns      []ColumnConfig `json:"columns"`
	Generated    bool           `json:"generated"`
	CreatedAt    string         `json:"createdAt"`
	UpdatedAt    string         `json:"updatedAt"`
}

// ImportPayload imports a database table into the generator.
type ImportPayload struct {
	TableName    string `json:"tableName"`
	ModuleName   string `json:"moduleName"`
	ClassName    string `json:"className"`
	BusinessName string `json:"businessName"`
	RoutePrefix  string `json:"routePrefix"`
}

// UpdatePayload updates the import configuration of a table.
type UpdatePayload struct {
	ModuleName   string         `json:"moduleName"`
	ClassName    string         `json:"className"`
	BusinessName string         `json:"businessName"`
	RoutePrefix  string         `json:"routePrefix"`
	Columns      []ColumnConfig `json:"columns"`
}

// GeneratePayload drives the generate action.
type GeneratePayload struct {
	// ConfirmOverwrite allows overwriting target files that exist without the
	// codegen marker. Conflicts refuse generation when false.
	ConfirmOverwrite bool `json:"confirmOverwrite"`
}

// Artifact is one rendered output file.
type Artifact struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// PreviewResponse renders every artifact without touching the filesystem.
type PreviewResponse struct {
	Artifacts []Artifact `json:"artifacts"`
}

// GenerateResponse reports what the generate action did.
type GenerateResponse struct {
	Written         []string `json:"written"`
	Overwritten     []string `json:"overwritten"`
	Conflicts       []string `json:"conflicts"`
	PermissionCount int      `json:"permissionCount"`
	MenuURI         string   `json:"menuUri"`
	Note            string   `json:"note"`
}

// Page wraps a paginated list result.
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// CandidateTable is a database table offered for import.
type CandidateTable struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}
