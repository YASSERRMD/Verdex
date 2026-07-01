package openai_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YASSERRMD/verdex/packages/adapters/openai"
	"github.com/YASSERRMD/verdex/packages/provider"
)

func mockOpenAIChatResponse() []byte {
	resp := map[string]any{
		"id":     "chatcmpl-test001",
		"object": "chat.completion",
		"model":  "gpt-4o",
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "Hello from GPT.",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 4,
			"total_tokens":      14,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func mockOpenAIEmbedResponse(n int) []byte {
	data := make([]map[string]any, n)
	for i := 0; i < n; i++ {
		data[i] = map[string]any{
			"object":    "embedding",
			"index":     i,
			"embedding": []float64{0.1, 0.2, 0.3},
		}
	}
	resp := map[string]any{
		"object": "list",
		"model":  "text-embedding-3-small",
		"data":   data,
		"usage": map[string]any{
			"prompt_tokens": n * 3,
			"total_tokens":  n * 3,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func mockModelsResponse() []byte {
	resp := map[string]any{
		"object": "list",
		"data": []map[string]any{
			{"id": "gpt-4o", "object": "model"},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestOpenAIAdapter_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(mockOpenAIChatResponse()) //nolint:errcheck
		case "/v1/embeddings":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(mockOpenAIEmbedResponse(2)) //nolint:errcheck
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(mockModelsResponse()) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter, err := openai.New(openai.Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Run("ID", func(t *testing.T) {
		if adapter.ID() != "openai" {
			t.Errorf("ID() = %q, want %q", adapter.ID(), "openai")
		}
	})

	t.Run("Capabilities", func(t *testing.T) {
		cap := adapter.Capabilities()
		if cap.ProviderID != "openai" {
			t.Errorf("Capabilities().ProviderID = %q, want %q", cap.ProviderID, "openai")
		}
		if !cap.SupportsEmbedding {
			t.Error("Capabilities().SupportsEmbedding must be true for OpenAI")
		}
		if cap.MaxContextTokens <= 0 {
			t.Error("Capabilities().MaxContextTokens must be positive")
		}
	})

	t.Run("Chat", func(t *testing.T) {
		ctx := t.Context()
		req := provider.ChatRequest{
			Messages:  []provider.Message{{Role: "user", Content: "hi"}},
			MaxTokens: 64,
		}
		resp, err := adapter.Chat(ctx, req)
		if err != nil {
			t.Fatalf("Chat() error: %v", err)
		}
		if resp.Content != "Hello from GPT." {
			t.Errorf("Chat() Content = %q, want %q", resp.Content, "Hello from GPT.")
		}
		if resp.ID != "chatcmpl-test001" {
			t.Errorf("Chat() ID = %q, want %q", resp.ID, "chatcmpl-test001")
		}
		if resp.Usage.TotalTokens != 14 {
			t.Errorf("Chat() TotalTokens = %d, want 14", resp.Usage.TotalTokens)
		}
	})

	t.Run("ChatStream", func(t *testing.T) {
		finish := "stop"
		chunks := []map[string]any{
			{
				"id":      "chatcmpl-stream-001",
				"object":  "chat.completion.chunk",
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": "Hi!"}, "finish_reason": nil}},
			},
			{
				"id":      "chatcmpl-stream-001",
				"object":  "chat.completion.chunk",
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": &finish}},
			},
		}

		srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			if !ok {
				return
			}
			for _, c := range chunks {
				b, _ := json.Marshal(c)
				fmt.Fprintf(w, "data: %s\n\n", b) //nolint:errcheck
				flusher.Flush()
			}
			fmt.Fprintf(w, "data: [DONE]\n\n") //nolint:errcheck
			flusher.Flush()
		}))
		defer srv2.Close()

		adapter2, err := openai.New(openai.Config{APIKey: "key", BaseURL: srv2.URL})
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}

		ctx := t.Context()
		ch, err := adapter2.ChatStream(ctx, provider.ChatRequest{
			Messages: []provider.Message{{Role: "user", Content: "hi"}},
			Stream:   true,
		})
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
			t.Error("ChatStream() did not emit Done chunk")
		}
		if len(deltas) == 0 {
			t.Error("ChatStream() emitted no deltas")
		}
	})

	t.Run("Embed", func(t *testing.T) {
		ctx := t.Context()
		resp, err := adapter.Embed(ctx, provider.EmbedRequest{Texts: []string{"hello", "world"}})
		if err != nil {
			t.Fatalf("Embed() error: %v", err)
		}
		if len(resp.Embeddings) != 2 {
			t.Errorf("Embed() returned %d embeddings, want 2", len(resp.Embeddings))
		}
		if resp.Dimensions != 3 {
			t.Errorf("Embed() Dimensions = %d, want 3", resp.Dimensions)
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		ctx := t.Context()
		if err := adapter.HealthCheck(ctx); err != nil {
			t.Errorf("HealthCheck() error: %v", err)
		}
	})
}

func TestOpenAIAdapter_Validate(t *testing.T) {
	_, err := openai.New(openai.Config{APIKey: ""})
	if err == nil {
		t.Fatal("New() with empty APIKey must return error")
	}
}
