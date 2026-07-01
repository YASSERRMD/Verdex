package gemini

import (
	"encoding/json"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// mapMessages converts provider-neutral messages to the Gemini "contents" format.
// System messages are extracted and returned separately as the systemInstruction
// string since Gemini passes them via a dedicated top-level field.
//
// Gemini uses roles "user" and "model" (not "assistant").
func mapMessages(messages []provider.Message) (contents []map[string]any, systemInstruction string) {
	contents = make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case "system":
			systemInstruction = m.Content
		case "assistant":
			contents = append(contents, map[string]any{
				"role":  "model",
				"parts": []map[string]any{{"text": m.Content}},
			})
		default: // "user"
			contents = append(contents, map[string]any{
				"role":  "user",
				"parts": []map[string]any{{"text": m.Content}},
			})
		}
	}
	return contents, systemInstruction
}

// geminiGenerateResponse models the JSON returned by :generateContent.
type geminiGenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		Index         int    `json:"index"`
		SafetyRatings []any  `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// mapChatResponse converts a raw Gemini :generateContent response body into a
// provider-neutral *provider.ChatResponse.
func mapChatResponse(body []byte) (*provider.ChatResponse, error) {
	var raw geminiGenerateResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("gemini: parsing chat response: %w", err)
	}

	content := ""
	finishReason := ""
	if len(raw.Candidates) > 0 {
		c := raw.Candidates[0]
		finishReason = c.FinishReason
		for _, part := range c.Content.Parts {
			content += part.Text
		}
	}

	return &provider.ChatResponse{
		ID:           "", // Gemini does not return a completion ID
		Content:      content,
		FinishReason: finishReason,
		Usage: provider.TokenUsage{
			InputTokens:  raw.UsageMetadata.PromptTokenCount,
			OutputTokens: raw.UsageMetadata.CandidatesTokenCount,
			TotalTokens:  raw.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

// geminiStreamChunk models a single SSE event from :streamGenerateContent.
// The payload mirrors :generateContent but contains only a partial candidate.
type geminiStreamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}

// mapGeminiStreamChunk converts a raw SSE data payload from Gemini into a
// *provider.StreamChunk. Returns (nil, false) for empty events.
func mapGeminiStreamChunk(data string) (chunk *provider.StreamChunk, stop bool) {
	var raw geminiStreamChunk
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, false
	}
	if len(raw.Candidates) == 0 {
		return nil, false
	}
	cand := raw.Candidates[0]

	text := ""
	for _, part := range cand.Content.Parts {
		text += part.Text
	}

	if cand.FinishReason != "" && cand.FinishReason != "FINISH_REASON_UNSPECIFIED" {
		return &provider.StreamChunk{
			Delta:        text,
			FinishReason: cand.FinishReason,
			Done:         true,
		}, true
	}

	if text != "" {
		return &provider.StreamChunk{Delta: text}, false
	}
	return nil, false
}

// geminiEmbedBatchResponse models the JSON returned by :batchEmbedContents.
type geminiEmbedBatchResponse struct {
	Embeddings []struct {
		Values []float64 `json:"values"`
	} `json:"embeddings"`
}

// mapEmbedResponse converts a raw Gemini batch-embed response into a
// provider-neutral *provider.EmbedResponse.
func mapEmbedResponse(body []byte) (*provider.EmbedResponse, error) {
	var raw geminiEmbedBatchResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("gemini: parsing embed response: %w", err)
	}

	embeddings := make([][]float64, len(raw.Embeddings))
	dims := 0
	for i, e := range raw.Embeddings {
		embeddings[i] = e.Values
		if len(e.Values) > dims {
			dims = len(e.Values)
		}
	}

	return &provider.EmbedResponse{
		Embeddings: embeddings,
		Dimensions: dims,
	}, nil
}
