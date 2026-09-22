package kadmin

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
)

func seedReportFixture(t *testing.T) (*Store, *fakeDatumDB, *gin.Engine, string, string, int64, int64) {
	t.Helper()
	store, db := newDatumAuthStore(t, nil)
	// alice 拥有文件；bob 举报 alice 的文件
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumUser(t, db, "bob", "secret5", "1", false)
	seedApprovedFile(db, 1001, db.users["alice"].UserID, "试卷A.pdf", "高数", 1, 2024, 1)
	seedActivityBookmark(t, db, 55)
	engine := datumEngine(t, store)
	bobToken := profileLogin(t, store, engine, "bob", "secret5", "cap-bob")
	aliceToken := profileLogin(t, store, engine, "alice", "secret5", "cap-alice")
	bobID := db.users["bob"].UserID
	aliceID := db.users["alice"].UserID
	return store, db, engine, bobToken, aliceToken, bobID, aliceID
}

// adminLoginForAudit 直接用 authService 签发管理端 JWT（走真实 docker redis），
// 供 requireAuth 保护的审核端点使用；redis 不可达时跳过测试。
func adminLoginForAudit(t *testing.T, store *Store) string {
	t.Helper()
	t.Setenv("KADMIN_REDIS_HOST", "127.0.0.1")
	t.Setenv("KADMIN_REDIS_PORT", "26379")
	t.Setenv("KADMIN_REDIS_PASSWORD", "pezmax_redis_pwd")
	auth := &authService{
		keyPrefix:  "test:admin:auth",
		redis:      newAuthRedisClientFromEnv(),
		secret:     []byte("test-admin-secret"),
		accessTTL:  time.Hour,
		refreshTTL: 24 * time.Hour,
		issuer:     "test",
	}
	tokens, err := auth.issueTokenPair(1)
	if err != nil {
		t.Skipf("管理端 redis 不可达，跳过审核链路单测：%v", err)
	}
	// 审核 handler 里 requireAuth 使用的是同一个 store.auth；测试里用注入的
	// authService 签发，再让 handler 解析同一 secret。
	store.auth = auth
	return tokens.AccessToken
}

func TestDatumReportCreateListTimeline(t *testing.T) {
	_, _, engine, bobToken, aliceToken, bobID, aliceID := seedReportFixture(t)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/report", bobToken, map[string]interface{}{
		"fileId": int64(1001), "userId": bobID, "reason": "内容违规", "remark": "第一段抄袭",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body %s", recorder.Code, recorder.Body.String())
	}
	reportID := toDatumString(datumBody(t, recorder)["data"].(map[string]interface{})["reportId"])

	recorder = datumJSON(t, engine, http.MethodPost, "/datum/report", bobToken, map[string]interface{}{
		"fileId": int64(1001), "userId": bobID, "reason": "再报一次",
	})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body %s", recorder.Code, recorder.Body.String())
	}

	recorder = datumJSON(t, engine, http.MethodPost, "/datum/report", aliceToken, map[string]interface{}{
		"fileId": int64(1001), "userId": aliceID, "reason": "自首",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("self-report status = %d", recorder.Code)
	}

	recorder = datumJSON(t, engine, http.MethodGet, "/datum/report/timeline/"+reportID, bobToken, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("timeline status = %d body %s", recorder.Code, recorder.Body.String())
	}
	events := datumBody(t, recorder)["data"].([]interface{})
	if len(events) != 1 || events[0].(map[string]interface{})["step"] != "提交举报" {
		t.Fatalf("events = %v", events)
	}
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/report/timeline/"+reportID, aliceToken, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("foreign timeline status = %d", recorder.Code)
	}

	recorder = datumJSON(t, engine, http.MethodGet, "/datum/report/list?userId="+toDatumString(bobID)+"&pageNum=1&pageSize=10", bobToken, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d body %s", recorder.Code, recorder.Body.String())
	}
	body := datumBody(t, recorder)
	if body["total"].(float64) != 1 {
		t.Fatalf("total = %v", body["total"])
	}
	row := body["rows"].([]interface{})[0].(map[string]interface{})
	if row["fileName"] != "试卷A.pdf" || row["reason"] != "内容违规" {
		t.Fatalf("row = %v", row)
	}

	recorder = datumJSON(t, engine, http.MethodPut, "/datum/report", bobToken, map[string]interface{}{
		"reportId": toDatumInt64(reportID), "reason": "内容违规（补充证据）", "remark": "截图",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d body %s", recorder.Code, recorder.Body.String())
	}
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/report/"+reportID, aliceToken, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("foreign delete status = %d", recorder.Code)
	}
	_ = strings.TrimSpace("")
}

