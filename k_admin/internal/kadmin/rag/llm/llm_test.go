package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func envValues(values map[string]string) func(string, string) string {
	return func(key, fallback string) string {
		if value, ok := values[key]; ok {
			return value
		}
		return fallback
	}
}

func TestConfigFromEnvDisabledByDefault(t *testing.T) {
	config, err := ConfigFromEnv(envValues(nil))
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if config.Enabled {
		t.Fatal("RAG models must be disabled by default")
	}

	runtime, err := New(context.Background(), config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer runtime.Close()
	if runtime.Enabled() {
		t.Fatal("disabled runtime reported enabled")
	}
	if runtime.ChatModel() != nil || runtime.Embedder() != nil || runtime.VisionModel() != nil {
		t.Fatal("disabled runtime must not expose models")
	}
	if status := runtime.Status(); status.Enabled || status.Framework != "eino" {
		t.Fatalf("unexpected disabled status: %+v", status)
	}
}

func TestConfigFromEnvValidation(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{
			name:   "enabled without providers",
			values: map[string]string{"KADMIN_RAG_ENABLED": "true"},
			want:   "至少需要配置",
		},
		{
			name: "provider without model",
			values: map[string]string{
				"KADMIN_RAG_ENABLED":       "true",
				"KADMIN_RAG_CHAT_BASE_URL": "http://llm-gateway:8000/v1",
			},
			want: "KADMIN_RAG_CHAT_MODEL",
		},
		{
			name: "invalid enabled value",
			values: map[string]string{
				"KADMIN_RAG_ENABLED": "maybe",
			},
			want: "KADMIN_RAG_ENABLED",
		},
		{
			name: "invalid timeout",
			values: map[string]string{
				"KADMIN_RAG_ENABLED":      "true",
				"KADMIN_RAG_CHAT_MODEL":   "chat-model",
				"KADMIN_RAG_CHAT_TIMEOUT": "soon",
			},
			want: "KADMIN_RAG_CHAT_TIMEOUT",
		},
		{
			name: "embedding dimension exceeds pgvector HNSW limit",
			values: map[string]string{
				"KADMIN_RAG_ENABLED":         "true",
				"KADMIN_RAG_EMBEDDING_MODEL": "bge-m3",
				"KADMIN_RAG_EMBEDDING_DIM":   "2001",
			},
			want: "KADMIN_RAG_EMBEDDING_DIM",
		},
		{
			name: "invalid base url",
			values: map[string]string{
				"KADMIN_RAG_ENABLED":       "true",
				"KADMIN_RAG_CHAT_MODEL":    "chat-model",
				"KADMIN_RAG_CHAT_BASE_URL": "llm-gateway:8000",
			},
			want: "KADMIN_RAG_CHAT_BASE_URL",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ConfigFromEnv(envValues(test.values))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %q does not contain %q", err, test.want)
			}
		})
	}
}

func TestConfigFromEnvVisionInheritsChatGateway(t *testing.T) {
	config, err := ConfigFromEnv(envValues(map[string]string{
		"KADMIN_RAG_ENABLED":         "true",
		"KADMIN_RAG_CHAT_BASE_URL":   "http://llm-gateway:8000/v1",
		"KADMIN_RAG_CHAT_API_KEY":    "chat-key",
		"KADMIN_RAG_CHAT_MODEL":      "chat-model",
		"KADMIN_RAG_VISION_MODEL":    "vision-model",
		"KADMIN_RAG_EMBEDDING_MODEL": "bge-m3",
		"KADMIN_RAG_EMBEDDING_DIM":   "1024",
	}))
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if config.Vision.BaseURL != config.Chat.BaseURL {
		t.Fatalf("vision base url = %q, want %q", config.Vision.BaseURL, config.Chat.BaseURL)
	}
	if config.Vision.APIKey != config.Chat.APIKey {
		t.Fatal("vision api key did not inherit chat key")
	}
}

