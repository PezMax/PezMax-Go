package kadmin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/files"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/jobs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loginlogs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/monitor"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

func TestRegisterLogRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerLogRoutes(engine.Group("/api"), &Store{})

	wanted := map[string]bool{
		"DELETE /api/logs":     false,
		"DELETE /api/logs/:id": false,
		"GET /api/logs":        false,
		"GET /api/logs/:id":    false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := wanted[key]; exists {
			wanted[key] = true
		}
	}
	for route, registered := range wanted {
		if !registered {
			t.Fatalf("route %s was not registered", route)
		}
	}
}

func TestParseManagedLogFilter(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(
		"GET",
		"/api/logs?page=2&pageSize=500&eventType=request&level=warn&method=post&success=false&statusCode=401&startedAt=2026-07-22T00:00:00Z&endedAt=2026-07-22T12:00:00Z",
		nil,
	)

	filter, err := parseManagedLogFilter(context)
	if err != nil {
		t.Fatalf("parse valid filter: %v", err)
	}
	if filter.Page != 2 || filter.PageSize != maxLogPageSize || filter.Method != "POST" {
		t.Fatalf("unexpected pagination or method: %#v", filter)
	}
	if filter.Success == nil || *filter.Success || filter.StatusCode == nil || *filter.StatusCode != 401 {
		t.Fatalf("unexpected result filters: %#v", filter)
	}

	where, args := managedLogWhere(filter)
	for _, condition := range []string{"l.event_type = ?", "l.level = ?", "l.method = ?", "l.success = ?", "l.status_code = ?", "l.occurred_at >= ?", "l.occurred_at <= ?"} {
		if !strings.Contains(where, condition) {
			t.Fatalf("missing condition %q in %q", condition, where)
		}
	}
	if len(args) != 7 {
		t.Fatalf("expected 7 query arguments, got %d", len(args))
	}
}

func TestManagedLogKeywordSearchIncludesBusinessAuditFields(t *testing.T) {
	where, args := managedLogWhere(managedLogFilter{Keyword: "resource-42"})
	for _, field := range []string{"l.input ILIKE ?", "l.metadata::text ILIKE ?"} {
		if !strings.Contains(where, field) {
			t.Fatalf("business audit search is missing %q in %q", field, where)
		}
	}
	if len(args) != 9 {
		t.Fatalf("keyword query arguments = %d, want 9", len(args))
	}
}

func TestParseManagedLogFilterRejectsInvalidValues(t *testing.T) {
	for _, query := range []string{
		"eventType=unknown",
		"level=notice",
		"success=maybe",
		"statusCode=99",
		"startedAt=not-a-date",
		"startedAt=2026-07-23T00:00:00Z&endedAt=2026-07-22T00:00:00Z",
	} {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest("GET", "/api/logs?"+query, nil)
		if _, err := parseManagedLogFilter(context); err == nil {
			t.Fatalf("expected query %q to fail validation", query)
		}
	}
}

func TestUserHasLogPermission(t *testing.T) {
	user := models.UserModel{Permissions: []models.PermissionModel{{Slug: logListPermission}}}
	if !userHasPermission(user, logListPermission) {
		t.Fatal("expected matching log permission to be accepted")
	}
	if userHasPermission(user, logDeletePermission) {
		t.Fatal("did not expect an unrelated permission to be accepted")
	}
	user.Permissions = []models.PermissionModel{{Slug: "*"}}
	if !userHasPermission(user, logDeletePermission) {
		t.Fatal("expected wildcard permission to be accepted")
	}
}

func TestDefaultFilePermissions(t *testing.T) {
	wanted := map[string]bool{
		files.UploadPermission: false,
		files.ReadPermission:   false,
		files.DeletePermission: false,
	}
	for _, seed := range defaultPermissionSeeds {
		if _, ok := wanted[seed.Slug]; ok {
			wanted[seed.Slug] = true
		}
	}
	for permission, found := range wanted {
		if !found {
			t.Fatalf("file permission %s was not seeded", permission)
		}
	}

	user := models.UserModel{Permissions: []models.PermissionModel{{Slug: "system:file:other"}}}
	if userHasPermission(user, files.UploadPermission) {
		t.Fatal("unrelated file permission must not grant upload access")
	}
}

