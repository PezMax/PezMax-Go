package kadmin

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// rankStateCached probes the rank state cache through the fake redis session.
func rankStateCached(t *testing.T, store *Store) bool {
	t.Helper()
	_, err := store.datum.redis.do("GET", store.rankStateKey())
	return !errors.Is(err, errRedisNil)
}

// ---------------------------------------------------------------------------
// reads: list / detail
// ---------------------------------------------------------------------------

func TestDatumUserListFiltersAndPagination(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false) // userID 7
	seedDatumUser(t, db, "bob", "secret5", "0", false)   // userID 8
	seedDatumUser(t, db, "alina", "secret5", "1", false) // userID 9
	engine := datumEngine(t, store)

	recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/list", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	rows, _ := body["rows"].([]interface{})
	if len(rows) != 3 || body["total"].(float64) != 3 {
		t.Fatalf("rows/total = %d/%v, want 3/3", len(rows), body["total"])
	}
	for _, raw := range rows {
		row := raw.(map[string]interface{})
		if _, leaked := row["password"]; leaked {
			t.Fatal("list payload must not carry the password hash")
		}
	}

	// userName 模糊 + status 过滤
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/user/list?userName=ali&status=1", "", nil)
	body = datumBody(t, recorder)
	rows, _ = body["rows"].([]interface{})
	if len(rows) != 2 {
		t.Fatalf("filtered rows = %d, want 2 (alice, alina)", len(rows))
	}
	names := map[string]bool{}
	for _, raw := range rows {
		names[raw.(map[string]interface{})["userName"].(string)] = true
	}
	if !names["alice"] || !names["alina"] {
		t.Fatalf("filtered names = %v", names)
	}

	// status 0 只出封禁行
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/user/list?status=0", "", nil)
	body = datumBody(t, recorder)
	rows, _ = body["rows"].([]interface{})
	if len(rows) != 1 || rows[0].(map[string]interface{})["userName"] != "bob" {
		t.Fatalf("banned-only rows = %v", rows)
	}

	// 分页参数
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/user/list?pageNum=1&pageSize=2", "", nil)
	body = datumBody(t, recorder)
	rows, _ = body["rows"].([]interface{})
	if len(rows) != 2 || body["total"].(float64) != 3 {
		t.Fatalf("paged rows/total = %d/%v, want 2/3", len(rows), body["total"])
	}
}

func TestDatumUserDetailPublicProjection(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	db.users["alice"].Remark = "活跃用户"
	db.users["alice"].CreateTime = "2026-01-02 03:04:05"
	engine := datumEngine(t, store)

	// 匿名可读（桌面端排行/抽屉场景），载荷不含密码
	recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/7", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", recorder.Code, recorder.Body.String())
	}
	data := datumBody(t, recorder)["data"].(map[string]interface{})
	if data["userId"].(float64) != 7 || data["userName"] != "alice" || data["status"] != "1" {
		t.Fatalf("detail = %v", data)
	}
	if data["remark"] != "活跃用户" || data["createTime"] != "2026-01-02 03:04:05" {
		t.Fatalf("detail meta = %v", data)
	}
	if _, leaked := data["password"]; leaked {
		t.Fatal("detail payload must not carry the password hash")
	}

	if recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/999", "", nil); recorder.Code != http.StatusNotFound {
		t.Fatalf("missing user status = %d", recorder.Code)
	}
	if recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/abc", "", nil); recorder.Code != http.StatusNotFound {
		// 非数字段不属于任何契约端点，分发器按 404 处理
		t.Fatalf("non-numeric rest status = %d body %s", recorder.Code, recorder.Body.String())
	}
}

// ---------------------------------------------------------------------------
// writes: create / update / delete
// ---------------------------------------------------------------------------

func TestDatumUserCreateRequiresAuthAndValidates(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)
	token := datumBookmarkLogin(t, engine, store, "alice", "secret5")

	payload := map[string]interface{}{
		"userName": "carol", "password": "initial-pass", "avatar": "/c.png",
		"count": 4, "remark": "运营代建",
	}
	// 未登录 → 401
	if recorder := datumJSON(t, engine, http.MethodPost, "/datum/user", "", payload); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d", recorder.Code)
	}
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user", token, payload)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body %s", recorder.Code, recorder.Body.String())
	}
	user := db.users["carol"]
	if user == nil {
		t.Fatal("user was not created")
	}
	if user.Status != "1" || user.Count != 4 || user.Remark != "运营代建" {
		t.Fatalf("created user = %+v", user)
	}
	if !verifyDatumSecret(user.Password, "initial-pass") {
		t.Fatal("password was not hashed on create")
	}

	// 重复用户名 → 409
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user", token, payload)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body %s", recorder.Code, recorder.Body.String())
	}
	// 弱密码 → 400
	weak := map[string]interface{}{"userName": "dave", "password": "123"}
	if recorder = datumJSON(t, engine, http.MethodPost, "/datum/user", token, weak); recorder.Code != http.StatusBadRequest {
		t.Fatalf("weak password status = %d", recorder.Code)
	}
	// 非法状态 → 400
	badStatus := map[string]interface{}{"userName": "dave", "password": "123456", "status": "2"}
	if recorder = datumJSON(t, engine, http.MethodPost, "/datum/user", token, badStatus); recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad status status = %d", recorder.Code)
	}
}

