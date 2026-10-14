package kadmin

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/files"
)

// 电子书模块契约：目录表 ptmj_ebook（后台 codegen 模块管理）进入
// /datum/ebook/list；预览走 /datum/ebook/content（inline、不落记录）；
// 下载走 /datum/download/ebook（datum 会话鉴权、落 ptmj_ebook_download）。

func seedEbook(db *fakeDatumDB, ebookID, userID int64, name, author, format, subject string, status int64) {
	db.ebooks = append(db.ebooks, map[string]interface{}{
		"ebook_id": ebookID, "user_id": userID, "ebook_name": name, "author": author,
		"publisher": "机械工业出版社", "cover_url": "", "ebook_url": "http://127.0.0.1:29000/ptmj/books/go.epub",
		"ebook_size": int64(2048), "ebook_format": format, "ebook_subject": subject, "ebook_type": int64(1),
		"reviewer": "", "ebook_status": status, "del_flag": int64(0), "remark": "",
	})
}

func TestDatumEbookList(t *testing.T) {
	_, db, engine, _ := seedActivityFixture(t)
	seedEbook(db, 2001, 7, "Go语言实战", "Kennedy", "epub", "编程", 1)
	seedEbook(db, 2002, 7, "未上架的书", "某人", "pdf", "编程", 0)

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/ebook/list", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ebook list status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	rows, _ := body["rows"].([]interface{})
	if body["total"].(float64) != 1 || len(rows) != 1 {
		t.Fatalf("ebook list = %v（应只含已上架电子书）", body)
	}
	row := rows[0].(map[string]interface{})
	if row["ebookId"].(float64) != 2001 {
		t.Fatalf("ebook row = %v", row)
	}
	if row["ebookName"] != "Go语言实战" || row["author"] != "Kennedy" || row["ebookFormat"] != "epub" {
		t.Fatalf("ebook fields = %v", row)
	}
	if row["type"] != "ebook" {
		t.Fatalf("ebook row type = %v", row["type"])
	}

	// 未上架（ebook_status=0）不出现在列表
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/ebook/list?pageSize=100", "", nil)
	rows = datumBody(t, recorder)["rows"].([]interface{})
	for _, item := range rows {
		if item.(map[string]interface{})["ebookId"].(float64) == 2002 {
			t.Fatal("unapproved ebook leaked into list")
		}
	}
}

// 预览流：inline 便于 Chromium PDF 阅读器与 epub.js 拉流，不落下载记录。
func TestDatumEbookPreviewContent(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedEbook(db, 2001, 7, "Go语言实战", "Kennedy", "epub", "编程", 1)

	localRoot := t.TempDir()
	t.Setenv("KADMIN_MINIO_ENABLED", "false")
	t.Setenv("KADMIN_FILE_LOCAL_ROOT", localRoot)
	payload := "PK.epub-e2e"
	target := filepath.Join(localRoot, "books", "go.epub")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte(payload), 0o644); err != nil {
		t.Fatalf("write object: %v", err)
	}

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/ebook/content?ebookId=2001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("content status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.String(); got != payload {
		t.Fatalf("content body = %q", got)
	}
	disposition := recorder.Header().Get("Content-Disposition")
	if !strings.HasPrefix(disposition, "inline") {
		t.Fatalf("disposition = %q, want inline", disposition)
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "epub") {
		t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
	}
	if recorder.Header().Get("ETag") == "" {
		t.Fatal("ETag missing on preview stream")
	}
	if db.ebookDownloads != 0 {
		t.Fatalf("preview must not write download records, got %d", db.ebookDownloads)
	}

	// 未上架且非属主 → 404
	seedEbook(db, 2002, 999, "未上架的书", "某人", "pdf", "编程", 0)
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/ebook/content?ebookId=2002", token, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unapproved content status = %d", recorder.Code)
	}
}

// 下载流：attachment + 落 ptmj_ebook_download 记录 + 书名补格式后缀。
func TestDatumEbookDownloadStream(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedEbook(db, 2001, 7, "Go语言实战", "Kennedy", "epub", "编程", 1)

	localRoot := t.TempDir()
	t.Setenv("KADMIN_MINIO_ENABLED", "false")
	t.Setenv("KADMIN_FILE_LOCAL_ROOT", localRoot)

	target := filepath.Join(localRoot, "books", "go.epub")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("PK.epub-e2e"), 0o644); err != nil {
		t.Fatalf("write object: %v", err)
	}

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/download/ebook?ebookId=2001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("stream status = %d body %s", recorder.Code, recorder.Body.String())
	}
	disposition := recorder.Header().Get("Content-Disposition")
	// 非_ascii 书名按 RFC 5987 编码进 filename*，校验附件语义 + 格式后缀。
	if !strings.HasPrefix(disposition, "attachment") || !strings.Contains(disposition, "filename*=utf-8''") || !strings.HasSuffix(disposition, ".epub") {
		t.Fatalf("disposition = %q, want attachment with format suffix", disposition)
	}
	if len(db.downloads) != 0 {
		t.Fatal("ebook download must not write ptmj_file_download records")
	}
	if db.ebookDownloads != 1 {
		t.Fatalf("ebook download records = %d, want 1", db.ebookDownloads)
	}
}

