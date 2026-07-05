// Package gemini provides an [provider.LLMProvider] adapter for the Google
// Gemini REST API (v1beta).
package gemini

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
	defaultBaseURL   = "https://generativelanguage.googleapis.com"
	defaultModelID   = "gemini-1.5-pro"
	defaultTimeout   = 120 * time.Second
	maxRetryAttempts = 3
)

// Config holds all tunable parameters for the Gemini adapter.
type Config struct {
	// APIKey is the Google AI Studio API key (required).
	APIKey string
	// BaseURL overrides the default endpoint. Useful for proxies and tests.
	// Defaults to "https://generativelanguage.googleapis.com".
	BaseURL string
	// ModelID is the Gemini model to use.
	// Defaults to "gemini-1.5-pro".
	ModelID string
	// Timeout is the per-request HTTP timeout. Defaults to 120 s.
	Timeout time.Duration
}

func (c Config) withDefaults() Config {
	if c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}
	if c.ModelID == "" {
		c.ModelID = defaultModelID
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	return c
}

// Validate returns an error if the config is missing required fields.
func (c Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("gemini: APIKey must not be empty")
	}
	return nil
}

// GeminiAdapter implements [provider.LLMProvider] for the Gemini API.
// It is safe for concurrent use.
type GeminiAdapter struct {
	cfg    Config
	client *http.Client
}

// New creates a new GeminiAdapter and validates the configuration.
func New(cfg Config) (*GeminiAdapter, error) {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &GeminiAdapter{
		cfg:    cfg,
		client: shared.BuildHTTPClient(cfg.Timeout),
	}, nil
}

// ID returns the stable provider identifier "gemini".
func (a *GeminiAdapter) ID() string { return "gemini" }

// Capabilities advertises Gemini 1.5 Pro support.
func (a *GeminiAdapter) Capabilities() provider.Capability {
	return provider.Capability{
		SupportedTasks:    []provider.TaskType{provider.TaskChat, provider.TaskEmbed, provider.TaskReason, provider.TaskExtract},
		MaxContextTokens:  1_000_000,
		MaxOutputTokens:   8_192,
		SupportsStreaming:  true,
		SupportsEmbedding: true,
		ProviderID:        "gemini",
		ModelID:           a.cfg.ModelID,
	}
}

// Chat sends a non-streaming request to
// POST /v1beta/models/{model}:generateContent.
func (a *GeminiAdapter) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	start := time.Now()

	payload, err := buildGeneratePayload(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: serialising request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		a.cfg.BaseURL, a.cfg.ModelID, a.cfg.APIKey)

	var respBody []byte
	var statusCode int

	err = shared.WithRetry(ctx, maxRetryAttempts, func() error {
		var doErr error
		respBody, statusCode, doErr = shared.DoRequest(ctx, a.client, http.MethodPost, url,
			a.jsonHeaders(), payload)
		if doErr != nil {
			return &shared.RetryableError{Err: doErr}
		}
		return shared.MapHTTPStatus(statusCode, respBody, "gemini")
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

// ChatStream sends a streaming request to
// POST /v1beta/models/{model}:streamGenerateContent.
func (a *GeminiAdapter) ChatStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	payload, err := buildGeneratePayload(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: serialising stream request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s",
		a.cfg.BaseURL, a.cfg.ModelID, a.cfg.APIKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("gemini: building stream request: %w", err)
	}
	for k, v := range a.jsonHeaders() {
		httpReq.Header.Set(k, v)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &provider.ProviderError{
			ProviderID: "gemini",
			Code:       "transport_error",
			Message:    err.Error(),
			Underlying: provider.ErrProviderUnavailable,
		}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close() // #nosec G104 -- response already read into body above; the error from discarding a now-unused body is not actionable //nolint:errcheck
		return nil, shared.MapHTTPStatus(resp.StatusCode, body, "gemini")
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
				ch <- provider.StreamChunk{FinishReason: "STOP", Done: true}
				return
			}
			chunk, stop := mapGeminiStreamChunk(data)
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

// Embed calls POST /v1beta/models/{embedModel}:embedContent.
func (a *GeminiAdapter) Embed(ctx context.Context, req provider.EmbedRequest) (*provider.EmbedResponse, error) {
	if len(req.Texts) == 0 {
		return nil, fmt.Errorf("gemini: %w: Texts must not be empty", provider.ErrInvalidRequest)
	}

	embedModel := req.Model
	if embedModel == "" {
		embedModel = "text-embedding-004"
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:batchEmbedContents?key=%s",
		a.cfg.BaseURL, embedModel, a.cfg.APIKey)

	// Build batch embed request.
	requests := make([]map[string]any, len(req.Texts))
	for i, t := range req.Texts {
		requests[i] = map[string]any{
			"model": "models/" + embedModel,
			"content": map[string]any{
				"parts": []map[string]any{{"text": t}},
			},
		}
	}
	payload, err := json.Marshal(map[string]any{"requests": requests})
	if err != nil {
		return nil, fmt.Errorf("gemini: serialising embed request: %w", err)
	}

	var respBody []byte
	var statusCode int

	err = shared.WithRetry(ctx, maxRetryAttempts, func() error {
		var doErr error
		respBody, statusCode, doErr = shared.DoRequest(ctx, a.client, http.MethodPost, url,
			a.jsonHeaders(), payload)
		if doErr != nil {
			return &shared.RetryableError{Err: doErr}
		}
		return shared.MapHTTPStatus(statusCode, respBody, "gemini")
	})
	if err != nil {
		return nil, err
	}

	return mapEmbedResponse(respBody)
}

// HealthCheck calls GET /v1beta/models to verify connectivity and key validity.
func (a *GeminiAdapter) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1beta/models?key=%s", a.cfg.BaseURL, a.cfg.APIKey)
	body, status, err := shared.DoRequest(ctx, a.client, http.MethodGet, url, a.jsonHeaders(), nil)
	if err != nil {
		return &provider.ProviderError{
			ProviderID: "gemini",
			Code:       "transport_error",
			Message:    err.Error(),
			Underlying: provider.ErrProviderUnavailable,
		}
	}
	return shared.MapHTTPStatus(status, body, "gemini")
}

// --- internal helpers ---

func (a *GeminiAdapter) jsonHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}

func buildGeneratePayload(req provider.ChatRequest) ([]byte, error) {
	contents, systemInstruction := mapMessages(req.Messages)
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	body := map[string]any{
		"contents": contents,
		"generationConfig": map[string]any{
			"maxOutputTokens": maxTokens,
			"temperature":     req.Temperature,
		},
	}
	if systemInstruction != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": systemInstruction}},
		}
	}
	if len(req.StopSequences) > 0 {
		cfg := body["generationConfig"].(map[string]any)
		cfg["stopSequences"] = req.StopSequences
	}
	return json.Marshal(body)
}
