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

// storedModelConfigVersion marks the multi-model document layout. Documents
// without a version are the legacy single-model-per-role layout and are
// converted on read.
const storedModelConfigVersion = 2

// ragModelKinds lists the model roles the RAG runtime consumes together with
// the default request timeout applied when an entry omits one.
var ragModelKinds = map[string]time.Duration{
	"chat":      llm.DefaultChatTimeout,
	"embedding": llm.DefaultEmbeddingTimeout,
	"vision":    llm.DefaultChatTimeout,
	"ocr":       llm.DefaultChatTimeout,
	"rerank":    llm.DefaultChatTimeout,
}

type encryptedModelDocument struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// modelEntry is one saved model endpoint. Several entries may share a kind;
// exactly one of them is active per kind and feeds the online runtime.
type modelEntry struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	Model          string `json:"model"`
	BaseURL        string `json:"baseUrl"`
	APIKey         string `json:"apiKey"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type storedModelConfig struct {
	Version    int               `json:"version"`
	Enabled    bool              `json:"enabled"`
	Dimensions int               `json:"dimensions"`
	Models     []modelEntry      `json:"models"`
	Active     map[string]string `json:"active"`
}

type modelInput struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	Model          string `json:"model"`
	BaseURL        string `json:"baseUrl"`
	APIKey         string `json:"apiKey"`
	ClearAPIKey    bool   `json:"clearApiKey"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Active         bool   `json:"active"`
}

type modelConfigInput struct {
	Enabled    bool         `json:"enabled"`
	Dimensions int          `json:"dimensions"`
	Models     []modelInput `json:"models"`
}

