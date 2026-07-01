package openai

import (
	"encoding/json"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// mapMessages converts provider-neutral messages to the OpenAI Chat
// Completions format, preserving role names verbatim (OpenAI accepts
// "system", "user", and "assistant").
func mapMessages(messages []provider.Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		out = append(out, map[string]any{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	return out
}

// openAIChatResponse models the top-level JSON returned by the OpenAI Chat
// Completions endpoint (non-streaming).
type openAIChatResponse struct {
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

// mapChatResponse converts a raw OpenAI response body into a provider-neutral
// *provider.ChatResponse.
func mapChatResponse(body []byte) (*provider.ChatResponse, error) {
	var raw openAIChatResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("openai: parsing chat response: %w", err)
	}

	content := ""
	finishReason := ""
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

// openAIEmbedResponse models the JSON returned by POST /v1/embeddings.
type openAIEmbedResponse struct {
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

// mapEmbedResponse converts a raw OpenAI embeddings response into a
// provider-neutral *provider.EmbedResponse.
func mapEmbedResponse(body []byte) (*provider.EmbedResponse, error) {
	var raw openAIEmbedResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("openai: parsing embed response: %w", err)
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

// openAIStreamDelta is a single chunk in the streaming response.
type openAIStreamDelta struct {
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

// mapOpenAIStreamChunk converts a raw SSE data payload from the OpenAI
// streaming endpoint into a *provider.StreamChunk.
// Returns (nil, false) for events that carry no user-visible content.
// Returns (chunk, true) when the stream is finished.
func mapOpenAIStreamChunk(data string) (chunk *provider.StreamChunk, stop bool) {
	var raw openAIStreamDelta
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