func TestDatumUserUpdateMergesAndBans(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	engine := datumEngine(t, store)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumUser(t, db, "bob", "secret5", "1", false)
	token := datumBookmarkLogin(t, engine, store, "alice", "secret5")

	// 预热排行缓存，封禁后应被失效
	rankRec := datumJSON(t, engine, http.MethodGet, "/datum/user/rank", "", nil)
	if rankRec.Code != http.StatusOK {
		t.Fatalf("rank warmup status = %d", rankRec.Code)
	}
	if !rankStateCached(t, store) {
		t.Fatal("rank state was not cached after warmup")
	}

	// JSON 数字形态的 status（桌面端双形态）+ 局部合并
	recorder := datumJSON(t, engine, http.MethodPut, "/datum/user", token, map[string]interface{}{
		"userId": 8, "status": 0, "remark": "违规封禁",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("ban status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if db.users["bob"].Status != "0" || db.users["bob"].Remark != "违规封禁" {
		t.Fatalf("banned user = %+v", db.users["bob"])
	}
	if db.users["bob"].Avatar != "/a.png" || db.users["bob"].Count != 3 {
		t.Fatalf("unprovided fields were overwritten: %+v", db.users["bob"])
	}
	if rankStateCached(t, store) {
		t.Fatal("rank cache was not invalidated after ban")
	}

	// 解封走同一端点
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/user", token, map[string]interface{}{
		"userId": "8", "status": "1",
	})
	if recorder.Code != http.StatusOK || db.users["bob"].Status != "1" {
		t.Fatalf("unban = %d %+v", recorder.Code, db.users["bob"])
	}

	// 密码重置（管理端改密）
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/user", token, map[string]interface{}{
		"userId": 8, "password": "brand-new-pass",
	})
	if recorder.Code != http.StatusOK || !verifyDatumSecret(db.users["bob"].Password, "brand-new-pass") {
		t.Fatalf("password reset = %d body %s", recorder.Code, recorder.Body.String())
	}

	// 重名为他人用户名 → 409；自身同名允许（无变化）
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/user", token, map[string]interface{}{
		"userId": 8, "userName": "alice",
	})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("rename clash status = %d", recorder.Code)
	}
	// 非法状态 → 400
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/user", token, map[string]interface{}{
		"userId": 8, "status": 2,
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad status status = %d", recorder.Code)
	}
	// 不存在的用户 → 404
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/user", token, map[string]interface{}{
		"userId": 999, "status": "0",
	})
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing user status = %d", recorder.Code)
	}
	// 未登录 → 401
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/user", "", map[string]interface{}{"userId": 8, "status": "0"})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d", recorder.Code)
	}
}

func TestDatumUserDelete(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", true)
	seedDatumUser(t, db, "bob", "secret5", "1", true)
	engine := datumEngine(t, store)
	token := datumBookmarkLogin(t, engine, store, "alice", "secret5")

	// 未登录 → 401
	if recorder := datumJSON(t, engine, http.MethodDelete, "/datum/user/8", "", nil); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d", recorder.Code)
	}
	// 自删保护
	if recorder := datumJSON(t, engine, http.MethodDelete, "/datum/user/7", token, nil); recorder.Code != http.StatusBadRequest {
		t.Fatalf("self-delete status = %d", recorder.Code)
	}
	// 正常删除：用户与密保一并清除
	recorder := datumJSON(t, engine, http.MethodDelete, "/datum/user/8", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if db.findUserByID(8) != nil {
		t.Fatal("user row survived delete")
	}
	if _, exists := db.security[8]; exists {
		t.Fatal("security row survived delete")
	}
	// 重复删除 → 404
	if recorder = datumJSON(t, engine, http.MethodDelete, "/datum/user/8", token, nil); recorder.Code != http.StatusNotFound {
		t.Fatalf("re-delete status = %d", recorder.Code)
	}
}

// ---------------------------------------------------------------------------
// admin security reset
// ---------------------------------------------------------------------------

func TestDatumUserResetSecurityAnswers(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", true)
	engine := datumEngine(t, store)
	token := datumBookmarkLogin(t, engine, store, "alice", "secret5")

	// 成套替换：新问答入库且答案重新哈希
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/resetSecurityAnswers", token, map[string]string{
		"userName": "alice",
		"securityQuestionOne":   "新问题一", "securityAnswerOne": "na1",
		"securityQuestionTwo":   "新问题二", "securityAnswerTwo": "na2",
		"securityQuestionThree": "新问题三", "securityAnswerThree": "na3",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("reset status = %d body %s", recorder.Code, recorder.Body.String())
	}
	row := db.security[7]
	if row[0] != "新问题一|新问题二|新问题三" {
		t.Fatalf("questions = %q", row[0])
	}
	parts := strings.Split(row[1], "|")
	if len(parts) != 3 || !verifyDatumSecret(parts[0], "na1") || !verifyDatumSecret(parts[2], "na3") {
		t.Fatal("answers were not re-hashed")
	}
	if verifyDatumSecret(parts[0], "a1") {
		t.Fatal("old answer still valid after reset")
	}

	// 全空 → 清除密保
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetSecurityAnswers", token, map[string]string{"userName": "alice"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("clear status = %d", recorder.Code)
	}
	if _, exists := db.security[7]; exists {
		t.Fatal("security row survived clear")
	}

	// 部分提供 → 400
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetSecurityAnswers", token, map[string]string{
		"userName": "alice", "securityQuestionOne": "只有一个",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("partial status = %d", recorder.Code)
	}
	// 用户不存在 → 404；未登录 → 401
	if recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetSecurityAnswers", token, map[string]string{"userName": "ghost"}); recorder.Code != http.StatusNotFound {
		t.Fatalf("missing user status = %d", recorder.Code)
	}
	if recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetSecurityAnswers", "", map[string]string{"userName": "alice"}); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d", recorder.Code)
	}
}
