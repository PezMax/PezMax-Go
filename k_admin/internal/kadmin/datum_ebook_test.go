package kadmin

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/files"
)

// 电子书模块契约：fileType=7 的已过审文件进入 /datum/ebook/list；下载复用
// 统一的 /datum/download/file（同一条鉴权/记录/整形路径，不设专有端点）。
func TestDatumEbookList(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t) // 已有 fileType=1 的文件 1001
	seedApprovedFile(db, 2001, 7, "Go语言实战.epub", "编程", 7, 2025, 1)
	seedApprovedFile(db, 2002, 7, "未上架的书.epub", "编程", 7, 2025, 0)

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/ebook/list", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ebook list status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	rows, _ := body["rows"].([]interface{})
	if body["total"].(float64) != 1 || len(rows) != 1 {
		t.Fatalf("ebook list = %v（应只含已过审电子书）", body)
	}
	row := rows[0].(map[string]interface{})
	if row["fileId"].(float64) != 2001 || row["fileType"].(float64) != 7 {
		t.Fatalf("ebook row = %v", row)
	}
	if !strings.HasSuffix(row["fileName"].(string), ".epub") {
		t.Fatalf("ebook name = %v", row["fileName"])
	}

	// 试卷（fileType=1）不进电子书列表
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/ebook/list?pageSize=100", token, nil)
	rows = datumBody(t, recorder)["rows"].([]interface{})
	for _, item := range rows {
		if item.(map[string]interface{})["fileId"].(float64) == 1001 {
			t.Fatal("non-ebook file leaked into ebook list")
		}
	}
}

// 电子书下载走统一契约：与 TestDatumStreamDownloadWritesRecord 同路径，
// 这里冒烟验证电子书文件可流式下载且落下载记录。
func TestDatumEbookUnifiedDownload(t *testing.T) {
	store, db, engine, token := seedActivityFixture(t)
	seedApprovedFile(db, 2001, 7, "Go语言实战.epub", "编程", 7, 2025, 1)

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
	db.files[1]["file_url"] = "http://127.0.0.1:29000/ptmj/books/go.epub"
	db.files[1]["file_format"] = "epub"

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/download/file?fileId=2001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("stream status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.String(); got != "PK.epub-e2e" {
		t.Fatalf("streamed body = %q", got)
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "epub") {
		t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
	}
	if len(db.downloads) == 0 {
		t.Fatal("download record missing")
	}
	_ = store
}

// 统一下载路径（电子书与试卷共用）支持 Range 断点续传：
// 带 Range 请求返回 206 + Content-Range + 剩余字节，并携带强 ETag 供
// 客户端 If-Range 校验；不带 Range 仍为完整 200。
func TestDatumStreamDownloadRangeResume(t *testing.T) {
	_, db, engine, token := seedActivityFixture(t)
	seedApprovedFile(db, 2001, 7, "Go语言实战.epub", "编程", 7, 2025, 1)

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
	db.files[1]["file_url"] = "http://127.0.0.1:29000/ptmj/books/go.epub"
	db.files[1]["file_format"] = "epub"

	// 完整请求：200 + ETag
	recorder := datumJSON(t, engine, http.MethodGet, "/datum/download/file?fileId=2001", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("full download status = %d", recorder.Code)
	}
	etag := recorder.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag missing on full download")
	}

	// 断点续传：bytes=3- → 206 + Content-Range + 剩余字节
	recorder = datumJSONWithHeaders(t, engine, http.MethodGet, "/datum/download/file?fileId=2001", token, nil,
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

	// Range 越界 → 416（客户端据此判定半成品失效）
	recorder = datumJSONWithHeaders(t, engine, http.MethodGet, "/datum/download/file?fileId=2001", token, nil,
		map[string]string{"Range": fmt.Sprintf("bytes=%d-", len(payload)+10)})
	if recorder.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("out-of-range status = %d", recorder.Code)
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

	body, info, err := files.OpenStoredObject(t.Context(), "ptmj", "books/go.epub")
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
