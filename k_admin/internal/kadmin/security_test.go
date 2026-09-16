package kadmin

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"time"
)

type fakeSecurityRedis struct {
	mu       sync.Mutex
	values   map[string]string
	counters map[string]int
}

func newFakeSecurityRedis() *fakeSecurityRedis {
	return &fakeSecurityRedis{values: make(map[string]string), counters: make(map[string]int)}
}

func (f *fakeSecurityRedis) do(args ...string) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(args) == 0 {
		return nil, errors.New("missing command")
	}
	switch args[0] {
	case "SET":
		if len(args) >= 5 && args[3] == "NX" {
			if _, exists := f.values[args[1]]; exists {
				return nil, errRedisNil
			}
		}
		f.values[args[1]] = args[2]
		return "OK", nil
	case "GET":
		value, exists := f.values[args[1]]
		if !exists {
			return nil, errRedisNil
		}
		return value, nil
	case "GETDEL":
		value, exists := f.values[args[1]]
		if !exists {
			return nil, errRedisNil
		}
		delete(f.values, args[1])
		return value, nil
	case "EXISTS":
		_, exists := f.values[args[1]]
		if exists {
			return int64(1), nil
		}
		return int64(0), nil
	case "DEL":
		var deleted int64
		for _, key := range args[1:] {
			if _, exists := f.values[key]; exists {
				delete(f.values, key)
				deleted++
			}
			delete(f.counters, key)
		}
		return deleted, nil
	case "EVAL":
		return f.eval(args)
	default:
		return nil, errors.New("unsupported fake redis command: " + args[0])
	}
}

func (f *fakeSecurityRedis) eval(args []string) (interface{}, error) {
	if strings.Contains(args[1], "INCR") {
		failureKey, lockKey := args[3], args[4]
		threshold, _ := strconv.Atoi(args[6])
		f.counters[failureKey]++
		if f.counters[failureKey] >= threshold {
			f.values[lockKey] = "1"
			delete(f.counters, failureKey)
			return int64(1), nil
		}
		return int64(0), nil
	}
	key, expected := args[3], args[4]
	if f.values[key] != expected {
		return int64(0), nil
	}
	if strings.Contains(args[1], "EXPIRE") {
		return int64(1), nil
	}
	if len(args) >= 7 {
		f.values[key] = args[5]
	} else {
		delete(f.values, key)
	}
	return int64(1), nil
}

func fakeSecurityService() (*securityService, *fakeSecurityRedis) {
	redis := newFakeSecurityRedis()
	return &securityService{keyPrefix: "test:security", redis: redis, secret: []byte("test-secret")}, redis
}

func TestCaptchaIsRasterizedSingleUseAndRejectsWrongAnswer(t *testing.T) {
	service, redis := fakeSecurityService()
	challenge, err := service.issueCaptcha(time.Minute)
	if err != nil {
		t.Fatalf("issue captcha: %v", err)
	}
	if challenge.ID == "" || challenge.ExpiresIn != 60 || !strings.HasPrefix(challenge.Image, "data:image/png;base64,") {
		t.Fatalf("unexpected challenge: %#v", challenge)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(challenge.Image, "data:image/png;base64,"))
	if err != nil {
		t.Fatalf("decode captcha image: %v", err)
	}
	image, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode captcha PNG: %v", err)
	}
	if bounds := image.Bounds(); bounds.Dx() != 190 || bounds.Dy() != 56 {
		t.Fatalf("captcha image bounds = %v", bounds)
	}
	if stored := redis.values[service.captchaKey(challenge.ID)]; stored == "" {
		t.Fatal("captcha digest was not stored")
	}

	const id = "known-captcha"
	const answer = "012345"
	if err := service.storeCaptcha(id, answer, time.Minute); err != nil {
		t.Fatalf("store known captcha: %v", err)
	}
	if err := service.verifyCaptcha(id, answer); err != nil {
		t.Fatalf("verify captcha: %v", err)
	}
	if !errors.Is(service.verifyCaptcha(id, answer), errCaptchaInvalid) {
		t.Fatal("captcha replay should be rejected")
	}

	const secondID = "wrong-captcha"
	if err := service.storeCaptcha(secondID, answer, time.Minute); err != nil {
		t.Fatalf("store second captcha: %v", err)
	}
	if !errors.Is(service.verifyCaptcha(secondID, "999999"), errCaptchaInvalid) {
		t.Fatal("wrong captcha answer should be rejected")
	}
	if !errors.Is(service.verifyCaptcha(secondID, answer), errCaptchaInvalid) {
		t.Fatal("wrong answer must consume the challenge")
	}
}

