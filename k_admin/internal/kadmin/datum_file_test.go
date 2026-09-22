package kadmin

import (
	"bytes"
	"context"

	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func seedApprovedFile(db *fakeDatumDB, fileID, userID int64, name, subject string, fileType, year int64, status int64) {
	db.files = append(db.files, map[string]interface{}{
		"file_id": fileID, "user_id": userID, "file_name": name, "file_url": "http://files.example.com/ptmj/a.pdf",
		"file_size": int64(1024), "file_format": "pdf", "file_year": year, "file_type": fileType,
		"file_school": "QLU", "file_subject": subject, "reviewer": "", "file_status": status,
		"del_flag": int64(0), "remark": "",
	})
}

func treeHashOf(t *testing.T, body map[string]interface{}) string {
	t.Helper()
	hash, _ := body["hash"].(string)
	if hash == "" {
		t.Fatalf("missing hash in response: %v", body)
	}
	return hash
}

func TestDatumTreeHashStateFlow(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedApprovedFile(db, 1001, 7, "试卷A.pdf", "高数", 1, 2024, 1)
	seedApprovedFile(db, 1002, 7, "资料B.md", "英语", 4, 2023, 1)
	engine := datumEngine(t, store)

	// 首次：完整树 + 哈希
	recorder := datumJSON(t, engine, http.MethodGet, "/datum/file/tree", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("tree status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	hash := treeHashOf(t, body)
	if body["unchanged"] != false {
		t.Fatalf("first fetch must not be unchanged: %v", body)
	}
	tree := body["data"].([]interface{})
	if len(tree) == 0 {
		t.Fatalf("tree empty: %v", body)
	}
	typeNode := tree[0].(map[string]interface{})
	if typeNode["type"] != "folder" || typeNode["label"] != "期末" {
		t.Fatalf("type node = %v", typeNode)
	}
	subjects := typeNode["children"].([]interface{})
	leafYear := subjects[0].(map[string]interface{})["children"].([]interface{})
	fileLeaf := leafYear[0].(map[string]interface{})["children"].([]interface{})[0].(map[string]interface{})
	if fileLeaf["type"] != "file" || fileLeaf["fileId"].(float64) != 1001 {
		t.Fatalf("file leaf = %v", fileLeaf)
	}

	// 再次请求（不带 hash）：同哈希、完整载荷
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/file/tree", "", nil)
	body = datumBody(t, recorder)
	if treeHashOf(t, body) != hash || body["unchanged"] != false {
		t.Fatalf("second fetch should reuse cache: %v", body["unchanged"])
	}

	// 携带相同 hash：unchanged=true 且无载荷
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/file/tree?hash="+hash, "", nil)
	body = datumBody(t, recorder)
	if body["unchanged"] != true || treeHashOf(t, body) != hash {
		t.Fatalf("matching hash should yield unchanged: %v", body)
	}
	if data, ok := body["data"].(interface{}); ok && data != nil {
		_, isList := data.([]interface{})
		if isList {
			t.Fatalf("unchanged response must not carry the payload: %v", body["data"])
		}
	}

	// 携带过期 hash：完整载荷
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/file/tree?hash=stale0000", "", nil)
	if datumBody(t, recorder)["unchanged"] != false {
		t.Fatal("stale hash must return full payload")
	}

	// 失效后重建：哈希刷新
	snapshot := db.files
	db.files = append(snapshot, map[string]interface{}{
		"file_id": int64(1003), "user_id": int64(7), "file_name": "新文件.pdf", "file_url": "http://x/y.pdf",
		"file_size": int64(1), "file_format": "pdf", "file_year": int64(2025), "file_type": int64(1),
		"file_school": "QLU", "file_subject": "高数", "file_status": int64(1), "del_flag": int64(0),
	})
	store.invalidateDatumTree()
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/file/tree", "", nil)
	body = datumBody(t, recorder)
	if treeHashOf(t, body) == hash {
		t.Fatal("hash must refresh after invalidation")
	}
}

func TestDatumRankHashFlow(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/rank", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("rank status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	hash := treeHashOf(t, body)
	rows := body["data"].([]interface{})
	if len(rows) != 1 {
		t.Fatalf("rank rows = %v", rows)
	}
	item := rows[0].(map[string]interface{})
	if item["userName"] != "alice" || item["count"].(float64) != 3 {
		t.Fatalf("rank item = %v", item)
	}

	recorder = datumJSON(t, engine, http.MethodGet, "/datum/user/rank?hash="+hash, "", nil)
	if datumBody(t, recorder)["unchanged"] != true {
		t.Fatal("matching rank hash should yield unchanged")
	}

	store.invalidateDatumRank()
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/user/rank", "", nil)
	if treeHashOf(t, datumBody(t, recorder)) == hash {
		t.Fatal("rank hash must refresh after invalidation")
	}
}

func TestDatumFileUploadInvalidatesTreeAndCounts(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")

	// 基线树哈希
	recorder := datumJSON(t, engine, http.MethodGet, "/datum/file/tree", "", nil)
	baseline := treeHashOf(t, datumBody(t, recorder))

	// 替换存储函数：记录 objectKey，返回固定 URL
	var putKeys []string
	originalPut := datumFilePut
	datumFilePut = func(ctx context.Context, objectKey string, body []byte, contentType string) (string, error) {
		putKeys = append(putKeys, objectKey)
		return "http://files.example.com/ptmj/" + objectKey, nil
	}
	defer func() { datumFilePut = originalPut }()

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	part, _ := writer.CreateFormFile("file", "笔记.txt")
	_, _ = part.Write([]byte("hello 世界"))
	_ = writer.WriteField("fileName", "笔记")
	_ = writer.WriteField("fileSubject", "高数")
	_ = writer.WriteField("fileSchool", "QLU")
	_ = writer.WriteField("fileYear", "2025")
	_ = writer.WriteField("fileType", "1")
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/datum/file", bytes.NewReader(payload.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d body %s", recorder.Code, recorder.Body.String())
	}
	uploadBody := datumBody(t, recorder)
	if uploadBody["code"].(float64) != 0 {
		t.Fatalf("upload body = %v", uploadBody)
	}
	if len(putKeys) != 1 || !strings.HasPrefix(putKeys[0], "高数/QLU/期末/2025/") || !strings.HasSuffix(putKeys[0], ".txt") {
		t.Fatalf("objectKey = %v", putKeys)
	}

	// 行写入 + 计数 +1
	if len(db.files) != 1 {
		t.Fatalf("files = %d", len(db.files))
	}
	if db.users["alice"].Count != 4 {
		t.Fatalf("count = %d, want 4", db.users["alice"].Count)
	}

	// 树哈希已刷新
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/file/tree", "", nil)
	if treeHashOf(t, datumBody(t, recorder)) == baseline {
		t.Fatal("tree hash must change after upload")
	}

	// 未登录上传 → 401
	payload2 := &bytes.Buffer{}
	writer2 := multipart.NewWriter(payload2)
	part2, _ := writer2.CreateFormFile("file", "x.txt")
	_, _ = part2.Write([]byte("x"))
	_ = writer2.Close()
	request2 := httptest.NewRequest(http.MethodPost, "/datum/file", bytes.NewReader(payload2.Bytes()))
	request2.Header.Set("Content-Type", writer2.FormDataContentType())
	recorder2 := httptest.NewRecorder()
	engine.ServeHTTP(recorder2, request2)
	if recorder2.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous upload status = %d", recorder2.Code)
	}
}

