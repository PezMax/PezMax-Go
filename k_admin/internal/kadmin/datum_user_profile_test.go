package kadmin

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// profileLogin logs the seeded user in and returns the bearer token.
func profileLogin(t *testing.T, store *Store, engine *gin.Engine, username, password, captchaUUID string) string {
	t.Helper()
	seedDatumCaptcha(t, store, captchaUUID)
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": username, "password": password, "code": "123456", "uuid": captchaUUID,
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d body %s", recorder.Code, recorder.Body.String())
	}
	return datumBody(t, recorder)["data"].(map[string]interface{})["token"].(string)
}

func TestDatumProfileReadAndStats(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/desktop/user/profile", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("profile status = %d body %s", recorder.Code, recorder.Body.String())
	}
	profile := datumBody(t, recorder)["data"].(map[string]interface{})
	if profile["userName"] != "alice" || profile["userId"].(float64) != 7 || profile["count"].(float64) != 3 {
		t.Fatalf("profile = %v", profile)
	}

	recorder = datumJSON(t, engine, http.MethodGet, "/datum/desktop/user/profile/stats", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("stats status = %d body %s", recorder.Code, recorder.Body.String())
	}
	stats := datumBody(t, recorder)["data"].(map[string]interface{})
	// fake 返回固定计数：uploads=user.Count(3)、downloads=5、fileFav=3、bookmarkFav=2
	if stats["uploadCount"].(float64) != 3 || stats["downloadCount"].(float64) != 5 ||
		stats["favoriteCount"].(float64) != 5 || stats["fileFavoriteCount"].(float64) != 3 ||
		stats["bookmarkFavoriteCount"].(float64) != 2 {
		t.Fatalf("stats = %v", stats)
	}

	// 未登录访问 → 401
	if recorder := datumJSON(t, engine, http.MethodGet, "/datum/desktop/user/profile", "", nil); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d", recorder.Code)
	}
}

func TestDatumUpdateUserNameAndAvatar(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumUser(t, db, "bob", "secret5", "1", false)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")

	// 改名成功
	recorder := datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/username", token, map[string]string{"userName": "alice2"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("rename status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if db.users["alice"].UserName != "alice2" {
		t.Fatalf("user_name = %q", db.users["alice"].UserName)
	}

	// 与已有用户重名 → 409
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/username", token, map[string]string{"userName": "bob"})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body %s", recorder.Code, recorder.Body.String())
	}

	// 空名 → 400
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/username", token, map[string]string{"userName": "  "})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("empty status = %d", recorder.Code)
	}

	// 头像地址更新
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/avatar", token, map[string]string{"avatar": "/avatar/new.png"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("avatar status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if db.users["alice"].Avatar != "/avatar/new.png" {
		t.Fatalf("avatar = %q", db.users["alice"].Avatar)
	}
}

func TestDatumUploadAvatarStoresLocally(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")

	localRoot := t.TempDir()
	t.Setenv("KADMIN_MINIO_ENABLED", "false")
	t.Setenv("KADMIN_DATUM_AVATAR_LOCAL_ROOT", localRoot)

	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := part.Write(png); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/datum/desktop/user/profile/avatar/upload", bytes.NewReader(payload.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d body %s", recorder.Code, recorder.Body.String())
	}
	data := datumBody(t, recorder)["data"].(map[string]interface{})
	url, _ := data["url"].(string)
	if !strings.HasPrefix(url, "/api/uploads/avatars/datum/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("url = %q", url)
	}
	if data["storage"] != "local" {
		t.Fatalf("storage = %v", data["storage"])
	}
	// 文件确实落盘
	relative := strings.TrimPrefix(strings.TrimPrefix(url, "/api/uploads/"), "avatars/datum/")
	written, err := os.ReadFile(filepath.Join(localRoot, "avatars", "datum", relative))
	if err != nil || !bytes.Equal(written, png) {
		t.Fatalf("stored file mismatch: %v", err)
	}
	if !strings.HasPrefix(db.users["alice"].Avatar, "/api/uploads/avatars/datum/") {
		t.Fatalf("user avatar = %q", db.users["alice"].Avatar)
	}

	// 非图片 → 400
	var textPayload bytes.Buffer
	textWriter := multipart.NewWriter(&textPayload)
	textPart, _ := textWriter.CreateFormFile("file", "note.txt")
	_, _ = textPart.Write([]byte("plain text"))
	_ = textWriter.Close()
	request = httptest.NewRequest(http.MethodPost, "/datum/desktop/user/profile/avatar/upload", bytes.NewReader(textPayload.Bytes()))
	request.Header.Set("Content-Type", textWriter.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("non-image status = %d", recorder.Code)
	}
}

