package kadmin

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterApplicationRoutesIncludesEveryModule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	store := &Store{conn: &fakeLogConnection{menuRows: []map[string]interface{}{{"id": int64(99)}}}}
	if err := registerApplicationRoutes(engine.Group("/api"), store); err != nil {
		t.Fatalf("register application routes: %v", err)
	}

	wanted := map[string]bool{
		"GET /api/auth/captcha":                  false,
		"POST /api/auth/login":                   false,
		"GET /api/user/info":                     false,
		"GET /api/menu/all":                      false,
		"GET /api/rbac/overview":                 false,
		"GET /api/admin-menus":                   false,
		"GET /api/users":                         false,
		"PUT /api/users/:id/unlock":              false,
		"GET /api/dictionaries/overview":         false,
		"POST /api/files":                        false,
		"POST /api/users/avatar":                 false,
		"GET /api/system/config/login":           false,
		"GET /api/logs":                          false,
		"GET /api/login-audits":                  false,
		"PATCH /api/login-audits/retention":      false,
		"GET /api/jobs":                          false,
		"GET /api/job-logs":                      false,
		"GET /api/system-monitor":                false,
		"PATCH /api/system-monitor/status":       false,
		"GET /api/load-ranking/status":           false,
		"PATCH /api/load-ranking/status":         false,
		"GET /api/load-ranking/rankings":         false,
		"GET /api/codegen/candidates":            false,
		"GET /api/codegen/tables":                false,
		"POST /api/codegen/tables/import":        false,
		"GET /api/codegen/configs/:id":           false,
		"PUT /api/codegen/configs/:id":           false,
		"DELETE /api/codegen/configs/:id":        false,
		"POST /api/codegen/configs/:id/preview":  false,
		"POST /api/codegen/configs/:id/generate": false,
		"GET /api/codegen/configs/:id/download":  false,
		"GET /api/product":                       false,
		"GET /api/product/:id":                   false,
		"POST /api/product":                      false,
		"PUT /api/product/:id":                   false,
		"DELETE /api/product/:id":                false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wanted[key]; ok {
			wanted[key] = true
		}
	}
	for route, registered := range wanted {
		if !registered {
			t.Errorf("application route %s was not registered", route)
		}
	}
}
