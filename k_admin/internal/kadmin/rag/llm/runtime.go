package llm

import (
	"context"
	"fmt"
	"net/http"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
)

// Runtime owns Eino model components configured for the backend. It can be
// shared safely between requests; per-call options belong to the caller.
type Runtime struct {
	config    Config
	chat      model.ToolCallingChatModel
	vision    model.ToolCallingChatModel
	embedding embedding.Embedder
	clients   []*http.Client
}

// NewFromEnv parses the RAG model environment and builds the configured Eino
// components. It performs no network calls; provider connectivity is checked
// only when a component is invoked.
func NewFromEnv(ctx context.Context, getenv func(string, string) string) (*Runtime, error) {
	config, err := ConfigFromEnv(getenv)
	if err != nil {
		return nil, err
	}
	return New(ctx, config)
}

// New builds Eino ChatModel / Embedder components. Disabled configuration
// returns an inert runtime so existing backend behavior remains unchanged.
func New(ctx context.Context, config Config) (*Runtime, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	runtime := &Runtime{config: config}
	if !config.Enabled {
		return runtime, nil
	}

	if config.Chat.Model != "" {
		chat, client, err := newChatModel(ctx, config.Chat)
		if err != nil {
			return nil, fmt.Errorf("创建 Eino ChatModel 失败: %w", err)
		}
		runtime.chat = chat
		runtime.clients = append(runtime.clients, client)
	}
	if config.Embedding.Model != "" {
		runtime.embedding = newOpenAIEmbedder(config.Embedding)
	}
	if config.Vision.Model != "" {
		vision, client, err := newChatModel(ctx, config.Vision)
		if err != nil {
			runtime.Close()
			return nil, fmt.Errorf("创建 Eino Vision ChatModel 失败: %w", err)
		}
		runtime.vision = vision
		runtime.clients = append(runtime.clients, client)
	}
	return runtime, nil
}

// Enabled reports whether the runtime was explicitly enabled.
func (r *Runtime) Enabled() bool {
	return r != nil && r.config.Enabled
}
func newChatModel(ctx context.Context, config ProviderConfig) (model.ToolCallingChatModel, *http.Client, error) {
	client := &http.Client{Timeout: config.Timeout}
	chat, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:     config.APIKey,
		BaseURL:    config.BaseURL,
		Model:      config.Model,
		Timeout:    config.Timeout,
		HTTPClient: client,
	})
	if err != nil {
		client.CloseIdleConnections()
		return nil, nil, err
	}
	return chat, client, nil
}

// ChatModel returns nil when chat is not configured or RAG is disabled.
func (r *Runtime) ChatModel() model.ToolCallingChatModel {
	if r == nil {
		return nil
	}
	return r.chat
}

// VisionModel returns nil when a separate vision model is not configured.
func (r *Runtime) VisionModel() model.ToolCallingChatModel {
	if r == nil {
		return nil
	}
	return r.vision
}

// Embedder returns nil when embeddings are not configured or RAG is disabled.
func (r *Runtime) Embedder() embedding.Embedder {
	if r == nil {
		return nil
	}
	return r.embedding
}

// Status returns safe model metadata without credentials.
func (r *Runtime) Status() Status {
	if r == nil {
		return statusFromConfig(Config{})
	}
	return statusFromConfig(r.config)
}

// Close releases idle HTTP connections owned by the model runtime.
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	for _, client := range r.clients {
		client.CloseIdleConnections()
	}
	if embedder, ok := r.embedding.(*openAIEmbedder); ok {
		embedder.close()
	}
}
