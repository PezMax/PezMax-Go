package kadmin

import (
	"net/http"

	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func seedActivityFixture(t *testing.T) (*Store, *fakeDatumDB, *gin.Engine, string) {
	t.Helper()
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumUser(t, db, "bob", "secret5", "1", false)
	seedApprovedFile(db, 1001, 7, "试卷A.pdf", "高数", 1, 2024, 1)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")
	return store, db, engine, token
}

func seedActivityBookmark(t *testing.T, db *fakeDatumDB, bookmarkID int64) {
	t.Helper()
	db.bookmarks[bookmarkID] = map[string]interface{}{
		"id": bookmarkID, "user_id": int64(7), "url": "https://go.dev", "title": "Go 官网",
		"description": "", "cover_image": "", "subject": "工具", "resource_type": "tool",
		"collection": "", "status": int64(1), "del_flag": int64(0),
	}
}

func TestDatumFavoriteToggleOwnership(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)

	// 冒充他人 → 403
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/favorite", token, map[string]int64{"fileId": 1001, "userId": 999})
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("impersonate status = %d", recorder.Code)
	}
	// 文件不存在 → 404
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/favorite", token, map[string]int64{"fileId": 4040, "userId": 7})
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing file status = %d", recorder.Code)
	}
	// 正常收藏
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/favorite", token, map[string]int64{"fileId": 1001, "userId": 7})
	if recorder.Code != http.StatusOK {
		t.Fatalf("favorite status = %d body %s", recorder.Code, recorder.Body.String())
	}
	// 重复收藏 → 409
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/favorite", token, map[string]int64{"fileId": 1001, "userId": 7})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body %s", recorder.Code, recorder.Body.String())
	}
	// 状态查询
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/favorite/1001", token, nil)
	if datumBody(t, recorder)["data"].(map[string]interface{})["favorited"] != true {
		t.Fatal("favorited should be true")
	}
	// 我的收藏列表（RuoYi rows/total 顶层）
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/desktop/favorite/list/7?pageNum=1&pageSize=10", token, nil)
	body := datumBody(t, recorder)
	if body["total"].(float64) != 1 || len(body["rows"].([]interface{})) != 1 {
		t.Fatalf("my favorites = %v", body)
	}
	// 取消收藏
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/desktop/favorite/7/1001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("remove status = %d", recorder.Code)
	}
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/favorite/1001", token, nil)
	if datumBody(t, recorder)["data"].(map[string]interface{})["favorited"] != false {
		t.Fatal("favorited should be false after removal")
	}
	// 再次取消 → 404
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/desktop/favorite/7/1001", token, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("remove-again status = %d", recorder.Code)
	}
	_ = db
}

func TestDatumDownloadRecordCrud(t *testing.T) {
	store, db, engine, token := seedActivityFixture(t)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/download", token, map[string]interface{}{"fileId": 1001, "userId": 7})
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if len(db.downloads) != 1 {
		t.Fatalf("records = %d", len(db.downloads))
	}
	downloadID := toDatumInt64(db.downloads[0]["download_id"])

	// 详情（属主）
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/download/"+toDatumString(downloadID), token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d", recorder.Code)
	}
	// 他人详情 → 404
	bobToken := profileLogin(t, store, engine, "bob", "secret5", "cap-bob")
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/download/"+toDatumString(downloadID), bobToken, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("foreign detail status = %d", recorder.Code)
	}

	// 备注更新
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/download", token, map[string]interface{}{
		"downloadId": downloadID, "remark": "期末复习用",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d", recorder.Code)
	}
	if db.downloads[0]["remark"] != "期末复习用" {
		t.Fatalf("remark = %v", db.downloads[0]["remark"])
	}

	// 批量删除
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/download/"+toDatumString(downloadID), token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d", recorder.Code)
	}
	if len(db.downloads) != 0 {
		t.Fatal("record not deleted")
	}
}

func TestDatumMyDownloadsListsJoinedRows(t *testing.T) {
	store, db, engine, token := seedActivityFixture(t)
	_ = store
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/download", token, map[string]interface{}{"fileId": 1001, "userId": 7})
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d", recorder.Code)
	}

	// 他人 userId → 403
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/desktop/download/list/999?pageNum=1&pageSize=10", token, nil)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("foreign list status = %d", recorder.Code)
	}

	recorder = datumJSON(t, engine, http.MethodGet, "/datum/desktop/download/list/7?pageNum=1&pageSize=10", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	if body["total"].(float64) != 1 {
		t.Fatalf("total = %v", body["total"])
	}
	rows := body["rows"].([]interface{})
	row := rows[0].(map[string]interface{})
	if row["fileName"] != "试卷A.pdf" {
		t.Fatalf("row = %v", row)
	}

	// 按文件删除
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/desktop/download/7/1001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("remove status = %d", recorder.Code)
	}
	if len(db.downloads) != 0 {
		t.Fatal("download record not removed")
	}
}

func TestDatumBookmarkFavoriteFlow(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedActivityBookmark(t, db, 55)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/bookmark/favorite", token, map[string]int64{"bookmarkId": 55, "userId": 7})
	if recorder.Code != http.StatusOK {
		t.Fatalf("add status = %d body %s", recorder.Code, recorder.Body.String())
	}
	// 重复 → 409
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/bookmark/favorite", token, map[string]int64{"bookmarkId": 55, "userId": 7})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d", recorder.Code)
	}
	// 不存在的书签 → 404
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/bookmark/favorite", token, map[string]int64{"bookmarkId": 404, "userId": 7})
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing bookmark status = %d", recorder.Code)
	}
	// 关系列表
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/bookmark/favorite/list?pageNum=1&pageSize=10", token, nil)
	if datumBody(t, recorder)["total"].(float64) != 1 {
		t.Fatalf("relations = %s", recorder.Body.String())
	}
	// 我的书签收藏（联书签）
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/desktop/bookmark/favorite/list/7?pageNum=1&pageSize=10", token, nil)
	rows := datumBody(t, recorder)["rows"].([]interface{})
	if len(rows) != 1 || rows[0].(map[string]interface{})["title"] != "Go 官网" {
		t.Fatalf("my bookmark favorites = %s", recorder.Body.String())
	}
	// 取消
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/desktop/bookmark/favorite/7/55", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("remove status = %d", recorder.Code)
	}
	if len(db.bookmarkFavs) != 0 {
		t.Fatal("bookmark favorite not removed")
	}
}

func TestDatumStreamDownloadWritesRecord(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	localRoot := t.TempDir()
	t.Setenv("KADMIN_MINIO_ENABLED", "false")
	t.Setenv("KADMIN_FILE_LOCAL_ROOT", localRoot)

	// 本地落一份对象内容（模拟上传产物）
	target := filepath.Join(localRoot, "E2E", "a.pdf")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("%PDF-e2e"), 0o644); err != nil {
		t.Fatalf("write object: %v", err)
	}
	// file_url 指向公网地址（新上传入库形态），服务端解析出 bucket/key 后代理取流
	db.files[0]["file_url"] = "http://127.0.0.1:29000/ptmj/E2E/a.pdf"

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/download/file?fileId=1001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("stream status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.String(); got != "%PDF-e2e" {
		t.Fatalf("streamed body = %q", got)
	}
	if disposition := recorder.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "attachment") {
		t.Fatalf("disposition = %q", disposition)
	}
	if len(db.downloads) != 1 || db.downloads[0]["file_id"] != int64(1001) {
		t.Fatalf("download record missing: %v", db.downloads)
	}
}
