package vectorindex_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// fakeEmbeddingService is a deterministic, hash-free test double for
// embedding.EmbeddingService. It never calls any provider: each text is
// mapped to a small bag-of-words vector over a fixed vocabulary, so texts
// sharing vocabulary produce a high cosine similarity and texts sharing
// none produce a near-zero (orthogonal) similarity — exactly the property
// the recall tests in inmemory_test.go and service_test.go need, without
// this test file depending on packages/provider.
type fakeEmbeddingService struct {
	vocab []string
}

// vectorFor deterministically maps text to a fixed-length vector: one
// dimension per vocabulary word, set to 1 if the word (case-insensitively)
// appears in text, plus one trailing "bias" dimension so an empty overlap
// never produces an all-zero vector.
func (f *fakeEmbeddingService) vectorFor(text string) embedding.EmbeddingVector {
	lower := strings.ToLower(text)
	vec := make(embedding.EmbeddingVector, len(f.vocab)+1)
	for i, word := range f.vocab {
		if strings.Contains(lower, word) {
			vec[i] = 1
		}
	}
	vec[len(f.vocab)] = 0.01
	return vec
}

// Embed implements embedding.EmbeddingService.
func (f *fakeEmbeddingService) Embed(_ context.Context, texts []string) ([]embedding.EmbeddedText, error) {
	out := make([]embedding.EmbeddedText, len(texts))
	for i, text := range texts {
		vec := f.vectorFor(text)
		out[i] = embedding.EmbeddedText{
			ContentHash: fmt.Sprintf("fake-%d", i),
			Text:        text,
			Vector:      vec,
			Dimensions:  len(vec),
			ModelID:     "fake-model",
			ProviderID:  "fake-provider",
			Version:     1,
			CreatedAt:   time.Unix(0, 0).UTC(),
		}
	}
	return out, nil
}

// EmbedChunked implements embedding.EmbeddingService.
func (f *fakeEmbeddingService) EmbedChunked(ctx context.Context, text string, _ embedding.ChunkConfig) ([]embedding.EmbeddedText, error) {
	return f.Embed(ctx, []string{text})
}

// Invalidate implements embedding.EmbeddingService.
func (f *fakeEmbeddingService) Invalidate(context.Context, string) error {
	return nil
}

// ModelVersion implements embedding.EmbeddingService.
func (f *fakeEmbeddingService) ModelVersion() string {
	return "fake-provider/fake-model/v1"
}

// newFakeEmbeddingService constructs a fakeEmbeddingService over vocab.
func newFakeEmbeddingService(vocab ...string) *fakeEmbeddingService {
	return &fakeEmbeddingService{vocab: vocab}
}

// mustCreateNode creates node in store, failing the test on error.
func mustCreateNode(tb testingTB, store graph.GraphStore, node irac.Node) {
	tb.Helper()
	if err := store.CreateNode(context.Background(), node); err != nil {
		tb.Fatalf("CreateNode(%s): %v", node.ID, err)
	}
}

// testingTB is the minimal subset of testing.TB this file needs, avoiding
// an import of the "testing" package itself in a file that only defines
// helpers (kept separate from _test.go files that call testing.T directly,
// per this repo's existing helpers_test.go convention in sibling
// packages).
type testingTB interface {
	Helper()
	Fatalf(format string, args ...interface{})
}