func TestDatumFileDeleteInvalidatesAndCountsDown(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedApprovedFile(db, 1001, 7, "试卷.pdf", "高数", 1, 2024, 1)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/file/tree", "", nil)
	baseline := treeHashOf(t, datumBody(t, recorder))

	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/file/1001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if toDatumInt64(db.files[0]["del_flag"]) != 1 {
		t.Fatal("file not soft-deleted")
	}
	if db.users["alice"].Count != 2 {
		t.Fatalf("count = %d, want 2", db.users["alice"].Count)
	}
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/file/tree", "", nil)
	if treeHashOf(t, datumBody(t, recorder)) == baseline {
		t.Fatal("tree hash must change after delete")
	}
}

func TestDatumFileDetailOwnerVisibility(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedApprovedFile(db, 1001, 7, "待审.pdf", "高数", 1, 2024, 0) // pending
	engine := datumEngine(t, store)

	// 匿名看不到未审核
	recorder := datumJSON(t, engine, http.MethodGet, "/datum/file/1001", "", nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("anonymous pending status = %d", recorder.Code)
	}
	// 属主可见
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/file/1001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("owner pending status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if datumBody(t, recorder)["data"].(map[string]interface{})["fileName"] != "待审.pdf" {
		t.Fatal("detail payload mismatch")
	}
}
