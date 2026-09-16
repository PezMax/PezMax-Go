package loginlogs

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuditWhereUsesExpectedFilters(t *testing.T) {
	filter := Filter{Account: "admin", IP: "127.0.0.1", Status: StatusFailed, Result: ResultInvalidPassword}
	where, args := auditWhere(filter)
	for _, condition := range []string{"a.account ILIKE ?", "a.ip ILIKE ?", "a.status = ?", "a.result = ?"} {
		if !strings.Contains(where, condition) {
			t.Fatalf("missing condition %q in %q", condition, where)
		}
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 arguments, got %d", len(args))
	}
}

func TestParseFilterValidatesStatusResultAndTime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/api/login-audits?page=2&pageSize=500&account=admin&ip=127.0.0.1&status=failed&result=invalid_password&startedAt=2026-07-28T00:00:00Z&endedAt=2026-07-29T00:00:00Z", nil)
	filter, err := parseFilter(context)
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}
	if filter.Page != 2 || filter.PageSize != maxPageSize || filter.Status != StatusFailed || filter.Result != ResultInvalidPassword {
		t.Fatalf("unexpected filter: %#v", filter)
	}

	for _, query := range []string{"status=unknown", "result=unknown", "startedAt=invalid", "startedAt=2026-07-29T00:00:00Z&endedAt=2026-07-28T00:00:00Z"} {
		context, _ = gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest("GET", "/api/login-audits?"+query, nil)
		if _, err := parseFilter(context); err == nil {
			t.Fatalf("expected query %q to fail", query)
		}
	}
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	pass := func(c *gin.Context) { c.Next() }
	RegisterRoutes(engine.Group("/api"), nil, Dependencies{
		RequireAuth:       pass,
		RequirePermission: func(...string) gin.HandlerFunc { return pass },
	})
	wanted := map[string]bool{
		"GET /api/login-audits": false, "DELETE /api/login-audits": false,
		"POST /api/login-audits/cleanup": false, "GET /api/login-audits/retention": false,
		"PATCH /api/login-audits/retention": false,
	}
	for _, route := range engine.Routes() {
		wanted[route.Method+" "+route.Path] = true
	}
	for route, found := range wanted {
		if !found {
			t.Fatalf("route %s was not registered", route)
		}
	}
}

func TestUniqueIDsAndValidResults(t *testing.T) {
	ids := uniqueIDs([]int64{0, 3, 3, -1, 5})
	if len(ids) != 2 || ids[0] != 3 || ids[1] != 5 {
		t.Fatalf("unexpected ids: %#v", ids)
	}
	for _, result := range []string{ResultSuccess, ResultAccountNotFound, ResultInvalidPassword, ResultAccountDisabled, ResultAccountLocked, ResultAccountUnlocked, ResultCaptchaInvalid, ResultSystemError} {
		if !validResult(result) {
			t.Fatalf("expected %s to be valid", result)
		}
	}
	if validResult("password") {
		t.Fatal("unexpected result accepted")
	}
}

func TestAttemptDoesNotAcceptSensitiveCredentialFields(t *testing.T) {
	typeOfAttempt := reflect.TypeOf(Attempt{})
	for i := 0; i < typeOfAttempt.NumField(); i++ {
		name := strings.ToLower(typeOfAttempt.Field(i).Name)
		for _, sensitive := range []string{"password", "token", "cookie", "secret"} {
			if strings.Contains(name, sensitive) {
				t.Fatalf("attempt must not contain sensitive field %q", typeOfAttempt.Field(i).Name)
			}
		}
	}
}

func TestBrowserLabelPrioritizesChromiumSignatures(t *testing.T) {
	chrome := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36"
	if got := browserLabel(chrome, "Safari", "537.36"); got != "Chrome 126.0.0.0" {
		t.Fatalf("chrome label = %q", got)
	}
	edge := chrome + " Edg/126.0.0.0"
	if got := browserLabel(edge, "Chrome", "126.0.0.0"); got != "Edge 126.0.0.0" {
		t.Fatalf("edge label = %q", got)
	}
	safari := "Mozilla/5.0 Version/17.5 Safari/605.1.15"
	if got := browserLabel(safari, "Safari", "605.1.15"); got != "Safari 17.5" {
		t.Fatalf("safari label = %q", got)
	}
}
