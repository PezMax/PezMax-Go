package kadmin

import (
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/rag/llm"
)

func TestRAGModelConfigStoresMultipleModelsPerKind(t *testing.T) {
	input := modelConfigInput{
		Enabled:    true,
		Dimensions: llm.DefaultEmbeddingDim,
		Models: []modelInput{
			{ID: "chat-primary", Kind: "chat", Name: "主审核", Model: "gpt-4.1-mini", BaseURL: "https://api.example.com/v1", APIKey: "primary-key", TimeoutSeconds: 60, Active: true},
			{ID: "chat-backup", Kind: "chat", Name: "备用", Model: "qwen-max", BaseURL: "https://backup.example.com/v1", TimeoutSeconds: 45},
			{ID: "embed", Kind: "embedding", Model: "text-embedding-3-large", BaseURL: "https://api.example.com/v1", Active: true},
			{ID: "vision", Kind: "vision", Model: "gpt-4.1-mini", BaseURL: "https://api.example.com/v1", TimeoutSeconds: 90},
		},
	}
	stored, err := input.toStored(storedModelConfig{})
	if err != nil {
		t.Fatalf("toStored: %v", err)
	}
	if len(stored.Models) != 4 {
		t.Fatalf("stored models = %d, want 4", len(stored.Models))
	}
	config := stored.toLLMConfig()
	if config.Chat.Model != "gpt-4.1-mini" {
		t.Fatalf("active chat = %q, want gpt-4.1-mini", config.Chat.Model)
	}
	if config.Vision.Model != "" {
		t.Fatalf("inactive vision leaked into runtime config: %#v", config.Vision)
	}
	if config.Embedding.Timeout != llm.DefaultEmbeddingTimeout {
		t.Fatalf("embedding timeout = %v, want default %v", config.Embedding.Timeout, llm.DefaultEmbeddingTimeout)
	}

	view := stored.toView()
	if len(view.Models) != 4 {
		t.Fatalf("view models = %d, want 4", len(view.Models))
	}
	active := map[string]bool{}
	for _, model := range view.Models {
		if model.ID == "chat-primary" && !model.APIKeySet {
			t.Fatalf("view lost the apiKeySet marker: %#v", model)
		}
		if model.ID == "chat-backup" && model.APIKeySet {
			t.Fatalf("view reported a key for a model without one: %#v", model)
		}
		active[model.ID] = model.Active
	}
	if !active["chat-primary"] || active["chat-backup"] || !active["embed"] || active["vision"] {
		t.Fatalf("active flags = %#v", active)
	}
}

func TestRAGModelConfigRejectsTwoActiveModelsOfSameKind(t *testing.T) {
	input := modelConfigInput{
		Enabled:    true,
		Dimensions: llm.DefaultEmbeddingDim,
		Models: []modelInput{
			{ID: "a", Kind: "chat", Model: "gpt-4.1-mini", BaseURL: "https://api.example.com/v1", Active: true},
			{ID: "b", Kind: "chat", Model: "qwen-max", BaseURL: "https://api.example.com/v1", Active: true},
		},
	}
	if _, err := input.toStored(storedModelConfig{}); err == nil || !strings.Contains(err.Error(), "只能启用一个模型") {
		t.Fatalf("two active chat models error = %v", err)
	}
}

func TestRAGModelConfigKeepsStoredKeyUnlessCleared(t *testing.T) {
	current := storedModelConfig{
		Dimensions: llm.DefaultEmbeddingDim,
		Models: []modelEntry{
			{ID: "chat-primary", Kind: "chat", Model: "gpt-4.1-mini", BaseURL: "https://api.example.com/v1", APIKey: "stored-key", TimeoutSeconds: 60},
		},
		Active: map[string]string{"chat": "chat-primary"},
	}
	kept, err := modelConfigInput{
		Dimensions: llm.DefaultEmbeddingDim,
		Models:     []modelInput{{ID: "chat-primary", Kind: "chat", Model: "gpt-4.1-mini", BaseURL: "https://api.example.com/v1"}},
	}.toStored(current)
	if err != nil {
		t.Fatalf("toStored keep: %v", err)
	}
	if kept.Models[0].APIKey != "stored-key" {
		t.Fatalf("blank apiKey dropped the stored key: %#v", kept.Models[0])
	}
	if kept.Enabled {
		t.Fatal("disabled input enabled the runtime")
	}

	cleared, err := modelConfigInput{
		Dimensions: llm.DefaultEmbeddingDim,
		Models:     []modelInput{{ID: "chat-primary", Kind: "chat", Model: "gpt-4.1-mini", BaseURL: "https://api.example.com/v1", ClearAPIKey: true}},
	}.toStored(current)
	if err != nil {
		t.Fatalf("toStored clear: %v", err)
	}
	if cleared.Models[0].APIKey != "" {
		t.Fatalf("clearApiKey kept the stored key: %#v", cleared.Models[0])
	}

	replaced, err := modelConfigInput{
		Dimensions: llm.DefaultEmbeddingDim,
		Models:     []modelInput{{ID: "chat-primary", Kind: "chat", Model: "gpt-4.1-mini", BaseURL: "https://api.example.com/v1", APIKey: "new-key"}},
	}.toStored(current)
	if err != nil {
		t.Fatalf("toStored replace: %v", err)
	}
	if replaced.Models[0].APIKey != "new-key" {
		t.Fatalf("explicit apiKey not stored: %#v", replaced.Models[0])
	}
}

func TestRAGModelConfigInputValidation(t *testing.T) {
	cases := []struct {
		name    string
		input   modelConfigInput
		wantErr string
	}{
		{
			name: "unknown kind",
			input: modelConfigInput{Enabled: true, Models: []modelInput{
				{Kind: "audio", Model: "whisper", BaseURL: "https://api.example.com/v1", Active: true},
			}},
			wantErr: "不受支持",
		},
		{
			name: "missing model name",
			input: modelConfigInput{Models: []modelInput{
				{Kind: "chat", BaseURL: "https://api.example.com/v1"},
			}},
			wantErr: "模型名称不能为空",
		},
		{
			name: "invalid base url",
			input: modelConfigInput{Models: []modelInput{
				{Kind: "chat", Model: "gpt-4.1-mini", BaseURL: "api.example.com"},
			}},
			wantErr: "http://",
		},
		{
			name: "timeout out of range",
			input: modelConfigInput{Models: []modelInput{
				{Kind: "chat", Model: "gpt-4.1-mini", BaseURL: "https://api.example.com/v1", TimeoutSeconds: 900},
			}},
			wantErr: "1 到 600",
		},
		{
			name: "enabled without active model",
			input: modelConfigInput{Enabled: true, Models: []modelInput{
				{Kind: "chat", Model: "gpt-4.1-mini", BaseURL: "https://api.example.com/v1"},
			}},
			wantErr: "至少启用一个模型",
		},
		{
			name:    "unsupported embedding dimensions",
			input:   modelConfigInput{Dimensions: 1536, Models: []modelInput{}},
			wantErr: "1024",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := testCase.input.toStored(storedModelConfig{})
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error = %v, want contains %q", err, testCase.wantErr)
			}
		})
	}
}