// 电子书下载同样支持 Range 断点续传（与试卷下载同一条 ServeContent 路径）。
func TestDatumEbookDownloadRangeResume(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedEbook(db, 2001, 7, "Go语言实战", "Kennedy", "epub", "编程", 1)

	localRoot := t.TempDir()
	t.Setenv("KADMIN_MINIO_ENABLED", "false")
	t.Setenv("KADMIN_FILE_LOCAL_ROOT", localRoot)

	payload := "PK.epub-e2e"
	target := filepath.Join(localRoot, "books", "go.epub")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte(payload), 0o644); err != nil {
		t.Fatalf("write object: %v", err)
	}

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/download/ebook?ebookId=2001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("full download status = %d", recorder.Code)
	}
	etag := recorder.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag missing on full ebook download")
	}

	recorder = datumJSONWithHeaders(t, engine, http.MethodGet, "/datum/download/ebook?ebookId=2001", token, nil,
		map[string]string{"Range": "bytes=3-"})
	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("range download status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.String(); got != payload[3:] {
		t.Fatalf("range body = %q, want %q", got, payload[3:])
	}
	contentRange := recorder.Header().Get("Content-Range")
	if !strings.HasPrefix(contentRange, "bytes 3-") || !strings.HasSuffix(contentRange, fmt.Sprintf("/%d", len(payload))) {
		t.Fatalf("content-range = %q", contentRange)
	}
	if recorder.Header().Get("ETag") != etag {
		t.Fatalf("ETag changed between requests: %q vs %q", recorder.Header().Get("ETag"), etag)
	}
}

// 学科聚合：subjects 接口只统计已上架电子书，按学科去重计数。
func TestDatumEbookSubjects(t *testing.T) {
	_, db, engine, _ := seedActivityFixture(t)
	seedEbook(db, 2001, 7, "Go语言实战", "Kennedy", "epub", "编程", 1)
	seedEbook(db, 2002, 7, "算法导论", "Cormen", "pdf", "编程", 1)
	seedEbook(db, 2003, 7, "未上架的书", "某人", "pdf", "编程", 0)

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/ebook/subjects", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("subjects status = %d body %s", recorder.Code, recorder.Body.String())
	}
	data := datumBody(t, recorder)["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("subjects = %v（未上架不应计入）", data)
	}
	row := data[0].(map[string]interface{})
	// 与 /datum/file/subjects 同构：EbookSubjectCount 无 json tag，键为 Go 字段名
	if row["Subject"] != "编程" || row["Total"].(float64) != 2 {
		t.Fatalf("subject row = %v", row)
	}
}

// 收藏契约：与试卷收藏同语义（冒充 403 / 重复 409 / 状态查询 / 取消后 404）。
func TestDatumEbookFavoriteToggle(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedEbook(db, 2001, 7, "Go语言实战", "Kennedy", "epub", "编程", 1)

	// 冒充他人 → 403
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/ebook/favorite", token, map[string]int64{"ebookId": 2001, "userId": 999})
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("impersonate status = %d", recorder.Code)
	}
	// 正常收藏
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/ebook/favorite", token, map[string]int64{"ebookId": 2001, "userId": 7})
	if recorder.Code != http.StatusOK {
		t.Fatalf("favorite status = %d body %s", recorder.Code, recorder.Body.String())
	}
	// 重复收藏 → 409
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/ebook/favorite", token, map[string]int64{"ebookId": 2001, "userId": 7})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body %s", recorder.Code, recorder.Body.String())
	}
	// 状态查询
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/ebook/favorite?ebookId=2001", token, nil)
	if datumBody(t, recorder)["data"].(map[string]interface{})["favorited"] != true {
		t.Fatal("favorited should be true")
	}
	// 收藏列表（收藏状态集合）
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/ebook/favorite/list", token, nil)
	ids := datumBody(t, recorder)["data"].([]interface{})
	if len(ids) != 1 || ids[0].(float64) != 2001 {
		t.Fatalf("favorite list = %v", ids)
	}
	// 取消收藏
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/ebook/favorite/2001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("remove status = %d", recorder.Code)
	}
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/ebook/favorite?ebookId=2001", token, nil)
	if datumBody(t, recorder)["data"].(map[string]interface{})["favorited"] != false {
		t.Fatal("favorited should be false after removal")
	}
	// 再次取消 → 404
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/ebook/favorite/2001", token, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("remove-again status = %d", recorder.Code)
	}
	_ = db
}

