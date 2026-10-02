package codegen

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/config"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// codegenReviewPostgres dials the opt-in isolated introspection database.
// The name prefix is a hard guard against pointing this at real business data.
func codegenReviewPostgres(t *testing.T) db.Connection {
	t.Helper()
	dsn := os.Getenv("KADMIN_TEST_CODEGEN_DSN")
	if dsn == "" {
		t.Skip("set KADMIN_TEST_CODEGEN_DSN to a disposable pezmax_codegen_test_* database")
	}
	fields := map[string]string{}
	for _, part := range strings.Fields(dsn) {
		if key, value, ok := strings.Cut(part, "="); ok {
			fields[key] = strings.Trim(value, "'")
		}
	}
	if !strings.HasPrefix(fields["dbname"], "pezmax_codegen_test_") {
		t.Fatal("codegen introspection regression requires an isolated pezmax_codegen_test_* database")
	}
	raw, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	// 复现审计缺陷的表形态：另一张表存在与目标表主键同名的约束 shared_key。
	for _, statement := range []string{
		`DROP TABLE IF EXISTS public.codegen_review_foreign`,
		`DROP TABLE IF EXISTS public.codegen_review_single`,
		`DROP TABLE IF EXISTS public.codegen_review_composite`,
		`CREATE TABLE public.codegen_review_single (
			id INT NOT NULL,
			payload TEXT,
			CONSTRAINT shared_key PRIMARY KEY (id)
		)`,
		`CREATE TABLE public.codegen_review_foreign (
			foreign_id INT NOT NULL,
			CONSTRAINT shared_key FOREIGN KEY (foreign_id) REFERENCES public.codegen_review_single (id)
		)`,
		`CREATE TABLE public.codegen_review_composite (
			a INT NOT NULL,
			b INT NOT NULL,
			payload TEXT,
			CONSTRAINT codegen_review_composite_pkey PRIMARY KEY (a, b)
		)`,
	} {
		if _, err := raw.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	connection := db.GetPostgresqlDB().InitDB(map[string]config.Database{
		"default": {
			Driver: db.DriverPostgresql,
			Host:   fields["host"],
			Port:   fields["port"],
			User:   fields["user"],
			Pwd:    fields["password"],
			Name:   fields["dbname"],
		},
	})
	t.Cleanup(func() {
		for _, statement := range []string{
			`DROP TABLE IF EXISTS public.codegen_review_foreign`,
			`DROP TABLE IF EXISTS public.codegen_review_single`,
			`DROP TABLE IF EXISTS public.codegen_review_composite`,
		} {
			_, _ = raw.Exec(statement)
		}
	})
	return connection
}

// The single-key table shares the constraint name shared_key with another
// table's foreign key. Constraint identity must include the table: the
// introspector must see exactly {id} for the single key, the render guard must
// accept it across preview/generate/download, and a real composite key must
// stay rejected on all three entry points.
func TestPrimaryKeyIntrospectionMatchesFullTableIdentity(t *testing.T) {
	connection := codegenReviewPostgres(t)
	introspector := newIntrospector(connection)
	singleKeys, err := introspector.primaryKeyColumns("codegen_review_single")
	if err != nil {
		t.Fatal(err)
	}
	if len(singleKeys) != 1 || !singleKeys["id"] {
		t.Fatalf("single primary key introspected as %v; cross-table constraint name leaked", singleKeys)
	}
	compositeKeys, err := introspector.primaryKeyColumns("codegen_review_composite")
	if err != nil {
		t.Fatal(err)
	}
	if len(compositeKeys) != 2 || !compositeKeys["a"] || !compositeKeys["b"] {
		t.Fatalf("composite primary key introspected as %v", compositeKeys)
	}

	if err := EnsureSchema(connection); err != nil {
		t.Fatal(err)
	}
	engine := newCodegenTestEngine(t.TempDir(), connection)
	importConfig := func(t *testing.T, table string) int64 {
		t.Helper()
		words := strings.Split(strings.ReplaceAll(strings.TrimPrefix(table, "codegen_review_"), "_", " "), " ")
		className := "CodegenReview"
		for _, word := range words {
			className += strings.ToUpper(word[:1]) + word[1:]
		}
		routePrefix := strings.ReplaceAll(table, "_", "-")
		payload := `{"tableName":"` + table + `","moduleName":"` + table + `","className":"` + className + `","businessName":"` + table + `","routePrefix":"` + routePrefix + `"}`
		response := codegenReviewRequest(t, engine, http.MethodPost, "/api/codegen/tables/import", payload, http.StatusOK)
		var body struct {
			Data TableConfig `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Data.ID <= 0 {
			t.Fatalf("import of %s returned id %d: %s", table, body.Data.ID, response.Body.String())
		}
		return body.Data.ID
	}
	singleID := importConfig(t, "codegen_review_single")
	for _, endpoint := range []struct{ method, action string }{
		{http.MethodPost, "preview"}, {http.MethodPost, "generate"}, {http.MethodGet, "download"},
	} {
		t.Run("single-key-accepted/"+endpoint.action, func(t *testing.T) {
			request := httptest.NewRequest(endpoint.method, "/api/codegen/configs/"+strconvFormatID(singleID)+"/"+endpoint.action, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("single-key table must generate: %s %s status=%d body=%s", endpoint.method, endpoint.action, recorder.Code, recorder.Body.String())
			}
		})
	}
	// 真实联合主键在导入入口即被拒绝（validateConfig 早于 render 三入口）；
	// preview/download/generate 的联合键拒绝契约由 fakeConn 端点回归覆盖。
	t.Run("composite-key-rejected/import", func(t *testing.T) {
		payload := `{"tableName":"codegen_review_composite","moduleName":"codegen_review_composite","className":"CodegenReviewComposite","businessName":"codegen_review_composite","routePrefix":"codegen-review-composite"}`
		request := httptest.NewRequest(http.MethodPost, "/api/codegen/tables/import", strings.NewReader(payload))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "composite primary key") {
			t.Fatalf("composite-key table must be rejected at import: status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	})
}

func strconvFormatID(id int64) string {
	data, _ := json.Marshal(id)
	return string(data)
}

func codegenReviewRequest(t *testing.T, engine *gin.Engine, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != status {
		t.Fatalf("%s %s status=%d, want %d; body=%s", method, path, recorder.Code, status, recorder.Body.String())
	}
	return recorder
}
