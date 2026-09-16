package loadrank

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

// --- fakes ------------------------------------------------------------------

type fakeSettings struct {
	mu      sync.Mutex
	enabled bool
	savedBy int64
}

func (s *fakeSettings) LoadEnabled() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled, nil
}

func (s *fakeSettings) SaveEnabled(enabled bool, updatedBy int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	s.savedBy = updatedBy
	return nil
}

type fakeBucketStore struct {
	mu           sync.Mutex
	upserts      []bucketRow
	queries      []bucketQueryFilter
	rows         []bucketRow
	prunedBefore time.Time
	pruned       bool
	failUpsert   bool
}

func (s *fakeBucketStore) UpsertBucket(bucketStart time.Time, row bucketAggregate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failUpsert {
		return errFakeStore
	}
	s.upserts = append(s.upserts, bucketRow{
		bucketStart:     bucketStart,
		route:           row.route,
		method:          row.method,
		statusCode:      row.statusCode,
		requestCount:    row.requestCount,
		errorCount:      row.errorCount,
		totalDurationMs: row.totalDurationMs,
		maxDurationMs:   row.maxDurationMs,
	})
	return nil
}

func (s *fakeBucketStore) QueryBuckets(filter bucketQueryFilter) ([]bucketRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, filter)
	return s.rows, nil
}

func (s *fakeBucketStore) PruneBefore(before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruned = true
	s.prunedBefore = before
	return nil
}

var errFakeStore = &fakeStoreError{}

type fakeStoreError struct{}

func (e *fakeStoreError) Error() string { return "fake store error" }

// mergedUpserts returns the upsert rows merged by identity, mirroring the
// ON CONFLICT accumulation semantics of the repository.
func (s *fakeBucketStore) mergedUpserts() []bucketRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	merged := make(map[string]*bucketRow)
	order := make([]string, 0)
	for _, row := range s.upserts {
		key := row.bucketStart.Format(time.RFC3339Nano) + "|" + row.route + "|" + row.method + "|" + itoa(row.statusCode)
		if existing, ok := merged[key]; ok {
			existing.requestCount += row.requestCount
			existing.errorCount += row.errorCount
			existing.totalDurationMs += row.totalDurationMs
			if row.maxDurationMs > existing.maxDurationMs {
				existing.maxDurationMs = row.maxDurationMs
			}
			continue
		}
		copied := row
		merged[key] = &copied
		order = append(order, key)
	}
	result := make([]bucketRow, 0, len(order))
	for _, key := range order {
		result = append(result, *merged[key])
	}
	return result
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}

func event(status int64, duration int64, method string, path string) models.OperationLogEvent {
	return models.OperationLogEvent{
		Path: path, Method: method, StatusCode: &status, DurationMs: &duration,
	}
}

func int64Ptr(value int64) *int64 { return &value }

// --- route index ------------------------------------------------------------

func TestRouteIndexResolvesGinTemplates(t *testing.T) {
	index := NewRouteIndex([]gin.RouteInfo{
		{Method: "GET", Path: "/api/users/:id"},
		{Method: "GET", Path: "/api/users"},
		{Method: "POST", Path: "/api/users/:id/roles"},
		{Method: "GET", Path: "/files/*path"},
		{Method: "PUT", Path: "/api/orders/:orderId/items/:itemId"},
	})
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{method: "GET", path: "/api/users/42", want: "/api/users/{id}"},
		{method: "GET", path: "/api/users", want: "/api/users"},
		{method: "GET", path: "/api/users/", want: "/api/users"},
		{method: "GET", path: "/api/users/42/", want: "/api/users/{id}"},
		{method: "POST", path: "/api/users/9/roles", want: "/api/users/{id}/roles"},
		{method: "GET", path: "/files/a/b/c.png", want: "/files/{path...}"},
		{method: "PUT", path: "/api/orders/o-1/items/2", want: "/api/orders/{orderId}/items/{itemId}"},
		{method: "GET", path: "/api/orders/o-1/items/2", want: ""},
		{method: "DELETE", path: "/api/users/42", want: ""},
	}
	for _, testCase := range cases {
		if got := index.Resolve(testCase.method, testCase.path); got != testCase.want {
			t.Fatalf("Resolve(%s %s) = %q, want %q", testCase.method, testCase.path, got, testCase.want)
		}
	}
}

