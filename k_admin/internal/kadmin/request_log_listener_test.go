package kadmin

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

func TestCaptureRequestLogInputRedactsSensitiveValuesAndRestoresBody(t *testing.T) {
	body := `{"username":"admin","password":"secret","profile":{"accessToken":"token","city":"Shanghai"}}`
	request := httptest.NewRequest("POST", "/api/auth/login?token=query-token&view=full", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")

	captured := captureRequestLogInput(request)
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(captured), &payload); err != nil {
		t.Fatalf("captured input is not valid JSON: %v", err)
	}
	encoded := string(mustJSON(t, payload))
	if strings.Contains(encoded, "secret") || strings.Contains(encoded, "query-token") {
		t.Fatalf("captured input leaked a sensitive value: %s", encoded)
	}
	if !strings.Contains(encoded, "[REDACTED]") || !strings.Contains(encoded, "Shanghai") {
		t.Fatalf("captured input did not retain safe fields and redaction markers: %s", encoded)
	}

	restored, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatalf("read restored request body: %v", err)
	}
	if string(restored) != body {
		t.Fatalf("request body was not restored: got %q", restored)
	}
}

func TestRequestLogEventClassifiesFailedAPIRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("DELETE", "/api/users/9", nil)
	context.Request.Header.Set("User-Agent", "listener-test")
	context.Writer.WriteHeader(503)

	event := (&RequestLogListener{}).requestEvent(
		context,
		time.Now().Add(-25*time.Millisecond),
		"request-9",
		"trace-9",
		"",
	)
	if event.EventType != models.OperationLogEventTypeRequest || event.Level != models.OperationLogLevelError {
		t.Fatalf("unexpected event classification: %#v", event)
	}
	if event.Source != "vbenapi" || event.Module != "users" || event.Action != "/api/users/9" {
		t.Fatalf("unexpected event routing metadata: %#v", event)
	}
	if event.StatusCode == nil || *event.StatusCode != 503 || event.Success == nil || *event.Success {
		t.Fatalf("unexpected response result: %#v", event)
	}
	if event.DurationMs == nil || *event.DurationMs < 0 {
		t.Fatalf("unexpected duration: %#v", event.DurationMs)
	}
}

func TestRequestLogIdentifierValidation(t *testing.T) {
	if got := requestLogIdentifier(" request_1:part-2 "); got != "request_1:part-2" {
		t.Fatalf("expected valid identifier, got %q", got)
	}
	if got := requestLogIdentifier("request id"); got != "" {
		t.Fatalf("expected identifier with spaces to be rejected, got %q", got)
	}
	if got := requestLogIdentifier(strings.Repeat("a", 65)); got != "" {
		t.Fatalf("expected oversized identifier to be rejected, got %q", got)
	}
}

func TestShouldSkipOnlySuccessfulLogListRequests(t *testing.T) {
	if !shouldSkipRequestLog("GET", "/api/logs", 200) {
		t.Fatal("expected a successful log list request to be skipped")
	}
	for _, request := range []struct {
		method string
		path   string
		status int
	}{
		{method: "GET", path: "/api/logs", status: 401},
		{method: "GET", path: "/api/logs/1", status: 200},
		{method: "DELETE", path: "/api/logs", status: 200},
	} {
		if shouldSkipRequestLog(request.method, request.path, request.status) {
			t.Fatalf("did not expect request to be skipped: %#v", request)
		}
	}
}

func mustJSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	return encoded
}
