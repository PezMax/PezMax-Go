package codegen

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
)

func TestTypeMappingsAndNaming(t *testing.T) {
	if mapping := mappingFor("int8", "bigint", 0); mapping.GoType != "int64" || mapping.TSType != "number" {
		t.Fatalf("int8 mapping = %#v", mapping)
	}
	if mapping := mappingFor("text", "text", 0); mapping.Control != "textarea" {
		t.Fatalf("text control = %q, want textarea", mapping.Control)
	}
	if mapping := mappingFor("varchar", "character varying", 300); mapping.Control != "textarea" {
		t.Fatalf("long varchar control = %q, want textarea", mapping.Control)
	}
	if mapping := mappingFor("unknown_type", "unknown", 0); mapping.GoType != "string" {
		t.Fatalf("unknown type fallback = %#v", mapping)
	}
	if name := goFieldName("created_at"); name != "CreatedAt" {
		t.Fatalf("goFieldName = %q", name)
	}
	if name := goFieldName("type"); name != "TypeField" {
		t.Fatalf("reserved word field = %q", name)
	}
	if name := jsonFieldName("created_at"); name != "createdAt" {
		t.Fatalf("jsonFieldName = %q", name)
	}
	if label := humanLabel("stock_quantity"); label != "Stock Quantity" {
		t.Fatalf("humanLabel = %q", label)
	}
}

func TestDeriveImportNames(t *testing.T) {
	module, class, prefix, business := deriveImportNames("products")
	if module != "product" || class != "Product" || prefix != "product" || business != "Products" {
		t.Fatalf("deriveImportNames(products) = %q %q %q %q", module, class, prefix, business)
	}
	module, _, _, _ = deriveImportNames("order_items")
	if module != "order_item" {
		t.Fatalf("deriveImportNames(order_items) module = %q", module)
	}
}

func TestInferColumnConfigs(t *testing.T) {
	columns := []introspectedColumn{
		{Name: "id", UDTName: "int8", DataType: "bigint", Nullable: false, PrimaryKey: true},
		{Name: "name", UDTName: "varchar", DataType: "character varying", Nullable: false, MaxLength: 100},
		{Name: "price", UDTName: "numeric", DataType: "numeric", Nullable: true},
		{Name: "created_at", UDTName: "timestamptz", DataType: "timestamp with time zone", Nullable: false},
	}
	configs := inferColumnConfigs(columns)
	if len(configs) != 4 {
		t.Fatalf("inferred %d columns", len(configs))
	}
	byName := map[string]ColumnConfig{}
	for _, config := range configs {
		byName[config.Name] = config
	}
	if !byName["id"].IsPK || byName["id"].Creatable || byName["id"].Queryable {
		t.Fatalf("id defaults wrong: %#v", byName["id"])
	}
	if !byName["name"].Required || !byName["name"].Queryable || byName["name"].GoType != "string" {
		t.Fatalf("name defaults wrong: %#v", byName["name"])
	}
	if byName["price"].Required {
		t.Fatalf("nullable price must not be required: %#v", byName["price"])
	}
	if byName["created_at"].Creatable || byName["created_at"].Editable || byName["created_at"].Queryable {
		t.Fatalf("timestamp audit column defaults wrong: %#v", byName["created_at"])
	}
}

