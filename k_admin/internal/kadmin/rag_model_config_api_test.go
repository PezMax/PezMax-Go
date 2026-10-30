package kadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/rag/llm"
	"github.com/gin-gonic/gin"
)

func TestRAGModelConfigViewRedactsSecrets(t *testing.T) {
	config := llm.Config{
		Enabled: true,
		Chat: llm.ProviderConfig{
			BaseURL: "https://user:password@llm.example/v1?token=hidden",
			APIKey:  "api-secret",
			Model:   "chat-model",
			Timeout: llm.DefaultChatTimeout,
		},
	}
	models, err := llm.New(context.Background(), config)
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	defer models.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/rag/models", nil)
	store := &Store{ragModels: llm.NewManager(models)}
	store.getRAGModelConfig(context)

	body := recorder.Body.String()
	for _, secret := range []string{"api-secret", "password", "token=hidden"} {
		if strings.Contains(body, secret) {
			t.Fatalf("model config response leaked %q: %s", secret, body)
		}
	}
	if !strings.Contains(body, "https://llm.example/v1") || !strings.Contains(body, `"apiKeySet":true`) {
		t.Fatalf("response missing safe endpoint metadata: %s", body)
	}
}

func TestRAGModelConfigRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerRAGModelConfigRoutes(router.Group("/api"), &Store{})
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, "/api/rag/models", strings.NewReader(`{}`))
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s status = %d, want 401", method, recorder.Code)
		}
	}
}
