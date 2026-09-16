package kadmin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"
)

type generatedSwaggerDocument struct {
	BasePath            string                                `json:"basePath"`
	Paths               map[string]map[string]json.RawMessage `json:"paths"`
	SecurityDefinitions map[string]json.RawMessage            `json:"securityDefinitions"`
}

func TestRegisterSwagger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterSwagger(engine, true)

	redirect := httptest.NewRecorder()
	engine.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/swagger", nil))
	if redirect.Code != http.StatusMovedPermanently || redirect.Header().Get("Location") != swaggerIndexPath {
		t.Fatalf("unexpected swagger redirect: status=%d location=%q", redirect.Code, redirect.Header().Get("Location"))
	}

	index := httptest.NewRecorder()
	engine.ServeHTTP(index, httptest.NewRequest(http.MethodGet, swaggerIndexPath, nil))
	if index.Code != http.StatusOK || !strings.Contains(index.Body.String(), "SwaggerUIBundle") {
		t.Fatalf("swagger index unavailable: status=%d body=%q", index.Code, index.Body.String())
	}

	document := httptest.NewRecorder()
	engine.ServeHTTP(document, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if document.Code != http.StatusOK || !json.Valid(document.Body.Bytes()) {
		t.Fatalf("swagger document unavailable: status=%d body=%q", document.Code, document.Body.String())
	}
}

func TestRegisterSwaggerCanBeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterSwagger(engine, false)

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, swaggerIndexPath, nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("disabled swagger status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSwaggerDocumentsAllKAdminRoutes(t *testing.T) {
	documentJSON, err := swag.ReadDoc()
	if err != nil {
		t.Fatalf("read generated swagger document: %v", err)
	}
	var document generatedSwaggerDocument
	if err := json.Unmarshal([]byte(documentJSON), &document); err != nil {
		t.Fatalf("decode generated swagger document: %v", err)
	}
	if document.BasePath != "/api" {
		t.Fatalf("swagger basePath = %q, want /api", document.BasePath)
	}
	if _, ok := document.SecurityDefinitions["BearerAuth"]; !ok {
		t.Fatal("swagger document does not define BearerAuth")
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	store := &Store{conn: &fakeLogConnection{menuRows: []map[string]interface{}{{"id": int64(99)}}}}
	if err := registerApplicationRoutes(engine.Group("/api"), store); err != nil {
		t.Fatalf("register application routes: %v", err)
	}
	aliases := map[string]string{
		"GET /menu/list":            "/menu/all",
		"GET /system/config/":       "/system/config",
		"GET /system/config/login/": "/system/config/login",
		"GET /user/menu/list":       "/user/menu",
		"PUT /system/config/":       "/system/config",
	}

	for _, route := range engine.Routes() {
		path := strings.TrimPrefix(route.Path, "/api")
		path = strings.ReplaceAll(path, ":id", "{id}")
		path = strings.ReplaceAll(path, "*path", "{path}")
		if alias, ok := aliases[route.Method+" "+path]; ok {
			path = alias
		}
		operations, ok := document.Paths[path]
		if !ok {
			t.Errorf("route %s %s is missing from Swagger", route.Method, route.Path)
			continue
		}
		if _, ok := operations[strings.ToLower(route.Method)]; !ok {
			t.Errorf("method %s for route %s is missing from Swagger", route.Method, route.Path)
		}
	}
}
