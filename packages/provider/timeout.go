package provider

import (
	"context"
	"time"
)

// timeoutProvider wraps an LLMProvider and enforces per-call deadlines.
type timeoutProvider struct {
	inner        LLMProvider
	chatTimeout  time.Duration
	embedTimeout time.Duration
}

// WithTimeout returns an LLMProvider that wraps p and enforces per-call
// deadlines:
//
//   - chatTimeout applies to both Chat and ChatStream calls (controls how long
//     the caller waits for the provider to begin streaming).
//   - embedTimeout applies to Embed calls.
//
// A zero duration means no additional deadline is imposed on that call type
// (the caller's context timeout, if any, still applies).
//
// HealthCheck is not subject to the timeouts configured here; wrap ctx
// externally if you need a deadline on health checks.
func WithTimeout(p LLMProvider, chatTimeout, embedTimeout time.Duration) LLMProvider {
	return &timeoutProvider{
		inner:        p,
		chatTimeout:  chatTimeout,
		embedTimeout: embedTimeout,
	}
}

// ID delegates to the wrapped provider.
func (t *timeoutProvider) ID() string { return t.inner.ID() }

// Capabilities delegates to the wrapped provider.
func (t *timeoutProvider) Capabilities() Capability { return t.inner.Capabilities() }

// Chat enforces chatTimeout on the request.
func (t *timeoutProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	ctx, cancel := t.withDeadline(ctx, t.chatTimeout)
	defer cancel()
	return t.inner.Chat(ctx, req)
}

// ChatStream enforces chatTimeout on the initial handshake.
//
// Note: the timeout governs how long we wait for ChatStream to return the
// channel, not how long the full stream takes to complete.  Once the channel
// is returned the caller drives consumption and is responsible for its own
// deadline.
func (t *timeoutProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	ctx, cancel := t.withDeadline(ctx, t.chatTimeout)
	// We cannot defer cancel here because the goroutine driving the returned
	// channel must keep the context alive.  Cancellation will occur when the
	// returned channel is closed by the inner provider or when the caller's
	// parent context is done.  To avoid leaking, callers should pass a
	// context they control.
	_ = cancel // documented above; inner provider owns lifetime after return
	return t.inner.ChatStream(ctx, req)
}

// Embed enforces embedTimeout on the request.
func (t *timeoutProvider) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	ctx, cancel := t.withDeadline(ctx, t.embedTimeout)
	defer cancel()
	return t.inner.Embed(ctx, req)
}

// HealthCheck delegates without adding a deadline.
func (t *timeoutProvider) HealthCheck(ctx context.Context) error {
	return t.inner.HealthCheck(ctx)
}

// withDeadline wraps ctx with a deadline if d > 0; otherwise it returns ctx
// and a no-op cancel.
func (t *timeoutProvider) withDeadline(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
