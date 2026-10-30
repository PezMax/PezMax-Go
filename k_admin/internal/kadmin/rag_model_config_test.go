package kadmin

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/rag/llm"
)

func TestRAGModelConfigRoundTripIsEncrypted(t *testing.T) {
	t.Setenv(ragModelConfigKeyEnv, strings.Repeat("ab", 32))
	path := t.TempDir() + string(os.PathSeparator) + "rag_models.enc"
	t.Setenv(ragModelConfigPathEnv, path)
	input := modelConfigInput{
		Enabled:    true,
		Dimensions: llm.DefaultEmbeddingDim,
		Models: []modelInput{
			{Kind: "chat", Name: "主审核", Model: "chat-model", BaseURL: "https://llm.example/v1", APIKey: "super-secret-key", TimeoutSeconds: 60, Active: true},
			{Kind: "embedding", Model: "embed-model", BaseURL: "https://llm.example/v1", APIKey: "embed-secret", TimeoutSeconds: 30, Active: true},
		},
	}
	stored, err := input.toStored(storedModelConfig{})
	if err != nil {
		t.Fatalf("toStored: %v", err)
	}
	if err := saveRAGModelConfig(stored); err != nil {
		t.Fatalf("saveRAGModelConfig: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read encrypted document: %v", err)
	}
	for _, plaintext := range []string{"super-secret-key", "embed-secret", "chat-model"} {
		if strings.Contains(string(content), plaintext) {
			t.Fatalf("encrypted document contains plaintext %q: %s", plaintext, content)
		}
	}
	got, persisted, err := loadStoredRAGModelConfig()
	if err != nil {
		t.Fatalf("loadStoredRAGModelConfig: %v", err)
	}
	if !persisted {
		t.Fatal("persisted document reported as missing")
	}
	config := got.toLLMConfig()
	if !config.Enabled || config.Chat.Model != "chat-model" || config.Chat.APIKey != "super-secret-key" {
		t.Fatalf("chat round trip = %#v", config.Chat)
	}
	if config.Embedding.Model != "embed-model" || config.Embedding.Dimensions != llm.DefaultEmbeddingDim {
		t.Fatalf("embedding round trip = %#v", config.Embedding)
	}
	runtimeConfig, err := loadRAGModelConfig()
	if err != nil {
		t.Fatalf("loadRAGModelConfig: %v", err)
	}
	if runtimeConfig.Chat.Model != config.Chat.Model {
		t.Fatalf("runtime config chat = %#v, want %#v", runtimeConfig.Chat, config.Chat)
	}

	t.Setenv(ragModelConfigKeyEnv, strings.Repeat("cd", 32))
	if _, _, err := loadStoredRAGModelConfig(); err == nil || !strings.Contains(err.Error(), "解密失败") {
		t.Fatalf("wrong key error = %v, want decryption failure", err)
	}
}

func TestRAGModelConfigLoadsLegacyDocument(t *testing.T) {
	t.Setenv(ragModelConfigKeyEnv, strings.Repeat("ab", 32))
	t.Setenv(ragModelConfigPathEnv, t.TempDir()+string(os.PathSeparator)+"rag_models.enc")
	legacy := llm.Config{
		Enabled: true,
		Chat: llm.ProviderConfig{
			BaseURL: "https://llm.example/v1",
			APIKey:  "legacy-secret",
			Model:   "legacy-chat",
			Timeout: llm.DefaultChatTimeout,
		},
		Embedding: llm.EmbeddingConfig{Dimensions: llm.DefaultEmbeddingDim},
	}
	plain, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy config: %v", err)
	}
	document, err := encryptRAGModelDocument(plain)
	if err != nil {
		t.Fatalf("encryptRAGModelDocument: %v", err)
	}
	if err := os.WriteFile(ragModelConfigPath(), document, 0o600); err != nil {
		t.Fatalf("write legacy document: %v", err)
	}
	stored, persisted, err := loadStoredRAGModelConfig()
	if err != nil {
		t.Fatalf("loadStoredRAGModelConfig: %v", err)
	}
	if !persisted || len(stored.Models) != 1 {
		t.Fatalf("legacy migration = %#v (persisted %v)", stored, persisted)
	}
	entry := stored.Models[0]
	if entry.Kind != "chat" || entry.Model != "legacy-chat" || entry.APIKey != "legacy-secret" {
		t.Fatalf("legacy entry = %#v", entry)
	}
	if stored.Active["chat"] != entry.ID || entry.TimeoutSeconds != int(llm.DefaultChatTimeout.Seconds()) {
		t.Fatalf("legacy entry not active with default timeout: %#v", stored)
	}
}

func TestRAGModelConfigRequiresEncryptionKey(t *testing.T) {
	t.Setenv(ragModelConfigKeyEnv, "")
	if _, err := modelConfigEncryptionKey(); err == nil {
		t.Fatal("missing encryption key unexpectedly accepted")
	}
}
