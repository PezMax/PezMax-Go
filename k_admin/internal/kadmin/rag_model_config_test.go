package kadmin

import (
	"os"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/rag/llm"
)

func TestRAGModelConfigRoundTripIsEncrypted(t *testing.T) {
	t.Setenv(ragModelConfigKeyEnv, strings.Repeat("ab", 32))
	path := t.TempDir() + string(os.PathSeparator) + "rag_models.enc"
	t.Setenv(ragModelConfigPathEnv, path)
	want := llm.Config{
		Enabled: true,
		Chat: llm.ProviderConfig{
			BaseURL: "https://llm.example/v1",
			APIKey:  "super-secret-key",
			Model:   "chat-model",
			Timeout: llm.DefaultChatTimeout,
		},
	}
	if err := saveRAGModelConfig(want); err != nil {
		t.Fatalf("saveRAGModelConfig: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read encrypted document: %v", err)
	}
	if strings.Contains(string(content), want.Chat.APIKey) || strings.Contains(string(content), want.Chat.Model) {
		t.Fatalf("encrypted document contains plaintext credentials or model: %s", content)
	}
	got, err := loadRAGModelConfig()
	if err != nil {
		t.Fatalf("loadRAGModelConfig: %v", err)
	}
	if got.Chat != want.Chat || !got.Enabled {
		t.Fatalf("round trip config = %#v, want %#v", got, want)
	}

	t.Setenv(ragModelConfigKeyEnv, strings.Repeat("cd", 32))
	if _, err := loadRAGModelConfig(); err == nil || !strings.Contains(err.Error(), "解密失败") {
		t.Fatalf("wrong key error = %v, want decryption failure", err)
	}
}

func TestRAGModelConfigRequiresEncryptionKey(t *testing.T) {
	t.Setenv(ragModelConfigKeyEnv, "")
	if _, err := modelConfigEncryptionKey(); err == nil {
		t.Fatal("missing encryption key unexpectedly accepted")
	}
}