func TestDatumReportAuditLinkage(t *testing.T) {
	store, db, engine, bobToken, _, bobID, aliceID := seedReportFixture(t)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/report", bobToken, map[string]interface{}{
		"fileId": int64(1001), "userId": bobID, "reason": "内容违规",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d", recorder.Code)
	}
	reportID := toDatumString(datumBody(t, recorder)["data"].(map[string]interface{})["reportId"])

	// datum 会话不能审核 → 401（requireAuth 是管理端 JWT）
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/report/audit/"+reportID, bobToken, map[string]string{"result": "1"})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("datum-token audit status = %d body %s", recorder.Code, recorder.Body.String())
	}

	adminToken := adminLoginForAudit(t, store)
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/report/audit/"+reportID, adminToken, map[string]string{"result": "9"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad result status = %d body %s", recorder.Code, recorder.Body.String())
	}

	recorder = datumJSON(t, engine, http.MethodPost, "/datum/report/audit/"+reportID, adminToken, map[string]string{"result": "1", "remark": "确认违规"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("audit status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if toDatumInt64(db.files[0]["file_status"]) != 3 {
		t.Fatalf("file status = %v, want 3", db.files[0]["file_status"])
	}
	if len(db.notifications) != 1 {
		t.Fatalf("notifications = %d", len(db.notifications))
	}
	notification := db.notifications[0]
	if toDatumString(notification["material_title_snapshot"]) != "试卷A.pdf" ||
		toDatumInt64(notification["upload_user_id"]) != aliceID ||
		toDatumString(notification["notify_type"]) != "4" {
		t.Fatalf("notification = %v", notification)
	}
	report, found, err := datum.NewReportRepo(store.conn).FindByID(toDatumInt64(reportID))
	if err != nil || !found || report.Result != "1" {
		t.Fatalf("report = %+v found = %v err = %v", report, found, err)
	}
}

func TestDatumBookmarkReportFlowAndAudit(t *testing.T) {
	store, db, engine, bobToken, _, bobID, _ := seedReportFixture(t)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/bookmarkReport", bobToken, map[string]interface{}{
		"bookmarkId": int64(55), "userId": bobID, "reason": "死链", "remark": "",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body %s", recorder.Code, recorder.Body.String())
	}
	reportID := toDatumString(datumBody(t, recorder)["data"].(map[string]interface{})["reportId"])

	recorder = datumJSON(t, engine, http.MethodPost, "/datum/bookmarkReport", bobToken, map[string]interface{}{
		"bookmarkId": int64(55), "userId": bobID, "reason": "再报",
	})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d", recorder.Code)
	}
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/bookmarkReport", bobToken, map[string]interface{}{
		"reportId": toDatumInt64(reportID), "reason": "死链（已验证）",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d", recorder.Code)
	}
	recorder = datumJSON(t, engine, http.MethodGet, "/datum/bookmarkReport/timeline/"+reportID, bobToken, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("timeline status = %d", recorder.Code)
	}

	adminToken := adminLoginForAudit(t, store)
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/bookmarkReport/audit/"+reportID, adminToken, map[string]string{"result": "1"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("audit status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if toDatumInt64(db.bookmarks[55]["status"]) != 3 {
		t.Fatalf("bookmark status = %v, want 3", db.bookmarks[55]["status"])
	}
	if len(db.notifications) != 0 {
		t.Fatalf("bookmark audit must not create notifications: %v", db.notifications)
	}
	recorder = datumJSON(t, engine, http.MethodPut, "/datum/bookmarkReport", bobToken, map[string]interface{}{
		"reportId": toDatumInt64(reportID), "reason": "再改",
	})
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("post-audit update status = %d", recorder.Code)
	}
	_ = store
}

func TestDatumReportDeleteOwnPending(t *testing.T) {
	store, _, engine, bobToken, _, bobID, _ := seedReportFixture(t)
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/report", bobToken, map[string]interface{}{
		"fileId": int64(1001), "userId": bobID, "reason": "误报待删",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d", recorder.Code)
	}
	reportID := toDatumString(datumBody(t, recorder)["data"].(map[string]interface{})["reportId"])
	recorder = datumJSON(t, engine, http.MethodDelete, "/datum/report/"+reportID, bobToken, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d body %s", recorder.Code, recorder.Body.String())
	}
	_, found, err := datum.NewReportRepo(store.conn).FindByID(toDatumInt64(reportID))
	if err != nil || found {
		t.Fatalf("report should be deleted, found = %v err = %v", found, err)
	}
}
