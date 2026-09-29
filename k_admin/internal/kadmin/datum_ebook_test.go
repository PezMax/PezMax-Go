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