func TestDatumPasswordVerifyAndUpdate(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")

	// 错误密码 → 400
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/desktop/user/profile/password/verify", token, map[string]string{"password": "wrong"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("wrong verify status = %d", recorder.Code)
	}
	// 正确密码 → 200
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/desktop/user/profile/password/verify", token, map[string]string{"password": "secret5"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("verify status = %d", recorder.Code)
	}

	// 修改密码：旧密码错误 → 400
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/password", token, map[string]string{"oldPassword": "wrong", "newPassword": "brand-new-1"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("wrong old status = %d", recorder.Code)
	}
	// 新密码过短 → 400
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/password", token, map[string]string{"oldPassword": "secret5", "newPassword": "123"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("short new status = %d", recorder.Code)
	}
	// 正常修改
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/password", token, map[string]string{"oldPassword": "secret5", "newPassword": "brand-new-1"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if !verifyDatumSecret(db.users["alice"].Password, "brand-new-1") {
		t.Fatal("password hash not updated")
	}

	// 旧密码修改后不再有效
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/desktop/user/profile/password/verify", token, map[string]string{"password": "secret5"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("stale verify status = %d", recorder.Code)
	}
}

func TestDatumSecurityReadUpdateAndAnswerVerify(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", true)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")

	// 读取密保问题（不返回答案）
	recorder := datumJSON(t, engine, http.MethodGet, "/datum/desktop/user/profile/security", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("security status = %d body %s", recorder.Code, recorder.Body.String())
	}
	questions := datumBody(t, recorder)["data"].(map[string]interface{})["questions"].([]interface{})
	if len(questions) != 3 || questions[0] != "你小学的名字" {
		t.Fatalf("questions = %v", questions)
	}
	if strings.Contains(recorder.Body.String(), "answer") {
		t.Fatal("security read must not leak answers")
	}

	// 单答案校验：a2 命中 / miss 不命中
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/desktop/user/profile/security/answer/verify", token, map[string]string{"answer": "a2"})
	if recorder.Code != http.StatusOK || datumBody(t, recorder)["data"].(map[string]interface{})["verified"] != true {
		t.Fatalf("answer verify = %s", recorder.Body.String())
	}
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/desktop/user/profile/security/answer/verify", token, map[string]string{"answer": "nope"})
	if datumBody(t, recorder)["data"].(map[string]interface{})["verified"] != false {
		t.Fatalf("answer miss = %s", recorder.Body.String())
	}

	// 更新密保
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/security", token, map[string]string{
		"securityQuestionOne": "新问题1", "securityAnswerOne": "n1",
		"securityQuestionTwo": "新问题2", "securityAnswerTwo": "n2",
		"securityQuestionThree": "新问题3", "securityAnswerThree": "n3",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("security update status = %d body %s", recorder.Code, recorder.Body.String())
	}
	row := db.security[7]
	if row[0] != "新问题1|新问题2|新问题3" {
		t.Fatalf("questions row = %q", row[0])
	}
	parts := strings.Split(row[1], "|")
	if !verifyDatumSecret(parts[0], "n1") || !verifyDatumSecret(parts[2], "n3") {
		t.Fatal("answers not re-hashed")
	}

	// 已登录密保重置密码
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/password/by-security", token, map[string]string{
		"securityAnswerOne": "n1", "securityAnswerTwo": "n2", "securityAnswerThree": "n3",
		"newPassword": "reset-by-sec-1",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("reset status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if !verifyDatumSecret(db.users["alice"].Password, "reset-by-sec-1") {
		t.Fatal("password not reset")
	}
	// 错误答案 → 400
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/desktop/user/profile/password/by-security", token, map[string]string{
		"securityAnswerOne": "bad", "securityAnswerTwo": "n2", "securityAnswerThree": "n3",
		"newPassword": "another-new-1",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad answers status = %d", recorder.Code)
	}
}
