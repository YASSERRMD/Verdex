package provider

import "context"

// TokenAccountingHook is an optional side-channel for recording token usage
// produced by every LLM call.
//
// Implementations might write to a database, publish metrics, or enforce
// budget limits.  RecordUsage must not block or mutate the ChatResponse /
// EmbedResponse — it is called synchronously after each successful call.
type TokenAccountingHook interface {
	// RecordUsage is called after every successful Chat or Embed call.
	//
	// providerID is the ID() of the provider that handled the request.
	// task identifies which TaskType was performed.
	// usage contains the token counts returned by the provider.
	RecordUsage(ctx context.Context, providerID string, usage TokenUsage, task TaskType)
}

// NoOpAccountingHook is a TokenAccountingHook that does nothing.
// It is useful as a default when no accounting is needed.
type NoOpAccountingHook struct{}

// RecordUsage implements TokenAccountingHook by doing nothing.
func (NoOpAccountingHook) RecordUsage(_ context.Context, _ string, _ TokenUsage, _ TaskType) {}

// hookedProvider wraps an LLMProvider and calls a TokenAccountingHook after
// every successful Chat or Embed call.
type hookedProvider struct {
	inner LLMProvider
	hook  TokenAccountingHook
}

// HookedProvider wraps p so that hook.RecordUsage is called after every
// successful Chat or Embed response.  The hook is not called for stream
// chunks (use a post-stream aggregation pattern for that).
//
// If hook is nil, a NoOpAccountingHook is used.
func HookedProvider(p LLMProvider, hook TokenAccountingHook) LLMProvider {
	if hook == nil {
		hook = NoOpAccountingHook{}
	}
	return &hookedProvider{inner: p, hook: hook}
}

// ID delegates to the wrapped provider.
func (h *hookedProvider) ID() string { return h.inner.ID() }

// Capabilities delegates to the wrapped provider.
func (h *hookedProvider) Capabilities() Capability { return h.inner.Capabilities() }

// Chat calls the inner provider and records token usage on success.
func (h *hookedProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	resp, err := h.inner.Chat(ctx, req)
	if err == nil && resp != nil {
		h.hook.RecordUsage(ctx, h.inner.ID(), resp.Usage, TaskChat)
	}
	return resp, err
}

// ChatStream delegates to the inner provider.
//
// Token accounting for streaming responses is the responsibility of the
// caller; individual StreamChunks do not carry token usage information.
func (h *hookedProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	return h.inner.ChatStream(ctx, req)
}

// Embed calls the inner provider and records token usage on success.
func (h *hookedProvider) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	resp, err := h.inner.Embed(ctx, req)
	if err == nil && resp != nil {
		h.hook.RecordUsage(ctx, h.inner.ID(), resp.Usage, TaskEmbed)
	}
	return resp, err
}

// HealthCheck delegates to the inner provider.
func (h *hookedProvider) HealthCheck(ctx context.Context) error {
	return h.inner.HealthCheck(ctx)
}
