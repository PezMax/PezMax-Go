package kadmin

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"

	"testing"
)

func seedNotificationFixture(t *testing.T) (*Store, *fakeDatumDB, *gin.Engine, string, int64) {
	t.Helper()
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)
	token := profileLogin(t, store, engine, "alice", "secret5", "cap-1")
	return store, db, engine, token, db.users["alice"].UserID
}

func notificationCreateBody(notifyType, displayMode string, extra map[string]interface{}) map[string]interface{} {
	body := map[string]interface{}{
		"notifyType":  notifyType,
		"title":       "E2E 通知 " + notifyType + displayMode,
		"content":     "通知内容",
		"status":      "0",
		"sort":        int64(5),
		"displayMode": displayMode,
	}
	for key, value := range extra {
		body[key] = value
	}
	return body
}

func TestDatumNotificationCrudAndFeeds(t *testing.T) {
	store, db, engine, token, aliceID := seedNotificationFixture(t)
	// 管理写（新增/修改/删除）走管理端 JWT + datum:notification:manage 权限；
	// datum 会话只能读（列表/详情/弹窗/滚动）。
	adminToken := adminLoginForAudit(t, store)
	adminWriteSeq := 0
	adminWrite := func(method, path string, body interface{}) *httptest.ResponseRecorder {
		t.Helper()
		adminWriteSeq++
		return datumJSONWithHeaders(t, engine, method, path, adminToken, body,
			map[string]string{"Idempotency-Key": "datum-notif-" + toDatumString(int64(adminWriteSeq))})
	}

	// datum 会话调用管理写 → 401
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/notification", token,
		notificationCreateBody("1", "0", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("datum-token create status = %d body %s", recorder.Code, recorder.Body.String())
	}

	// 创建弹窗通知（类型 1）
	recorder = adminWrite(http.MethodPost, "/datum/notification", notificationCreateBody("1", "0", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("create popup status = %d body %s", recorder.Code, recorder.Body.String())
	}
	popupNotifyID := toDatumInt64(db.notifications[0]["notify_id"])

	// 创建滚动通知（类型 5，带展示开始时间）
	recorder = adminWrite(http.MethodPost, "/datum/notification",
		notificationCreateBody("5", "1", map[string]interface{}{"publishStart": "2026-09-22 00:00:00", "scrollTimeInterval": int64(45)}))
	if recorder.Code != http.StatusOK {
		t.Fatalf("create scroll status = %d body %s", recorder.Code, recorder.Body.String())
	}

	// 非法类型 → 400
	recorder = adminWrite(http.MethodPost, "/datum/notification", notificationCreateBody("9", "0", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad type status = %d", recorder.Code)
	}
	// 类型 4 缺 materialId → 400
	recorder = adminWrite(http.MethodPost, "/datum/notification", notificationCreateBody("4", "0", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("type4 without material status = %d", recorder.Code)
	}

	// 管理列表（TableDataInfo 顶层）
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/notification/list?pageNum=1&pageSize=10", token, nil)
	body := datumBody(t, recorder)
	if body["total"].(float64) != 2 || len(body["rows"].([]interface{})) != 2 {
		t.Fatalf("list = %v", body)
	}
	row := body["rows"].([]interface{})[0].(map[string]interface{})
	if row["notifyType"] == "" || row["title"] == "" {
		t.Fatalf("row fields = %v", row)
	}

	// 类型过滤
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/notification/list?notifyType=5&pageNum=1&pageSize=10", token, nil)
	rows := datumBody(t, recorder)["rows"].([]interface{})
	if len(rows) != 1 || rows[0].(map[string]interface{})["notifyType"] != "5" {
		t.Fatalf("filtered rows = %v", rows)
	}

	// 详情
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/notification/"+toDatumString(popupNotifyID), token, nil)
	detail := datumBody(t, recorder)["data"].(map[string]interface{})
	if detail["title"].(string) != "E2E 通知 10" {
		t.Fatalf("detail = %v", detail)
	}

	// 更新（标题 + 状态禁用）
	recorder = adminWrite(http.MethodPut, "/datum/notification", map[string]interface{}{
		"notifyId": popupNotifyID, "notifyType": "1", "title": "更新后的标题", "content": "新内容",
		"status": "1", "sort": int64(1), "displayMode": "0",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if db.notifications[0]["title"] != "更新后的标题" || toDatumString(db.notifications[0]["status"]) != "1" {
		t.Fatalf("updated notification = %v", db.notifications[0])
	}

	// 弹窗端点：禁用通知不出现
	recorder = datumJSON(t, engine, http.MethodGet, "/system/notification/user/popup?userId="+toDatumString(aliceID), token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("popup status = %d body %s", recorder.Code, recorder.Body.String())
	}
	popupRows := datumBody(t, recorder)["data"].([]interface{})
	for _, item := range popupRows {
		if item.(map[string]interface{})["notifyId"].(float64) == float64(popupNotifyID) {
			t.Fatalf("disabled notification leaked to popup: %v", item)
		}
	}

	// 重新启用后再弹窗出现
	recorder = adminWrite(http.MethodPut, "/datum/notification", map[string]interface{}{
		"notifyId": popupNotifyID, "notifyType": "1", "title": "更新后的标题", "content": "新内容",
		"status": "0", "sort": int64(1), "displayMode": "0",
	})
	recorder = datumJSON(t, engine, http.MethodGet, "/system/notification/user/popup?userId="+toDatumString(aliceID), token, nil)
	found := false
	for _, item := range datumBody(t, recorder)["data"].([]interface{}) {
		if item.(map[string]interface{})["notifyId"].(float64) == float64(popupNotifyID) {
			found = true
		}
	}
	if !found {
		t.Fatal("enabled popup notification missing")
	}

	// 滚动端点：只有 display_mode=1 的通知
	recorder = datumJSON(t, engine, http.MethodGet, "/system/notification/user/scroll", token, nil)
	scrollRows := datumBody(t, recorder)["data"].([]interface{})
	if len(scrollRows) != 1 || scrollRows[0].(map[string]interface{})["displayMode"] != "1" {
		t.Fatalf("scroll rows = %v", scrollRows)
	}

	// popup userId 冒充 → 403
	recorder = datumJSON(t, engine, http.MethodGet, "/system/notification/user/popup?userId=999", token, nil)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("impersonated popup status = %d", recorder.Code)
	}

	// 删除
	recorder = adminWrite(http.MethodDelete, "/datum/notification/"+toDatumString(popupNotifyID), nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d", recorder.Code)
	}
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/notification/"+toDatumString(popupNotifyID), token, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("post-delete detail status = %d", recorder.Code)
	}
	_ = store
}

// respondNotificationFeed 的纯单测：协议形状不依赖 Redis/DB（e2e 见
// TestDatumNotificationFeedHashProtocol，需本机 Redis）。
func TestRespondNotificationFeedHash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rows := []gin.H{{"notifyId": int64(7), "title": "hello"}}
	requestFeed := func(target string) map[string]interface{} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
		respondNotificationFeed(ctx, rows)
		var envelope map[string]interface{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		return envelope
	}

	first := requestFeed("/system/notification/user/scroll")
	hash, _ := first["hash"].(string)
	if hash == "" || first["unchanged"] != false {
		t.Fatalf("first feed = %v", first)
	}
	if _, ok := first["data"].([]interface{}); !ok {
		t.Fatalf("data must stay a JSON array, got %T", first["data"])
	}
	if code := first["code"].(float64); code != 0 {
		t.Fatalf("code = %v", first["code"])
	}

	unchanged := requestFeed("/system/notification/user/scroll?hash=" + hash)
	if unchanged["unchanged"] != true || unchanged["data"] != nil || unchanged["hash"] != hash {
		t.Fatalf("unchanged feed = %v", unchanged)
	}

	changed := requestFeed("/system/notification/user/scroll?hash=deadbeef")
	if changed["unchanged"] != false {
		t.Fatalf("changed feed = %v", changed)
	}
	if _, ok := changed["data"].([]interface{}); !ok {
		t.Fatalf("changed feed must carry data, got %T", changed["data"])
	}
}

// 通知喂给端复用 file tree 的 hash 轮询协议：携带上次 hash 时未变化则不带
// data；内容变化（管理写接口落库）后 hash 必变，客户端拿到新列表。
func TestDatumNotificationFeedHashProtocol(t *testing.T) {
	store, db, engine, token, aliceID := seedNotificationFixture(t)
	adminToken := adminLoginForAudit(t, store)
	publishScroll := func() {
		t.Helper()
		recorder := datumJSONWithHeaders(t, engine, http.MethodPost, "/datum/notification", adminToken,
			notificationCreateBody("5", "1", map[string]interface{}{"publishStart": "2026-09-22 00:00:00"}),
			map[string]string{"Idempotency-Key": "datum-notif-hash-" + toDatumString(int64(len(db.notifications) + 1))})
		if recorder.Code != http.StatusOK {
			t.Fatalf("create scroll status = %d body %s", recorder.Code, recorder.Body.String())
		}
	}

	// 首次拉取：data + hash 齐备，unchanged=false
	recorder := datumJSON(t, engine, http.MethodGet, "/system/notification/user/scroll", token, nil)
	body := datumBody(t, recorder)
	firstHash, _ := body["hash"].(string)
	if firstHash == "" || body["unchanged"] != false || len(body["data"].([]interface{})) != 1 {
		t.Fatalf("first scroll = %v", body)
	}

	// 携带相同 hash：unchanged=true 且不传 data
	recorder = datumJSON(t, engine, http.MethodGet, "/system/notification/user/scroll?hash="+firstHash, token, nil)
	body = datumBody(t, recorder)
	if body["unchanged"] != true || body["data"] != nil || body["hash"] != firstHash {
		t.Fatalf("unchanged scroll = %v", body)
	}

	// 内容变化后旧 hash 失效：回新列表与新 hash
	publishScroll()
	recorder = datumJSON(t, engine, http.MethodGet, "/system/notification/user/scroll?hash="+firstHash, token, nil)
	body = datumBody(t, recorder)
	secondHash, _ := body["hash"].(string)
	if body["unchanged"] != false || secondHash == "" || secondHash == firstHash || len(body["data"].([]interface{})) != 2 {
		t.Fatalf("changed scroll = %v", body)
	}

	// popup 端点同样支持（按用户内容哈希）
	recorder = datumJSON(t, engine, http.MethodGet, "/system/notification/user/popup?userId="+toDatumString(aliceID), token, nil)
	body = datumBody(t, recorder)
	if body["unchanged"] != false || body["hash"] == nil {
		t.Fatalf("first popup = %v", body)
	}
	popupHash, _ := body["hash"].(string)
	recorder = datumJSON(t, engine, http.MethodGet,
		"/system/notification/user/popup?userId="+toDatumString(aliceID)+"&hash="+popupHash, token, nil)
	body = datumBody(t, recorder)
	if body["unchanged"] != true {
		t.Fatalf("unchanged popup = %v", body)
	}
}
