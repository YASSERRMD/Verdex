package anthropic_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YASSERRMD/verdex/packages/adapters/anthropic"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// mockAnthropicResponse returns a minimal valid Anthropic Messages API response.
func mockAnthropicChatResponse() []byte {
	resp := map[string]any{
		"id":           "msg_test_001",
		"type":         "message",
		"role":         "assistant",
		"model":        "claude-3-5-sonnet-20241022",
		"stop_reason":  "end_turn",
		"stop_sequence": nil,
		"content": []map[string]any{
			{"type": "text", "text": "Hello from Claude."},
		},
		"usage": map[string]any{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// mockModelsResponse returns a minimal valid /v1/models response for HealthCheck.
func mockModelsResponse() []byte {
	resp := map[string]any{
		"data": []map[string]any{
			{"id": "claude-3-5-sonnet-20241022", "type": "model"},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestAnthropicAdapter_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/messages":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(mockAnthropicChatResponse()) //nolint:errcheck
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(mockModelsResponse()) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter, err := anthropic.New(anthropic.Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Run("ID", func(t *testing.T) {
		if adapter.ID() != "anthropic" {
			t.Errorf("ID() = %q, want %q", adapter.ID(), "anthropic")
		}
	})

	t.Run("Capabilities", func(t *testing.T) {
		cap := adapter.Capabilities()
		if cap.ProviderID != "anthropic" {
			t.Errorf("Capabilities().ProviderID = %q, want %q", cap.ProviderID, "anthropic")
		}
		if cap.MaxContextTokens <= 0 {
			t.Error("Capabilities().MaxContextTokens must be positive")
		}
		if !cap.SupportsStreaming {
			t.Error("Capabilities().SupportsStreaming must be true")
		}
		if cap.SupportsEmbedding {
			t.Error("Capabilities().SupportsEmbedding must be false for Anthropic")
		}
	})

	t.Run("Chat", func(t *testing.T) {
		ctx := t.Context()
		req := provider.ChatRequest{
			Messages:  []provider.Message{{Role: "user", Content: "hello"}},
			MaxTokens: 64,
		}
		resp, err := adapter.Chat(ctx, req)
		if err != nil {
			t.Fatalf("Chat() error: %v", err)
		}
		if resp == nil {
			t.Fatal("Chat() returned nil")
		}
		if resp.Content != "Hello from Claude." {
			t.Errorf("Chat() Content = %q, want %q", resp.Content, "Hello from Claude.")
		}
		if resp.ID != "msg_test_001" {
			t.Errorf("Chat() ID = %q, want %q", resp.ID, "msg_test_001")
		}
		if resp.Usage.InputTokens != 10 {
			t.Errorf("Chat() Usage.InputTokens = %d, want 10", resp.Usage.InputTokens)
		}
	})

	t.Run("ChatStream", func(t *testing.T) {
		// Build a minimal SSE stream that sends a text delta and then message_stop.
		srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			if !ok {
				return
			}
			events := []string{
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi!"}}`,
				`{"type":"message_stop"}`,
			}
			for _, ev := range events {
				fmt.Fprintf(w, "data: %s\n\n", ev) //nolint:errcheck
				flusher.Flush()
			}
		}))
		defer srv2.Close()

		adapter2, err := anthropic.New(anthropic.Config{
			APIKey:  "test-key",
			BaseURL: srv2.URL,
		})
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}

		ctx := t.Context()
		req := provider.ChatRequest{
			Messages: []provider.Message{{Role: "user", Content: "hello"}},
			Stream:   true,
		}
		ch, err := adapter2.ChatStream(ctx, req)
		if err != nil {
			t.Fatalf("ChatStream() error: %v", err)
		}

		var deltas []string
		var gotDone bool
		for chunk := range ch {
			if chunk.Done {
				gotDone = true
				break
			}
			if chunk.Delta != "" {
				deltas = append(deltas, chunk.Delta)
			}
		}
		if !gotDone {
			t.Error("ChatStream() did not emit a Done chunk")
		}
		if len(deltas) == 0 {
			t.Error("ChatStream() emitted no delta chunks")
		}
	})

	t.Run("Embed_Unsupported", func(t *testing.T) {
		ctx := t.Context()
		_, err := adapter.Embed(ctx, provider.EmbedRequest{Texts: []string{"test"}})
		if err == nil {
			t.Fatal("Embed() must return error for Anthropic")
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		ctx := t.Context()
		if err := adapter.HealthCheck(ctx); err != nil {
			t.Errorf("HealthCheck() error: %v", err)
		}
	})
}

func TestAnthropicAdapter_Validate(t *testing.T) {
	_, err := anthropic.New(anthropic.Config{APIKey: ""})
	if err == nil {
		t.Fatal("New() with empty APIKey must return error")
	}
}
