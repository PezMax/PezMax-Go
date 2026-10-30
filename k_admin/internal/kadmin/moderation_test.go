package kadmin

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/textguard"
	"github.com/gin-gonic/gin"
)

// 敏感词拦截的行为边界：写入前拦截、命中即 400、命中词不回显、开关可关闭。

const blockedSample = "代考"

func seedSensitiveGuard(store *Store, words ...string) {
	store.sensitiveWords = textguard.FromWords(words)
}

func TestRegisterRejectsSensitiveUsername(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedSensitiveGuard(store, "badword")
	engine := datumEngine(t, store)
	seedDatumCaptcha(t, store, "cap-words")

	response := datumJSON(t, engine, http.MethodPost, "/datum/user/register", "", map[string]string{
		"username": "badword12", "password": "pass123", "confirmPassword": "pass123",
		"securityQuestionOne": "q1", "securityAnswerOne": "a1",
		"securityQuestionTwo": "q2", "securityAnswerTwo": "a2",
		"securityQuestionThree": "q3", "securityAnswerThree": "a3",
		"code": "123456", "uuid": "cap-words",
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body %s", response.Code, response.Body.String())
	}
	if db.users["badword12"] != nil {
		t.Fatal("blocked username was still created")
	}
	body := datumBody(t, response)
	if message, _ := body["msg"].(string); !strings.Contains(message, "用户名") || strings.Contains(message, "badword") {
		t.Fatalf("message = %q", message)
	}
}

func TestProfileUsernameUpdateRejectsSensitiveWord(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedSensitiveGuard(store, blockedSample)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-name")

	response := datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/username", token,
		map[string]string{"userName": "代 考 小王"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body %s", response.Code, response.Body.String())
	}
	if db.users["alice"] == nil || db.users["代 考 小王"] != nil {
		t.Fatal("blocked rename reached the repository")
	}
}

func TestBookmarkWritesRejectSensitiveText(t *testing.T) {
	store, db, engine, token := seedActivityFixture(t)
	seedSensitiveGuard(store, blockedSample)
	seedActivityBookmark(t, db, 9001)

	create := datumJSON(t, engine, http.MethodPost, "/datum/bookmark", token, map[string]string{
		"url": "https://example.com/a", "title": "高数笔记", "description": "提 供 代考 服务",
	})
	if create.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d body %s", create.Code, create.Body.String())
	}
	if len(db.bookmarks) != 1 {
		t.Fatalf("blocked bookmark was inserted: %d rows", len(db.bookmarks))
	}

	update := datumJSON(t, engine, http.MethodPut, "/datum/bookmark", token, map[string]any{
		"id": 9001, "title": "代考真题",
	})
	if update.Code != http.StatusBadRequest {
		t.Fatalf("update status = %d body %s", update.Code, update.Body.String())
	}
	if db.bookmarks[9001]["title"] != "Go 官网" {
		t.Fatalf("blocked update reached the repository: %v", db.bookmarks[9001]["title"])
	}

	// 正常内容不受影响。
	ok := datumJSON(t, engine, http.MethodPost, "/datum/bookmark", token, map[string]string{
		"url": "https://example.com/b", "title": "线性代数讲义", "description": "第一章 行列式",
	})
	if ok.Code != http.StatusOK {
		t.Fatalf("clean create status = %d body %s", ok.Code, ok.Body.String())
	}
}

func TestUploadsRejectSensitiveMetadataBeforeStorage(t *testing.T) {
	store, _, engine, token := seedActivityFixture(t)
	seedSensitiveGuard(store, blockedSample)
	originalPut := datumFilePut
	t.Cleanup(func() { datumFilePut = originalPut })
	stored := ""
	datumFilePut = func(_ context.Context, key string, _ io.Reader, _ int64, _ string) (string, error) {
		stored = key
		return "http://files.example.com/" + key, nil
	}

	for _, test := range []struct {
		name   string
		route  string
		file   string
		fields map[string]string
	}{
		{
			name:  "file name",
			route: "/datum/file",
			file:  "代考答案.pdf",
			fields: map[string]string{
				"fileName": "代考答案.pdf", "fileSubject": "math", "fileYear": "2024", "fileType": "1",
			},
		},
		{
			name:  "file school",
			route: "/datum/file",
			file:  "试卷.pdf",
			fields: map[string]string{
				"fileName": "试卷.pdf", "fileSubject": "math", "fileSchool": "代考中学", "fileYear": "2024", "fileType": "1",
			},
		},
		{
			name:   "ebook name",
			route:  "/datum/ebook/upload",
			file:   "代考宝典.epub",
			fields: map[string]string{"ebookName": "代考宝典.epub", "ebookSubject": "综合", "ebookType": "1"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stored = ""
			response := uploadMultipart(t, engine, test.route, token, test.file, test.fields)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body %s", response.Code, response.Body.String())
			}
			if stored != "" {
				t.Fatalf("blocked upload was stored at %q", stored)
			}
		})
	}
}

func TestSensitiveWordsConfigSwitchDisablesBlocking(t *testing.T) {
	store, db, engine, token := seedActivityFixture(t)
	// overrides 通过 KADMIN_SYSTEM_CONFIG_PATH 落到临时配置文件
	_ = db
	seedSensitiveGuard(store, blockedSample)
	values, err := store.readSystemConfig()
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	values[sensitiveWordsEnabledKey] = "false"
	if err := store.writeSystemConfig(values); err != nil {
		t.Fatalf("write config: %v", err)
	}

	response := datumJSON(t, engine, http.MethodPost, "/datum/bookmark", token, map[string]string{
		"url": "https://example.com/c", "title": "代考资料",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", response.Code, response.Body.String())
	}
}

func TestDefaultSensitiveWordsFileIsShippedAndLoadable(t *testing.T) {
	t.Setenv(sensitiveWordsPathEnv, "")
	guard := textguard.New()
	guard.SetPath(resolveSensitiveWordsPath(""))
	words := guard.Words()
	if len(words) == 0 {
		t.Fatalf("默认词表为空，路径 %q", resolveSensitiveWordsPath(""))
	}
	if _, hit := guard.Check("提供代考服务"); !hit {
		t.Fatalf("默认词表未命中代考：%v", words)
	}
}

// uploadMultipart posts a multipart form to a datum upload route.
func uploadMultipart(t *testing.T, engine *gin.Engine, route, token, fileName string, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("payload")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(payload.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}
