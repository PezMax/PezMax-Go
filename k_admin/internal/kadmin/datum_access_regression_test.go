package kadmin

import (
	"net/http"
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
