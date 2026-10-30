package kadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/rag/llm"
	"github.com/gin-gonic/gin"
)

func TestRAGModelConfigViewRedactsSecrets(t *testing.T) {
	// The handler prefers the encrypted document; point it at a temporary,
	// absent path so the test exercises the bootstrap environment fallback.
	t.Setenv(ragModelConfigPathEnv, t.TempDir()+string(os.PathSeparator)+"rag_models.enc")
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

func TestRAGModelConfigUpdateSavesMultipleModelsAndRedactsKeys(t *testing.T) {
	t.Setenv(ragModelConfigKeyEnv, strings.Repeat("ab", 32))
	t.Setenv(ragModelConfigPathEnv, t.TempDir()+string(os.PathSeparator)+"rag_models.enc")
	base, err := llm.New(context.Background(), llm.Config{})
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	store := &Store{ragModels: llm.NewManager(base)}
	payload := `{"enabled":true,"dimensions":1024,"models":[` +
		`{"kind":"chat","name":"主模型","model":"chat-primary","baseUrl":"https://llm.example/v1","apiKey":"chat-secret","timeoutSeconds":60,"active":true},` +
		`{"kind":"chat","name":"备用","model":"chat-backup","baseUrl":"https://backup.example/v1","timeoutSeconds":45},` +
		`{"kind":"embedding","model":"embed-model","baseUrl":"https://llm.example/v1","apiKey":"embed-secret","active":true}]}`
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodPut, "/api/rag/models", strings.NewReader(payload))
	store.updateRAGModelConfig(ginContext)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, secret := range []string{"chat-secret", "embed-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("update response leaked %q: %s", secret, body)
		}
	}
	if strings.Count(body, `"kind":"chat"`) != 2 || strings.Count(body, `"active":true`) != 2 {
		t.Fatalf("update response lost stored models or active flags: %s", body)
	}
	stored, persisted, err := loadStoredRAGModelConfig()
	if err != nil || !persisted {
		t.Fatalf("persisted document missing: %v (persisted %v)", err, persisted)
	}
	if len(stored.Models) != 3 {
		t.Fatalf("stored models = %d, want 3", len(stored.Models))
	}
	config := store.ragModels.Config()
	if config.Chat.Model != "chat-primary" || config.Chat.APIKey != "chat-secret" {
		t.Fatalf("runtime chat config = %#v", config.Chat)
	}
	if config.Embedding.Model != "embed-model" || config.Embedding.Dimensions != 1024 {
		t.Fatalf("runtime embedding config = %#v", config.Embedding)
	}
	keys := map[string]string{}
	for _, entry := range stored.Models {
		keys[entry.ID] = entry.APIKey
	}
	for _, entry := range stored.Models {
		if entry.Model == "chat-backup" && entry.APIKey != "" {
			t.Fatalf("backup entry unexpectedly has a key: %#v", entry)
		}
	}
	if len(keys) != 3 {
		t.Fatalf("model ids not unique: %#v", keys)
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
