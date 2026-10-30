package llm

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultChatTimeout      = 60 * time.Second
	DefaultEmbeddingTimeout = 30 * time.Second
	DefaultEmbeddingDim     = 1024
	maxEmbeddingDim         = 2000 // pgvector HNSW supports at most 2000 dimensions.
	defaultOpenAIBaseURL    = "https://api.openai.com/v1"
)

// ProviderConfig describes one OpenAI-compatible chat endpoint.
type ProviderConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// EmbeddingConfig describes one OpenAI-compatible embedding endpoint.
type EmbeddingConfig struct {
	ProviderConfig
	Dimensions int
}

// Config is the backend Eino model configuration. The API key is intentionally
// kept out of status responses and logs.
type Config struct {
	Enabled   bool
	Chat      ProviderConfig
	Embedding EmbeddingConfig
	Vision    ProviderConfig
	OCR       ProviderConfig
	Rerank    ProviderConfig
}

// ConfigFromEnv loads the server-side model configuration. Disabled RAG keeps
// the existing backend behavior unchanged and does not require model settings.
func ConfigFromEnv(getenv func(string, string) string) (Config, error) {
	if getenv == nil {
		getenv = func(string, string) string { return "" }
	}

	enabled, err := parseBool(getenv("KADMIN_RAG_ENABLED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("KADMIN_RAG_ENABLED: %w", err)
	}
	if !enabled {
		return Config{}, nil
	}

	chat, err := providerFromEnv(getenv, "KADMIN_RAG_CHAT", DefaultChatTimeout)
	if err != nil {
		return Config{}, err
	}
	embedding, err := embeddingFromEnv(getenv)
	if err != nil {
		return Config{}, err
	}
	vision, err := providerFromEnv(getenv, "KADMIN_RAG_VISION", chat.Timeout)
	if err != nil {
		return Config{}, err
	}
	if vision.Model != "" {
		if vision.BaseURL == defaultOpenAIBaseURL && chat.BaseURL != defaultOpenAIBaseURL {
			vision.BaseURL = chat.BaseURL
		}
		if vision.APIKey == "" {
			vision.APIKey = chat.APIKey
		}
	}

	config := Config{Enabled: true, Chat: chat, Embedding: embedding, Vision: vision}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func providerFromEnv(getenv func(string, string) string, prefix string, defaultTimeout time.Duration) (ProviderConfig, error) {
	model := strings.TrimSpace(getenv(prefix+"_MODEL", ""))
	baseURL := strings.TrimSpace(getenv(prefix+"_BASE_URL", ""))
	apiKey := strings.TrimSpace(getenv(prefix+"_API_KEY", ""))
	timeoutRaw := strings.TrimSpace(getenv(prefix+"_TIMEOUT", ""))
	if model == "" && (baseURL != "" || apiKey != "" || timeoutRaw != "") {
		return ProviderConfig{}, fmt.Errorf("%s_MODEL 必须与其他 %s 配置一起设置", prefix, prefix)
	}
	timeout, err := durationEnv(getenv, prefix+"_TIMEOUT", defaultTimeout)
	if err != nil {
		return ProviderConfig{}, err
	}
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	baseURL = normalizeBaseURL(baseURL)
	if model != "" {
		if err := validateBaseURL(prefix+"_BASE_URL", baseURL); err != nil {
			return ProviderConfig{}, err
		}
	}
	return ProviderConfig{BaseURL: baseURL, APIKey: apiKey, Model: model, Timeout: timeout}, nil
}

func embeddingFromEnv(getenv func(string, string) string) (EmbeddingConfig, error) {
	provider, err := providerFromEnv(getenv, "KADMIN_RAG_EMBEDDING", DefaultEmbeddingTimeout)
	if err != nil {
		return EmbeddingConfig{}, err
	}
	dim, err := positiveIntEnv(getenv, "KADMIN_RAG_EMBEDDING_DIM", DefaultEmbeddingDim)
	if err != nil {
		return EmbeddingConfig{}, err
	}
	if dim > maxEmbeddingDim {
		return EmbeddingConfig{}, fmt.Errorf("KADMIN_RAG_EMBEDDING_DIM 必须不超过 %d（pgvector HNSW 限制）", maxEmbeddingDim)
	}
	if provider.Model != "" && dim != DefaultEmbeddingDim {
		return EmbeddingConfig{}, fmt.Errorf("KADMIN_RAG_EMBEDDING_DIM 当前必须为 %d（rag_chunk.embedding 固定维度）", DefaultEmbeddingDim)
	}
	return EmbeddingConfig{ProviderConfig: provider, Dimensions: dim}, nil
}

func validateBaseURL(key, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s 必须是带 http:// 或 https:// 的完整 URL", key)
	}
	return nil
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Chat.Model == "" && c.Embedding.Model == "" && c.Vision.Model == "" && c.OCR.Model == "" && c.Rerank.Model == "" {
		return fmt.Errorf("至少需要配置一个 RAG 模型")
	}
	providers := [...]struct {
		name     string
		provider ProviderConfig
	}{{"chat", c.Chat}, {"vision", c.Vision}, {"ocr", c.OCR}, {"rerank", c.Rerank}}
	for _, item := range providers {
		if err := validateProvider(item.name, item.provider); err != nil {
			return err
		}
	}
	if c.Embedding.Model != "" {
		if err := validateProvider("embedding", c.Embedding.ProviderConfig); err != nil {
			return err
		}
		if c.Embedding.Dimensions < 1 || c.Embedding.Dimensions > maxEmbeddingDim {
			return fmt.Errorf("embedding dimensions 必须在 1 到 %d 之间", maxEmbeddingDim)
		}
	}
	return nil
}

func validateProvider(name string, provider ProviderConfig) error {
	if provider.Model == "" {
		return nil
	}
	if provider.BaseURL == "" {
		return fmt.Errorf("%s 模型已配置但 BaseURL 为空", name)
	}
	if err := validateBaseURL("KADMIN_RAG_"+strings.ToUpper(name)+"_BASE_URL", provider.BaseURL); err != nil {
		return err
	}
	if provider.Timeout <= 0 {
		return fmt.Errorf("%s 模型超时必须大于 0", name)
	}
	return nil
}

func durationEnv(getenv func(string, string) string, key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(key, ""))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s 必须是正数 duration，例如 60s", key)
	}
	return value, nil
}