func TestNormalizeRouteTemplateBoundsVariableSegments(t *testing.T) {
	cases := map[string]string{
		"/api/users/42":       "/api/users/{id}",
		"/api/users/42/posts": "/api/users/{id}/posts",
		"/api/files/9e107d9d-372b-4a1f-8b8a-2f1c0f4c2e9a": "/api/files/{id}",
		"/api/search/hello":       "/api/search/hello",
		"":                        "/",
		"/api/orders/1/items/2/3": "/api/orders/{id}/items/{id}/{id}",
	}
	for input, wanted := range cases {
		if got := normalizeRouteTemplate(input); got != wanted {
			t.Fatalf("normalizeRouteTemplate(%q) = %q, want %q", input, got, wanted)
		}
	}
}

// --- aggregation 统计口径 -----------------------------------------------------

func TestAggregateRowsComputesStatistics(t *testing.T) {
	bucket := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	rows := []bucketRow{
		{bucketStart: bucket, route: "/api/users", method: "GET", statusCode: 200, requestCount: 10, errorCount: 0, totalDurationMs: 500, maxDurationMs: 200},
		{bucketStart: bucket, route: "/api/users", method: "GET", statusCode: 500, requestCount: 2, errorCount: 2, totalDurationMs: 900, maxDurationMs: 500},
		{bucketStart: bucket.Add(time.Minute), route: "/api/users", method: "GET", statusCode: 200, requestCount: 30, errorCount: 0, totalDurationMs: 1500, maxDurationMs: 150},
		{bucketStart: bucket, route: "/api/login", method: "POST", statusCode: 401, requestCount: 5, errorCount: 5, totalDurationMs: 100, maxDurationMs: 40},
	}
	items, windowSeconds := aggregateRows(rows, "route")
	if windowSeconds != 120 {
		t.Fatalf("windowSeconds = %v, want 120", windowSeconds)
	}
	if len(items) != 2 {
		t.Fatalf("groups = %d, want 2", len(items))
	}
	var users, login *RankingItem
	for index := range items {
		if items[index].Route == "/api/users" {
			users = &items[index]
		}
		if items[index].Route == "/api/login" {
			login = &items[index]
		}
	}
	if users == nil || login == nil {
		t.Fatalf("expected both routes: %#v", items)
	}
	if users.RequestCount != 42 || users.ErrorCount != 2 {
		t.Fatalf("users counts = %d/%d, want 42/2", users.RequestCount, users.ErrorCount)
	}
	// errorRate = 2/42; avgDuration = (500+900+1500)/42; qps = 42/120
	if almostEqual(users.ErrorRate, 2.0/42.0) == false {
		t.Fatalf("users errorRate = %v, want %v", users.ErrorRate, 2.0/42.0)
	}
	if almostEqual(users.AvgDurationMs, 2900.0/42.0) == false {
		t.Fatalf("users avgDuration = %v, want %v", users.AvgDurationMs, 2900.0/42.0)
	}
	if almostEqual(users.QPS, 42.0/120.0) == false {
		t.Fatalf("users qps = %v, want %v", users.QPS, 42.0/120.0)
	}
	if users.MaxDurationMs != 500 {
		t.Fatalf("users maxDuration = %d, want 500", users.MaxDurationMs)
	}
	if login.RequestCount != 5 || login.ErrorRate != 1 || login.MaxDurationMs != 40 {
		t.Fatalf("login group unexpected: %#v", login)
	}
}

func TestAggregateRowsGroupByMethodAndStatus(t *testing.T) {
	bucket := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	rows := []bucketRow{
		{bucketStart: bucket, route: "/a", method: "GET", statusCode: 200, requestCount: 4, totalDurationMs: 40},
		{bucketStart: bucket, route: "/b", method: "POST", statusCode: 404, requestCount: 2, errorCount: 2, totalDurationMs: 20},
	}
	byMethod, _ := aggregateRows(rows, "method")
	if len(byMethod) != 2 || byMethod[0].Method == "" || byMethod[1].Method == "" {
		t.Fatalf("unexpected method groups: %#v", byMethod)
	}
	byStatus, _ := aggregateRows(rows, "status")
	if len(byStatus) != 2 {
		t.Fatalf("status groups = %d, want 2", len(byStatus))
	}
	for _, item := range byStatus {
		if item.StatusCode == nil {
			t.Fatalf("status group missing status code: %#v", item)
		}
	}
}

