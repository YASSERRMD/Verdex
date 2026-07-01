package provider

import "time"

// Message is a single turn in a conversation.
type Message struct {
	// Role is one of "user", "assistant", or "system".
	Role    string
	Content string
}

// ChatRequest describes a chat-completion request in provider-neutral terms.
type ChatRequest struct {
	Messages      []Message
	MaxTokens     int
	Temperature   float64
	Stream        bool
	StopSequences []string
	// Metadata carries arbitrary string key/value pairs forwarded to the
	// underlying provider where supported (e.g. user IDs, trace IDs).
	Metadata map[string]string
}

// ChatResponse is the provider-neutral result of a non-streaming chat call.
type ChatResponse struct {
	// ID is the provider-assigned completion identifier.
	ID           string
	Content      string
	FinishReason string
	Usage        TokenUsage
	Latency      time.Duration
}

// StreamChunk is a single delta event produced by a streaming chat call.
type StreamChunk struct {
	Delta        string
	FinishReason string
	Done         bool
}

// EmbedRequest describes an embedding request.
type EmbedRequest struct {
	Texts []string
	// Model is the embedding model identifier; may be empty to use the
	// provider's default embedding model.
	Model string
}

// EmbedResponse holds the embeddings returned by the provider.
type EmbedResponse struct {
	Embeddings [][]float64
	Dimensions int
	Usage      TokenUsage
}

// TokenUsage records input, output, and total token counts for a single call.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// TaskType classifies the kind of work a provider call performs.
type TaskType string

const (
	// TaskChat is a conversational chat-completion task.
	TaskChat TaskType = "chat"
	// TaskEmbed is a text-embedding task.
	TaskEmbed TaskType = "embed"
	// TaskReason is a structured reasoning / chain-of-thought task.
	TaskReason TaskType = "reason"
	// TaskExtract is an information-extraction task.
	TaskExtract TaskType = "extract"
)

// Capability describes what a specific provider/model combination can do.
type Capability struct {
	// SupportedTasks lists the TaskType values this provider handles.
	SupportedTasks []TaskType
	// MaxContextTokens is the maximum combined input token limit.
	MaxContextTokens int
	// MaxOutputTokens is the maximum number of tokens the model can generate.
	MaxOutputTokens int
	// SupportsStreaming indicates whether ChatStream is functional.
	SupportsStreaming bool
	// SupportsEmbedding indicates whether Embed is functional.
	SupportsEmbedding bool
	// ProviderID identifies the provider (e.g. "anthropic", "openai").
	ProviderID string
	// ModelID identifies the specific model (e.g. "claude-3-5-sonnet-20241022").
	ModelID string
}
