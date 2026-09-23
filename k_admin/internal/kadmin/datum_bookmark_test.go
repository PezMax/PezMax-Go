package kadmin

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// fake DB bookkeeping for ptmj_bookmark
// ---------------------------------------------------------------------------

// matchBookmarks emulates bookmarkFilterWhere: filter values are extracted
// once per query, in the repo's clause order (subject, resource_type,
// collection, url, user_id, keyword×3). FindByID/insert never reach this.
func (f *fakeDatumDB) matchBookmarks(query string, args []interface{}) []map[string]interface{} {
	argIndex := 0
	want := func(marker string) (string, bool) {
		if !strings.Contains(query, marker) {
			return "", false
		}
		var value string
		if argIndex < len(args) {
			value = toDatumString(args[argIndex])
		}
		argIndex++
		return value, true
	}
	wantSubject, hasSubject := want("subject = ?")
	wantResource, hasResource := want("resource_type = ?")
	wantCollection, hasCollection := want("collection = ?")
	wantURL, hasURL := want("url = ?")
	wantUser, hasUser := want("user_id = ?")
	wantKeyword := ""
	if strings.Contains(query, "ILIKE") {
		if argIndex < len(args) {
			wantKeyword = strings.Trim(toDatumString(args[argIndex]), "%")
		}
	}
	rows := []map[string]interface{}{}
	for _, bookmark := range f.bookmarks {
		if toDatumInt64(bookmark["del_flag"]) != 0 {
			continue
		}
		if strings.Contains(query, "status = 1") && toDatumInt64(bookmark["status"]) != 1 {
			continue
		}
		if hasSubject && toDatumString(bookmark["subject"]) != wantSubject {
			continue
		}
		if hasResource && toDatumString(bookmark["resource_type"]) != wantResource {
			continue
		}
		if hasCollection && toDatumString(bookmark["collection"]) != wantCollection {
			continue
		}
		if hasURL && toDatumString(bookmark["url"]) != wantURL {
			continue
		}
		if hasUser && toDatumString(bookmark["user_id"]) != wantUser {
			continue
		}
		if wantKeyword != "" || strings.Contains(query, "ILIKE") {
			if !strings.Contains(toDatumString(bookmark["title"]), wantKeyword) &&
				!strings.Contains(toDatumString(bookmark["description"]), wantKeyword) &&
				!strings.Contains(toDatumString(bookmark["url"]), wantKeyword) {
				continue
			}
		}
		rows = append(rows, bookmark)
	}
	sort.Slice(rows, func(i, j int) bool {
		return toDatumInt64(rows[i]["id"]) > toDatumInt64(rows[j]["id"])
	})
	return rows
}

func seedBookmarkRow(db *fakeDatumDB, id, userID int64, title, url, resourceType, collection string, status int64) {
	db.bookmarks[id] = map[string]interface{}{
		"id": id, "user_id": userID, "url": url, "title": title,
		"description": "", "cover_image": "", "subject": "",
		"resource_type": resourceType, "collection": collection,
		"status": status, "del_flag": int64(0),
		"create_time": "2026-09-23 09:00:00", "update_time": "2026-09-23 09:00:00", "remark": "",
	}
}

// datumBookmarkLogin seeds a captcha and logs the seeded user in, returning
// the datum session token.
func datumBookmarkLogin(t *testing.T, engine *gin.Engine, store *Store, username, password string) string {
	t.Helper()
	seedDatumCaptcha(t, store, "cap-bm")
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": username, "password": password, "code": "123456", "uuid": "cap-bm",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d body %s", recorder.Code, recorder.Body.String())
	}
	data, _ := datumBody(t, recorder)["data"].(map[string]interface{})
	token, _ := data["token"].(string)
	if token == "" {
		t.Fatalf("missing token in %v", data)
	}
	return token
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