func TestValidateConfig(t *testing.T) {
	valid := TableConfig{
		TableName: "products", ModuleName: "product", ClassName: "Product",
		BusinessName: "产品", RoutePrefix: "product",
		Columns: []ColumnConfig{
			{Name: "id", GoType: "int64", TSType: "number", Listed: true, IsPK: true},
			{Name: "name", GoType: "string", TSType: "string", Listed: true, Creatable: true, Editable: true, Control: "input"},
		},
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	invalid := []TableConfig{
		{ModuleName: "Product", ClassName: "Product", BusinessName: "x", RoutePrefix: "product", Columns: valid.Columns},
		{ModuleName: "product", ClassName: "product", BusinessName: "x", RoutePrefix: "product", Columns: valid.Columns},
		{ModuleName: "product", ClassName: "Product", BusinessName: "x", RoutePrefix: "users", Columns: valid.Columns},
		{ModuleName: "product", ClassName: "Product", BusinessName: "x", RoutePrefix: "product"},
	}
	for index, config := range invalid {
		config.TableName = "products"
		if err := validateConfig(config); err == nil {
			t.Fatalf("invalid config %d accepted", index)
		}
	}
}

func sampleConfig() TableConfig {
	return TableConfig{
		TableName: "products", ModuleName: "product", ClassName: "Product",
		BusinessName: "产品", RoutePrefix: "products",
		Columns: []ColumnConfig{
			{Name: "id", Label: "ID", GoType: "int64", TSType: "number", Control: "number", Listed: true, IsPK: true},
			{Name: "name", Label: "名称", GoType: "string", TSType: "string", Control: "input", Listed: true, Queryable: true, Creatable: true, Editable: true, Required: true},
			{Name: "price", Label: "价格", GoType: "float64", TSType: "number", Control: "number", Listed: true, Creatable: true, Editable: true},
			{Name: "published_at", Label: "发布日期", GoType: "string", TSType: "string", Control: "date", Listed: true, Creatable: true, Editable: true},
			{Name: "created_at", Label: "创建时间", GoType: "string", TSType: "string", Control: "datetime", Listed: true},
		},
	}
}

func TestRenderArtifactsProducesCompilableShapes(t *testing.T) {
	artifacts, err := renderArtifacts(sampleConfig(), []registryModule{{Module: "product"}})
	if err != nil {
		t.Fatalf("render artifacts: %v", err)
	}
	byPath := map[string]string{}
	for _, artifact := range artifacts {
		byPath[artifact.Path] = artifact.Content
	}
	expected := []string{
		"internal/kadmin/generated/product/types.go",
		"internal/kadmin/generated/product/repository.go",
		"internal/kadmin/generated/product/routes.go",
		"internal/kadmin/generated/registry.go",
		"admin-web/apps/web-antd/src/api/kadmin/generated/product.ts",
		"admin-web/apps/web-antd/src/views/kadmin/generated/product/ProductListView.vue",
	}
	for _, path := range expected {
		if content := byPath[path]; content == "" {
			t.Fatalf("artifact %s missing", path)
		} else if !strings.Contains(content, markerGo) && !strings.Contains(content, markerVue) {
			t.Fatalf("artifact %s lacks the codegen marker", path)
		}
	}
	routes := byPath["internal/kadmin/generated/product/routes.go"]
	for _, expectedSnippet := range []string{
		`RegisterAuditResource("products", "product"`,
		`RegisterIdempotentRoute("products")`, `RequirePermission(CreatePermission)`, `goadmin_permissions`,
		"@Router /products [get]", "@Router /products [post]", "@Router /products/{id} [delete]",
	} {
		if !strings.Contains(routes, expectedSnippet) {
			t.Fatalf("generated routes missing %q", expectedSnippet)
		}
	}
	types := byPath["internal/kadmin/generated/product/types.go"]
	for _, expectedSnippet := range []string{
		`system:product:list`, `system:product:create`, "type ProductPayload struct",
	} {
		if !strings.Contains(types, expectedSnippet) {
			t.Fatalf("generated types missing %q", expectedSnippet)
		}
	}
	repository := byPath["internal/kadmin/generated/product/repository.go"]
	for _, expectedSnippet := range []string{
		`INSERT INTO public.products (name, price, published_at)`,
		`UPDATE public.products SET name = ?, price = ?, published_at = ?`,
		`ORDER BY id DESC`, `RETURNING id`, `optionalTime(payload.PublishedAt)`,
	} {
		if !strings.Contains(repository, expectedSnippet) {
			t.Fatalf("generated repository missing %q", expectedSnippet)
		}
	}
	vue := byPath["admin-web/apps/web-antd/src/views/kadmin/generated/product/ProductListView.vue"]
	for _, expectedSnippet := range []string{
		"getProductList", "createProduct", "updateProduct", "deleteProduct", "system:product:create", "a-modal",
	} {
		if !strings.Contains(vue, expectedSnippet) {
			t.Fatalf("generated vue page missing %q", expectedSnippet)
		}
	}
	if strings.Contains(vue, "[[") || strings.Contains(vue, "]]") {
		t.Fatal("generated vue page contains unresolved template delimiters")
	}
	registry := byPath["internal/kadmin/generated/registry.go"]
	if !strings.Contains(registry, `generated/product"`) || !strings.Contains(registry, "product.Register(api") {
		t.Fatalf("generated registry is missing the module wiring: %s", registry)
	}
}

func TestWriterConflictPolicyAndPathSafety(t *testing.T) {
	root := t.TempDir()
	writer := NewWriter(root)
	artifacts := []Artifact{
		{Path: "internal/kadmin/generated/product/types.go", Content: markerGo + "\npackage product\n"},
		{Path: "admin-web/apps/web-antd/src/api/kadmin/generated/product.ts", Content: markerGo + "\nexport {};\n"},
	}
	plan, err := writer.plan(artifacts)
	if err != nil {
		t.Fatalf("plan fresh write: %v", err)
	}
	if len(plan.toCreate) != 2 || len(plan.conflicts) != 0 {
		t.Fatalf("fresh plan = create:%d conflicts:%d", len(plan.toCreate), len(plan.conflicts))
	}
	if err := writer.apply(plan); err != nil {
		t.Fatalf("apply fresh write: %v", err)
	}
	plan, err = writer.plan(artifacts)
	if err != nil {
		t.Fatalf("replan: %v", err)
	}
	if len(plan.toOverwrite) != 2 || len(plan.conflicts) != 0 {
		t.Fatalf("replan should overwrite generated files, got overwrite:%d conflicts:%d", len(plan.toOverwrite), len(plan.conflicts))
	}
	handPath := filepath.Join(root, "internal", "kadmin", "generated", "product", "manual.go")
	if err := os.WriteFile(handPath, []byte("package product\n// hand-written\n"), 0o644); err != nil {
		t.Fatalf("write hand file: %v", err)
	}
	plan, err = writer.plan([]Artifact{{Path: "internal/kadmin/generated/product/manual.go", Content: markerGo + "\npackage product\n"}})
	if err != nil {
		t.Fatalf("plan conflict: %v", err)
	}
	if len(plan.conflicts) != 1 || plan.conflicts[0] != "internal/kadmin/generated/product/manual.go" {
		t.Fatalf("conflict plan = %#v", plan.conflicts)
	}
	for _, escape := range []string{"../evil.go", "admin-web/../../evil.go", "internal/kadmin/secret.go"} {
		if _, err := writer.plan([]Artifact{{Path: escape, Content: "x"}}); err == nil {
			t.Fatalf("escape path %q was accepted", escape)
		}
	}
}

func TestBuildZipContainsEveryArtifact(t *testing.T) {
	archive, err := buildZip([]Artifact{
		{Path: "a/types.go", Content: markerGo + "\npackage a\n"},
		{Path: "b/list.vue", Content: markerVue + "\n<template></template>\n"},
	})
	if err != nil {
		t.Fatalf("build zip: %v", err)
	}
	if len(archive) == 0 || !strings.HasPrefix(string(archive[:2]), "PK") {
		t.Fatal("zip archive is empty or malformed")
	}
}

// fakeConn answers the queries the codegen module issues during handler tests.
type fakeConn struct {
	db.Connection
	configRows    []map[string]interface{}
	columnRows    []map[string]interface{}
	pkRows        []map[string]interface{}
	commentRows   []map[string]interface{}
	candidateRows []map[string]interface{}
	generatedRows []map[string]interface{}
}

func (f *fakeConn) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	switch {
	case strings.Contains(query, "information_schema.columns"):
		return f.columnRows, nil
	case strings.Contains(query, "information_schema.table_constraints"):
		return f.pkRows, nil
	case strings.Contains(query, "pg_attribute"):
		return f.commentRows, nil
	case strings.Contains(query, "pg_catalog.pg_tables"):
		return f.candidateRows, nil
	case strings.Contains(query, "FROM public.kadmin_codegen_tables"):
		if strings.Contains(query, "WHERE generated = TRUE") {
			return f.generatedRows, nil
		}
		return f.configRows, nil
	case strings.Contains(query, "INSERT INTO public.kadmin_codegen_tables"):
		return []map[string]interface{}{{"id": int64(1)}}, nil
	default:
		return nil, nil
	}
}

