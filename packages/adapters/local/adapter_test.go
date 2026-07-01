package local_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/adapters/local"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// --- mock response builders ---

func mockChatResponse(id, content string) []byte {
	finish := "stop"
	resp := map[string]any{
		"id":     id,
		"object": "chat.completion",
		"model":  "llama3:8b",
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": finish,
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     8,
			"completion_tokens": 5,
			"total_tokens":      13,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func mockEmbedResponse(n int) []byte {
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
		"model":  "nomic-embed-text",
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
			{"id": "llama3:8b", "object": "model"},
			{"id": "nomic-embed-text", "object": "model"},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// --- test server ---

// newMockServer returns an httptest.Server that simulates an Ollama-compatible
// endpoint with /v1/chat/completions, /v1/embeddings, and /v1/models.
func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		stream, _ := body["stream"].(bool)

		if stream {
			finish := "stop"
			chunks := []map[string]any{
				{
					"id":     "local-stream-001",
					"object": "chat.completion.chunk",
					"choices": []map[string]any{
						{"index": 0, "delta": map[string]any{"content": "Hello"}, "finish_reason": nil},
					},
				},
				{
					"id":     "local-stream-001",
					"object": "chat.completion.chunk",
					"choices": []map[string]any{
						{"index": 0, "delta": map[string]any{}, "finish_reason": &finish},
					},
				},
			}
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
			fmt.Fprint(w, "data: [DONE]\n\n") //nolint:errcheck
			flusher.Flush()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mockChatResponse("local-001", "Hi from local model!")) //nolint:errcheck
	})

	mux.HandleFunc("/v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		inputs, _ := body["input"].([]any)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mockEmbedResponse(len(inputs))) //nolint:errcheck
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mockModelsResponse()) //nolint:errcheck
	})

	return httptest.NewServer(mux)
}

// --- tests ---