func TestDatumBookmarkListVisibilityAndFilters(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false) // userID 7
	seedDatumUser(t, db, "bob", "secret5", "1", false)   // userID 8
	seedBookmarkRow(db, 201, 7, "高数资源", "https://e.com/1", "study", "期末", 1)
	seedBookmarkRow(db, 202, 7, "待审书签", "https://e.com/2", "study", "", 0)
	seedBookmarkRow(db, 203, 8, "娱乐站", "https://e.com/3", "entertainment", "", 1)
	engine := datumEngine(t, store)

	// 匿名列表：仅已审核，按 id 倒序
	recorder := datumJSON(t, engine, http.MethodGet, "/datum/bookmark/list?pageNum=1&pageSize=50", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	if body["code"].(float64) != 0 {
		t.Fatalf("list body = %v", body)
	}
	rows := body["rows"].([]interface{})
	if len(rows) != 2 {
		t.Fatalf("anonymous rows = %v", rows)
	}
	if first := rows[0].(map[string]interface{}); first["id"].(float64) != 203 {
		t.Fatalf("order by id desc expected 203 first, got %v", first)
	}

	// title 模糊搜索
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/bookmark/list?title=%E9%AB%98%E6%95%B0", "", nil)
	rows = datumBody(t, recorder)["rows"].([]interface{})
	if len(rows) != 1 || rows[0].(map[string]interface{})["id"].(float64) != 201 {
		t.Fatalf("title filter rows = %v", rows)
	}

	// url 精确匹配（桌面端保存封面后回查）
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/bookmark/list?url=https://e.com/3", "", nil)
	rows = datumBody(t, recorder)["rows"].([]interface{})
	if len(rows) != 1 || rows[0].(map[string]interface{})["id"].(float64) != 203 {
		t.Fatalf("url filter rows = %v", rows)
	}

	// 带 userId：出该用户全部状态（含待审 202）
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/bookmark/list?userId=7", "", nil)
	rows = datumBody(t, recorder)["rows"].([]interface{})
	if len(rows) != 2 {
		t.Fatalf("owner rows = %v", rows)
	}
}

func TestDatumBookmarkCreatePendingAndDetail(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)
	token := datumBookmarkLogin(t, engine, store, "alice", "secret5")

	// 未登录 → 401
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/bookmark", "", map[string]string{
		"url": "https://example.com/x", "title": "x",
	})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous create status = %d", recorder.Code)
	}

	// 缺 url → 400
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/bookmark", token, map[string]string{"title": "无网址"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing url status = %d", recorder.Code)
	}

	// 正常创建：落库为待审，双键返回 bookmarkId/id
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/bookmark", token, map[string]string{
		"url": "https://example.com/x", "title": "新 书签", "resourceType": "entertainment",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	if body["code"].(float64) != 0 {
		t.Fatalf("create body = %v", body)
	}
	data := body["data"].(map[string]interface{})
	bookmarkID := int64(data["bookmarkId"].(float64))
	if bookmarkID <= 0 || int64(data["id"].(float64)) != bookmarkID {
		t.Fatalf("create data = %v", data)
	}
	row, exists := db.bookmarks[bookmarkID]
	if !exists {
		t.Fatalf("bookmark %d not stored", bookmarkID)
	}
	if toDatumInt64(row["status"]) != 0 || toDatumInt64(row["user_id"]) != 7 {
		t.Fatalf("stored row = %v", row)
	}

	// 匿名详情看不到待审；属主凭会话可见
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/bookmark/"+toDatumString(bookmarkID), "", nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("anonymous pending detail status = %d", recorder.Code)
	}
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/bookmark/"+toDatumString(bookmarkID), token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("owner pending detail status = %d", recorder.Code)
	}
	if detail := datumBody(t, recorder); detail["data"].(map[string]interface{})["title"] != "新 书签" {
		t.Fatalf("detail = %v", detail)
	}
}

func TestDatumBookmarkUpdateMergeAndOwnership(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumUser(t, db, "bob", "secret5", "1", false)
	seedBookmarkRow(db, 301, 7, "原标题", "https://e.com/a", "study", "", 1)
	engine := datumEngine(t, store)
	alice := datumBookmarkLogin(t, engine, store, "alice", "secret5")
	bob := datumBookmarkLogin(t, engine, store, "bob", "secret5")

	// 桌面端封面同步只发 {id, coverImage}：其余字段必须保持原值
	recorder := datumJSON(t, engine, http.MethodPut, "/datum/bookmark", alice, map[string]string{
		"id": "301", "coverImage": "https://cdn.example.com/c.png",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("partial update status = %d body %s", recorder.Code, recorder.Body.String())
	}
	row := db.bookmarks[301]
	if toDatumString(row["cover_image"]) != "https://cdn.example.com/c.png" {
		t.Fatalf("cover not updated: %v", row)
	}
	if toDatumString(row["title"]) != "原标题" || toDatumString(row["url"]) != "https://e.com/a" {
		t.Fatalf("merge clobbered fields: %v", row)
	}

	// 非属主 → 404
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/bookmark", bob, map[string]string{
		"id": "301", "title": "抢改",
	})
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("foreign update status = %d", recorder.Code)
	}

	// 缺 id → 400
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/bookmark", alice, map[string]string{"title": "无 ID"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing id status = %d", recorder.Code)
	}
}

