// Package anthropic provides an [provider.LLMProvider] adapter for the
// Anthropic Messages API (claude-3 model family).
package anthropic

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
	defaultBaseURL   = "https://api.anthropic.com"
	defaultModel     = "claude-3-5-sonnet-20241022"
	defaultTimeout   = 120 * time.Second
	anthropicVersion = "2023-06-01"
	maxRetryAttempts = 3
)

// Config holds all tunable parameters for the Anthropic adapter.
type Config struct {
	// APIKey is the Anthropic API key (required).
	APIKey string
	// BaseURL overrides the default API endpoint. Useful for proxies and tests.
	// Defaults to "https://api.anthropic.com".
	BaseURL string
	// DefaultModel is the model ID used when the request does not specify one.
	// Defaults to "claude-3-5-sonnet-20241022".
	DefaultModel string
	// Timeout is the per-request HTTP timeout. Defaults to 120 s.
	Timeout time.Duration
}

func (c Config) withDefaults() Config {
	if c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}
	if c.DefaultModel == "" {
		c.DefaultModel = defaultModel
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	return c
}

// Validate returns an error if the config is missing required fields.
func (c Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("anthropic: APIKey must not be empty")
	}
	return nil
}

// AnthropicAdapter implements [provider.LLMProvider] for the Anthropic API.
// It is safe for concurrent use.
type AnthropicAdapter struct {
	cfg    Config
	client *http.Client
}

// New creates a new AnthropicAdapter and validates the configuration.
func New(cfg Config) (*AnthropicAdapter, error) {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &AnthropicAdapter{
		cfg:    cfg,
		client: shared.BuildHTTPClient(cfg.Timeout),
	}, nil
}

// ID returns the stable provider identifier "anthropic".
func (a *AnthropicAdapter) ID() string { return "anthropic" }

// Capabilities advertises what Claude-3.5 can do.
func (a *AnthropicAdapter) Capabilities() provider.Capability {
	return provider.Capability{
		SupportedTasks:    []provider.TaskType{provider.TaskChat, provider.TaskReason, provider.TaskExtract},
		MaxContextTokens:  200_000,
		MaxOutputTokens:   8_192,
		SupportsStreaming:  true,
		SupportsEmbedding: false,
		ProviderID:        "anthropic",
		ModelID:           a.cfg.DefaultModel,
	}
}

// Chat sends a non-streaming request to POST /v1/messages.
func (a *AnthropicAdapter) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	start := time.Now()

	payload, err := buildChatPayload(req, a.cfg.DefaultModel, false)
	if err != nil {
		return nil, fmt.Errorf("anthropic: serialising request: %w", err)
	}

	var respBody []byte
	var statusCode int

	err = shared.WithRetry(ctx, maxRetryAttempts, func() error {
		var doErr error
		respBody, statusCode, doErr = shared.DoRequest(ctx, a.client, http.MethodPost,
			a.cfg.BaseURL+"/v1/messages",
			a.headers(), payload)
		if doErr != nil {
			return &shared.RetryableError{Err: doErr}
		}
		return shared.MapHTTPStatus(statusCode, respBody, "anthropic")
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

// ChatStream sends a streaming request to POST /v1/messages with stream:true.
// It returns a channel that emits [provider.StreamChunk] values and closes
// after the final chunk or on error.
func (a *AnthropicAdapter) ChatStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	payload, err := buildChatPayload(req, a.cfg.DefaultModel, true)
	if err != nil {
		return nil, fmt.Errorf("anthropic: serialising stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cfg.BaseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("anthropic: building stream request: %w", err)
	}
	for k, v := range a.headers() {
		httpReq.Header.Set(k, v)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &provider.ProviderError{
			ProviderID: "anthropic",
			Code:       "transport_error",
			Message:    err.Error(),
			Underlying: provider.ErrProviderUnavailable,
		}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close() // #nosec G104 -- response already read into body above; the error from discarding a now-unused body is not actionable //nolint:errcheck
		return nil, shared.MapHTTPStatus(resp.StatusCode, body, "anthropic")
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
				ch <- provider.StreamChunk{FinishReason: "end_turn", Done: true}
				return
			}
			chunk, stop := mapAnthropicStreamEvent(data)
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

// Embed is not supported by Anthropic. It returns ErrProviderUnavailable.
func (a *AnthropicAdapter) Embed(_ context.Context, _ provider.EmbedRequest) (*provider.EmbedResponse, error) {
	return nil, &provider.ProviderError{
		ProviderID: "anthropic",
		Code:       "unsupported",
		Message:    "Anthropic does not provide a native embedding endpoint",
		Underlying: provider.ErrProviderUnavailable,
	}
}

// HealthCheck calls GET /v1/models to verify connectivity and API key validity.
func (a *AnthropicAdapter) HealthCheck(ctx context.Context) error {
	body, status, err := shared.DoRequest(ctx, a.client, http.MethodGet,
		a.cfg.BaseURL+"/v1/models",
		a.headers(), nil)
	if err != nil {
		return &provider.ProviderError{
			ProviderID: "anthropic",
			Code:       "transport_error",
			Message:    err.Error(),
			Underlying: provider.ErrProviderUnavailable,
		}
	}
	return shared.MapHTTPStatus(status, body, "anthropic")
}

// --- internal helpers ---

func (a *AnthropicAdapter) headers() map[string]string {
	return map[string]string{
		"x-api-key":         a.cfg.APIKey,
		"anthropic-version": anthropicVersion,
		"Content-Type":      "application/json",
	}
}

func buildChatPayload(req provider.ChatRequest, model string, stream bool) ([]byte, error) {
	systemMsg, msgs := extractSystemMessage(req.Messages)
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	body := map[string]any{
		"model":      model,
		"messages":   mapMessages(msgs),
		"max_tokens": maxTokens,
		"stream":     stream,
	}
	if req.Temperature != 0 {
		body["temperature"] = req.Temperature
	}
	if systemMsg != "" {
		body["system"] = systemMsg
	}
	if len(req.StopSequences) > 0 {
		body["stop_sequences"] = req.StopSequences
	}
	return json.Marshal(body)
}
