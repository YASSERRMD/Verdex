// Package openai provides an [provider.LLMProvider] adapter for the OpenAI
// Chat Completions and Embeddings APIs.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/YASSERRMD/verdex/packages/adapters/shared"
	"github.com/YASSERRMD/verdex/packages/provider"
)

const (
	defaultBaseURL   = "https://api.openai.com"
	defaultChatModel = "gpt-4o"
	defaultEmbedModel = "text-embedding-3-small"
	defaultTimeout   = 120 * time.Second
	maxRetryAttempts = 3
)

// Config holds all tunable parameters for the OpenAI adapter.
type Config struct {
	// APIKey is the OpenAI API key (required).
	APIKey string
	// BaseURL overrides the default endpoint. Useful for Azure OpenAI and tests.
	// Defaults to "https://api.openai.com".
	BaseURL string
	// ChatModel is the model used for Chat / ChatStream calls.
	// Defaults to "gpt-4o".
	ChatModel string
	// EmbedModel is the model used for Embed calls.
	// Defaults to "text-embedding-3-small".
	EmbedModel string
	// Timeout is the per-request HTTP timeout. Defaults to 120 s.
	Timeout time.Duration
}

func (c Config) withDefaults() Config {
	if c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}
	if c.ChatModel == "" {
		c.ChatModel = defaultChatModel
	}
	if c.EmbedModel == "" {
		c.EmbedModel = defaultEmbedModel
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	return c
}

// Validate returns an error if the config is missing required fields.
func (c Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("openai: APIKey must not be empty")
	}
	return nil
}

// OpenAIAdapter implements [provider.LLMProvider] for the OpenAI API.
// It is safe for concurrent use.
type OpenAIAdapter struct {
	cfg    Config
	client *http.Client
}

// New creates a new OpenAIAdapter and validates the configuration.
func New(cfg Config) (*OpenAIAdapter, error) {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &OpenAIAdapter{
		cfg:    cfg,
		client: shared.BuildHTTPClient(cfg.Timeout),
	}, nil
}

// ID returns the stable provider identifier "openai".
func (a *OpenAIAdapter) ID() string { return "openai" }

// Capabilities advertises GPT-4o and text-embedding-3-small support.
func (a *OpenAIAdapter) Capabilities() provider.Capability {
	return provider.Capability{
		SupportedTasks:    []provider.TaskType{provider.TaskChat, provider.TaskEmbed, provider.TaskReason, provider.TaskExtract},
		MaxContextTokens:  128_000,
		MaxOutputTokens:   4_096,
		SupportsStreaming:  true,
		SupportsEmbedding: true,
		ProviderID:        "openai",
		ModelID:           a.cfg.ChatModel,
	}
}

// Chat sends a non-streaming request to POST /v1/chat/completions.
func (a *OpenAIAdapter) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	start := time.Now()

	payload, err := buildChatPayload(req, a.cfg.ChatModel, false)
	if err != nil {
		return nil, fmt.Errorf("openai: serialising request: %w", err)
	}

	var respBody []byte
	var statusCode int

	err = shared.WithRetry(ctx, maxRetryAttempts, func() error {
		var doErr error
		respBody, statusCode, doErr = shared.DoRequest(ctx, a.client, http.MethodPost,
			a.cfg.BaseURL+"/v1/chat/completions",
			a.headers(), payload)
		if doErr != nil {
			return &shared.RetryableError{Err: doErr}
		}
		return shared.MapHTTPStatus(statusCode, respBody, "openai")
	})
	if err != nil {
		return nil, err
	}

	chatResp, err := mapChatResponse(respBody)
	if err != nil {
		return nil, err
	}
	chatResp.Latency = time.Since(start)
	return chatResp, nil
}