func TestDatumBookmarkDeleteOwnership(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumUser(t, db, "bob", "secret5", "1", false)
	seedBookmarkRow(db, 301, 7, "alice 的书签", "https://e.com/a", "study", "", 1)
	engine := datumEngine(t, store)
	alice := datumBookmarkLogin(t, engine, store, "alice", "secret5")
	bob := datumBookmarkLogin(t, engine, store, "bob", "secret5")

	// 非属主删除 → 404，行不受影响
	recorder := datumJSON(t, engine, http.MethodDelete, "/datum/bookmark/301", bob, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("foreign delete status = %d", recorder.Code)
	}
	if toDatumInt64(db.bookmarks[301]["del_flag"]) != 0 {
		t.Fatal("foreign delete must not touch the row")
	}

	// 属主删除 → 软删
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/bookmark/301", alice, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("owner delete status = %d", recorder.Code)
	}
	if toDatumInt64(db.bookmarks[301]["del_flag"]) != 1 {
		t.Fatalf("row not soft-deleted: %v", db.bookmarks[301])
	}
}

func TestDatumBookmarkUploadCover(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumUser(t, db, "bob", "secret5", "1", false)
	seedBookmarkRow(db, 401, 7, "alice 书签", "https://e.com/a", "study", "", 1)
	engine := datumEngine(t, store)

	var putKeys []string
	originalPut := datumFilePut
	datumFilePut = func(_ context.Context, objectKey string, _ []byte, _ string) (string, error) {
		putKeys = append(putKeys, objectKey)
		return "https://cdn.example.com/ptmj/" + objectKey, nil
	}
	t.Cleanup(func() { datumFilePut = originalPut })

	alice := datumBookmarkLogin(t, engine, store, "alice", "secret5")
	bob := datumBookmarkLogin(t, engine, store, "bob", "secret5")

	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 13}
	postCover := func(token, bookmarkID string, body []byte) *httptest.ResponseRecorder {
		payload := &bytes.Buffer{}
		writer := multipart.NewWriter(payload)
		part, _ := writer.CreateFormFile("file", "cover.png")
		_, _ = part.Write(body)
		if bookmarkID != "" {
			_ = writer.WriteField("bookmarkId", bookmarkID)
		}
		_ = writer.Close()
		request := httptest.NewRequest(http.MethodPost, "/datum/bookmark/uploadCover", bytes.NewReader(payload.Bytes()))
		request.Header.Set("Content-Type", writer.FormDataContentType())
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		return recorder
	}

	// 未登录 → 401
	if recorder := postCover("", "401", pngBytes); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous cover status = %d", recorder.Code)
	}

	// 缺 bookmarkId → 400
	if recorder := postCover(alice, "", pngBytes); recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing bookmarkId status = %d", recorder.Code)
	}

	// 非属主 → 404
	if recorder := postCover(bob, "401", pngBytes); recorder.Code != http.StatusNotFound {
		t.Fatalf("foreign cover status = %d", recorder.Code)
	}

	// 非图片 → 400
	if recorder := postCover(alice, "401", []byte("plain text not an image")); recorder.Code != http.StatusBadRequest {
		t.Fatalf("non-image status = %d", recorder.Code)
	}

	// 属主上传 PNG → 200，入库 URL，cover_image 同步
	recorder := postCover(alice, "401", pngBytes)
	if recorder.Code != http.StatusOK {
		t.Fatalf("cover status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	if body["code"].(float64) != 0 {
		t.Fatalf("cover body = %v", body)
	}
	data := body["data"].(map[string]interface{})
	if data["url"] == "" || int64(data["bookmarkId"].(float64)) != 401 {
		t.Fatalf("cover data = %v", data)
	}
	if len(putKeys) != 1 || !strings.HasPrefix(putKeys[0], "bookmarks/401/") || !strings.HasSuffix(putKeys[0], ".png") {
		t.Fatalf("objectKey = %v", putKeys)
	}
	if toDatumString(db.bookmarks[401]["cover_image"]) != data["url"] {
		t.Fatalf("cover_image not synced: %v", db.bookmarks[401])
	}
}
