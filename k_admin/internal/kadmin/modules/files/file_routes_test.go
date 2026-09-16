package files

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func passMiddleware(c *gin.Context) {
	c.Next()
}

func passPermission(...string) gin.HandlerFunc {
	return passMiddleware
}

func testDependencies() Dependencies {
	return Dependencies{
		RequireAuth:       passMiddleware,
		RequireAdmin:      passMiddleware,
		RequirePermission: passPermission,
	}
}

func TestRegisterExistingUploadRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine.Group("/api"), testDependencies())

	wanted := map[string]bool{
		"GET /api/uploads/*path": false,
		"POST /api/users/avatar": false,
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

func TestRegisterManagedFileRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine.Group("/api"), testDependencies())

	wanted := map[string]bool{
		"DELETE /api/files/:id":      false,
		"GET /api/files/:id":         false,
		"GET /api/files/:id/content": false,
		"POST /api/files":            false,
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

func TestManagedFileRoutesReturn401WithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	dependencies := testDependencies()
	dependencies.RequireAuth = func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized})
	}
	Register(engine.Group("/api"), dependencies)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/files", nil)

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
}

func TestAPIUploadURLEscapesObjectKeySegments(t *testing.T) {
	actual := apiUploadURL("avatars/20260725/avatar name.png")
	want := "/api/uploads/avatars/20260725/avatar%20name.png"
	if actual != want {
		t.Fatalf("apiUploadURL() = %q, want %q", actual, want)
	}
}
