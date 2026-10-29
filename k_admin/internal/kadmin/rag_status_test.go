package kadmin

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/rag/llm"
	"github.com/gin-gonic/gin"
)

func TestRAGStatusHandlerReturnsRedactedConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	models, err := llm.New(context.Background(), llm.Config{
		Enabled: true,
		Chat: llm.ProviderConfig{
			BaseURL: "https://user:pass@llm.example.com/v1",
			APIKey:  "top-secret",
			Model:   "chat-model",
			Timeout: llm.DefaultChatTimeout,
		},
	})
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	defer models.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/api/rag/status", nil)

	store := &Store{ragModels: models}
	store.ragStatus(context)

	var envelope struct {
		Code int        `json:"code"`
		Data llm.Status `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != 0 || !envelope.Data.Enabled || !envelope.Data.Chat.Configured {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
	if envelope.Data.Chat.BaseURL != "https://llm.example.com/v1" {
		t.Fatalf("base url not sanitized: %q", envelope.Data.Chat.BaseURL)
	}
	for _, secret := range []string{"top-secret", "pass"} {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Fatalf("response leaked %q: %s", secret, recorder.Body.String())
		}
	}
}

func TestRAGStatusHandlerWithoutRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/api/rag/status", nil)

	store := &Store{}
	store.ragStatus(context)

	var envelope struct {
		Code int        `json:"code"`
		Data llm.Status `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != 0 || envelope.Data.Enabled || envelope.Data.Framework != "eino" {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestRAGStatusRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerRAGRoutes(router.Group("/api"), &Store{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/rag/status", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != 401 {
		t.Fatalf("unauthenticated status request returned %d, want 401", recorder.Code)
	}
}
