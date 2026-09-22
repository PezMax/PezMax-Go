package datum

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

// scriptedConn implements db.Connection (via embedding) and routes queries
// to handler functions registered by the tests, capturing every statement.
type scriptedConn struct {
	db.Connection
	mu        sync.Mutex
	executed  []string
	handlers  []scriptedHandler
	unmatched []string
}

type scriptedHandler struct {
	match func(query string, args []interface{}) bool
	run   func(query string, args []interface{}) ([]map[string]interface{}, error)
}

type scriptedResult struct{}

func (scriptedResult) LastInsertId() (int64, error) { return 0, nil }
func (scriptedResult) RowsAffected() (int64, error) { return 1, nil }

func newScriptedConn() *scriptedConn {
	return &scriptedConn{}
}

func (s *scriptedConn) on(match func(query string, args []interface{}) bool, run func(query string, args []interface{}) ([]map[string]interface{}, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, scriptedHandler{match: match, run: run})
}

// onSQL matches when the query contains every fragment (order-insensitive).
func (s *scriptedConn) onSQL(run func(query string, args []interface{}) ([]map[string]interface{}, error), fragments ...string) {
	s.on(func(query string, _ []interface{}) bool {
		for _, fragment := range fragments {
			if !strings.Contains(query, fragment) {
				return false
			}
		}
		return true
	}, run)
}

func (s *scriptedConn) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	s.mu.Lock()
	handlers := append([]scriptedHandler(nil), s.handlers...)
	s.executed = append(s.executed, "Q: "+query)
	s.mu.Unlock()
	for _, handler := range handlers {
		if handler.match(query, args) {
			return handler.run(query, args)
		}
	}
	s.mu.Lock()
	s.unmatched = append(s.unmatched, query)
	s.mu.Unlock()
	return nil, fmt.Errorf("scriptedConn: no handler for query %q", query)
}

func (s *scriptedConn) Exec(query string, args ...interface{}) (sql.Result, error) {
	s.mu.Lock()
	handlers := append([]scriptedHandler(nil), s.handlers...)
	s.executed = append(s.executed, "E: "+query)
	s.mu.Unlock()
	for _, handler := range handlers {
		if handler.match(query, args) {
			if _, err := handler.run(query, args); err != nil {
				return nil, err
			}
			return scriptedResult{}, nil
		}
	}
	s.mu.Lock()
	s.unmatched = append(s.unmatched, query)
	s.mu.Unlock()
	return scriptedResult{}, nil
}

func (s *scriptedConn) statementCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.executed)
}

// ---------------------------------------------------------------------------
// schema
// ---------------------------------------------------------------------------

func TestEnsureSchemaCreatesAllTenTablesIdempotently(t *testing.T) {
	conn := newScriptedConn()
	if err := EnsureSchema(conn); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	created := 0
	for _, statement := range conn.executed {
		if strings.Contains(statement, "CREATE TABLE IF NOT EXISTS") {
			created++
		}
	}
	if created != 10 {
		t.Fatalf("created %d tables, want 10", created)
	}
	tables := []string{
		"ptmj_user", "ptmj_security", "ptmj_file", "ptmj_file_download", "ptmj_file_favorite",
		"ptmj_report", "ptmj_bookmark", "ptmj_bookmark_favorite", "ptmj_bookmark_report", "ptmj_notification",
	}
	for _, table := range tables {
		found := false
		for _, statement := range conn.executed {
			if strings.Contains(statement, "CREATE TABLE IF NOT EXISTS "+table+" ") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("table %s was not created", table)
		}
	}
	// 幂等：第二次执行同样成功（语句全部为 IF NOT EXISTS，天然可重复）
	first := conn.statementCount()
	if err := EnsureSchema(conn); err != nil {
		t.Fatalf("EnsureSchema rerun: %v", err)
	}
	if conn.statementCount() != first*2 {
		t.Fatalf("rerun executed %d statements, want %d", conn.statementCount()-first, first)
	}
}

