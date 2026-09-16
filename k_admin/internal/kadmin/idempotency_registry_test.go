package kadmin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterIdempotentCreateRouteMarksOnlyCreatePrefix(t *testing.T) {
	RegisterIdempotentCreateRoute("test-products")
	if !requiresIdempotency(http.MethodPost, "/api/test-products") {
		t.Fatal("registered create prefix must require an idempotency key")
	}
	for _, tt := range []struct{ method, path string }{
		{http.MethodPost, "/api/test-products/1"},
		{http.MethodPut, "/api/test-products"},
		{http.MethodGet, "/api/test-products"},
		{http.MethodPost, "/api/test-unregistered"},
	} {
		if requiresIdempotency(tt.method, tt.path) {
			t.Fatalf("%s %s must not require an idempotency key", tt.method, tt.path)
		}
	}
	if !requiresIdempotency(http.MethodPost, "/api/users") {
		t.Fatal("built-in whitelist entry was lost")
	}
	if !requiresIdempotency(http.MethodPost, "/api/jobs/8/run") {
		t.Fatal("built-in nested whitelist entry was lost")
	}
}

func TestIdempotencyMiddlewareHonorsRegisteredCreateRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	RegisterIdempotentCreateRoute("test-gadgets")
	store := &Store{auth: &authService{secret: []byte("test-secret")}}
	engine := gin.New()
	engine.Use(store.idempotencyMiddleware())
	engine.POST("/api/test-gadgets", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	engine.POST("/api/test-unlisted", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	registered := httptest.NewRequest(http.MethodPost, "/api/test-gadgets", strings.NewReader(`{}`))
	registered.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, registered)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("registered create route status = %d, want 401; body=%s", response.Code, response.Body.String())
	}

	unlisted := httptest.NewRequest(http.MethodPost, "/api/test-unlisted", strings.NewReader(`{}`))
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, unlisted)
	if response.Code != http.StatusNoContent {
		t.Fatalf("unlisted create route status = %d, want 204; body=%s", response.Code, response.Body.String())
	}
}

func TestRegisterIdempotentCreateRouteRejectsInvalidPrefix(t *testing.T) {
	for _, prefix := range []string{"", "/", "a/b", " a/b "} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic for prefix %q", prefix)
				}
			}()
			RegisterIdempotentCreateRoute(prefix)
		}()
	}
}
