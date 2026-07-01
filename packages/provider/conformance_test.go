package provider_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// ProviderConformanceTest verifies that p satisfies the LLMProvider contract.
//
// Any provider adapter can call this function from its own test suite to
// confirm it meets the interface requirements before integration.
//
// Checks performed:
//  1. ID() returns a non-empty string.
//  2. Capabilities() returns a non-zero Capability (ProviderID matches ID()).
//  3. Chat() returns a non-nil response with non-empty Content.
//  4. ChatStream() returns a channel that emits at least one chunk and closes.
//  5. Embed() returns a response with Embeddings whose length matches the input
//     and whose inner vectors have the advertised Dimensions length.
//  6. HealthCheck() returns nil under a background context.
func ProviderConformanceTest(t *testing.T, p provider.LLMProvider) {
	t.Helper()
	ctx := context.Background()

	t.Run("ID", func(t *testing.T) {
		if id := p.ID(); id == "" {
			t.Fatal("ID() must return a non-empty string")
		}
	})

	t.Run("Capabilities", func(t *testing.T) {
		cap := p.Capabilities()
		if cap.ProviderID == "" {
			t.Error("Capabilities().ProviderID must be non-empty")
		}
		if cap.ProviderID != p.ID() {
			t.Errorf("Capabilities().ProviderID %q must match ID() %q", cap.ProviderID, p.ID())
		}
		if cap.MaxContextTokens <= 0 {
			t.Error("Capabilities().MaxContextTokens must be positive")
		}
	})

	t.Run("Chat", func(t *testing.T) {
		req := provider.ChatRequest{
			Messages: []provider.Message{
				{Role: "user", Content: "hello"},
			},
			MaxTokens:   64,
			Temperature: 0.0,
		}
		resp, err := p.Chat(ctx, req)
		if err != nil {
			t.Fatalf("Chat() returned unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Chat() returned nil response")
		}
		if resp.Content == "" {
			t.Error("Chat() response Content must not be empty")
		}
	})

	t.Run("ChatStream", func(t *testing.T) {
		req := provider.ChatRequest{
			Messages: []provider.Message{
				{Role: "user", Content: "hello"},
			},
			MaxTokens: 64,
			Stream:    true,
		}
		ch, err := p.ChatStream(ctx, req)
		if err != nil {
			t.Fatalf("ChatStream() returned unexpected error: %v", err)
		}
		if ch == nil {
			t.Fatal("ChatStream() returned nil channel")
		}

		var received int
		var gotDone bool
		for chunk := range ch {
			received++
			if chunk.Done {
				gotDone = true
				break
			}
		}
		if received == 0 {
			t.Error("ChatStream() channel must emit at least one chunk")
		}
		if !gotDone {
			t.Error("ChatStream() must emit a chunk with Done=true")
		}
	})

	t.Run("Embed", func(t *testing.T) {
		texts := []string{"judicial reasoning", "legal precedent"}
		req := provider.EmbedRequest{Texts: texts}
		resp, err := p.Embed(ctx, req)
		if err != nil {
			t.Fatalf("Embed() returned unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("Embed() returned nil response")
		}
		if len(resp.Embeddings) != len(texts) {
			t.Errorf("Embed() returned %d embeddings, want %d", len(resp.Embeddings), len(texts))
		}
		if resp.Dimensions <= 0 {
			t.Error("Embed() Dimensions must be positive")
		}
		for i, vec := range resp.Embeddings {
			if len(vec) != resp.Dimensions {
				t.Errorf("Embed() embedding[%d] has length %d, want Dimensions=%d", i, len(vec), resp.Dimensions)
			}
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		if err := p.HealthCheck(ctx); err != nil {
			t.Fatalf("HealthCheck() returned unexpected error: %v", err)
		}
	})
}
