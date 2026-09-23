package kadmin

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newDatumTestStore(t *testing.T, overrides map[string]string) (*Store, *fakeSecurityRedis) {
	t.Helper()
	values := defaultSystemConfigValues()
	for key, value := range overrides {
		values[key] = value
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("marshal system config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "system-config.json")
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write system config: %v", err)
	}
	t.Setenv(systemConfigPathEnv, path)
	redis := newFakeSecurityRedis()
	store := &Store{}
	store.security = &securityService{keyPrefix: "test:security", redis: redis, secret: []byte("test-secret")}
	return store, redis
}

func TestDatumCaptchaImageRouteIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, _ := newDatumTestStore(t, nil)
	engine := gin.New()
	registerDatumRoutes(engine, store)

	registered := false
	for _, route := range engine.Routes() {
		// gin(v1.3) GET 树限制：/datum/user 全部读接口经通配分发器路由，
		// captchaImage 由 datumUserGet 分发（行为由下方用例覆盖）。
		if route.Method+" "+route.Path == "GET /datum/user/*rest" {
			registered = true
			break
		}
	}
	if !registered {
		t.Fatal("route GET /datum/user/*rest was not registered")
	}

	preflight := httptest.NewRecorder()
	engine.ServeHTTP(preflight, httptest.NewRequest(http.MethodOptions, "/datum/user/captchaImage", nil))
	if preflight.Code != http.StatusNoContent {
		t.Fatalf("CORS preflight status = %d, want 204", preflight.Code)
	}
}

func TestDatumCaptchaImageStaysOnRegardlessOfAdminSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// security.captcha_enabled 默认且显式关闭时，应用端验证码仍保持开启：
	// 该开关只约束管理后台登录，不约束桌面端。
	store, redis := newDatumTestStore(t, map[string]string{"security.captcha_enabled": "false"})
	engine := gin.New()
	registerDatumRoutes(engine, store)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/datum/user/captchaImage", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			CaptchaEnabled bool   `json:"captchaEnabled"`
			UUID           string `json:"uuid"`
			Img            string `json:"img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("body code = %d, want 0", body.Code)
	}
	if !body.Data.CaptchaEnabled || body.Data.UUID == "" || body.Data.Img == "" {
		t.Fatalf("desktop captcha must stay enabled even when the admin switch is off: %#v", body.Data)
	}
	if stored := redis.values[store.security.captchaKey(body.Data.UUID)]; stored == "" {
		t.Fatal("challenge digest was not stored")
	}
}

func TestDatumCaptchaImageIssuesBareBase64JpegChallenge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, redis := newDatumTestStore(t, map[string]string{
		"security.captcha_enabled":     "true",
		"security.captcha_ttl_seconds": "60",
	})
	engine := gin.New()
	registerDatumRoutes(engine, store)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/datum/user/captchaImage", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body struct {
		Data struct {
			CaptchaEnabled bool   `json:"captchaEnabled"`
			UUID           string `json:"uuid"`
			Img            string `json:"img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.Data.CaptchaEnabled || body.Data.UUID == "" || body.Data.Img == "" {
		t.Fatalf("enabled captcha should issue a challenge: %#v", body.Data)
	}
	if strings.Contains(body.Data.Img, "data:") {
		t.Fatal("img must be a bare base64 payload; the desktop client adds the data URI prefix")
	}
	raw, err := base64.StdEncoding.DecodeString(body.Data.Img)
	if err != nil {
		t.Fatalf("decode img payload: %v", err)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("img payload is not a valid JPEG: %v", err)
	}
	if config.Width != 190 || config.Height != 56 {
		t.Fatalf("captcha image bounds = %dx%d, want 190x56", config.Width, config.Height)
	}
	if stored := redis.values[store.security.captchaKey(body.Data.UUID)]; stored == "" {
		t.Fatal("challenge digest was not stored")
	}
}

func TestVerifyDatumCaptchaConsumesChallengeAndReportsStatusError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, _ := newDatumTestStore(t, nil)
	if err := store.security.storeCaptcha("known", "123456", time.Minute); err != nil {
		t.Fatalf("store captcha: %v", err)
	}
	engine := gin.New()
	engine.POST("/check", func(c *gin.Context) {
		if store.verifyDatumCaptcha(c, c.Query("uuid"), c.Query("code")) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		}
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/check?uuid=known&code=999999", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid captcha status = %d, want 400", recorder.Code)
	}
	var failure struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &failure); err != nil {
		t.Fatalf("decode failure body: %v", err)
	}
	if failure.Code != http.StatusBadRequest || failure.Msg == "" {
		t.Fatalf("failure envelope = %#v, want code 400 with a message", failure)
	}

	if err := store.security.storeCaptcha("known-2", "654321", time.Minute); err != nil {
		t.Fatalf("store second captcha: %v", err)
	}
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/check?uuid=known-2&code=654321", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("valid captcha should pass, got status %d body %s", recorder.Code, recorder.Body.String())
	}
}