func TestAggregateRowsEmptyAndZeroCounts(t *testing.T) {
	items, windowSeconds := aggregateRows(nil, "route")
	if len(items) != 0 || windowSeconds != 0 {
		t.Fatalf("empty aggregation = %#v, %v", items, windowSeconds)
	}
	// Zero request counts must not divide by zero.
	bucket := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	rows := []bucketRow{
		{bucketStart: bucket, route: "/api/empty", method: "GET", statusCode: 200, requestCount: 0, errorCount: 0, totalDurationMs: 0},
	}
	items, windowSeconds = aggregateRows(rows, "route")
	if windowSeconds != 60 {
		t.Fatalf("single bucket window = %v, want 60", windowSeconds)
	}
	if len(items) != 1 {
		t.Fatalf("groups = %d, want 1", len(items))
	}
	item := items[0]
	if item.ErrorRate != 0 || item.AvgDurationMs != 0 || item.QPS != 0 {
		t.Fatalf("zero-count group must have zero ratios: %#v", item)
	}
}

// --- sampler -----------------------------------------------------------------

func TestSamplerObserveIsNoopWhileDisabled(t *testing.T) {
	store := &fakeBucketStore{}
	sampler, err := NewSampler(SamplerOptions{
		Settings: &fakeSettings{}, Buckets: store, FlushInterval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create sampler: %v", err)
	}
	defer sampler.Close()

	sampler.Observe(event(200, 5, "GET", "/api/users"))
	sampler.Observe(event(500, 5, "GET", "/api/users"))
	time.Sleep(60 * time.Millisecond)

	store.mu.Lock()
	upserts := len(store.upserts)
	pruned := store.pruned
	store.mu.Unlock()
	if upserts != 0 || pruned {
		t.Fatalf("disabled sampler wrote metrics: upserts=%d pruned=%v", upserts, pruned)
	}
}

func TestSamplerAggregatesAndFlushesBoundedKeys(t *testing.T) {
	store := &fakeBucketStore{}
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	sampler, err := NewSampler(SamplerOptions{
		Settings:      &fakeSettings{},
		Buckets:       store,
		FlushInterval: time.Hour,
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("create sampler: %v", err)
	}
	defer sampler.Close()
	sampler.SetRouteIndex(NewRouteIndex([]gin.RouteInfo{
		{Method: "GET", Path: "/api/users/:id"},
	}))
	if _, err := sampler.SetEnabled(true, 7); err != nil {
		t.Fatalf("enable sampling: %v", err)
	}

	// Two parameterized requests fold into one template bucket.
	sampler.Observe(event(200, 10, "GET", "/api/users/1"))
	sampler.Observe(event(200, 30, "GET", "/api/users/2"))
	sampler.Observe(event(500, 60, "GET", "/api/users/3"))
	// Unmatched path falls back to heuristic normalization.
	sampler.Observe(event(404, 5, "GET", "/api/unknown/42"))
	// Invalid status codes are dropped.
	invalid := int64(99)
	sampler.Observe(models.OperationLogEvent{Path: "/api/users/1", Method: "GET", StatusCode: &invalid, DurationMs: int64Ptr(1)})

	sampler.drain()

	rows := store.mergedUpserts()
	if len(rows) != 3 {
		t.Fatalf("upserted rows = %d, want 3: %#v", len(rows), rows)
	}
	var users, users500, unknown *bucketRow
	for index := range rows {
		switch {
		case rows[index].route == "/api/users/{id}" && rows[index].statusCode == 200:
			users = &rows[index]
		case rows[index].route == "/api/users/{id}" && rows[index].statusCode == 500:
			users500 = &rows[index]
		case rows[index].route == "/api/unknown/{id}":
			unknown = &rows[index]
		}
	}
	if users == nil || users.requestCount != 2 || users.totalDurationMs != 40 || users.maxDurationMs != 30 {
		t.Fatalf("users 200 bucket unexpected: %#v", users)
	}
	if users500 == nil || users500.requestCount != 1 || users500.errorCount != 1 {
		t.Fatalf("users 500 bucket unexpected: %#v", users500)
	}
	if unknown == nil || unknown.requestCount != 1 || unknown.errorCount != 1 {
		t.Fatalf("unknown bucket unexpected: %#v", unknown)
	}
}

func TestSamplerAccumulatesAcrossFlushesAndDrainsOnDisable(t *testing.T) {
	store := &fakeBucketStore{}
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	sampler, err := NewSampler(SamplerOptions{
		Settings:      &fakeSettings{},
		Buckets:       store,
		FlushInterval: time.Hour,
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("create sampler: %v", err)
	}
	defer sampler.Close()
	if _, err := sampler.SetEnabled(true, 7); err != nil {
		t.Fatalf("enable sampling: %v", err)
	}

	sampler.Observe(event(200, 10, "GET", "/api/users"))
	sampler.drain()
	sampler.Observe(event(200, 20, "GET", "/api/users"))

	// Disabling drains the second batch, then writes stop.
	if _, err := sampler.SetEnabled(false, 8); err != nil {
		t.Fatalf("disable sampling: %v", err)
	}
	beforeDisable := len(store.mergedUpserts())
	sampler.Observe(event(200, 30, "GET", "/api/users"))
	sampler.drain()
	if got := len(store.mergedUpserts()); got != beforeDisable {
		t.Fatalf("disabled sampler wrote metrics: %d -> %d", beforeDisable, got)
	}

	rows := store.mergedUpserts()
	if len(rows) != 1 || rows[0].requestCount != 2 || rows[0].totalDurationMs != 30 {
		t.Fatalf("accumulated bucket unexpected: %#v", rows)
	}
}

func TestSamplerPrunesOnFlushTick(t *testing.T) {
	store := &fakeBucketStore{}
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	sampler, err := NewSampler(SamplerOptions{
		Settings:      &fakeSettings{},
		Buckets:       store,
		FlushInterval: 20 * time.Millisecond,
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("create sampler: %v", err)
	}
	defer sampler.Close()
	if _, err := sampler.SetEnabled(true, 1); err != nil {
		t.Fatalf("enable sampling: %v", err)
	}
	sampler.Observe(event(200, 1, "GET", "/api/users"))

	deadline := time.Now().Add(time.Second)
	for {
		store.mu.Lock()
		pruned := store.pruned
		store.mu.Unlock()
		if pruned {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sampler did not prune expired buckets")
		}
		time.Sleep(5 * time.Millisecond)
	}
	wantBefore := now.Add(-time.Duration(defaultRetentionDays) * 24 * time.Hour)
	store.mu.Lock()
	gotBefore := store.prunedBefore
	store.mu.Unlock()
	if !gotBefore.Equal(wantBefore) {
		t.Fatalf("prune boundary = %v, want %v", gotBefore, wantBefore)
	}
}

func TestSamplerClampsNegativeDurationsAndDropsNilFields(t *testing.T) {
	store := &fakeBucketStore{}
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	sampler, err := NewSampler(SamplerOptions{
		Settings: &fakeSettings{}, Buckets: store, FlushInterval: time.Hour,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("create sampler: %v", err)
	}
	defer sampler.Close()
	if _, err := sampler.SetEnabled(true, 1); err != nil {
		t.Fatalf("enable sampling: %v", err)
	}

	negative := int64(-4)
	sampler.Observe(models.OperationLogEvent{Path: "/api/x", Method: "GET", StatusCode: int64Ptr(200), DurationMs: &negative})
	sampler.Observe(models.OperationLogEvent{Path: "/api/x", Method: "GET", StatusCode: nil, DurationMs: int64Ptr(5)})
	sampler.Observe(models.OperationLogEvent{Path: "/api/x", Method: "GET", StatusCode: int64Ptr(200), DurationMs: nil})
	sampler.drain()

	rows := store.mergedUpserts()
	if len(rows) != 1 || rows[0].totalDurationMs != 0 || rows[0].requestCount != 1 {
		t.Fatalf("clamped bucket unexpected: %#v", rows)
	}
}

func TestSamplerStatusTracksEnabledAndLastFlush(t *testing.T) {
	settings := &fakeSettings{}
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	sampler, err := NewSampler(SamplerOptions{
		Settings: settings, Buckets: &fakeBucketStore{}, FlushInterval: time.Hour,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("create sampler: %v", err)
	}
	defer sampler.Close()

	if status := sampler.Status(); status.Enabled || status.BucketSeconds != 60 || status.RetentionDays != defaultRetentionDays {
		t.Fatalf("unexpected initial status: %#v", status)
	}
	if _, err := sampler.SetEnabled(true, 3); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if settings.savedBy != 3 {
		t.Fatalf("updated by = %d, want 3", settings.savedBy)
	}
	status, err := sampler.SetEnabled(false, 4)
	if err != nil || status.Enabled {
		t.Fatalf("disable failed: %v %#v", err, status)
	}
}

// --- query parsing -----------------------------------------------------------

func TestParseRankingQueryAppliesDefaultsAndValidation(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET",
		"/api/load-ranking/rankings?pageSize=500&method=post&statusCode=401&groupBy=method&dimension=qps&order=asc&startedAt=2026-07-31T00:00:00Z&endedAt=2026-07-31T01:00:00Z", nil)

	query, err := parseRankingQuery(context)
	if err != nil {
		t.Fatalf("parse valid query: %v", err)
	}
	if query.PageSize != maxPageSize || query.Method != "POST" {
		t.Fatalf("unexpected pagination/method: %#v", query)
	}
	if query.StatusCode == nil || *query.StatusCode != 401 {
		t.Fatalf("unexpected status filter: %#v", query.StatusCode)
	}
	if query.GroupBy != "method" || query.Dimension != "qps" || query.Order != "asc" {
		t.Fatalf("unexpected grouping/sort: %#v", query)
	}
	if query.StartedAt.Format(time.RFC3339) != "2026-07-31T00:00:00Z" || query.EndedAt.Format(time.RFC3339) != "2026-07-31T01:00:00Z" {
		t.Fatalf("unexpected time range: %v - %v", query.StartedAt, query.EndedAt)
	}

	invalid := []string{
		"/api/load-ranking/rankings?statusCode=99",
		"/api/load-ranking/rankings?statusCode=600",
		"/api/load-ranking/rankings?groupBy=user",
		"/api/load-ranking/rankings?dimension=latency",
		"/api/load-ranking/rankings?order=up",
		"/api/load-ranking/rankings?startedAt=not-a-time",
		"/api/load-ranking/rankings?startedAt=2026-07-31T02:00:00Z&endedAt=2026-07-31T01:00:00Z",
		"/api/load-ranking/rankings?startedAt=2026-01-01T00:00:00Z&endedAt=2026-08-01T00:00:00Z",
	}
	for _, path := range invalid {
		context.Request = httptest.NewRequest("GET", path, nil)
		if _, err := parseRankingQuery(context); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}

	// Defaults: last hour window, route grouping, requestCount desc.
	context.Request = httptest.NewRequest("GET", "/api/load-ranking/rankings", nil)
	query, err = parseRankingQuery(context)
	if err != nil {
		t.Fatalf("parse default query: %v", err)
	}
	if query.GroupBy != "route" || query.Dimension != "requestCount" || query.Order != "desc" {
		t.Fatalf("unexpected defaults: %#v", query)
	}
	if query.Page != 1 || query.PageSize != defaultPageSize {
		t.Fatalf("unexpected default pagination: %#v", query)
	}
	if query.EndedAt.Sub(query.StartedAt) != time.Hour {
		t.Fatalf("default window = %v, want 1h", query.EndedAt.Sub(query.StartedAt))
	}
}

func TestSortRankingItemsByDimension(t *testing.T) {
	items := []RankingItem{
		{Route: "/slow", RequestCount: 1, QPS: 0.01, ErrorRate: 0.5, AvgDurationMs: 900},
		{Route: "/busy", RequestCount: 100, QPS: 1, ErrorRate: 0.1, AvgDurationMs: 50},
		{Route: "/bad", RequestCount: 10, QPS: 0.1, ErrorRate: 0.9, AvgDurationMs: 100},
	}
	if sorted := sortRankingItems(items, "requestCount", "desc"); sorted[0].Route != "/busy" || sorted[2].Route != "/slow" {
		t.Fatalf("requestCount sort failed: %#v", sorted)
	}
	if sorted := sortRankingItems(items, "avgDurationMs", "desc"); sorted[0].Route != "/slow" {
		t.Fatalf("avgDuration sort failed: %#v", sorted)
	}
	if sorted := sortRankingItems(items, "errorRate", "desc"); sorted[0].Route != "/bad" {
		t.Fatalf("errorRate sort failed: %#v", sorted)
	}
	if sorted := sortRankingItems(items, "qps", "asc"); sorted[0].Route != "/slow" {
		t.Fatalf("qps asc sort failed: %#v", sorted)
	}
}

func TestSliceRankingPageBounds(t *testing.T) {
	items := make([]RankingItem, 5)
	for index := range items {
		items[index] = RankingItem{Route: "/r" + itoa(int64(index))}
	}
	page := sliceRankingPage(items, 1, 2)
	if len(page.Items) != 2 || page.Total != 5 {
		t.Fatalf("first page unexpected: %#v", page)
	}
	page = sliceRankingPage(items, 3, 2)
	if len(page.Items) != 1 {
		t.Fatalf("last page unexpected: %#v", page)
	}
	page = sliceRankingPage(items, 99, 2)
	if len(page.Items) != 0 || page.Total != 5 {
		t.Fatalf("out-of-range page unexpected: %#v", page)
	}
}

// --- routes ------------------------------------------------------------------

func TestLoadRankRoutesReadStatusUpdateAndRankings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeBucketStore{}
	store.rows = []bucketRow{
		{
			bucketStart: time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC),
			route:       "/api/users", method: "GET", statusCode: 200,
			requestCount: 10, totalDurationMs: 500, maxDurationMs: 100,
		},
	}
	now := time.Date(2026, 7, 31, 8, 30, 0, 0, time.UTC)
	sampler, err := NewSampler(SamplerOptions{
		Settings: &fakeSettings{}, Buckets: store, FlushInterval: time.Hour,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("create sampler: %v", err)
	}
	defer sampler.Close()

	engine := gin.New()
	RegisterRoutes(engine.Group("/api"), sampler, Dependencies{
		RequireAuth: func(c *gin.Context) { c.Next() },
		RequirePermission: func(...string) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
	})

	response := performRequest(engine, http.MethodGet, "/api/load-ranking/status", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Code int    `json:"code"`
		Data Status `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if envelope.Code != 0 || envelope.Data.Enabled || envelope.Data.BucketSeconds != 60 {
		t.Fatalf("unexpected status response: %#v", envelope)
	}

	response = performRequest(engine, http.MethodPatch, "/api/load-ranking/status", []byte(`{}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid PATCH = %d, body = %s", response.Code, response.Body.String())
	}

	response = performRequest(engine, http.MethodPatch, "/api/load-ranking/status", []byte(`{"enabled":true}`))
	if response.Code != http.StatusOK {
		t.Fatalf("PATCH = %d, body = %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode updated status: %v", err)
	}
	if envelope.Code != 0 || !envelope.Data.Enabled {
		t.Fatalf("unexpected enabled response: %#v", envelope)
	}

	response = performRequest(engine, http.MethodGet,
		"/api/load-ranking/rankings?startedAt=2026-07-31T08:00:00Z&endedAt=2026-07-31T09:00:00Z", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("GET rankings = %d, body = %s", response.Code, response.Body.String())
	}
	var rankingEnvelope struct {
		Code int             `json:"code"`
		Data RankingResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rankingEnvelope); err != nil {
		t.Fatalf("decode rankings: %v", err)
	}
	if rankingEnvelope.Code != 0 || rankingEnvelope.Data.Total != 1 || len(rankingEnvelope.Data.Items) != 1 {
		t.Fatalf("unexpected rankings response: %#v", rankingEnvelope)
	}
	item := rankingEnvelope.Data.Items[0]
	if item.Route != "/api/users" || item.RequestCount != 10 || item.AvgDurationMs != 50 || item.QPS <= 0 {
		t.Fatalf("unexpected ranking item: %#v", item)
	}
}

func performRequest(engine http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func almostEqual(left float64, right float64) bool {
	delta := left - right
	if delta < 0 {
		delta = -delta
	}
	return delta < 1e-9
}
