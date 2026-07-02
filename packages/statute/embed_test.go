package statute

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
)

// fakeEmbeddingService is a deterministic, no-op-provider test double for
// embedding.EmbeddingService. It never calls any real embedding provider:
// EmbedChunked splits on whitespace-run boundaries via a trivial fixed
// chunk size and Embed produces a fixed-length vector derived only from
// each text's length, so results are fully deterministic across runs.
type fakeEmbeddingService struct {
	embedCalls        int
	embedChunkedCalls int
	failOn            string // if non-empty, EmbedChunked returns an error when text contains this substring
}

var errFakeEmbeddingFailure = fmt.Errorf("fakeEmbeddingService: forced failure")

func (f *fakeEmbeddingService) Embed(ctx context.Context, texts []string) ([]embedding.EmbeddedText, error) {
	f.embedCalls++
	out := make([]embedding.EmbeddedText, len(texts))
	for i, text := range texts {
		out[i] = embedding.EmbeddedText{
			ContentHash: fmt.Sprintf("hash-%d-%d", len(text), i),
			Text:        text,
			Vector:      embedding.EmbeddingVector{float64(len(text)), 0, 1},
			Dimensions:  3,
			ModelID:     "fake-model",
			ProviderID:  "fake-provider",
			Version:     1,
			CreatedAt:   time.Unix(0, 0).UTC(),
		}
	}
	return out, nil
}

func (f *fakeEmbeddingService) EmbedChunked(ctx context.Context, text string, cfg embedding.ChunkConfig) ([]embedding.EmbeddedText, error) {
	f.embedChunkedCalls++
	if f.failOn != "" && contains(text, f.failOn) {
		return nil, errFakeEmbeddingFailure
	}
	// Deterministic "chunking": treat the whole text as a single chunk
	// regardless of cfg, since this test double only needs to prove
	// EmbedChunked was invoked with the rule's text, not exercise real
	// chunking logic (that's packages/embedding's own responsibility).
	return f.Embed(ctx, []string{text})
}

func (f *fakeEmbeddingService) Invalidate(ctx context.Context, contentHash string) error {
	return nil
}

func (f *fakeEmbeddingService) ModelVersion() string {
	return "fake-provider/fake-model/v1"
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

var _ embedding.EmbeddingService = (*fakeEmbeddingService)(nil)

func amendedRulesFixture(t *testing.T) []AmendedRule {
	t.Helper()
	act := syntheticAct(t)
	built, err := BuildRuleNodes(act, RuleBuildOptions{CaseID: "statute:AE"})
	if err != nil {
		t.Fatalf("BuildRuleNodes() error = %v", err)
	}
	tagged := TagRules(built, TagOptions{CategoryCode: "civil"})
	return ApplyAmendments(tagged, nil)
}

func TestEmbedRules(t *testing.T) {
	rules := amendedRulesFixture(t)
	fake := &fakeEmbeddingService{}

	embedded, err := EmbedRules(context.Background(), fake, rules, EmbedOptions{})
	if err != nil {
		t.Fatalf("EmbedRules() error = %v", err)
	}
	if len(embedded) != len(rules) {
		t.Fatalf("len(embedded) = %d, want %d", len(embedded), len(rules))
	}
	if fake.embedChunkedCalls != len(rules) {
		t.Errorf("embedChunkedCalls = %d, want %d", fake.embedChunkedCalls, len(rules))
	}
	for i, e := range embedded {
		if len(e.Embeddings) == 0 {
			t.Errorf("embedded[%d].Embeddings is empty", i)
		}
		if e.Embeddings[0].ModelID != "fake-model" {
			t.Errorf("ModelID = %q, want fake-model", e.Embeddings[0].ModelID)
		}
	}
}

func TestEmbedRules_NilService_SkipsEmbedding(t *testing.T) {
	rules := amendedRulesFixture(t)
	embedded, err := EmbedRules(context.Background(), nil, rules, EmbedOptions{})
	if err != nil {
		t.Fatalf("EmbedRules() error = %v", err)
	}
	if len(embedded) != len(rules) {
		t.Fatalf("len(embedded) = %d, want %d", len(embedded), len(rules))
	}
	for i, e := range embedded {
		if e.Embeddings != nil {
			t.Errorf("embedded[%d].Embeddings = %v, want nil when service is nil", i, e.Embeddings)
		}
	}
}

func TestEmbedRules_EmptyTextSkipped(t *testing.T) {
	rules := amendedRulesFixture(t)
	rules[0].Node.Text = "   "
	fake := &fakeEmbeddingService{}

	embedded, err := EmbedRules(context.Background(), fake, rules, EmbedOptions{})
	if err != nil {
		t.Fatalf("EmbedRules() error = %v", err)
	}
	if embedded[0].Embeddings != nil {
		t.Errorf("embedded[0].Embeddings = %v, want nil for blank text", embedded[0].Embeddings)
	}
	if fake.embedChunkedCalls != len(rules)-1 {
		t.Errorf("embedChunkedCalls = %d, want %d (one skipped)", fake.embedChunkedCalls, len(rules)-1)
	}
}

func TestEmbedRules_PropagatesError(t *testing.T) {
	rules := amendedRulesFixture(t)
	fake := &fakeEmbeddingService{failOn: rules[0].Node.Text}

	_, err := EmbedRules(context.Background(), fake, rules, EmbedOptions{})
	if err == nil {
		t.Fatal("EmbedRules() error = nil, want error")
	}
}
