package local

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
	defaultBaseURL       = "http://localhost:11434"
	defaultTimeout       = 300 * time.Second // large models may be slow
	defaultMaxConcurrency = 2
	maxRetryAttempts     = 1 // local servers rarely benefit from retries
)

// Config holds all tunable parameters for the LocalAdapter.
type Config struct {
	// BaseURL is the base URL of the OpenAI-compatible local server.
	// Defaults to "http://localhost:11434" (Ollama default).
	BaseURL string

	// ModelID is the model used for Chat / ChatStream calls, e.g. "llama3:8b".
	// Required.
	ModelID string

	// EmbedModelID is the model used for Embed calls, e.g. "nomic-embed-text".
	// When empty, embedding is disabled and Capabilities().SupportsEmbedding
	// returns false.
	EmbedModelID string

	// Timeout is the per-request HTTP timeout. Defaults to 300 s to
	// accommodate slow GGUF inference on CPU.
	Timeout time.Duration

	// MaxConcurrency caps the number of simultaneous in-flight requests.
	// Defaults to 2. Set to 1 for single-GPU deployments.
	MaxConcurrency int

	// OfflineMode signals that this adapter is operating in an air-gapped
	// environment. When true, the OfflineModeEnforcer validates that no
	// outbound network calls reach non-localhost URLs.
	OfflineMode bool
}

func (c Config) withDefaults() Config {
	if c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = defaultMaxConcurrency
	}
	return c
}

// Validate returns an error if the config is missing required fields.
func (c Config) Validate() error {
	if c.ModelID == "" {
		return fmt.Errorf("local: ModelID must not be empty")
	}
	return nil
}

// LocalAdapter implements [provider.LLMProvider] for local/self-hosted
// OpenAI-compatible endpoints (Ollama, LM Studio, vLLM, LocalAI, etc.).
// It is safe for concurrent use.
type LocalAdapter struct {
	cfg      Config
	client   *http.Client
	limiter  *ConcurrencyLimiter
	enforcer *OfflineModeEnforcer
}

// New creates a new LocalAdapter and validates the configuration.
func New(cfg Config) (*LocalAdapter, error) {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &LocalAdapter{
		cfg:      cfg,
		client:   shared.BuildHTTPClient(cfg.Timeout),
		limiter:  NewConcurrencyLimiter(cfg.MaxConcurrency),
		enforcer: NewOfflineModeEnforcer(false),
	}, nil
}

// ID returns the stable provider identifier "local:<ModelID>".
func (a *LocalAdapter) ID() string { return "local:" + a.cfg.ModelID }

// Capabilities returns the capability descriptor for this local adapter.
// SupportsEmbedding is true only when EmbedModelID is configured.
func (a *LocalAdapter) Capabilities() provider.Capability {
	tasks := []provider.TaskType{provider.TaskChat, provider.TaskReason, provider.TaskExtract}
	supportsEmbed := a.cfg.EmbedModelID != ""
	if supportsEmbed {
		tasks = append(tasks, provider.TaskEmbed)
	}
	return provider.Capability{
		SupportedTasks:    tasks,
		MaxContextTokens:  32_768, // conservative default; actual limit varies by model
		MaxOutputTokens:   4_096,
		SupportsStreaming:  true,
		SupportsEmbedding: supportsEmbed,
		ProviderID:        "local",
		ModelID:           a.cfg.ModelID,
	}
}

