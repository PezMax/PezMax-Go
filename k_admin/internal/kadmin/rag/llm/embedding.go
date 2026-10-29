package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
)

var _ embedding.Embedder = (*openAIEmbedder)(nil)

// openAIEmbedder implements Eino's embedding.Embedder contract for any
// OpenAI-compatible /embeddings endpoint. It is kept local instead of using
// eino-ext's embedding module because that module currently requires Go 1.23,
// while the backend build image remains Go 1.22.
type openAIEmbedder struct {
	endpoint   string
	apiKey     string
	model      string
	dimensions int
	client     *http.Client
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data  []embeddingItem `json:"data"`
	Error *apiError       `json:"error,omitempty"`
}

type embeddingItem struct {
	Embedding []float64 `json:"embedding"`
	Index     *int      `json:"index"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    any    `json:"code,omitempty"`
}

func newOpenAIEmbedder(config EmbeddingConfig) *openAIEmbedder {
	return &openAIEmbedder{
		endpoint:   strings.TrimRight(config.BaseURL, "/") + "/embeddings",
		apiKey:     config.APIKey,
		model:      config.Model,
		dimensions: config.Dimensions,
		client:     &http.Client{Timeout: config.Timeout},
	}
}

func (e *openAIEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) (result [][]float64, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, e.GetType(), components.ComponentOfEmbedding)
	options := embedding.GetCommonOptions(nil, opts...)
	modelName := e.model
	if options.Model != nil {
		modelName = *options.Model
	}
	callbackConfig := &embedding.Config{Model: modelName}
	ctx = callbacks.OnStart(ctx, &embedding.CallbackInput{Texts: texts, Config: callbackConfig})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
			return
		}
		callbacks.OnEnd(ctx, &embedding.CallbackOutput{Embeddings: result, Config: callbackConfig})
	}()
	if len(texts) == 0 {
		return [][]float64{}, nil
	}

	body, err := json.Marshal(embeddingRequest{Model: modelName, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("编码 embedding 请求失败: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 embedding 请求失败: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	response, err := e.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("embedding 请求失败: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return nil, fmt.Errorf("读取 embedding 响应失败: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("embedding 服务返回 HTTP %d", response.StatusCode)
	}

	var decoded embeddingResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return nil, fmt.Errorf("解析 embedding 响应失败: %w", err)
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("embedding 服务返回 API 错误")
	}
	if len(decoded.Data) != len(texts) {
		return nil, fmt.Errorf("embedding 返回数量错误：请求 %d 条，返回 %d 条", len(texts), len(decoded.Data))
	}

	indexed := false
	for _, item := range decoded.Data {
		if item.Index != nil {
			indexed = true
			break
		}
	}
	if indexed {
		seen := make([]bool, len(decoded.Data))
		for _, item := range decoded.Data {
			if item.Index == nil || *item.Index < 0 || *item.Index >= len(seen) || seen[*item.Index] {
				return nil, fmt.Errorf("embedding 返回的 index 无效")
			}
			seen[*item.Index] = true
		}
		sort.Slice(decoded.Data, func(i, j int) bool {
			return *decoded.Data[i].Index < *decoded.Data[j].Index
		})
	}
	result = make([][]float64, len(decoded.Data))
	for i, item := range decoded.Data {
		if len(item.Embedding) != e.dimensions {
			return nil, fmt.Errorf("embedding 维度不匹配：期望 %d，返回 %d（索引 %d）", e.dimensions, len(item.Embedding), i)
		}
		result[i] = item.Embedding
	}
	return result, nil
}
func (e *openAIEmbedder) GetType() string {
	return "OpenAICompatible"
}

func (e *openAIEmbedder) IsCallbacksEnabled() bool {
	return true
}

func (e *openAIEmbedder) close() {
	if e == nil || e.client == nil {
		return
	}
	e.client.CloseIdleConnections()
}

func compactErrorBody(body []byte) string {
	message := strings.Join(strings.Fields(string(body)), " ")
	if len(message) > 4096 {
		return message[:4096] + "..."
	}
	return message
}