// 举报契约：与文件举报同语义（原因必填 / 不能举报自己 / 唯一约束去重）。
func TestDatumEbookReportCreate(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedEbook(db, 2001, 7, "Go语言实战", "Kennedy", "epub", "编程", 1)
	seedEbook(db, 2002, 8, "别人的书", "某人", "pdf", "编程", 1)

	// 原因为空 → 400
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/ebook/report", token, map[string]interface{}{"ebookId": 2001, "reason": "  "})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("empty reason status = %d", recorder.Code)
	}
	// 举报自己的电子书 → 400
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/ebook/report", token, map[string]interface{}{"ebookId": 2001, "reason": "侵权"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("self report status = %d", recorder.Code)
	}
	// 正常举报
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/ebook/report", token, map[string]interface{}{"ebookId": 2002, "reason": "内容违规"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("report status = %d body %s", recorder.Code, recorder.Body.String())
	}
	// 同一用户重复举报同一本 → 409（uk_user_ebook）
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/ebook/report", token, map[string]interface{}{"ebookId": 2002, "reason": "再次举报"})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate report status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if len(db.ebookReports) != 1 {
		t.Fatalf("reports = %d, want 1", len(db.ebookReports))
	}
}

// 上传契约：与试卷上传同渠道——落库即待审（ebook_status=0）、计入同一
// ptmj_user.count 排行榜计数器、属主可删（计数同步回退）。
func TestDatumEbookUploadAndDelete(t *testing.T) {
	store, db, engine, _ := seedActivityFixture(t) // 计数断言基于 carol 自己的会话
	seedDatumUser(t, db, "carol", "secret5", "1", false)

	user := db.users["carol"]
	user.Count = 5

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
	part, _ := writer.CreateFormFile("file", "Go语言实战.epub")
	_, _ = part.Write([]byte("PK.epub-upload"))
	_ = writer.WriteField("ebookName", "Go语言实战")
	_ = writer.WriteField("author", "Kennedy")
	_ = writer.WriteField("publisher", "Manning")
	_ = writer.WriteField("ebookSubject", "编程")
	_ = writer.WriteField("ebookType", "1")
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/datum/ebook/upload", bytes.NewReader(payload.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+profileLogin(t, store, engine, "carol", "secret5", "cap-carol"))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if len(putKeys) != 1 || !strings.HasPrefix(putKeys[0], "ebook/编程/") || !strings.HasSuffix(putKeys[0], ".epub") {
		t.Fatalf("objectKey = %v", putKeys)
	}
	if len(db.ebooks) != 1 || toDatumInt64(db.ebooks[0]["ebook_status"]) != 0 {
		t.Fatalf("ebooks = %v（应落库为待审）", db.ebooks)
	}
	if user.Count != 6 {
		t.Fatalf("count = %d, want 6（排行榜计数器应 +1）", user.Count)
	}
	ebookID := toDatumInt64(db.ebooks[0]["ebook_id"])

	// 不支持的格式 → 400
	badPayload := &bytes.Buffer{}
	badWriter := multipart.NewWriter(badPayload)
	badPart, _ := badWriter.CreateFormFile("file", "书.docx")
	_, _ = badPart.Write([]byte("PK"))
	_ = badWriter.WriteField("ebookSubject", "编程")
	_ = badWriter.WriteField("ebookType", "1")
	_ = badWriter.Close()
	badRequest := httptest.NewRequest(http.MethodPost, "/datum/ebook/upload", bytes.NewReader(badPayload.Bytes()))
	badRequest.Header.Set("Content-Type", badWriter.FormDataContentType())
	badRequest.Header.Set("Authorization", "Bearer "+profileLogin(t, store, engine, "carol", "secret5", "cap-carol-2"))
	badRecorder := httptest.NewRecorder()
	engine.ServeHTTP(badRecorder, badRequest)
	if badRecorder.Code != http.StatusBadRequest {
		t.Fatalf("bad format status = %d body %s", badRecorder.Code, badRecorder.Body.String())
	}

	// 属主删除：软删 + 计数回退
	delRequest := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/datum/ebook/%d", ebookID), nil)
	delRequest.Header.Set("Authorization", "Bearer "+profileLogin(t, store, engine, "carol", "secret5", "cap-carol-3"))
	delRecorder := httptest.NewRecorder()
	engine.ServeHTTP(delRecorder, delRequest)
	if delRecorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d body %s", delRecorder.Code, delRecorder.Body.String())
	}
	if toDatumInt64(db.ebooks[0]["del_flag"]) != 1 {
		t.Fatal("ebook must be soft-deleted")
	}
	if user.Count != 5 {
		t.Fatalf("count = %d, want 5（删除应回退计数器）", user.Count)
	}
}