// Chat sends a non-streaming request to POST /v1/chat/completions on the
// local server.
func (a *LocalAdapter) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	if a.cfg.OfflineMode {
		if err := a.enforcer.CheckURL(a.cfg.BaseURL); err != nil {
			return nil, err
		}
	}

	if err := a.limiter.AcquireSlot(ctx); err != nil {
		return nil, err
	}
	defer a.limiter.ReleaseSlot()

	start := time.Now()

	payload, err := buildChatPayload(req, a.cfg.ModelID, false)
	if err != nil {
		return nil, fmt.Errorf("local: serialising chat request: %w", err)
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
		return shared.MapHTTPStatus(statusCode, respBody, "local")
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

// ChatStream sends a streaming request to POST /v1/chat/completions and
// returns a channel of SSE-decoded [provider.StreamChunk] values.
func (a *LocalAdapter) ChatStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	if a.cfg.OfflineMode {
		if err := a.enforcer.CheckURL(a.cfg.BaseURL); err != nil {
			return nil, err
		}
	}

	if err := a.limiter.AcquireSlot(ctx); err != nil {
		return nil, err
	}

	payload, err := buildChatPayload(req, a.cfg.ModelID, true)
	if err != nil {
		a.limiter.ReleaseSlot()
		return nil, fmt.Errorf("local: serialising stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cfg.BaseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		a.limiter.ReleaseSlot()
		return nil, fmt.Errorf("local: building stream request: %w", err)
	}
	for k, v := range a.headers() {
		httpReq.Header.Set(k, v)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		a.limiter.ReleaseSlot()
		return nil, &provider.ProviderError{
			ProviderID: a.ID(),
			Code:       "transport_error",
			Message:    err.Error(),
			Underlying: provider.ErrProviderUnavailable,
		}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		a.limiter.ReleaseSlot()
		return nil, shared.MapHTTPStatus(resp.StatusCode, body, "local")
	}

	ch := make(chan provider.StreamChunk, 8)
	go func() {
		defer close(ch)
		defer resp.Body.Close() //nolint:errcheck
		defer a.limiter.ReleaseSlot()

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
			// OpenAI-compatible servers signal end-of-stream with "[DONE]".
			if data == "[DONE]" {
				ch <- provider.StreamChunk{FinishReason: "stop", Done: true}
				return
			}
			chunk, stop := mapStreamChunk(data)
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

// Embed calls POST /v1/embeddings on the local server and returns
// provider-neutral embeddings.
//
// Returns an error if EmbedModelID is not configured.
func (a *LocalAdapter) Embed(ctx context.Context, req provider.EmbedRequest) (*provider.EmbedResponse, error) {
	if a.cfg.EmbedModelID == "" {
		return nil, fmt.Errorf("local: %w: EmbedModelID not configured", provider.ErrInvalidRequest)
	}
	if a.cfg.OfflineMode {
		if err := a.enforcer.CheckURL(a.cfg.BaseURL); err != nil {
			return nil, err
		}
	}
	if len(req.Texts) == 0 {
		return nil, fmt.Errorf("local: %w: Texts must not be empty", provider.ErrInvalidRequest)
	}

	if err := a.limiter.AcquireSlot(ctx); err != nil {
		return nil, err
	}
	defer a.limiter.ReleaseSlot()

	model := req.Model
	if model == "" {
		model = a.cfg.EmbedModelID
	}

	payload, err := json.Marshal(map[string]any{
		"input": req.Texts,
		"model": model,
	})
	if err != nil {
		return nil, fmt.Errorf("local: serialising embed request: %w", err)
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
		return shared.MapHTTPStatus(statusCode, respBody, "local")
	})
	if err != nil {
		return nil, err
	}

	return mapEmbedResponse(respBody)
}

// --- internal helpers ---

func (a *LocalAdapter) headers() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}

func buildChatPayload(req provider.ChatRequest, model string, stream bool) ([]byte, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	messages := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, map[string]any{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	body := map[string]any{
		"model":      model,
		"messages":   messages,
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

// chatResponse mirrors the OpenAI non-streaming chat completion envelope.
type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func mapChatResponse(body []byte) (*provider.ChatResponse, error) {
	var raw chatResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("local: parsing chat response: %w", err)
	}
	content, finishReason := "", ""
	if len(raw.Choices) > 0 {
		content = raw.Choices[0].Message.Content
		finishReason = raw.Choices[0].FinishReason
	}
	return &provider.ChatResponse{
		ID:           raw.ID,
		Content:      content,
		FinishReason: finishReason,
		Usage: provider.TokenUsage{
			InputTokens:  raw.Usage.PromptTokens,
			OutputTokens: raw.Usage.CompletionTokens,
			TotalTokens:  raw.Usage.TotalTokens,
		},
	}, nil
}

// embedResponse mirrors the OpenAI /v1/embeddings response envelope.
type embedResponse struct {
	Object string `json:"object"`
	Model  string `json:"model"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func mapEmbedResponse(body []byte) (*provider.EmbedResponse, error) {
	var raw embedResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("local: parsing embed response: %w", err)
	}
	embeddings := make([][]float64, len(raw.Data))
	dims := 0
	for _, d := range raw.Data {
		embeddings[d.Index] = d.Embedding
		if len(d.Embedding) > dims {
			dims = len(d.Embedding)
		}
	}
	return &provider.EmbedResponse{
		Embeddings: embeddings,
		Dimensions: dims,
		Usage: provider.TokenUsage{
			InputTokens: raw.Usage.PromptTokens,
			TotalTokens: raw.Usage.TotalTokens,
		},
	}, nil
}

// streamDelta is a single chunk in the OpenAI-compatible SSE stream.
type streamDelta struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func mapStreamChunk(data string) (chunk *provider.StreamChunk, stop bool) {
	var raw streamDelta
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, false
	}
	if len(raw.Choices) == 0 {
		return nil, false
	}
	choice := raw.Choices[0]
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		return &provider.StreamChunk{
			FinishReason: *choice.FinishReason,
			Done:         true,
		}, true
	}
	if choice.Delta.Content != "" {
		return &provider.StreamChunk{Delta: choice.Delta.Content}, false
	}
	return nil, false
}
