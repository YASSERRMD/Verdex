package provider

import "context"

// LLMProvider is the contract every concrete LLM adapter must satisfy.
//
// Verdex routes all model calls through this interface so that adapters for
// Anthropic, OpenAI, Azure OpenAI, local models, etc. can be registered and
// swapped without touching business logic.
//
// Implementations MUST be safe for concurrent use from multiple goroutines.
type LLMProvider interface {
	// ID returns the stable, unique identifier for this provider instance
	// (e.g. "anthropic", "openai-gpt4o").  The value must be non-empty and
	// match the key used when registering with the Registry.
	ID() string

	// Capabilities returns the static capability descriptor for this
	// provider/model pair.  The returned value must not be mutated by the
	// caller.
	Capabilities() Capability

	// Chat sends a non-streaming chat-completion request and blocks until
	// the full response is available or ctx is cancelled.
	//
	// Implementations should honour ctx.Done() and return a wrapped
	// context error promptly.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// ChatStream sends a streaming chat-completion request.  It returns a
	// read-only channel that emits StreamChunk values.  The channel is
	// closed after the final chunk (Done == true) is sent or when an error
	// occurs.
	//
	// The caller must drain the channel to completion; failing to do so
	// leaks the goroutine that drives it.  Cancelling ctx causes the
	// goroutine to stop and close the channel promptly.
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)

	// Embed returns vector embeddings for the supplied texts.
	//
	// Implementations that do not support embeddings should return
	// ErrProviderUnavailable.
	Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)

	// HealthCheck verifies that the provider's upstream endpoint is
	// reachable and returning valid responses.  It should be fast (a single
	// lightweight API ping) and honour ctx for timeouts.
	HealthCheck(ctx context.Context) error
}
