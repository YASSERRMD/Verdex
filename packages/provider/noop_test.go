package provider_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// TestNoOpProvider_ConformanceTest verifies that NoOpProvider passes the
// full provider conformance suite.
func TestNoOpProvider_ConformanceTest(t *testing.T) {
	p := provider.DefaultNoOpProvider()
	ProviderConformanceTest(t, p)
}

// TestNoOpProvider_CustomContent verifies that FixedContent is reflected in
// the Chat response.
func TestNoOpProvider_CustomContent(t *testing.T) {
	p := &provider.NoOpProvider{
		FixedContent:    "custom judicial reasoning response",
		EmbedDimensions: 8,
	}

	req := provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "summarise the case"},
		},
		MaxTokens: 128,
	}

	resp, err := p.Chat(t.Context(), req)
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}
	if resp.Content != "custom judicial reasoning response" {
		t.Errorf("Chat() Content = %q, want %q", resp.Content, "custom judicial reasoning response")
	}
}

// TestNoOpProvider_EmbedDimensions verifies that the returned embeddings match
// the configured EmbedDimensions.
func TestNoOpProvider_EmbedDimensions(t *testing.T) {
	const wantDims = 16
	p := &provider.NoOpProvider{
		FixedContent:    "noop response",
		EmbedDimensions: wantDims,
	}

	texts := []string{"legal text one", "legal text two", "legal text three"}
	resp, err := p.Embed(t.Context(), provider.EmbedRequest{Texts: texts})
	if err != nil {
		t.Fatalf("Embed() unexpected error: %v", err)
	}
	if resp.Dimensions != wantDims {
		t.Errorf("Embed() Dimensions = %d, want %d", resp.Dimensions, wantDims)
	}
	for i, vec := range resp.Embeddings {
		if len(vec) != wantDims {
			t.Errorf("Embed() embedding[%d] length = %d, want %d", i, len(vec), wantDims)
		}
	}
}

// TestNoOpProvider_EmptyTexts_ReturnsError verifies that Embed rejects an
// empty Texts slice.
func TestNoOpProvider_EmptyTexts_ReturnsError(t *testing.T) {
	p := provider.DefaultNoOpProvider()

	_, err := p.Embed(t.Context(), provider.EmbedRequest{Texts: []string{}})
	if err == nil {
		t.Fatal("Embed() with empty Texts expected error, got nil")
	}
}