// 我的电子书举报列表：与 /datum/report/list 同契约（rows/total，含书名），
// 按"举报人"维度查询（user_id 是举报者）。
func TestDatumEbookReportList(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedEbook(db, 2002, 8, "别人的书", "某人", "pdf", "编程", 1)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/ebook/report", token, map[string]interface{}{"ebookId": 2002, "reason": "内容违规"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("report status = %d body %s", recorder.Code, recorder.Body.String())
	}

	// alice（举报人，会话用户 7）查看自己的举报列表
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/ebook/report/list?userId=7", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("report list status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	rows, _ := body["rows"].([]interface{})
	if body["total"].(float64) != 1 || len(rows) != 1 {
		t.Fatalf("report list = %v", body)
	}
	row := rows[0].(map[string]interface{})
	if row["ebookName"] != "别人的书" || row["result"] != "0" {
		t.Fatalf("report row = %v", row)
	}

	// 冒充他人 → 403（bob 会话用户是 8，查询 userId=7 应被拒）
	store2, _, engine2, _ := seedActivityFixture(t)
	bobToken := profileLogin(t, store2, engine2, "bob", "secret5", "cap-bob-impersonate")
	recorder = datumJSON(t, engine2, http.MethodGet, "/datum/ebook/report/list?userId=7", bobToken, nil)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("impersonate status = %d", recorder.Code)
	}
}

// 我的电子书收藏详情列表（桌面端收藏页用）：联查书目信息。
func TestDatumEbookFavoriteDetailedList(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedEbook(db, 2001, 7, "Go语言实战", "Kennedy", "epub", "编程", 1)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/ebook/favorite", token, map[string]int64{"ebookId": 2001, "userId": 7})
	if recorder.Code != http.StatusOK {
		t.Fatalf("favorite status = %d", recorder.Code)
	}

	recorder = datumJSON(t, engine, http.MethodGet, "/datum/desktop/ebook/favorite/list/7?pageNum=1&pageSize=10", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("favorite list status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	rows, _ := body["rows"].([]interface{})
	if body["total"].(float64) != 1 || len(rows) != 1 {
		t.Fatalf("favorite list = %v", body)
	}
	row := rows[0].(map[string]interface{})
	if row["ebookName"] != "Go语言实战" || row["ebookFormat"] != "epub" {
		t.Fatalf("favorite row = %v", row)
	}

	// 移除后为空
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/desktop/ebook/favorite/7/2001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("remove status = %d", recorder.Code)
	}
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/desktop/ebook/favorite/list/7", token, nil)
	if datumBody(t, recorder)["total"].(float64) != 0 {
		t.Fatal("favorite list should be empty after removal")
	}
}

// OpenStoredObject 装配契约：MinIO 关闭时回落本地 datum 根；bucket 以存量
// URL 解析结果为准的语义由调用方保证，这里验证本地兜底可用。
func TestOpenStoredObjectLocalFallback(t *testing.T) {
	localRoot := t.TempDir()
	t.Setenv("KADMIN_MINIO_ENABLED", "false")
	t.Setenv("KADMIN_FILE_LOCAL_ROOT", localRoot)

	if err := os.MkdirAll(filepath.Join(localRoot, "books"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localRoot, "books", "go.epub"), []byte("PK.epub"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	body, info, err := files.OpenStoredObject(context.Background(), "ptmj", "books/go.epub")
	if err != nil {
		t.Fatalf("open stored object: %v", err)
	}
	defer body.Close()
	if info.Size != int64(len("PK.epub")) {
		t.Fatalf("size = %d", info.Size)
	}
	// seekable：http.ServeContent 的 Range 断点续传依赖
	if _, ok := body.(interface{ Seek(int64, int) (int64, error) }); !ok {
		t.Fatalf("object stream is not seekable: %T", body)
	}
}