func TestEnsureSchemaRequiresConnection(t *testing.T) {
	if err := EnsureSchema(nil); err == nil {
		t.Fatal("nil connection should fail")
	}
}

func TestEnsureSchemaFailsFast(t *testing.T) {
	conn := newScriptedConn()
	conn.onSQL(func(_ string, _ []interface{}) ([]map[string]interface{}, error) {
		return nil, errors.New("boom")
	}, "CREATE TABLE")
	if err := EnsureSchema(conn); err == nil || !strings.Contains(err.Error(), "initialize datum schema") {
		t.Fatalf("error = %v, want wrapped schema error", err)
	}
}

// ---------------------------------------------------------------------------
// repositories
// ---------------------------------------------------------------------------

func TestFileRepoListAppliesAnonymousFilterAndScansRows(t *testing.T) {
	conn := newScriptedConn()
	var listArgs []interface{}
	var listQuery string
	conn.onSQL(func(query string, args []interface{}) ([]map[string]interface{}, error) {
		return []map[string]interface{}{{"count": int64(1)}}, nil
	}, "count(*)")
	conn.onSQL(func(query string, args []interface{}) ([]map[string]interface{}, error) {
		listQuery, listArgs = query, args
		return []map[string]interface{}{{
			"file_id": int64(1007), "user_id": int64(7), "file_name": "试卷.docx",
			"file_url": "http://files.example.com/ptmj/a.docx", "file_size": int64(2048),
			"file_format": "docx", "file_year": int64(2024), "file_type": int64(1),
			"file_school": "QLU", "file_subject": "高数", "reviewer": nil,
			"file_status": int64(1), "del_flag": int64(0), "create_by": "7",
			"create_time": nil, "update_by": nil, "update_time": nil, "remark": "期末/final",
		}}, nil
	}, "FROM ptmj_file")

	page, err := NewFileRepo(conn).List(FileFilter{OnlyApproved: true, FileType: 1, FileSubject: "高数", Page: 2, PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, required := range []string{"del_flag = 0", "file_status = 1", "file_type = ?", "file_subject = ?"} {
		if !strings.Contains(listQuery, required) {
			t.Errorf("query missing %q: %s", required, listQuery)
		}
	}
	if len(listArgs) != 4 { // type, subject, limit, offset（owner 条件未启用）
		t.Fatalf("args = %v, want 4 values", listArgs)
	}
	if listArgs[2] != 10 || listArgs[3] != 10 { // LIMIT 10 OFFSET 10
		t.Fatalf("limit/offset args = %v", listArgs[2:])
	}
	files := page.Items.([]File)
	if len(files) != 1 || files[0].FileID != 1007 || files[0].FileName != "试卷.docx" ||
		files[0].FileSchool != "QLU" || files[0].Remark != "期末/final" || files[0].Reviewer != "" {
		t.Fatalf("scanned file = %+v", files[0])
	}
	if page.Total != 1 || page.Page != 2 || page.PageSize != 10 {
		t.Fatalf("page = %+v", page)
	}
}

func TestFileRepoOwnerFilterSkipsApprovedGate(t *testing.T) {
	conn := newScriptedConn()
	conn.onSQL(func(_ string, _ []interface{}) ([]map[string]interface{}, error) {
		return []map[string]interface{}{{"count": int64(0)}}, nil
	}, "count(*)")
	conn.onSQL(func(_ string, _ []interface{}) ([]map[string]interface{}, error) {
		return nil, nil
	}, "FROM ptmj_file")
	_, err := NewFileRepo(conn).List(FileFilter{UserID: 7})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, statement := range conn.executed {
		if strings.Contains(statement, "file_status = 1") {
			t.Fatal("owner list must not force the approved gate")
		}
	}
}

func TestFileRepoInsertPropagatesDuplicateKey(t *testing.T) {
	conn := newScriptedConn()
	conn.onSQL(func(_ string, _ []interface{}) ([]map[string]interface{}, error) {
		return nil, errors.New("duplicate key value")
	}, "INSERT INTO ptmj_file")
	if _, err := NewFileRepo(conn).Insert(FilePayload{UserID: 7, FileName: "x", FileSubject: "s"}); err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("insert duplicate error = %v", err)
	}
}

func TestNotificationRepoActivePopupFilters(t *testing.T) {
	conn := newScriptedConn()
	var popupQuery string
	conn.onSQL(func(query string, _ []interface{}) ([]map[string]interface{}, error) {
		popupQuery = query
		return []map[string]interface{}{{
			"notify_id": int64(3), "notify_type": "1", "title": "v1.2 发布", "content": "升级",
			"status": "0", "sort": int64(9), "display_mode": "0",
			"publish_start": nil, "publish_end": nil, "scroll_time_interval": int64(30),
		}}, nil
	}, "display_mode = '0'")

	items, err := NewNotificationRepo(conn).ActivePopup("2026-09-20 10:00:00", 7)
	if err != nil {
		t.Fatalf("ActivePopup: %v", err)
	}
	for _, required := range []string{"status = '0'", "publish_start IS NULL OR publish_start <=", "display_mode = '0'", "upload_user_id = ?"} {
		if !strings.Contains(popupQuery, required) {
			t.Errorf("query missing %q", required)
		}
	}
	if items[0].NotifyID != 3 || items[0].NotifyType != "1" || items[0].Title != "v1.2 发布" {
		t.Fatalf("scanned notification = %+v", items[0])
	}
}

func TestUserStatsTopUploadersAndCounters(t *testing.T) {
	conn := newScriptedConn()
	conn.onSQL(func(_ string, _ []interface{}) ([]map[string]interface{}, error) {
		return []map[string]interface{}{{"user_id": int64(7), "user_name": "alice", "avatar": "/a.png", "count": int64(12)}}, nil
	}, "FROM ptmj_user")
	ranks, err := NewUserStats(conn).TopUploaders(10)
	if err != nil || len(ranks) != 1 || ranks[0].UserName != "alice" || ranks[0].Uploads != 12 {
		t.Fatalf("ranks = %+v err = %v", ranks, err)
	}

	conn.onSQL(func(_ string, _ []interface{}) ([]map[string]interface{}, error) {
		return []map[string]interface{}{{"count": int64(5)}}, nil
	}, "ptmj_file_download")
	conn.onSQL(func(_ string, _ []interface{}) ([]map[string]interface{}, error) {
		return []map[string]interface{}{{"count": int64(3)}}, nil
	}, "ptmj_file_favorite")
	conn.onSQL(func(_ string, _ []interface{}) ([]map[string]interface{}, error) {
		return []map[string]interface{}{{"count": int64(2)}}, nil
	}, "ptmj_bookmark_favorite")
	uploads, downloads, fileFavorites, bookmarkFavorites, err := NewUserStats(conn).ProfileStats(7)
	if err != nil || uploads != 12 || downloads != 5 || fileFavorites != 3 || bookmarkFavorites != 2 {
		t.Fatalf("ProfileStats = (%d,%d,%d,%d) err = %v", uploads, downloads, fileFavorites, bookmarkFavorites, err)
	}
}

func TestScanHelpers(t *testing.T) {
	if got := ScanInt64(int64(9)); got != 9 {
		t.Fatalf("ScanInt64(int64) = %d", got)
	}
	if got := ScanInt64(float64(9)); got != 9 {
		t.Fatalf("ScanInt64(float64) = %d", got)
	}
	if got := ScanInt64([]byte("42")); got != 42 {
		t.Fatalf("ScanInt64([]byte) = %d", got)
	}
	if got := ScanInt64(nil); got != 0 {
		t.Fatalf("ScanInt64(nil) = %d", got)
	}
	if got := ScanString(nil); got != "" {
		t.Fatalf("ScanString(nil) = %q", got)
	}
	if got := ScanString([]byte("x")); got != "x" {
		t.Fatalf("ScanString([]byte) = %q", got)
	}
}