func TestLoginFailureLocksAndUnlocksAccount(t *testing.T) {
	service, _ := fakeSecurityService()
	policy := securityPolicy{
		LoginLockEnabled:        true,
		LoginFailureThreshold:   3,
		LoginIPFailureThreshold: 20,
		LoginFailureWindow:      time.Minute,
		LoginLockDuration:       time.Minute,
		LoginIPWhitelist:        []string{"127.0.0.1"},
	}
	for attempt := 1; attempt <= 2; attempt++ {
		locked, err := service.recordLoginFailure("Admin", "127.0.0.1", policy)
		if err != nil || locked {
			t.Fatalf("attempt %d: locked=%v err=%v", attempt, locked, err)
		}
	}
	locked, err := service.recordLoginFailure("admin", "127.0.0.1", policy)
	if err != nil || !locked {
		t.Fatalf("threshold attempt: locked=%v err=%v", locked, err)
	}
	locked, err = service.loginLocked("ADMIN", "127.0.0.1", policy)
	if err != nil || !locked {
		t.Fatalf("stored lock: locked=%v err=%v", locked, err)
	}
	if err := service.unlockLogin("admin", ""); err != nil {
		t.Fatalf("unlock login: %v", err)
	}
	locked, err = service.loginLocked("admin", "127.0.0.1", policy)
	if err != nil || locked {
		t.Fatalf("after unlock: locked=%v err=%v", locked, err)
	}
}

func TestIdempotencyMiddlewareAuthenticatesBeforeRequiringKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &Store{auth: &authService{secret: []byte("test-secret")}}
	engine := gin.New()
	engine.Use(store.idempotencyMiddleware())
	engine.POST("/api/users", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated protected mutation status = %d, want 401; body=%s", response.Code, response.Body.String())
	}
}

func TestIdempotencyCompletesReplaysAndRejectsChangedRequest(t *testing.T) {
	service, _ := fakeSecurityService()
	first, err := service.beginIdempotency("user:7", "request-123", "digest-a", time.Minute)
	if err != nil || first.reservation == nil {
		t.Fatalf("reserve request: %#v err=%v", first, err)
	}
	if _, err := service.beginIdempotency("user:7", "request-123", "digest-a", time.Minute); !errors.Is(err, errIdempotencyRunning) {
		t.Fatalf("concurrent duplicate error = %v", err)
	}
	if err := service.completeIdempotency(*first.reservation, 200, "application/json", []byte(`{"code":0}`), time.Minute); err != nil {
		t.Fatalf("complete request: %v", err)
	}
	replay, err := service.beginIdempotency("user:7", "request-123", "digest-a", time.Minute)
	if err != nil || replay.replay == nil || replay.replay.Status != 200 {
		t.Fatalf("replay request: %#v err=%v", replay, err)
	}
	if _, err := service.beginIdempotency("user:7", "request-123", "digest-b", time.Minute); !errors.Is(err, errIdempotencyConflict) {
		t.Fatalf("changed request error = %v", err)
	}
	encoded, _ := json.Marshal(replay.replay)
	if strings.Contains(string(encoded), "digest-b") {
		t.Fatal("stored response was unexpectedly replaced")
	}
}

func TestIdempotencyReservationCanBeRenewedByOwnerOnly(t *testing.T) {
	service, _ := fakeSecurityService()
	result, err := service.beginIdempotency("user:7", "request-renew", "digest", time.Minute)
	if err != nil || result.reservation == nil {
		t.Fatalf("reserve request: %#v err=%v", result, err)
	}
	if err := service.renewIdempotency(*result.reservation, time.Minute); err != nil {
		t.Fatalf("renew reservation: %v", err)
	}
	other := *result.reservation
	other.processing = `{"owner":"other"}`
	if err := service.renewIdempotency(other, time.Minute); err == nil {
		t.Fatal("a different owner must not renew the reservation")
	}
}

func TestBusinessAuditDescriptorsCoverCriticalMutations(t *testing.T) {
	tests := []struct {
		method, path, resource, action, id string
	}{
		{"POST", "/api/users", "user", "create", "new"},
		{"PUT", "/api/users/3/password", "user", "password", "3"},
		{"PUT", "/api/rbac/roles/4/menus", "role", "assign-menus", "4"},
		{"PUT", "/api/admin-menus", "menu", "reorder", "all"},
		{"DELETE", "/api/files/9", "file", "delete", "9"},
		{"POST", "/api/jobs/8/run", "job", "run", "8"},
	}
	for _, tt := range tests {
		descriptor, ok := describeBusinessMutation(tt.method, tt.path)
		if !ok || descriptor.Resource != tt.resource || descriptor.Action != tt.action || descriptor.ResourceID != tt.id {
			t.Fatalf("%s %s descriptor = %#v, ok=%v", tt.method, tt.path, descriptor, ok)
		}
	}
	if _, ok := describeBusinessMutation("GET", "/api/users"); ok {
		t.Fatal("read request must not create business audit event")
	}
}

func TestAuditSanitizerRedactsConfigSecretsAndImportContent(t *testing.T) {
	value := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"key": "auth.default_password", "value": "real-password"},
		},
		"content": "sensitive import payload",
	}
	encoded, _ := json.Marshal(sanitizeJSONValue(value, 0))
	text := string(encoded)
	if strings.Contains(text, "real-password") || strings.Contains(text, "sensitive import payload") {
		t.Fatalf("sensitive audit payload leaked: %s", text)
	}
}
