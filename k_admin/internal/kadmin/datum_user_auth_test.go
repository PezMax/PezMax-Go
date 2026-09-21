package kadmin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// fake DB（内嵌 db.Connection 接口以完整实现接口，仅覆盖 Query/Exec）
// ---------------------------------------------------------------------------

type fakeDatumDB struct {
	db.Connection
	mu       sync.Mutex
	users    map[string]*datumUser
	security map[int64][2]string
	nextID   int64
}

func newFakeDatumDB() *fakeDatumDB {
	return &fakeDatumDB{users: map[string]*datumUser{}, security: map[int64][2]string{}, nextID: 100}
}

var errFakeDuplicateKey = errors.New("duplicate key value violates unique constraint")
var errFakeUnsupportedQuery = errors.New("fakeDatumDB: unsupported query")

type fakeDatumResult struct{}

func (fakeDatumResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeDatumResult) RowsAffected() (int64, error) { return 1, nil }

func (f *fakeDatumDB) userRow(user *datumUser) map[string]interface{} {
	return map[string]interface{}{
		"user_id": user.UserID, "user_name": user.UserName, "password": user.Password,
		"avatar": user.Avatar, "count": user.Count, "status": user.Status,
	}
}

func (f *fakeDatumDB) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case strings.Contains(query, "INSERT INTO ptmj_user"):
		userName := toDatumString(args[0])
		if _, exists := f.users[strings.ToLower(userName)]; exists {
			return nil, errFakeDuplicateKey
		}
		f.nextID++
		user := &datumUser{UserID: f.nextID, UserName: userName, Password: toDatumString(args[1]), Avatar: toDatumString(args[2]), Status: "1"}
		f.users[strings.ToLower(userName)] = user
		return []map[string]interface{}{{"user_id": user.UserID}}, nil
	case strings.Contains(query, "FROM ptmj_user WHERE user_name"):
		user, exists := f.users[strings.ToLower(toDatumString(args[0]))]
		if !exists {
			return nil, nil
		}
		return []map[string]interface{}{f.userRow(user)}, nil
	case strings.Contains(query, "FROM ptmj_user WHERE user_id"):
		for _, user := range f.users {
			if user.UserID == toDatumInt64(args[0]) {
				return []map[string]interface{}{f.userRow(user)}, nil
			}
		}
		return nil, nil
	case strings.Contains(query, "FROM ptmj_security WHERE user_id"):
		row, exists := f.security[toDatumInt64(args[0])]
		if !exists {
			return nil, nil
		}
		return []map[string]interface{}{{"question": row[0], "answer": row[1]}}, nil
	}
	return nil, errFakeUnsupportedQuery
}

func (f *fakeDatumDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case strings.Contains(query, "UPDATE ptmj_user"):
		userID := toDatumInt64(args[1])
		for _, user := range f.users {
			if user.UserID == userID {
				user.Password = toDatumString(args[0])
			}
		}
	case strings.Contains(query, "INSERT INTO ptmj_security"):
		f.security[toDatumInt64(args[0])] = [2]string{toDatumString(args[1]), toDatumString(args[2])}
	}
	return fakeDatumResult{}, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mustHashSecret(t *testing.T, plain string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash secret: %v", err)
	}
	return string(hash)
}

func newDatumAuthStore(t *testing.T, overrides map[string]string) (*Store, *fakeDatumDB) {
	t.Helper()
	store, redis := newDatumTestStore(t, overrides)
	db := newFakeDatumDB()
	store.conn = db
	store.datum = newDatumIdentity("test:datum", redis, time.Hour)
	return store, db
}

func datumEngine(t *testing.T, store *Store) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerDatumRoutes(engine, store)
	return engine
}

func datumJSON(t *testing.T, engine *gin.Engine, method, path string, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func datumBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", recorder.Body.String(), err)
	}
	return body
}