type modelView struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	Model          string `json:"model"`
	BaseURL        string `json:"baseUrl"`
	APIKeySet      bool   `json:"apiKeySet"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Active         bool   `json:"active"`
}

type modelConfigView struct {
	Enabled        bool        `json:"enabled"`
	Dimensions     int         `json:"dimensions"`
	Models         []modelView `json:"models"`
	EncryptedStore bool        `json:"encryptedStore"`
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
	stored, persisted, err := loadStoredRAGModelConfig()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !persisted {
		stored = storedFromLLMConfig(s.ragModels.Config())
	}
	view := stored.toView()
	view.EncryptedStore = persisted
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
	current, persisted, err := loadStoredRAGModelConfig()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !persisted {
		current = storedFromLLMConfig(s.ragModels.Config())
	}
	stored, err := input.toStored(current)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	candidate, err := llm.New(c.Request.Context(), stored.toLLMConfig())
	if err != nil {
		fail(c, http.StatusBadRequest, "模型配置无法初始化")
		return
	}
	if err := saveRAGModelConfig(stored); err != nil {
		candidate.Close()
		fail(c, http.StatusInternalServerError, "加密模型配置保存失败；当前在线配置未改变")
		return
	}
	s.ragModels.Publish(candidate)
	view := stored.toView()
	view.EncryptedStore = true
	success(c, view)
}

// toStored merges the submitted model list into the persisted document. Blank
// API keys keep the previously stored key for the same model id; clearApiKey
// removes it. Every kind keeps at most one active entry.
func (input modelConfigInput) toStored(current storedModelConfig) (storedModelConfig, error) {
	stored := storedModelConfig{
		Version:    storedModelConfigVersion,
		Enabled:    input.Enabled,
		Dimensions: input.Dimensions,
		Models:     make([]modelEntry, 0, len(input.Models)),
		Active:     make(map[string]string, len(ragModelKinds)),
	}
	if stored.Dimensions == 0 {
		stored.Dimensions = current.Dimensions
	}
	if stored.Dimensions == 0 {
		stored.Dimensions = llm.DefaultEmbeddingDim
	}
	if stored.Dimensions != llm.DefaultEmbeddingDim {
		return storedModelConfig{}, fmt.Errorf("embedding dimensions 当前必须为 %d（rag_chunk.embedding 固定维度）", llm.DefaultEmbeddingDim)
	}
	keys := make(map[string]string, len(current.Models))
	for _, entry := range current.Models {
		keys[entry.ID] = entry.APIKey
	}
	seen := make(map[string]struct{}, len(input.Models))
	for index, item := range input.Models {
		kind := strings.ToLower(strings.TrimSpace(item.Kind))
		timeoutDefault, ok := ragModelKinds[kind]
		if !ok {
			return storedModelConfig{}, fmt.Errorf("第 %d 个模型的类型 %q 不受支持；可选值：chat、embedding、vision、ocr、rerank", index+1, strings.TrimSpace(item.Kind))
		}
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = newRAGModelID()
		}
		if _, duplicate := seen[id]; duplicate {
			return storedModelConfig{}, fmt.Errorf("模型 id %q 重复", id)
		}
		seen[id] = struct{}{}
		model := strings.TrimSpace(item.Model)
		if model == "" {
			return storedModelConfig{}, fmt.Errorf("第 %d 个模型的模型名称不能为空", index+1)
		}
		baseURL := strings.TrimSpace(item.BaseURL)
		if err := llm.ValidateBaseURL(kind+" baseUrl", baseURL); err != nil {
			return storedModelConfig{}, fmt.Errorf("第 %d 个模型：%w", index+1, err)
		}
		seconds := item.TimeoutSeconds
		if seconds == 0 {
			seconds = int(timeoutDefault / time.Second)
		}
		if seconds < 1 || seconds > 600 {
			return storedModelConfig{}, fmt.Errorf("%s：timeoutSeconds 必须在 1 到 600 秒之间", kind)
		}
		key := keys[id]
		if item.ClearAPIKey {
			key = ""
		} else if trimmed := strings.TrimSpace(item.APIKey); trimmed != "" {
			key = trimmed
		}
		stored.Models = append(stored.Models, modelEntry{
			ID:             id,
			Kind:           kind,
			Name:           strings.TrimSpace(item.Name),
			Model:          model,
			BaseURL:        baseURL,
			APIKey:         key,
			TimeoutSeconds: seconds,
		})
		if !item.Active {
			continue
		}
		if existing, active := stored.Active[kind]; active && existing != id {
			return storedModelConfig{}, fmt.Errorf("%s 分工只能启用一个模型", kind)
		}
		stored.Active[kind] = id
	}
	if stored.Enabled && len(stored.Active) == 0 {
		return storedModelConfig{}, fmt.Errorf("启用 RAG 模型层前需要至少启用一个模型")
	}
	return stored, nil
}

func (stored storedModelConfig) toView() modelConfigView {
	models := make([]modelView, 0, len(stored.Models))
	for _, entry := range stored.Models {
		models = append(models, modelView{
			ID:             entry.ID,
			Kind:           entry.Kind,
			Name:           entry.Name,
			Model:          entry.Model,
			BaseURL:        safeModelBaseURL(entry.BaseURL),
			APIKeySet:      entry.APIKey != "",
			TimeoutSeconds: entry.TimeoutSeconds,
			Active:         stored.Active[entry.Kind] == entry.ID,
		})
	}
	return modelConfigView{Enabled: stored.Enabled, Dimensions: stored.Dimensions, Models: models}
}

// toLLMConfig resolves the active entry per kind into the runtime config.
func (stored storedModelConfig) toLLMConfig() llm.Config {
	config := llm.Config{Enabled: stored.Enabled, Embedding: llm.EmbeddingConfig{Dimensions: stored.Dimensions}}
	for _, entry := range stored.Models {
		if stored.Active[entry.Kind] != entry.ID {
			continue
		}
		provider := llm.ProviderConfig{
			Model:   entry.Model,
			BaseURL: entry.BaseURL,
			APIKey:  entry.APIKey,
			Timeout: time.Duration(entry.TimeoutSeconds) * time.Second,
		}
		switch entry.Kind {
		case "chat":
			config.Chat = provider
		case "embedding":
			config.Embedding.ProviderConfig = provider
		case "vision":
			config.Vision = provider
		case "ocr":
			config.OCR = provider
		case "rerank":
			config.Rerank = provider
		}
	}
	if config.Embedding.Dimensions == 0 {
		config.Embedding.Dimensions = llm.DefaultEmbeddingDim
	}
	return config
}

// storedFromLLMConfig converts a resolved config (environment bootstrap or the
// legacy single-model document) into one entry per configured kind.
func storedFromLLMConfig(config llm.Config) storedModelConfig {
	stored := storedModelConfig{
		Version:    storedModelConfigVersion,
		Enabled:    config.Enabled,
		Dimensions: config.Embedding.Dimensions,
		Active:     make(map[string]string, len(ragModelKinds)),
	}
	if stored.Dimensions == 0 {
		stored.Dimensions = llm.DefaultEmbeddingDim
	}
	add := func(kind string, provider llm.ProviderConfig) {
		if provider.Model == "" {
			return
		}
		id := "env-" + kind
		seconds := int(provider.Timeout / time.Second)
		if seconds <= 0 {
			seconds = int(ragModelKinds[kind] / time.Second)
		}
		stored.Models = append(stored.Models, modelEntry{
			ID:             id,
			Kind:           kind,
			Model:          provider.Model,
			BaseURL:        provider.BaseURL,
			APIKey:         provider.APIKey,
			TimeoutSeconds: seconds,
		})
		stored.Active[kind] = id
	}
	add("chat", config.Chat)
	add("embedding", config.Embedding.ProviderConfig)
	add("vision", config.Vision)
	add("ocr", config.OCR)
	add("rerank", config.Rerank)
	return stored
}

func newRAGModelID() string {
	buffer := make([]byte, 6)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return fmt.Sprintf("model-%d", time.Now().UnixNano())
	}
	return "model-" + hex.EncodeToString(buffer)
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

// loadRAGModelConfig returns the effective runtime configuration: the active
// entries of the encrypted document, or the environment bootstrap when no
// document exists yet.
func loadRAGModelConfig() (llm.Config, error) {
	stored, persisted, err := loadStoredRAGModelConfig()
	if err != nil {
		return llm.Config{}, err
	}
	if persisted {
		return stored.toLLMConfig(), nil
	}
	return llm.ConfigFromEnv(func(key, fallback string) string {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
		return fallback
	})
}

func loadStoredRAGModelConfig() (storedModelConfig, bool, error) {
	content, err := os.ReadFile(ragModelConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return storedModelConfig{}, false, nil
	}
	if err != nil {
		return storedModelConfig{}, false, err
	}
	plain, err := decryptRAGModelDocument(content)
	if err != nil {
		return storedModelConfig{}, false, err
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(plain, &header); err != nil {
		return storedModelConfig{}, false, fmt.Errorf("解密后的模型配置无效")
	}
	switch header.Version {
	case storedModelConfigVersion:
		var stored storedModelConfig
		if err := json.Unmarshal(plain, &stored); err != nil {
			return storedModelConfig{}, false, fmt.Errorf("解密后的模型配置无效")
		}
		for _, entry := range stored.Models {
			if _, known := ragModelKinds[entry.Kind]; !known {
				return storedModelConfig{}, false, fmt.Errorf("模型配置包含未知分工 %q", entry.Kind)
			}
		}
		if err := stored.toLLMConfig().Validate(); err != nil {
			return storedModelConfig{}, false, fmt.Errorf("模型配置校验失败: %w", err)
		}
		if stored.Active == nil {
			stored.Active = map[string]string{}
		}
		return stored, true, nil
	case 0:
		var legacy llm.Config
		if err := json.Unmarshal(plain, &legacy); err != nil {
			return storedModelConfig{}, false, fmt.Errorf("解密后的模型配置无效")
		}
		if err := legacy.Validate(); err != nil {
			return storedModelConfig{}, false, fmt.Errorf("模型配置校验失败: %w", err)
		}
		return storedFromLLMConfig(legacy), true, nil
	default:
		return storedModelConfig{}, false, fmt.Errorf("不支持的模型文档版本 %d", header.Version)
	}
}

func saveRAGModelConfig(stored storedModelConfig) error {
	stored.Version = storedModelConfigVersion
	if stored.Active == nil {
		stored.Active = map[string]string{}
	}
	plain, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	document, err := encryptRAGModelDocument(plain)
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

func decryptRAGModelDocument(content []byte) ([]byte, error) {
	var document encryptedModelDocument
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("模型文档格式无效")
	}
	if document.Version != 1 {
		return nil, fmt.Errorf("不支持的模型文档版本 %d", document.Version)
	}
	key, err := modelConfigEncryptionKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(document.Nonce)
	if err != nil {
		return nil, fmt.Errorf("模型文档 nonce 无效")
	}
	sealed, err := base64.StdEncoding.DecodeString(document.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("模型文档密文无效")
	}
	plain, err := gcm.Open(nil, nonce, sealed, []byte(ragModelConfigAAD))
	if err != nil {
		return nil, fmt.Errorf("模型文档解密失败；请确认加密密钥与该文件匹配")
	}
	return plain, nil
}

func encryptRAGModelDocument(plain []byte) ([]byte, error) {
	key, err := modelConfigEncryptionKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plain, []byte(ragModelConfigAAD))
	return json.Marshal(encryptedModelDocument{Version: 1, Nonce: base64.StdEncoding.EncodeToString(nonce), Ciphertext: base64.StdEncoding.EncodeToString(sealed)})
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