// ChatStream sends a streaming request to POST /v1/chat/completions.
func (a *OpenAIAdapter) ChatStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	payload, err := buildChatPayload(req, a.cfg.ChatModel, true)
	if err != nil {
		return nil, fmt.Errorf("openai: serialising stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cfg.BaseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("openai: building stream request: %w", err)
	}
	for k, v := range a.headers() {
		httpReq.Header.Set(k, v)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &provider.ProviderError{
			ProviderID: "openai",
			Code:       "transport_error",
			Message:    err.Error(),
			Underlying: provider.ErrProviderUnavailable,
		}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close() // #nosec G104 -- response already read into body above; the error from discarding a now-unused body is not actionable //nolint:errcheck
		return nil, shared.MapHTTPStatus(resp.StatusCode, body, "openai")
	}

	ch := make(chan provider.StreamChunk, 8)
	go func() {
		defer close(ch)
		defer resp.Body.Close() //nolint:errcheck
		sr := shared.NewStreamReader(resp.Body)
		for {
			if ctx.Err() != nil {
				ch <- provider.StreamChunk{FinishReason: "cancelled", Done: true}
				return
			}
			data, done, readErr := sr.ReadEvent()
			if readErr != nil {
				ch <- provider.StreamChunk{FinishReason: "error", Done: true}
				return
			}
			if done {
				ch <- provider.StreamChunk{FinishReason: "stop", Done: true}
				return
			}
			// OpenAI signals end-of-stream with the literal string "[DONE]".
			if data == "[DONE]" {
				ch <- provider.StreamChunk{FinishReason: "stop", Done: true}
				return
			}
			chunk, stop := mapOpenAIStreamChunk(data)
			if chunk != nil {
				ch <- *chunk
			}
			if stop {
				return
			}
		}
	}()

	return ch, nil
}

// Embed calls POST /v1/embeddings and returns provider-neutral embeddings.
func (a *OpenAIAdapter) Embed(ctx context.Context, req provider.EmbedRequest) (*provider.EmbedResponse, error) {
	model := req.Model
	if model == "" {
		model = a.cfg.EmbedModel
	}
	if len(req.Texts) == 0 {
		return nil, fmt.Errorf("openai: %w: Texts must not be empty", provider.ErrInvalidRequest)
	}

	payload, err := json.Marshal(map[string]any{
		"input": req.Texts,
		"model": model,
	})
	if err != nil {
		return nil, fmt.Errorf("openai: serialising embed request: %w", err)
	}

	var respBody []byte
	var statusCode int

	err = shared.WithRetry(ctx, maxRetryAttempts, func() error {
		var doErr error
		respBody, statusCode, doErr = shared.DoRequest(ctx, a.client, http.MethodPost,
			a.cfg.BaseURL+"/v1/embeddings",
			a.headers(), payload)
		if doErr != nil {
			return &shared.RetryableError{Err: doErr}
		}
		return shared.MapHTTPStatus(statusCode, respBody, "openai")
	})
	if err != nil {
		return nil, err
	}

	return mapEmbedResponse(respBody)
}

// HealthCheck calls GET /v1/models to verify connectivity and API key validity.
func (a *OpenAIAdapter) HealthCheck(ctx context.Context) error {
	body, status, err := shared.DoRequest(ctx, a.client, http.MethodGet,
		a.cfg.BaseURL+"/v1/models",
		a.headers(), nil)
	if err != nil {
		return &provider.ProviderError{
			ProviderID: "openai",
			Code:       "transport_error",
			Message:    err.Error(),
			Underlying: provider.ErrProviderUnavailable,
		}
	}
	return shared.MapHTTPStatus(status, body, "openai")
}

// --- internal helpers ---

func (a *OpenAIAdapter) headers() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + a.cfg.APIKey,
		"Content-Type":  "application/json",
	}
}

func buildChatPayload(req provider.ChatRequest, model string, stream bool) ([]byte, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	body := map[string]any{
		"model":      model,
		"messages":   mapMessages(req.Messages),
		"max_tokens": maxTokens,
		"stream":     stream,
	}
	if req.Temperature != 0 {
		body["temperature"] = req.Temperature
	}
	if len(req.StopSequences) > 0 {
		body["stop"] = req.StopSequences
	}
	return json.Marshal(body)
}