func TestDefaultJobPermissionsAndMenu(t *testing.T) {
	wantedPermissions := map[string]bool{
		jobs.ListPermission:    false,
		jobs.CreatePermission:  false,
		jobs.UpdatePermission:  false,
		jobs.DeletePermission:  false,
		jobs.RunPermission:     false,
		jobs.LogListPermission: false,
	}
	for _, seed := range defaultPermissionSeeds {
		if _, ok := wantedPermissions[seed.Slug]; ok {
			wantedPermissions[seed.Slug] = true
		}
	}
	for permission, found := range wantedPermissions {
		if !found {
			t.Fatalf("job permission %s was not seeded", permission)
		}
	}

	binding, ok := vbenMenuRouteBindings["/kadmin/jobs"]
	if !ok || binding.Component != "/kadmin/components/JobManagementView" {
		t.Fatalf("unexpected job menu binding: %#v", binding)
	}
	for _, root := range defaultMenuSeeds {
		for _, child := range root.Children {
			if child.URI == "/kadmin/jobs" && child.Order == 9 {
				return
			}
		}
	}
	t.Fatal("default job menu seed was not found")
}

func TestDefaultMonitorPermissionsAndMenu(t *testing.T) {
	wantedPermissions := map[string]bool{
		monitor.ViewPermission:   false,
		monitor.UpdatePermission: false,
	}
	for _, seed := range defaultPermissionSeeds {
		if _, ok := wantedPermissions[seed.Slug]; ok {
			wantedPermissions[seed.Slug] = true
		}
	}
	for permission, found := range wantedPermissions {
		if !found {
			t.Fatalf("monitor permission %s was not seeded", permission)
		}
	}

	binding, ok := vbenMenuRouteBindings["/kadmin/monitor"]
	if !ok || binding.Component != "/kadmin/components/SystemMonitorView" {
		t.Fatalf("unexpected monitor menu binding: %#v", binding)
	}
	for _, root := range defaultMenuSeeds {
		for _, child := range root.Children {
			if child.URI == "/kadmin/monitor" && child.Order == 10 {
				return
			}
		}
	}
	t.Fatal("default monitor menu seed was not found")
}

func TestFilePermissionMiddlewareReturns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/api/files", (&Store{}).permissionRequired(func(*gin.Context) (models.UserModel, bool) {
		return models.UserModel{Permissions: []models.PermissionModel{{Slug: "system:file:other"}}}, true
	}, files.UploadPermission), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/files", nil)

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestDefaultLogMenuBinding(t *testing.T) {
	binding, ok := vbenMenuRouteBindings["/kadmin/logs"]
	if !ok || binding.Component != "/kadmin/components/LogManagementView" {
		t.Fatalf("unexpected log menu binding: %#v", binding)
	}
	for _, root := range defaultMenuSeeds {
		for _, child := range root.Children {
			if child.URI == "/kadmin/logs" && child.Order == 7 {
				return
			}
		}
	}
	t.Fatal("default log menu seed was not found")
}

func TestDefaultLoginAuditPermissionsAndMenu(t *testing.T) {
	wantedPermissions := map[string]bool{
		loginlogs.ListPermission: false, loginlogs.DeletePermission: false, loginlogs.RetentionPermission: false,
	}
	for _, seed := range defaultPermissionSeeds {
		if _, ok := wantedPermissions[seed.Slug]; ok {
			wantedPermissions[seed.Slug] = true
		}
	}
	for permission, found := range wantedPermissions {
		if !found {
			t.Fatalf("login audit permission %s was not seeded", permission)
		}
	}
	binding, ok := vbenMenuRouteBindings["/kadmin/login-audits"]
	if !ok || binding.Component != "/kadmin/components/LoginAuditManagementView" {
		t.Fatalf("unexpected login audit menu binding: %#v", binding)
	}
	for _, root := range defaultMenuSeeds {
		for _, child := range root.Children {
			if child.URI == "/kadmin/login-audits" && child.Order == 8 {
				return
			}
		}
	}
	t.Fatal("default login audit menu seed was not found")
}
