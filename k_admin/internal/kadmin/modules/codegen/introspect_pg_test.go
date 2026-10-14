package codegen

import (
	"database/sql"
	"encoding/json"
	"errors"
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
	raw, connection, err := openGuardedCodegenReview(dsn)
	if err != nil {
		t.Fatalf("guarded review setup: %v", err)
	}
	t.Cleanup(func() {
		for _, statement := range []string{
			`DROP TABLE IF EXISTS public.codegen_review_foreign`,
			`DROP TABLE IF EXISTS public.codegen_review_single`,
			`DROP TABLE IF EXISTS public.codegen_review_composite`,
		} {
			_, _ = raw.Exec(statement)
		}
		_ = raw.Close()
	})
	return connection
}

// codegenReviewGuardPrefix is the hard isolation prefix every database taking
// part in this regression must carry.
const codegenReviewGuardPrefix = "pezmax_codegen_test_"

// actualDatabase asks the connection that will run the DDL which database it
// is really on. Hand-parsing the DSN cannot answer this: quoting rules let a
// parameter such as application_name='x dbname=<fake>' hide the real target.
func actualDatabase(raw *sql.DB) (string, error) {
	var name string
	if err := raw.QueryRow(`SELECT current_database()`).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

// actualDatabaseViaGoAdmin asks the same question through the GoAdmin
// connection so every connection taking part in the test is verified.
func actualDatabaseViaGoAdmin(connection db.Connection) (string, error) {
	rows, err := connection.Query(`SELECT current_database()`)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", errors.New("current_database() returned no rows")
	}
	name, _ := rows[0]["current_database"].(string)
	return name, nil
}

// openGuardedCodegenReview prepares the fixture tables and the GoAdmin
// connection. It must refuse to run any DDL unless the database it is actually
// connected to carries the isolated test prefix.
func openGuardedCodegenReview(dsn string) (*sql.DB, db.Connection, error) {
	raw, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, err
	}
	raw.SetMaxOpenConns(1)
	// 护栏以真实连接为准：任何 DDL 之前先确认实际数据库前缀。
	actual, err := actualDatabase(raw)
	if err != nil {
		_ = raw.Close()
		return nil, nil, err
	}
	if !strings.HasPrefix(actual, codegenReviewGuardPrefix) {
		_ = raw.Close()
		return nil, nil, errors.New("codegen introspection regression actual database " + actual + " lacks the required " + codegenReviewGuardPrefix + " prefix")
	}
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
			_ = raw.Close()
			return nil, nil, err
		}
	}
	// GoAdmin 连接使用同一数据源（由驱动解析 DSN），并复核其实际库。
	connection := db.GetPostgresqlDB().InitDB(map[string]config.Database{
		"default": {
			Driver: db.DriverPostgresql,
			Dsn:    dsn,
		},
	})
	viaFramework, err := actualDatabaseViaGoAdmin(connection)
	if err != nil {
		_ = raw.Close()
		return nil, nil, err
	}
	if viaFramework != actual {
		_ = raw.Close()
		return nil, nil, errors.New("framework connection reached " + viaFramework + " instead of " + actual)
	}
	return raw, connection, nil
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

// 护栏负例：合法 DSN 允许带空格的引号参数。application_name 的引号值内
// 夹带的 dbname=pezmax_codegen_test_fake 可以骗过手工词法解析，但驱动仍会
// 连接 dbname= 指定的真实库。护栏必须在执行任何 DDL 前发现实际库不符合
// 测试前缀并拒绝。
func TestCodegenReviewGuardRejectsActualDatabaseOutsidePrefix(t *testing.T) {
	dsn := os.Getenv("KADMIN_TEST_CODEGEN_REJECT_DSN")
	if dsn == "" {
		t.Skip("set KADMIN_TEST_CODEGEN_REJECT_DSN to a disposable pezmax_codegen_guard_* database")
	}
	// 负例也只访问自有测试库。保持 DSN 原样，不能重定向到维护库。
	probe, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = probe.Close() })
	actual, err := actualDatabase(probe)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(actual, "pezmax_codegen_guard_") {
		t.Fatal("guard rejection regression requires an actual disposable pezmax_codegen_guard_* database")
	}
	// information_schema 会隐藏没有表权限的对象，不能据此断定清理目标不存在。
	const countReviewObjects = `SELECT count(*) FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND left(c.relname, length('codegen_review_')) = 'codegen_review_'`
	var objects int
	if err := probe.QueryRow(countReviewObjects).Scan(&objects); err != nil {
		t.Fatal(err)
	}
	if objects != 0 {
		t.Fatal("guard rejection regression requires a disposable database without existing review objects")
	}
	// 在调用 helper 前注册，覆盖错误返回及 panic；只清理由本用例创建的表。
	t.Cleanup(func() {
		for _, table := range []string{"codegen_review_foreign", "codegen_review_single", "codegen_review_composite"} {
			var exists bool
			if err := probe.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists); err != nil {
				t.Errorf("check review fixture cleanup: %v", err)
				continue
			}
			if exists {
				if _, err := probe.Exec("DROP TABLE public." + table); err != nil {
					t.Errorf("clean review fixture: %v", err)
				}
			}
		}
	})
	raw, connection, err := openGuardedCodegenReview(dsn)
	if raw != nil {
		t.Cleanup(func() { _ = raw.Close() })
	}
	if connection != nil {
		t.Cleanup(func() {
			for _, err := range connection.Close() {
				if err != nil {
					t.Errorf("close unexpected review connection: %v", err)
				}
			}
		})
	}
	if err == nil {
		t.Fatal("guard accepted a DSN whose actual database lacks the test prefix")
	}
	if !strings.Contains(err.Error(), codegenReviewGuardPrefix) {
		t.Fatalf("guard error should name the required prefix, got: %v", err)
	}
	// 检查清理执行前的现场，不能让清理掩盖被测护栏执行过 DDL。
	if err := probe.QueryRow(countReviewObjects).Scan(&objects); err != nil {
		t.Fatal(err)
	}
	if objects != 0 {
		t.Fatalf("guard ran DDL on the non-prefixed actual database: %d review objects found", objects)
	}
}