func positiveIntEnv(getenv func(string, string) string, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(getenv(key, ""))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s 必须是正整数", key)
	}
	return value, nil
}

func parseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "0", "false", "f", "no", "n", "off":
		return false, nil
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	default:
		return false, fmt.Errorf("必须是 true 或 false")
	}
}

func normalizeBaseURL(raw string) string {
	value := strings.TrimRight(strings.TrimSpace(raw), "/")
	if value == "" {
		return defaultOpenAIBaseURL
	}
	return value
}

// Status is safe to expose to an authenticated administrator: it contains no
// API key, request content, or authorization header.
// Status is safe to expose to an authenticated administrator: it contains no
// API key, request content, or authorization header.
type Status struct {
	Framework string          `json:"framework"`
	Enabled   bool            `json:"enabled"`
	Chat      ProviderStatus  `json:"chat"`
	Embedding EmbeddingStatus `json:"embedding"`
	Vision    ProviderStatus  `json:"vision"`
	OCR       ProviderStatus  `json:"ocr"`
	Rerank    ProviderStatus  `json:"rerank"`
}

type ProviderStatus struct {
	Configured bool   `json:"configured"`
	Model      string `json:"model,omitempty"`
	BaseURL    string `json:"baseUrl,omitempty"`
	Timeout    string `json:"timeout,omitempty"`
	APIKeySet  bool   `json:"apiKeySet"`
}

type EmbeddingStatus struct {
	ProviderStatus
	Dimensions int `json:"dimensions,omitempty"`
}

func providerStatus(provider ProviderConfig) ProviderStatus {
	return ProviderStatus{
		Configured: provider.Model != "",
		Model:      provider.Model,
		BaseURL:    safeBaseURL(provider.BaseURL),
		Timeout:    durationString(provider.Timeout),
		APIKeySet:  provider.APIKey != "",
	}
}

func statusFromConfig(config Config) Status {
	return Status{
		Framework: "eino",
		Enabled:   config.Enabled,
		Chat:      providerStatus(config.Chat),
		Embedding: EmbeddingStatus{
			ProviderStatus: providerStatus(config.Embedding.ProviderConfig),
			Dimensions:     config.Embedding.Dimensions,
		},
		OCR:    providerStatus(config.OCR),
		Rerank: providerStatus(config.Rerank),
	}
}

func safeBaseURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "[invalid]"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func durationString(value time.Duration) string {
	if value <= 0 {
		return ""
	}
	return value.String()
}
