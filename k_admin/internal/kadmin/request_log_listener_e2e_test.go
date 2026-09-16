package kadmin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loadrank"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
)

// fakeLogConnection stands in for the database. It records every statement so
// the test can assert both the log insert and the metric upsert, and it serves
// back the metric rows that were written so the rankings API has data to read.
type fakeLogConnection struct {
	db.Connection
	mu           sync.Mutex
	execQueries  []string
	queryQueries []string
	bucketRows   []map[string]interface{}
	// menuRows feeds the generated modules' menu lookups; seeded with the
	// /business directory row in route registration tests.
	menuRows []map[string]interface{}
}

func (f *fakeLogConnection) Name() string { return "sqlite" }

func (f *fakeLogConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.execQueries = append(f.execQueries, query)
	f.captureBucketRow(query, args)
	return fakeLogResult{}, nil
}

func (f *fakeLogConnection) ExecWith(_ *sql.Tx, _ string, query string, args ...interface{}) (sql.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.execQueries = append(f.execQueries, query)
	f.captureBucketRow(query, args)
	return fakeLogResult{}, nil
}

// captureBucketRow records metric upserts so the rankings API can serve back
// exactly what the sampler wrote.
func (f *fakeLogConnection) captureBucketRow(query string, args []interface{}) {
	if !strings.Contains(query, "kadmin_http_metric_buckets") || !strings.Contains(query, "ON CONFLICT") {
		return
	}
	row := map[string]interface{}{}
	fields := []string{"bucket_start", "route", "method", "status_code", "request_count", "error_count", "total_duration_ms", "max_duration_ms"}
	for index, field := range fields {
		if index < len(args) {
			row[field] = args[index]
		}
	}
	f.bucketRows = append(f.bucketRows, row)
}

func (f *fakeLogConnection) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryQueries = append(f.queryQueries, query)
	if strings.Contains(query, "FROM goadmin_menu") {
		return f.menuRows, nil
	}
	if strings.Contains(query, "SELECT enabled FROM public.kadmin_loadrank_settings") {
		return []map[string]interface{}{{"enabled": false}}, nil
	}
	if strings.Contains(query, "SUM(request_count)") {
		result := make([]map[string]interface{}, 0, len(f.bucketRows))
		for _, row := range f.bucketRows {
			copied := make(map[string]interface{}, len(row))
			for key, value := range row {
				copied[key] = value
			}
			result = append(result, copied)
		}
		return result, nil
	}
	return nil, nil
}

func (f *fakeLogConnection) recorded(queryPart string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	lower := strings.ToLower(queryPart)
	for _, query := range f.execQueries {
		if strings.Contains(strings.ToLower(query), lower) {
			return true
		}
	}
	return false
}

func (f *fakeLogConnection) metricRows() []map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]interface{}(nil), f.bucketRows...)
}

type fakeLogResult struct{}

func (fakeLogResult) LastInsertId() (int64, error) { return 1, nil }
func (fakeLogResult) RowsAffected() (int64, error) { return 1, nil }

// TestRequestLogListenerFeedsLoadRankingEndToEnd exercises the production
// wiring: a real Gin request flows through the listener middleware into the
// sampler, which upserts a template-normalized bucket; the rankings API then
// serves that bucket back through the repository.
func TestRequestLogListenerFeedsLoadRankingEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	connection := &fakeLogConnection{}
	listener := NewRequestLogListener(connection)
	defer listener.Close()

	engine := gin.New()
	engine.Use(listener.Middleware())
	sampler, err := loadrank.Register(engine.Group("/api"), loadrank.Dependencies{
		Connection: connection,
		RequireAuth: func(c *gin.Context) { c.Next() },
		RequirePermission: func(...string) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
	})
	if err != nil {
		t.Fatalf("register load ranking: %v", err)
	}
	defer sampler.Close()
	if _, err := sampler.SetEnabled(true, 1); err != nil {
		t.Fatalf("enable sampling: %v", err)
	}
	listener.AttachMetricSink(sampler)
	engine.GET("/api/users/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})
	sampler.SetRouteIndex(loadrank.NewRouteIndex(engine.Routes()))

	request := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("request status = %d, body = %s", response.Code, response.Body.String())
	}

	// The listener worker persists the log event first, then observes the
	// sampler; wait for the log insert, then drain by disabling sampling.
	deadline := time.Now().Add(2 * time.Second)
	for !connection.recorded("goadmin_operation_log") {
		if time.Now().After(deadline) {
			t.Fatal("request log event was never inserted")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := sampler.SetEnabled(false, 1); err != nil {
		t.Fatalf("disable sampling: %v", err)
	}

	rows := connection.metricRows()
	if len(rows) != 1 {
		t.Fatalf("metric upserts = %d, want 1: %#v", len(rows), rows)
	}
	row := rows[0]
	if row["route"] != "/api/users/{id}" || row["method"] != "GET" || row["status_code"] != int64(200) {
		t.Fatalf("unexpected metric bucket: %#v", row)
	}
	if row["request_count"] != int64(1) || row["error_count"] != int64(0) {
		t.Fatalf("unexpected metric counts: %#v", row)
	}

	rankingResponse := httptest.NewRecorder()
	engine.ServeHTTP(rankingResponse, httptest.NewRequest(
		http.MethodGet,
		"/api/load-ranking/rankings?startedAt=2026-07-31T00:00:00Z&endedAt=2026-07-31T23:59:59Z",
		nil,
	))
	if rankingResponse.Code != http.StatusOK {
		t.Fatalf("rankings status = %d, body = %s", rankingResponse.Code, rankingResponse.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				Route        string  `json:"route"`
				RequestCount int64   `json:"requestCount"`
				QPS          float64 `json:"qps"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rankingResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode rankings: %v", err)
	}
	if envelope.Code != 0 || len(envelope.Data.Items) != 1 {
		t.Fatalf("unexpected rankings envelope: %#v", envelope)
	}
	item := envelope.Data.Items[0]
	if item.Route != "/api/users/{id}" || item.RequestCount != 1 || item.QPS <= 0 {
		t.Fatalf("unexpected ranking item: %#v", item)
	}
}