func TestStatusRedactsCredentials(t *testing.T) {
	runtime, err := New(context.Background(), Config{
		Enabled: true,
		Chat: ProviderConfig{
			BaseURL: "https://user:pass@llm.example.com/v1?token=query-secret",
			APIKey:  "api-secret",
			Model:   "chat-model",
			Timeout: DefaultChatTimeout,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer runtime.Close()

	status := runtime.Status()
	if !status.Enabled || !status.Chat.Configured {
		t.Fatalf("unexpected status: %+v", status)
	}
	if !status.Chat.APIKeySet {
		t.Fatal("apiKeySet must be true when a key is configured")
	}
	if status.Chat.BaseURL != "https://llm.example.com/v1" {
		t.Fatalf("base url was not sanitized: %q", status.Chat.BaseURL)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	for _, secret := range []string{"api-secret", "pass", "query-secret"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("status leaked %q: %s", secret, encoded)
		}
	}
}

func TestRuntimeChatAndEmbedding(t *testing.T) {
	var mu sync.Mutex
	var chatAuthorization, chatBody string
	var embedAuthorization, embedBody string

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		switch request.URL.Path {
		case "/chat/completions":
			mu.Lock()
			chatAuthorization = request.Header.Get("Authorization")
			chatBody = string(body)
			mu.Unlock()
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{
				"id":"chatcmpl-test","object":"chat.completion","created":1700000000,
				"model":"chat-model",
				"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
			}`))
		case "/embeddings":
			mu.Lock()
			embedAuthorization = request.Header.Get("Authorization")
			embedBody = string(body)
			mu.Unlock()
			writer.Header().Set("Content-Type", "application/json")
			// Deliberately reversed: the embedder must restore input order.
			_, _ = writer.Write([]byte(`{
				"object":"list","model":"embed-model",
				"data":[
					{"object":"embedding","index":1,"embedding":[0.3,0.4]},
					{"object":"embedding","index":0,"embedding":[0.1,0.2]}
				]
			}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	runtime, err := New(context.Background(), Config{
		Enabled: true,
		Chat: ProviderConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "chat-model",
			Timeout: DefaultChatTimeout,
		},
		Embedding: EmbeddingConfig{
			ProviderConfig: ProviderConfig{
				BaseURL: server.URL,
				APIKey:  "test-key",
				Model:   "embed-model",
				Timeout: DefaultEmbeddingTimeout,
			},
			Dimensions: 2,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer runtime.Close()

	message, err := runtime.ChatModel().Generate(context.Background(), []*schema.Message{
		schema.SystemMessage("be terse"),
		schema.UserMessage("ping"),
	})
	if err != nil {
		t.Fatalf("chat Generate: %v", err)
	}
	if message.Content != "pong" {
		t.Fatalf("chat content = %q, want pong", message.Content)
	}

	vectors, err := runtime.Embedder().EmbedStrings(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("EmbedStrings: %v", err)
	}
	if len(vectors) != 2 {
		t.Fatalf("got %d vectors, want 2", len(vectors))
	}
	if len(vectors[0]) != 2 || vectors[0][0] != 0.1 || vectors[0][1] != 0.2 {
		t.Fatalf("first vector = %v, want [0.1 0.2]", vectors[0])
	}
	if len(vectors[1]) != 2 || vectors[1][0] != 0.3 || vectors[1][1] != 0.4 {
		t.Fatalf("second vector = %v, want [0.3 0.4]", vectors[1])
	}

	mu.Lock()
	defer mu.Unlock()
	if chatAuthorization != "Bearer test-key" || embedAuthorization != "Bearer test-key" {
		t.Fatalf("authorization headers = %q / %q", chatAuthorization, embedAuthorization)
	}
	for _, body := range []string{chatBody, embedBody} {
		if !strings.Contains(body, `"model"`) {
			t.Fatalf("request body missing model: %s", body)
		}
	}
	if !strings.Contains(chatBody, "ping") || !strings.Contains(embedBody, "first") {
		t.Fatalf("request bodies missing input text: %s / %s", chatBody, embedBody)
	}
}

func TestEmbeddingDimensionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2]}]}`))
	}))
	defer server.Close()

	runtime, err := New(context.Background(), Config{
		Enabled: true,
		Embedding: EmbeddingConfig{
			ProviderConfig: ProviderConfig{
				BaseURL: server.URL,
				Model:   "embed-model",
				Timeout: DefaultEmbeddingTimeout,
			},
			Dimensions: 3,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer runtime.Close()

	_, err = runtime.Embedder().EmbedStrings(context.Background(), []string{"text"})
	if err == nil || !strings.Contains(err.Error(), "维度不匹配") {
		t.Fatalf("expected dimension error, got %v", err)
	}
}