func (f *fakeConn) Exec(query string, args ...interface{}) (sql.Result, error) {
	return fakeResult{}, nil
}

type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) { return 1, nil }
func (fakeResult) RowsAffected() (int64, error) { return 1, nil }

func productConfigRows() []map[string]interface{} {
	columns, _ := json.Marshal(sampleConfig().Columns)
	return []map[string]interface{}{{
		"id":            int64(1),
		"table_name":    "products",
		"module_name":   "product",
		"class_name":    "Product",
		"business_name": "产品",
		"route_prefix":  "products",
		"columns":       string(columns),
		"generated":     false,
		"created_at":    nil,
		"updated_at":    nil,
	}}
}

func newCodegenTestEngine(root string, conn db.Connection) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterRoutes(engine.Group("/api"), Dependencies{
		Connection:  conn,
		RequireAuth: func(c *gin.Context) { c.Next() },
		RequirePermission: func(...string) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		RepoRoot: root,
	})
	return engine
}

func TestImportEndpointInfersColumnConfigs(t *testing.T) {
	conn := &fakeConn{
		columnRows: []map[string]interface{}{
			{"column_name": "id", "data_type": "bigint", "udt_name": "int8", "is_nullable": "NO", "column_default": nil, "character_maximum_length": nil},
			{"column_name": "name", "data_type": "character varying", "udt_name": "varchar", "is_nullable": "NO", "column_default": nil, "character_maximum_length": int64(100)},
			{"column_name": "created_at", "data_type": "timestamp with time zone", "udt_name": "timestamptz", "is_nullable": "NO", "column_default": nil, "character_maximum_length": nil},
		},
		pkRows:      []map[string]interface{}{{"column_name": "id"}},
		commentRows: []map[string]interface{}{},
	}
	engine := newCodegenTestEngine(t.TempDir(), conn)
	request := httptest.NewRequest(http.MethodPost, "/api/codegen/tables/import", strings.NewReader(`{"tableName":"products"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Code int         `json:"code"`
		Data TableConfig `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if envelope.Data.TableName != "products" || envelope.Data.ModuleName != "product" {
		t.Fatalf("imported config = %#v", envelope.Data)
	}
	if len(envelope.Data.Columns) != 3 || !envelope.Data.Columns[1].Required || !envelope.Data.Columns[1].Queryable {
		t.Fatalf("inferred columns wrong: %#v", envelope.Data.Columns)
	}
}

func TestGenerateEndpointRefusesConflictsWithoutConfirmation(t *testing.T) {
	root := t.TempDir()
	conn := &fakeConn{configRows: productConfigRows()}
	engine := newCodegenTestEngine(root, conn)
	handPath := filepath.Join(root, "internal", "kadmin", "generated", "product", "routes.go")
	if err := os.MkdirAll(filepath.Dir(handPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(handPath, []byte("package product\n// hand-written\n"), 0o644); err != nil {
		t.Fatalf("write hand file: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/codegen/configs/1/generate", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d, body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Code int              `json:"code"`
		Data GenerateResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if len(envelope.Data.Conflicts) != 1 || envelope.Data.Conflicts[0] != "internal/kadmin/generated/product/routes.go" {
		t.Fatalf("conflicts = %#v", envelope.Data.Conflicts)
	}
	// The hand-written file must remain untouched.
	content, err := os.ReadFile(handPath)
	if err != nil || !strings.Contains(string(content), "hand-written") {
		t.Fatalf("hand-written file was modified: %v", err)
	}

	confirmed := httptest.NewRequest(http.MethodPost, "/api/codegen/configs/1/generate", strings.NewReader(`{"confirmOverwrite":true}`))
	confirmed.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, confirmed)
	if response.Code != http.StatusOK {
		t.Fatalf("confirmed generate status = %d, body = %s", response.Code, response.Body.String())
	}
	content, err = os.ReadFile(handPath)
	if err != nil || !strings.Contains(string(content), markerGo) {
		t.Fatalf("confirmed generate did not overwrite conflict: %v", err)
	}
}

func TestPreviewEndpointRendersArtifactsWithoutWriting(t *testing.T) {
	root := t.TempDir()
	conn := &fakeConn{configRows: productConfigRows()}
	engine := newCodegenTestEngine(root, conn)
	request := httptest.NewRequest(http.MethodPost, "/api/codegen/configs/1/preview", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data PreviewResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if len(envelope.Data.Artifacts) != 6 {
		t.Fatalf("preview artifacts = %d, want 6", len(envelope.Data.Artifacts))
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preview must not write files, root has %d entries (err=%v)", len(entries), err)
	}
}

func TestCandidatesEndpointFiltersSystemTables(t *testing.T) {
	conn := &fakeConn{candidateRows: []map[string]interface{}{
		{"tablename": "products"},
		{"tablename": "goadmin_menu"},
		{"tablename": "kadmin_jobs"},
	}}
	engine := newCodegenTestEngine(t.TempDir(), conn)
	request := httptest.NewRequest(http.MethodGet, "/api/codegen/candidates", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("candidates status = %d", response.Code)
	}
	var envelope struct {
		Data []CandidateTable `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode candidates: %v", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].Name != "products" {
		t.Fatalf("candidates = %#v, want only products", envelope.Data)
	}
}
