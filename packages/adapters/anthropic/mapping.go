package anthropic

import (
	"encoding/json"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// extractSystemMessage separates the first "system" role message (if any) from
// the remaining messages. Anthropic's Messages API requires system prompts to
// be provided via a top-level "system" field rather than inline.
func extractSystemMessage(messages []provider.Message) (system string, rest []provider.Message) {
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			rest = append(rest, m)
		}
	}
	return system, rest
}

// mapMessages converts provider-neutral messages into the Anthropic Messages
// API format. System messages are excluded (callers should use
// extractSystemMessage first).
func mapMessages(messages []provider.Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		out = append(out, map[string]any{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	return out
}

// anthropicResponse models the JSON envelope returned by POST /v1/messages.
type anthropicResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Content      []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// mapChatResponse converts a raw Anthropic response body into a provider-neutral
// *provider.ChatResponse.
func mapChatResponse(body []byte) (*provider.ChatResponse, error) {
	var raw anthropicResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("anthropic: parsing response: %w", err)
	}

	content := ""
	for _, block := range raw.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &provider.ChatResponse{
		ID:           raw.ID,
		Content:      content,
		FinishReason: raw.StopReason,
		Usage: provider.TokenUsage{
			InputTokens:  raw.Usage.InputTokens,
			OutputTokens: raw.Usage.OutputTokens,
			TotalTokens:  raw.Usage.InputTokens + raw.Usage.OutputTokens,
		},
	}, nil
}

// anthropicStreamEvent models the SSE events emitted by the Anthropic streaming
// API. We handle a subset of event types relevant to content delivery.
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	// For message_start events.
	Message struct {
		ID    string `json:"id"`
		Model string `json:"model"`
	} `json:"message"`
}

// mapAnthropicStreamEvent converts a raw SSE data payload into a
// *provider.StreamChunk. It returns (nil, false) for events that carry no
// user-visible content. When stop is true the stream is finished.
func mapAnthropicStreamEvent(data string) (chunk *provider.StreamChunk, stop bool) {
	var ev anthropicStreamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		// Unparseable event — skip silently.
		return nil, false
	}

	switch ev.Type {
	case "content_block_delta":
		if ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
			return &provider.StreamChunk{Delta: ev.Delta.Text}, false
		}
	case "message_delta":
		if ev.Delta.StopReason != "" {
			return &provider.StreamChunk{FinishReason: ev.Delta.StopReason, Done: true}, true
		}
	case "message_stop":
		return &provider.StreamChunk{FinishReason: "end_turn", Done: true}, true
	}
	return nil, false
}
