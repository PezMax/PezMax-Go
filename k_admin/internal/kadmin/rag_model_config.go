package kadmin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/rag/llm"
	"github.com/gin-gonic/gin"
)

const ragModelConfigPathEnv = "KADMIN_RAG_MODEL_CONFIG_PATH"
const ragModelConfigKeyEnv = "KADMIN_RAG_CONFIG_ENCRYPTION_KEY"
const ragModelConfigAAD = "pezmax-go/rag-model-config/v1"

type encryptedModelDocument struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type modelProviderInput struct {
	Model         string `json:"model"`
	BaseURL       string `json:"baseUrl"`
	APIKey        string `json:"apiKey"`
	ClearAPIKey   bool   `json:"clearApiKey"`
	TimeoutSecond int    `json:"timeoutSeconds"`
}

type modelConfigInput struct {
	Enabled    bool               `json:"enabled"`
	Chat       modelProviderInput `json:"chat"`
	Embedding  modelProviderInput `json:"embedding"`
	Vision     modelProviderInput `json:"vision"`
	OCR        modelProviderInput `json:"ocr"`
	Rerank     modelProviderInput `json:"rerank"`
	Dimensions int                `json:"dimensions"`
}

type modelProviderView struct {
	Model         string `json:"model"`
	BaseURL       string `json:"baseUrl"`
	APIKeySet     bool   `json:"apiKeySet"`
	TimeoutSecond int    `json:"timeoutSeconds"`
}

type modelConfigView struct {
	Enabled        bool              `json:"enabled"`
	Chat           modelProviderView `json:"chat"`
	Embedding      modelProviderView `json:"embedding"`
	Vision         modelProviderView `json:"vision"`
	OCR            modelProviderView `json:"ocr"`
	Rerank         modelProviderView `json:"rerank"`
	Dimensions     int               `json:"dimensions"`
	EncryptedStore bool              `json:"encryptedStore"`
}

func registerRAGModelConfigRoutes(api *gin.RouterGroup, s *Store) {
	group := api.Group("/rag/models", s.requireAuth(), s.requireAdmin())
	group.GET("", s.getRAGModelConfig)
	group.PUT("", s.updateRAGModelConfig)
}

func (s *Store) getRAGModelConfig(c *gin.Context) {
	if s.ragModels == nil {
		fail(c, http.StatusServiceUnavailable, "RAG 模型运行时不可用")
		return
	}
	view := modelConfigToView(s.ragModels.Config())
	view.EncryptedStore = fileExists(ragModelConfigPath())
	success(c, view)
}

func (s *Store) updateRAGModelConfig(c *gin.Context) {
	if s.ragModels == nil {
		fail(c, http.StatusServiceUnavailable, "RAG 模型运行时不可用")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var input modelConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, http.StatusBadRequest, "模型配置格式无效")
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	current := s.ragModels.Config()
	config, err := input.toConfig(current)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	candidate, err := llm.New(c.Request.Context(), config)
	if err != nil {
		fail(c, http.StatusBadRequest, "模型配置无法初始化")
		return
	}
	if err := saveRAGModelConfig(config); err != nil {
		candidate.Close()
		fail(c, http.StatusInternalServerError, "加密模型配置保存失败；当前在线配置未改变")
		return
	}
	s.ragModels.Publish(candidate)
	view := modelConfigToView(config)
	view.EncryptedStore = true
	success(c, view)
}
func (input modelConfigInput) toConfig(current llm.Config) (llm.Config, error) {
	config := llm.Config{Enabled: input.Enabled}
	var err error
	config.Chat, err = input.Chat.toProvider(current.Chat, llm.DefaultChatTimeout)
	if err != nil {
		return llm.Config{}, fmt.Errorf("chat: %w", err)
	}
	config.Embedding.ProviderConfig, err = input.Embedding.toProvider(current.Embedding.ProviderConfig, llm.DefaultEmbeddingTimeout)
	if err != nil {
		return llm.Config{}, fmt.Errorf("embedding: %w", err)
	}
	config.Vision, err = input.Vision.toProvider(current.Vision, llm.DefaultChatTimeout)
	if err != nil {
		return llm.Config{}, fmt.Errorf("vision: %w", err)
	}
	config.OCR, err = input.OCR.toProvider(current.OCR, llm.DefaultChatTimeout)
	if err != nil {
		return llm.Config{}, fmt.Errorf("ocr: %w", err)
	}
	config.Rerank, err = input.Rerank.toProvider(current.Rerank, llm.DefaultChatTimeout)
	if err != nil {
		return llm.Config{}, fmt.Errorf("rerank: %w", err)
	}
	config.Embedding.Dimensions = input.Dimensions
	if config.Embedding.Dimensions == 0 {
		config.Embedding.Dimensions = llm.DefaultEmbeddingDim
	}
	if config.Embedding.Dimensions != llm.DefaultEmbeddingDim {
		return llm.Config{}, fmt.Errorf("embedding dimensions 当前必须为 %d（rag_chunk.embedding 固定维度）", llm.DefaultEmbeddingDim)
	}
	if err := config.Validate(); err != nil {
		return llm.Config{}, err
	}
	return config, nil
}

