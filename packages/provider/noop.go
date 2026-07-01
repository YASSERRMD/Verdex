package provider

import (
	"context"
	"fmt"
	"time"
)

// NoOpProvider is a deterministic stub that implements LLMProvider.
//
// It is designed for use in unit tests, CI pipelines, and local development
// environments where real LLM calls are unnecessary or undesirable.
//
// Behaviour:
//   - Chat always returns a fixed content string and synthetic token counts.
//   - ChatStream sends a single chunk containing the same fixed content and
//     then closes the channel.
//   - Embed returns a single fixed-dimension zero vector for each input text.
//   - HealthCheck always returns nil.
//   - All calls sleep for SimulatedLatency before returning (default: 0).
type NoOpProvider struct {
	// SimulatedLatency is the artificial delay added before each response.
	// Zero means no delay.
	SimulatedLatency time.Duration

	// FixedContent is the Content returned in every ChatResponse.
	// Defaults to "noop response".
	FixedContent string

	// EmbedDimensions is the length of each embedding vector returned by Embed.
	// Defaults to 4.
	EmbedDimensions int
}

// DefaultNoOpProvider returns a NoOpProvider with sensible defaults.
func DefaultNoOpProvider() *NoOpProvider {
	return &NoOpProvider{
		FixedContent:    "noop response",
		EmbedDimensions: 4,
	}
}

// ID returns the stable identifier for the no-op provider.
func (n *NoOpProvider) ID() string { return "noop" }

// Capabilities returns a Capability that advertises all task types.
func (n *NoOpProvider) Capabilities() Capability {
	return Capability{
		SupportedTasks:    []TaskType{TaskChat, TaskEmbed, TaskReason, TaskExtract},
		MaxContextTokens:  1_000_000,
		MaxOutputTokens:   4_096,
		SupportsStreaming:  true,
		SupportsEmbedding: true,
		ProviderID:        "noop",
		ModelID:           "noop-v1",
	}
}

// Chat returns a deterministic ChatResponse after sleeping SimulatedLatency.
func (n *NoOpProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if err := n.sleep(ctx); err != nil {
		return nil, err
	}

	content := n.fixedContent()
	inputTokens := n.countInputTokens(req.Messages)
	outputTokens := len(content)

	return &ChatResponse{
		ID:           "noop-chat-0001",
		Content:      content,
		FinishReason: "end_turn",
		Usage: TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		},
		Latency: n.SimulatedLatency,
	}, nil
}

// ChatStream returns a channel that emits one StreamChunk containing the fixed
// content followed by a terminal Done chunk.
func (n *NoOpProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 2)

	go func() {
		defer close(ch)

		if err := n.sleep(ctx); err != nil {
			ch <- StreamChunk{FinishReason: "error", Done: true}
			return
		}

		ch <- StreamChunk{Delta: n.fixedContent()}
		ch <- StreamChunk{FinishReason: "end_turn", Done: true}
	}()

	return ch, nil
}

// Embed returns a zero-vector of EmbedDimensions for each input text.
func (n *NoOpProvider) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	if err := n.sleep(ctx); err != nil {
		return nil, err
	}
	if len(req.Texts) == 0 {
		return nil, fmt.Errorf("provider noop: %w: Texts must not be empty", ErrInvalidRequest)
	}

	dims := n.embedDimensions()
	embeddings := make([][]float64, len(req.Texts))
	for i := range embeddings {
		embeddings[i] = make([]float64, dims)
	}

	inputTokens := 0
	for _, t := range req.Texts {
		inputTokens += len(t)
	}

	return &EmbedResponse{
		Embeddings: embeddings,
		Dimensions: dims,
		Usage: TokenUsage{
			InputTokens: inputTokens,
			TotalTokens: inputTokens,
		},
	}, nil
}

// HealthCheck always returns nil.
func (n *NoOpProvider) HealthCheck(_ context.Context) error { return nil }

// --- helpers ---

func (n *NoOpProvider) fixedContent() string {
	if n.FixedContent != "" {
		return n.FixedContent
	}
	return "noop response"
}

func (n *NoOpProvider) embedDimensions() int {
	if n.EmbedDimensions > 0 {
		return n.EmbedDimensions
	}
	return 4
}

func (n *NoOpProvider) sleep(ctx context.Context) error {
	if n.SimulatedLatency <= 0 {
		return ctx.Err()
	}
	select {
	case <-time.After(n.SimulatedLatency):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *NoOpProvider) countInputTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content)
	}
	return total
}
