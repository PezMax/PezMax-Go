package notifications

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
)

// queryStub answers the first query containing the given text.
type queryStub struct {
	contains string
	rows     []map[string]interface{}
}

// captureConnection records SQL and serves canned rows from ordered stubs.
type captureConnection struct {
	db.Connection
	queries     []string
	argsHistory [][]interface{}
	execQuery   string
	stubs       []queryStub
}

func (c *captureConnection) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	c.queries = append(c.queries, query)
	c.argsHistory = append(c.argsHistory, args)
	for _, stub := range c.stubs {
		if strings.Contains(query, stub.contains) {
			return stub.rows, nil
		}
	}
	return nil, nil
}

func (c *captureConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	c.execQuery = query
	return fakeResult{}, nil
}

func (c *captureConnection) executed(part string) bool {
	for _, query := range c.queries {
		if strings.Contains(query, part) {
			return true
		}
	}
	return false
}

type fakeResult struct{ affected int64 }

func (f fakeResult) LastInsertId() (int64, error) { return 1, nil }
func (f fakeResult) RowsAffected() (int64, error) { return f.affected, nil }

func notificationRow(id int64, isRead bool) map[string]interface{} {
	return map[string]interface{}{
		"id":         id,
		"title":      "标题",
		"content":    "内容",
		"link":       "/kadmin/users",
		"type":       TypeInfo,
		"is_read":    isRead,
		"created_at": nil,
	}
}

func newTestRepository() (*repository, *captureConnection) {
	conn := &captureConnection{stubs: []queryStub{
		{contains: "INSERT INTO public.kadmin_notifications", rows: []map[string]interface{}{{"id": int64(9)}}},
		{contains: "WHERE id =", rows: []map[string]interface{}{notificationRow(9, false)}},
		{contains: "WHERE is_read = FALSE", rows: []map[string]interface{}{{"count": int64(4)}}},
		{contains: "SELECT count(*)", rows: []map[string]interface{}{{"count": int64(2)}}},
	}}
	return newRepository(conn), conn
}

func TestListFiltersUnreadOnly(t *testing.T) {
	repo, conn := newTestRepository()
	if _, err := repo.list(Filter{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if last := conn.queries[len(conn.queries)-1]; strings.Contains(last, "WHERE") {
		t.Fatalf("plain list must not carry a WHERE clause: %q", last)
	}
	if _, err := repo.list(Filter{Page: 1, PageSize: 20, UnreadOnly: true}); err != nil {
		t.Fatalf("list unread: %v", err)
	}
	if last := conn.queries[len(conn.queries)-1]; !strings.Contains(last, "WHERE is_read = FALSE") {
		t.Fatalf("unread list missing WHERE clause: %q", last)
	}
}

func TestListReturnsUnreadCount(t *testing.T) {
	repo, conn := newTestRepository()
	conn.stubs[3].rows = []map[string]interface{}{{"count": int64(2)}}
	conn.stubs = append(conn.stubs, queryStub{
		contains: "ORDER BY created_at DESC",
		rows: []map[string]interface{}{
			notificationRow(2, true),
			notificationRow(1, false),
		},
	})
	page, err := repo.list(Filter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 2 || page.Unread != 4 || len(page.Items) != 2 {
		t.Fatalf("page = %#v, want total=2 unread=4 items=2", page)
	}
}

func TestCreatePersistsAndReturnsNotification(t *testing.T) {
	repo, conn := newTestRepository()
	item, err := repo.create(Payload{Title: "系统消息", Content: "正文", Link: "/kadmin/users"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if item.ID != 9 || item.Title != "标题" || item.IsRead {
		t.Fatalf("created item = %#v", item)
	}
	if !conn.executed("RETURNING id") {
		t.Fatalf("create must use RETURNING id, queries = %#v", conn.queries)
	}
	insertArgs := conn.argsHistory[0]
	if len(insertArgs) < 4 || insertArgs[0] != "系统消息" {
		t.Fatalf("create args = %#v", insertArgs)
	}
}

func TestCreateRejectsInvalidType(t *testing.T) {
	repo, _ := newTestRepository()
	if _, err := repo.create(Payload{Title: "x", Type: "urgent"}); !errors.Is(err, errInvalidType) {
		t.Fatalf("invalid type error = %v", err)
	}
}

func TestMarkReadAndDeleteNotFound(t *testing.T) {
	repo, conn := newTestRepository()
	conn.stubs[1].rows = nil
	if _, err := repo.markRead(42); !errors.Is(err, errNotificationNotFound) {
		t.Fatalf("markRead missing error = %v", err)
	}
	conn.execQuery = ""
	if err := repo.delete(42); err == nil || !strings.Contains(conn.execQuery, "DELETE FROM public.kadmin_notifications") {
		t.Fatalf("delete missing behavior: query=%q err=%v", conn.execQuery, err)
	}
}

func TestHandlerCRUDFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conn := &captureConnection{stubs: []queryStub{
		{contains: "INSERT INTO public.kadmin_notifications", rows: []map[string]interface{}{{"id": int64(9)}}},
		{contains: "WHERE id =", rows: []map[string]interface{}{notificationRow(9, false)}},
		{contains: "WHERE is_read = FALSE", rows: []map[string]interface{}{{"count": int64(1)}}},
		{contains: "SELECT count(*)", rows: []map[string]interface{}{{"count": int64(1)}}},
		{contains: "ORDER BY created_at DESC", rows: []map[string]interface{}{notificationRow(9, false)}},
	}}
	engine := gin.New()
	RegisterRoutes(engine.Group("/api"), Dependencies{
		Connection:  conn,
		RequireAuth: func(c *gin.Context) { c.Next() },
		RequirePermission: func(...string) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
	})

	request := httptest.NewRequest(http.MethodPost, "/api/notifications", strings.NewReader(`{"title":"系统消息","content":"正文","type":"info"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", response.Code, response.Body.String())
	}
	var createEnvelope struct {
		Code int          `json:"code"`
		Data Notification `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &createEnvelope); err != nil || createEnvelope.Data.ID != 9 {
		t.Fatalf("create envelope = %#v err=%v", createEnvelope, err)
	}

	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/notifications?page=1&pageSize=20", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d", response.Code)
	}
	var listEnvelope struct {
		Data Page `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listEnvelope); err != nil || listEnvelope.Data.Unread != 1 {
		t.Fatalf("list envelope = %#v err=%v", listEnvelope, err)
	}

	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/api/notifications/9/read", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("mark read status = %d", response.Code)
	}
	if !strings.Contains(conn.execQuery, "SET is_read = TRUE") {
		t.Fatalf("mark read SQL = %q", conn.execQuery)
	}

	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/api/notification-batch/read-all", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("mark all read status = %d", response.Code)
	}

	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/notification-batch/read", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("clear read status = %d", response.Code)
	}
	if !strings.Contains(conn.execQuery, "WHERE is_read = TRUE") {
		t.Fatalf("clear read SQL = %q", conn.execQuery)
	}
}