func (input modelProviderInput) toProvider(current llm.ProviderConfig, timeoutDefault time.Duration) (llm.ProviderConfig, error) {
	model := strings.TrimSpace(input.Model)
	baseURL := strings.TrimSpace(input.BaseURL)
	if model == "" && baseURL == "" && input.APIKey == "" && !input.ClearAPIKey {
		return llm.ProviderConfig{}, nil
	}
	key := strings.TrimSpace(input.APIKey)
	if input.ClearAPIKey {
		key = ""
	} else if key == "" {
		key = current.APIKey
	}
	seconds := input.TimeoutSecond
	if seconds == 0 {
		seconds = int(timeoutDefault.Seconds())
	}
	if seconds < 1 || seconds > 600 {
		return llm.ProviderConfig{}, fmt.Errorf("timeoutSeconds 必须在 1 到 600 秒之间")
	}
	return llm.ProviderConfig{Model: model, BaseURL: baseURL, APIKey: key, Timeout: time.Duration(seconds) * time.Second}, nil
}

func modelConfigToView(config llm.Config) modelConfigView {
	provider := func(value llm.ProviderConfig) modelProviderView {
		return modelProviderView{Model: value.Model, BaseURL: safeModelBaseURL(value.BaseURL), APIKeySet: value.APIKey != "", TimeoutSecond: int(value.Timeout.Seconds())}
	}
	return modelConfigView{Enabled: config.Enabled, Chat: provider(config.Chat), Embedding: provider(config.Embedding.ProviderConfig), Vision: provider(config.Vision), OCR: provider(config.OCR), Rerank: provider(config.Rerank), Dimensions: config.Embedding.Dimensions}
}

func safeModelBaseURL(raw string) string {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil {
		return "[invalid]"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func loadRAGModelConfig() (llm.Config, error) {
	path := ragModelConfigPath()
	content, err := os.ReadFile(path)
	if err == nil {
		var document encryptedModelDocument
		if err := json.Unmarshal(content, &document); err != nil {
			return llm.Config{}, fmt.Errorf("模型文档格式无效")
		}
		if document.Version != 1 {
			return llm.Config{}, fmt.Errorf("不支持的模型文档版本 %d", document.Version)
		}
		key, err := modelConfigEncryptionKey()
		if err != nil {
			return llm.Config{}, err
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return llm.Config{}, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return llm.Config{}, err
		}
		nonce, err := base64.StdEncoding.DecodeString(document.Nonce)
		if err != nil {
			return llm.Config{}, fmt.Errorf("模型文档 nonce 无效")
		}
		sealed, err := base64.StdEncoding.DecodeString(document.Ciphertext)
		if err != nil {
			return llm.Config{}, fmt.Errorf("模型文档密文无效")
		}
		plain, err := gcm.Open(nil, nonce, sealed, []byte(ragModelConfigAAD))
		if err != nil {
			return llm.Config{}, fmt.Errorf("模型文档解密失败；请确认加密密钥与该文件匹配")
		}
		var config llm.Config
		if err := json.Unmarshal(plain, &config); err != nil {
			return llm.Config{}, fmt.Errorf("解密后的模型配置无效")
		}
		if err := config.Validate(); err != nil {
			return llm.Config{}, fmt.Errorf("模型配置校验失败: %w", err)
		}
		return config, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return llm.Config{}, err
	}
	return llm.ConfigFromEnv(func(key, fallback string) string {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
		return fallback
	})
}

func saveRAGModelConfig(config llm.Config) error {
	key, err := modelConfigEncryptionKey()
	if err != nil {
		return err
	}
	plain, err := json.Marshal(config)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := gcm.Seal(nil, nonce, plain, []byte(ragModelConfigAAD))
	document, err := json.Marshal(encryptedModelDocument{Version: 1, Nonce: base64.StdEncoding.EncodeToString(nonce), Ciphertext: base64.StdEncoding.EncodeToString(sealed)})
	if err != nil {
		return err
	}
	path := ragModelConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".rag-model-config-*")
	if err != nil {
		return err
	}
	tempPath := temporary.Name()
	defer os.Remove(tempPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(document); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func modelConfigEncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(ragModelConfigKeyEnv))
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s 必须设置为 64 位十六进制编码的 32 字节密钥", ragModelConfigKeyEnv)
	}
	return key, nil
}

func ragModelConfigPath() string {
	configured := strings.TrimSpace(os.Getenv(ragModelConfigPathEnv))
	if configured == "" {
		configured = filepath.Join("data", "rag_models.enc")
	}
	return resolveSystemConfigPath(configured, systemConfigProjectRoot())
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