func seedDatumUser(t *testing.T, db *fakeDatumDB, username, password, status string, withSecurity bool) {
	t.Helper()
	user := &datumUser{UserID: 7, UserName: username, Password: mustHashSecret(t, password), Avatar: "/a.png", Count: 3, Status: status}
	db.users[strings.ToLower(username)] = user
	if withSecurity {
		db.security[user.UserID] = [2]string{
			"你小学的名字|你宠物名字|出生城市",
			strings.Join([]string{mustHashSecret(t, "a1"), mustHashSecret(t, "a2"), mustHashSecret(t, "a3")}, "|"),
		}
	}
}

func seedDatumCaptcha(t *testing.T, store *Store, uuid string) {
	t.Helper()
	if err := store.security.storeCaptcha(uuid, "123456", time.Minute); err != nil {
		t.Fatalf("store captcha: %v", err)
	}
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

func TestDatumLoginIssuesSession(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumCaptcha(t, store, "cap-1")
	engine := datumEngine(t, store)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "123456", "uuid": "cap-1",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", recorder.Code, recorder.Body.String())
	}
	data, _ := datumBody(t, recorder)["data"].(map[string]interface{})
	token, _ := data["token"].(string)
	if token == "" {
		t.Fatalf("missing token in %v", data)
	}
	if _, ok := data["expiresAt"].(float64); !ok {
		t.Fatalf("missing expiresAt in %v", data)
	}
	userID, err := store.datum.ResolveSession(token)
	if err != nil || userID != 7 {
		t.Fatalf("ResolveSession = (%d, %v), want (7, nil)", userID, err)
	}
}

func TestDatumLoginEnforcesConstraints(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "0", false) // status 0 = 封禁
	engine := datumEngine(t, store)

	// 验证码错误
	seedDatumCaptcha(t, store, "cap-wrong")
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "000000", "uuid": "cap-wrong",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("wrong captcha status = %d", recorder.Code)
	}

	// 封禁账号
	seedDatumCaptcha(t, store, "cap-banned")
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "123456", "uuid": "cap-banned",
	})
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("banned status = %d", recorder.Code)
	}
}

func TestDatumLoginLocksAfterRepeatedFailures(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)

	// 默认阈值为 5：连续 5 次错误密码后触发锁定
	for attempt := 1; attempt <= 5; attempt++ {
		seedDatumCaptcha(t, store, "cap-fail")
		recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
			"username": "alice", "password": "wrong-password", "code": "123456", "uuid": "cap-fail",
		})
		want := http.StatusBadRequest
		if attempt == 5 {
			want = http.StatusTooManyRequests
		}
		if recorder.Code != want {
			t.Fatalf("attempt %d status = %d body %s", attempt, recorder.Code, recorder.Body.String())
		}
	}
	// 锁定后即使密码正确也拒绝
	seedDatumCaptcha(t, store, "cap-locked")
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "123456", "uuid": "cap-locked",
	})
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("locked status = %d", recorder.Code)
	}
}

