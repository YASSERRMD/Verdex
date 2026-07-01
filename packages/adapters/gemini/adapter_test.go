package gemini_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/adapters/gemini"
	"github.com/YASSERRMD/verdex/packages/provider"
)

func mockGeminiChatResponse() []byte {
	resp := map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"parts": []map[string]any{{"text": "Hello from Gemini."}},
					"role":  "model",
				},
				"finishReason": "STOP",
				"index":        0,
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     8,
			"candidatesTokenCount": 4,
			"totalTokenCount":      12,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func mockGeminiBatchEmbedResponse(n int) []byte {
	embeddings := make([]map[string]any, n)
	for i := 0; i < n; i++ {
		embeddings[i] = map[string]any{"values": []float64{0.1, 0.2, 0.3, 0.4}}
	}
	resp := map[string]any{"embeddings": embeddings}
	b, _ := json.Marshal(resp)
	return b
}

func mockGeminiModelsResponse() []byte {
	resp := map[string]any{
		"models": []map[string]any{
			{"name": "models/gemini-1.5-pro", "displayName": "Gemini 1.5 Pro"},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestGeminiAdapter_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, ":generateContent"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(mockGeminiChatResponse()) //nolint:errcheck
		case strings.Contains(path, ":batchEmbedContents"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(mockGeminiBatchEmbedResponse(2)) //nolint:errcheck
		case strings.Contains(path, "/v1beta/models") && !strings.Contains(path, ":"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(mockGeminiModelsResponse()) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter, err := gemini.New(gemini.Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Run("ID", func(t *testing.T) {
		if adapter.ID() != "gemini" {
			t.Errorf("ID() = %q, want %q", adapter.ID(), "gemini")
		}
	})

	t.Run("Capabilities", func(t *testing.T) {
		cap := adapter.Capabilities()
		if cap.ProviderID != "gemini" {
			t.Errorf("Capabilities().ProviderID = %q, want %q", cap.ProviderID, "gemini")
		}
		if !cap.SupportsEmbedding {
			t.Error("Capabilities().SupportsEmbedding must be true for Gemini")
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
		if resp.Content != "Hello from Gemini." {
			t.Errorf("Chat() Content = %q, want %q", resp.Content, "Hello from Gemini.")
		}
		if resp.Usage.TotalTokens != 12 {
			t.Errorf("Chat() TotalTokens = %d, want 12", resp.Usage.TotalTokens)
		}
	})

	t.Run("ChatStream", func(t *testing.T) {
		srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			if !ok {
				return
			}
			events := []map[string]any{
				{
					"candidates": []map[string]any{
						{"content": map[string]any{"parts": []map[string]any{{"text": "Hello"}}}, "finishReason": ""},
					},
				},
				{
					"candidates": []map[string]any{
						{"content": map[string]any{"parts": []map[string]any{{"text": "!"}}} , "finishReason": "STOP"},
					},
				},
			}
			for _, ev := range events {
				b, _ := json.Marshal(ev)
				fmt.Fprintf(w, "data: %s\n\n", b) //nolint:errcheck
				flusher.Flush()
			}
		}))
		defer srv2.Close()

		adapter2, err := gemini.New(gemini.Config{APIKey: "key", BaseURL: srv2.URL})
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
		resp, err := adapter.Embed(ctx, provider.EmbedRequest{Texts: []string{"one", "two"}})
		if err != nil {
			t.Fatalf("Embed() error: %v", err)
		}
		if len(resp.Embeddings) != 2 {
			t.Errorf("Embed() returned %d embeddings, want 2", len(resp.Embeddings))
		}
		if resp.Dimensions != 4 {
			t.Errorf("Embed() Dimensions = %d, want 4", resp.Dimensions)
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		ctx := t.Context()
		if err := adapter.HealthCheck(ctx); err != nil {
			t.Errorf("HealthCheck() error: %v", err)
		}
	})
}

func TestGeminiAdapter_Validate(t *testing.T) {
	_, err := gemini.New(gemini.Config{APIKey: ""})
	if err == nil {
		t.Fatal("New() with empty APIKey must return error")
	}
}