func TestLocalAdapter_ID(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	a, err := local.New(local.Config{BaseURL: srv.URL, ModelID: "llama3:8b"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if got := a.ID(); got != "local:llama3:8b" {
		t.Errorf("ID() = %q, want %q", got, "local:llama3:8b")
	}
}

func TestLocalAdapter_Capabilities(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	t.Run("without_embed_model", func(t *testing.T) {
		a, _ := local.New(local.Config{BaseURL: srv.URL, ModelID: "llama3:8b"})
		cap := a.Capabilities()
		if cap.SupportsEmbedding {
			t.Error("SupportsEmbedding must be false when EmbedModelID is empty")
		}
		if cap.ProviderID != "local" {
			t.Errorf("ProviderID = %q, want %q", cap.ProviderID, "local")
		}
	})

	t.Run("with_embed_model", func(t *testing.T) {
		a, _ := local.New(local.Config{
			BaseURL:      srv.URL,
			ModelID:      "llama3:8b",
			EmbedModelID: "nomic-embed-text",
		})
		cap := a.Capabilities()
		if !cap.SupportsEmbedding {
			t.Error("SupportsEmbedding must be true when EmbedModelID is set")
		}
	})
}

func TestLocalAdapter_Chat(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	a, err := local.New(local.Config{BaseURL: srv.URL, ModelID: "llama3:8b"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := t.Context()
	resp, err := a.Chat(ctx, provider.ChatRequest{
		Messages:  []provider.Message{{Role: "user", Content: "hello"}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if resp.Content != "Hi from local model!" {
		t.Errorf("Chat() Content = %q, want %q", resp.Content, "Hi from local model!")
	}
	if resp.ID != "local-001" {
		t.Errorf("Chat() ID = %q, want %q", resp.ID, "local-001")
	}
	if resp.Usage.TotalTokens != 13 {
		t.Errorf("Chat() TotalTokens = %d, want 13", resp.Usage.TotalTokens)
	}
}

func TestLocalAdapter_ChatStream(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	a, err := local.New(local.Config{BaseURL: srv.URL, ModelID: "llama3:8b"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := t.Context()
	ch, err := a.ChatStream(ctx, provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
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
		t.Error("ChatStream() emitted no delta chunks")
	}
}

func TestLocalAdapter_Embed(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	a, err := local.New(local.Config{
		BaseURL:      srv.URL,
		ModelID:      "llama3:8b",
		EmbedModelID: "nomic-embed-text",
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := t.Context()
	resp, err := a.Embed(ctx, provider.EmbedRequest{Texts: []string{"foo", "bar"}})
	if err != nil {
		t.Fatalf("Embed() error: %v", err)
	}
	if len(resp.Embeddings) != 2 {
		t.Errorf("Embed() returned %d embeddings, want 2", len(resp.Embeddings))
	}
	if resp.Dimensions != 3 {
		t.Errorf("Embed() Dimensions = %d, want 3", resp.Dimensions)
	}
}

func TestLocalAdapter_Embed_NoEmbedModel(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	// EmbedModelID intentionally not set.
	a, _ := local.New(local.Config{BaseURL: srv.URL, ModelID: "llama3:8b"})
	_, err := a.Embed(t.Context(), provider.EmbedRequest{Texts: []string{"hi"}})
	if err == nil {
		t.Fatal("Embed() must fail when EmbedModelID is empty")
	}
}

func TestLocalAdapter_HealthCheck(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	a, _ := local.New(local.Config{BaseURL: srv.URL, ModelID: "llama3:8b"})
	if err := a.HealthCheck(t.Context()); err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}

func TestLocalAdapter_HealthCheck_Down(t *testing.T) {
	// Point at a port that is not listening.
	a, _ := local.New(local.Config{BaseURL: "http://127.0.0.1:19999", ModelID: "llama3:8b"})
	err := a.HealthCheck(t.Context())
	if err == nil {
		t.Fatal("HealthCheck() must return an error when the server is down")
	}
	if !errors.Is(err, local.ErrLocalEndpointDown) {
		t.Errorf("HealthCheck() error = %v, want to wrap ErrLocalEndpointDown", err)
	}
}

func TestDiscoverModels(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	models, err := local.DiscoverModels(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("DiscoverModels() error: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("DiscoverModels() returned %d models, want 2", len(models))
	}
	if models[0].ID != "llama3:8b" {
		t.Errorf("models[0].ID = %q, want %q", models[0].ID, "llama3:8b")
	}
}

func TestDiscoverModels_Unreachable(t *testing.T) {
	_, err := local.DiscoverModels(t.Context(), "http://127.0.0.1:19998")
	if err == nil {
		t.Fatal("DiscoverModels() must error when server is unreachable")
	}
	if !errors.Is(err, local.ErrOfflineEndpointUnreachable) {
		t.Errorf("DiscoverModels() error = %v, want to wrap ErrOfflineEndpointUnreachable", err)
	}
}

func TestConcurrencyLimiter(t *testing.T) {
	// Limiter with 1 slot; second acquire should block until first releases.
	limiter := local.NewConcurrencyLimiter(1)

	ctx := t.Context()
	if err := limiter.AcquireSlot(ctx); err != nil {
		t.Fatalf("first AcquireSlot() error: %v", err)
	}

	// Second acquire with a cancelled context must fail immediately.
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	err := limiter.AcquireSlot(cancelCtx)
	if !errors.Is(err, local.ErrConcurrencyLimitExceeded) {
		t.Errorf("second AcquireSlot() error = %v, want ErrConcurrencyLimitExceeded", err)
	}

	limiter.ReleaseSlot()
}

func TestConcurrencyLimiter_ConcurrentRequests(t *testing.T) {
	// Verify that at most MaxConcurrency requests are in-flight simultaneously.
	const maxConcurrency = 3
	const totalRequests = 9

	srv := newMockServer(t)
	defer srv.Close()

	a, err := local.New(local.Config{
		BaseURL:        srv.URL,
		ModelID:        "llama3:8b",
		MaxConcurrency: maxConcurrency,
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, totalRequests)

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := a.Chat(t.Context(), provider.ChatRequest{
				Messages:  []provider.Message{{Role: "user", Content: "hi"}},
				MaxTokens: 16,
			})
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent Chat() error: %v", err)
	}
}

func TestOfflineModeEnforcer_BlocksExternalURLs(t *testing.T) {
	enforcer := local.NewOfflineModeEnforcer(false)

	externalURLs := []string{
		"https://api.openai.com/v1/chat/completions",
		"https://api.anthropic.com/v1/messages",
		"http://10.0.0.1:8080/v1/models",
	}

	for _, u := range externalURLs {
		if err := enforcer.CheckURL(u); err == nil {
			t.Errorf("CheckURL(%q) must return error for external URL", u)
		}
	}
}

func TestOfflineModeEnforcer_AllowsLocalURLs(t *testing.T) {
	enforcer := local.NewOfflineModeEnforcer(false)

	localURLs := []string{
		"http://localhost:11434/v1/models",
		"http://127.0.0.1:11434/v1/chat/completions",
		"http://::1:8080/v1/embeddings",
	}

	for _, u := range localURLs {
		if err := enforcer.CheckURL(u); err != nil {
			t.Errorf("CheckURL(%q) must not error for local URL: %v", u, err)
		}
	}
}

func TestOfflineModeEnforcer_PanicMode(t *testing.T) {
	enforcer := local.NewOfflineModeEnforcer(true)

	defer func() {
		if r := recover(); r == nil {
			t.Error("CheckURL with external URL must panic in test mode")
		}
	}()

	// This must panic.
	_ = enforcer.CheckURL("https://api.openai.com/v1/chat/completions")
}

func TestLocalAdapter_Validate(t *testing.T) {
	_, err := local.New(local.Config{ModelID: ""})
	if err == nil {
		t.Fatal("New() with empty ModelID must return error")
	}
}