func TestDatumRegisterCreatesUserAndSecurity(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumCaptcha(t, store, "cap-reg")
	engine := datumEngine(t, store)
	payload := map[string]string{
		"username": "bob", "password": "pass123", "confirmPassword": "pass123",
		"securityQuestionOne": "q1", "securityAnswerOne": "a1",
		"securityQuestionTwo": "q2", "securityAnswerTwo": "a2",
		"securityQuestionThree": "q3", "securityAnswerThree": "a3",
		"code": "123456", "uuid": "cap-reg", "avatar": "/default.png",
	}
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/register", "", payload)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", recorder.Code, recorder.Body.String())
	}
	user := db.users["bob"]
	if user == nil {
		t.Fatal("user was not created")
	}
	if !verifyDatumSecret(user.Password, "pass123") || verifyDatumSecret(user.Password, "other") {
		t.Fatal("stored password hash mismatch")
	}
	security := db.security[user.UserID]
	if security[0] != "q1|q2|q3" {
		t.Fatalf("question = %q", security[0])
	}
	parts := strings.Split(security[1], "|")
	if len(parts) != 3 || !verifyDatumSecret(parts[0], "a1") || !verifyDatumSecret(parts[2], "a3") {
		t.Fatal("stored answer hashes mismatch")
	}

	// 重复用户名 → 409
	seedDatumCaptcha(t, store, "cap-reg-2")
	payload["uuid"], payload["code"] = "cap-reg-2", "123456"
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/register", "", payload)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestDatumGetInfoAndLogout(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumCaptcha(t, store, "cap-1")
	engine := datumEngine(t, store)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "123456", "uuid": "cap-1",
	})
	token := datumBody(t, recorder)["data"].(map[string]interface{})["token"].(string)

	recorder = datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("getInfo status = %d body %s", recorder.Code, recorder.Body.String())
	}
	data := datumBody(t, recorder)["data"].(map[string]interface{})
	user := data["user"].(map[string]interface{})
	if user["userId"].(float64) != 7 || user["userName"] != "alice" || user["status"] != "1" {
		t.Fatalf("getInfo user = %v", user)
	}
	roles := data["roles"].([]interface{})
	if len(roles) != 1 || roles[0] != "ptmj_user" {
		t.Fatalf("roles = %v", roles)
	}

	// 未携带 token → 401
	if recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", "", nil); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d", recorder.Code)
	}
	// 登出后会话失效
	if recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/logout", token, nil); recorder.Code != http.StatusOK {
		t.Fatalf("logout status = %d", recorder.Code)
	}
	if recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", token, nil); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout status = %d", recorder.Code)
	}
}

func TestDatumPasswordRecoveryFlow(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "old-pass-1", "1", true)
	engine := datumEngine(t, store)

	// 第一步：验证码校验并返回密保问题，签发重置工单
	seedDatumCaptcha(t, store, "cap-step1")
	recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/securityQuestions?userName=alice&code=123456&uuid=cap-step1", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("step1 status = %d body %s", recorder.Code, recorder.Body.String())
	}
	questions := datumBody(t, recorder)["data"].([]interface{})
	if len(questions) != 3 {
		t.Fatalf("questions = %v", questions)
	}

	// 第二步：同一 code/uuid + 工单 + 三问三答 → 重置密码
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetPasswordBySecurity", "", map[string]string{
		"username": "alice", "code": "123456", "uuid": "cap-step1",
		"securityAnswerOne": "a1", "securityAnswerTwo": "a2", "securityAnswerThree": "a3",
		"newPassword": "new-pass-9", "confirmPassword": "new-pass-9",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("step2 status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if verifyDatumSecret(db.users["alice"].Password, "old-pass-1") || !verifyDatumSecret(db.users["alice"].Password, "new-pass-9") {
		t.Fatal("password was not updated")
	}

	// 工单已被消费：重放同一请求 → 400
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetPasswordBySecurity", "", map[string]string{
		"username": "alice", "code": "123456", "uuid": "cap-step1",
		"securityAnswerOne": "a1", "securityAnswerTwo": "a2", "securityAnswerThree": "a3",
		"newPassword": "another-1", "confirmPassword": "another-1",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("replay status = %d", recorder.Code)
	}

	// 错误答案 → 400
	seedDatumCaptcha(t, store, "cap-step2")
	datumJSON(t, engine, http.MethodGet, "/datum/user/securityQuestions?userName=alice&code=123456&uuid=cap-step2", "", nil)
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetPasswordBySecurity", "", map[string]string{
		"username": "alice", "code": "123456", "uuid": "cap-step2",
		"securityAnswerOne": "wrong", "securityAnswerTwo": "a2", "securityAnswerThree": "a3",
		"newPassword": "another-1", "confirmPassword": "another-1",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("wrong answer status = %d", recorder.Code)
	}
}
