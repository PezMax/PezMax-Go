package kadmin

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// 公开文件/书签列表的权限契约：默认仅已审核；带 userId 时必须依次通过
// 合法 ID、有效会话、属主匹配三道校验，任何一步失败不得降级为全量查询。
func TestDatumOwnerListsRequireMatchingSession(t *testing.T) {
	for _, resource := range []string{"file", "bookmark"} {
		t.Run(resource, func(t *testing.T) {
			_, database, engine, token := seedActivityFixture(t)
			if resource == "file" {
				database.files = nil
				seedApprovedFile(database, 1001, 7, "approved.pdf", "math", 1, 2024, 1)
				seedApprovedFile(database, 1002, 7, "pending.pdf", "math", 1, 2024, 0)
			} else {
				seedBookmarkRow(database, 1001, 7, "approved", "https://example.com/approved", "course", "test", 1)
				seedBookmarkRow(database, 1002, 7, "pending", "https://example.com/pending", "course", "test", 0)
			}
			for _, test := range []struct {
				name, query, token string
				status, rows       int
			}{
				{"public", "", "", http.StatusOK, 1},
				{"anonymous-owner-query", "?userId=7", "", http.StatusUnauthorized, 0},
				{"invalid-session", "?userId=7", "invalid", http.StatusUnauthorized, 0},
				{"different-owner", "?userId=999", token, http.StatusForbidden, 0},
				{"zero-id", "?userId=0", token, http.StatusBadRequest, 0},
				{"invalid-id", "?userId=invalid", token, http.StatusBadRequest, 0},
				{"owner", "?userId=7", token, http.StatusOK, 2},
			} {
				t.Run(test.name, func(t *testing.T) {
					response := datumJSON(t, engine, http.MethodGet, "/datum/"+resource+"/list"+test.query, test.token, nil)
					if response.Code != test.status {
						t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
					}
					if response.Code == http.StatusOK {
						rows := datumBody(t, response)["rows"].([]interface{})
						if len(rows) != test.rows {
							t.Fatalf("rows = %d, want %d; body = %s", len(rows), test.rows, response.Body.String())
						}
					}
				})
			}
		})
	}
}

// 内容接口挂在无鉴权上下文的公开 GET 分发器下：待审电子书必须主动解析
// 可选会话，属主可预览，匿名/无效/他人一律 404；预览不得写下载记录。
func TestDatumPendingEbookPreviewRecognizesOwnerSession(t *testing.T) {
	_, database, engine, token := seedActivityFixture(t)
	seedEbook(database, 2001, 7, "pending-owner", "author", "epub", "test", 0)
	seedEbook(database, 2002, 999, "pending-other", "author", "epub", "test", 0)
	localRoot := t.TempDir()
	t.Setenv("KADMIN_MINIO_ENABLED", "false")
	t.Setenv("KADMIN_FILE_LOCAL_ROOT", localRoot)
	target := filepath.Join(localRoot, "books", "go.epub")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	const content = "PK.pending-owner-preview"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, ebookID, token string
		status               int
	}{
		{"anonymous", "2001", "", http.StatusNotFound},
		{"invalid-session", "2001", "invalid", http.StatusNotFound},
		{"different-owner", "2002", token, http.StatusNotFound},
		{"owner", "2001", token, http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := datumJSON(t, engine, http.MethodGet, "/datum/ebook/content?ebookId="+test.ebookID, test.token, nil)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
			if response.Code == http.StatusOK && response.Body.String() != content {
				t.Fatalf("preview body = %q", response.Body.String())
			}
		})
	}
	if database.ebookDownloads != 0 {
		t.Fatal("preview wrote download history")
	}
}
