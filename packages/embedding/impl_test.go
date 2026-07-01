package embedding_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// countingProvider wraps NoOpProvider and records how many times Embed is
// called so tests can assert batching behaviour.
type countingProvider struct {
	*provider.NoOpProvider
	embedCalls int64
}

func (c *countingProvider) Embed(ctx context.Context, req provider.EmbedRequest) (*provider.EmbedResponse, error) {
	atomic.AddInt64(&c.embedCalls, 1)
	return c.NoOpProvider.Embed(ctx, req)
}

func newCountingProvider(dims int) *countingProvider {
	p := provider.DefaultNoOpProvider()
	p.EmbedDimensions = dims
	return &countingProvider{NoOpProvider: p}
}

// --- cache-hit tests ---

func TestEmbed_CacheHitSkipsProvider(t *testing.T) {
	ctx := context.Background()
	cp := newCountingProvider(4)
	cache := embedding.NewInMemoryCache()
	svc := embedding.NewEmbeddingService(cp, cache)

	texts := []string{"legal clause alpha"}

	// First call: cache miss → provider is called.
	results, err := svc.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("first Embed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if atomic.LoadInt64(&cp.embedCalls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", cp.embedCalls)
	}

	// Second call with same text: cache hit → provider must NOT be called again.
	_, err = svc.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("second Embed: %v", err)
	}
	if atomic.LoadInt64(&cp.embedCalls) != 1 {
		t.Errorf("expected provider still called 1 time, got %d", cp.embedCalls)
	}
}

// --- cache-miss tests ---

func TestEmbed_CacheMissCallsProvider(t *testing.T) {
	ctx := context.Background()
	cp := newCountingProvider(4)
	cache := embedding.NewInMemoryCache()
	svc := embedding.NewEmbeddingService(cp, cache)

	texts := []string{"text one", "text two", "text three"}
	results, err := svc.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(results) != len(texts) {
		t.Errorf("expected %d results, got %d", len(texts), len(results))
	}
	if atomic.LoadInt64(&cp.embedCalls) < 1 {
		t.Error("expected at least one provider call")
	}
	// Cache should now hold all three texts.
	if cache.Len() != len(texts) {
		t.Errorf("expected cache size %d, got %d", len(texts), cache.Len())
	}
}

// --- batching tests ---

func TestEmbed_BatchingGroupsCorrectly(t *testing.T) {
	ctx := context.Background()
	cp := newCountingProvider(4)
	cache := embedding.NewInMemoryCache()
	// batchSize of 2 with 5 texts should produce ceil(5/2)=3 provider calls.
	svc := embedding.NewEmbeddingService(cp, cache, embedding.WithBatchSize(2))

	texts := []string{"a", "b", "c", "d", "e"}
	results, err := svc.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(results) != len(texts) {
		t.Errorf("expected %d results, got %d", len(texts), len(results))
	}

	const wantBatches = int64(3) // ceil(5/2)
	if got := atomic.LoadInt64(&cp.embedCalls); got != wantBatches {
		t.Errorf("expected %d batch calls, got %d", wantBatches, got)
	}
}

// --- chunker tests ---

func TestEmbedChunked_SplitsAtMaxTokens(t *testing.T) {
	ctx := context.Background()
	cp := newCountingProvider(4)
	cache := embedding.NewInMemoryCache()
	svc := embedding.NewEmbeddingService(cp, cache)

	// 20 words; MaxTokens=5 should produce 4 chunks.
	text := "one two three four five six seven eight nine ten " +
		"eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty"
	cfg := embedding.ChunkConfig{MaxTokens: 5, Overlap: 0}

	chunks, err := svc.EmbedChunked(ctx, text, cfg)
	if err != nil {
		t.Fatalf("EmbedChunked: %v", err)
	}
	if len(chunks) != 4 {
		t.Errorf("expected 4 chunks, got %d", len(chunks))
	}
}

// --- version-change re-embed test ---

func TestEmbed_ReEmbedTriggeredOnVersionChange(t *testing.T) {
	ctx := context.Background()
	cp := newCountingProvider(4)
	cache := embedding.NewInMemoryCache()
	reg := embedding.NewInMemoryVersionRegistry()

	svc := embedding.NewEmbeddingService(cp, cache,
		embedding.WithVersionRegistry(reg),
	)

	// Register initial version.
	if err := reg.RecordVersion(ctx, embedding.EmbeddingVersion{
		ModelID: "noop-v1", ProviderID: "noop", Dimensions: 4,
	}); err != nil {
		t.Fatalf("RecordVersion: %v", err)
	}

	text := "judicial precedent clause"
	results, err := svc.Embed(ctx, []string{text})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	firstVersion := results[0].Version

	// Simulate a model upgrade by recording a new version with different dimensions.
	if err := reg.RecordVersion(ctx, embedding.EmbeddingVersion{
		ModelID: "noop-v2", ProviderID: "noop", Dimensions: 8,
	}); err != nil {
		t.Fatalf("RecordVersion v2: %v", err)
	}

	cur, _ := reg.CurrentVersion(ctx)
	if cur.Version <= firstVersion {
		t.Errorf("registry version should have incremented; got %d, want > %d", cur.Version, firstVersion)
	}

	// NeedsReEmbed should report true for the old embedding.
	needs, err := embedding.NeedsReEmbed(ctx, reg, results[0])
	if err != nil {
		t.Fatalf("NeedsReEmbed: %v", err)
	}
	if !needs {
		t.Error("expected NeedsReEmbed to return true after version bump")
	}
}

// --- empty input ---

func TestEmbed_EmptyInputReturnsError(t *testing.T) {
	ctx := context.Background()
	cp := newCountingProvider(4)
	cache := embedding.NewInMemoryCache()
	svc := embedding.NewEmbeddingService(cp, cache)

	_, err := svc.Embed(ctx, nil)
	if !errors.Is(err, embedding.ErrEmptyInput) {
		t.Errorf("expected ErrEmptyInput, got %v", err)
	}

	_, err = svc.EmbedChunked(ctx, "", embedding.ChunkConfig{MaxTokens: 10})
	if !errors.Is(err, embedding.ErrEmptyInput) {
		t.Errorf("expected ErrEmptyInput for empty string, got %v", err)
	}
}

// --- invalidate ---

func TestInvalidate_ForcesProviderCallOnNextEmbed(t *testing.T) {
	ctx := context.Background()
	cp := newCountingProvider(4)
	cache := embedding.NewInMemoryCache()
	svc := embedding.NewEmbeddingService(cp, cache)

	text := "clause to invalidate"
	// Warm cache.
	if _, err := svc.Embed(ctx, []string{text}); err != nil {
		t.Fatal(err)
	}
	before := atomic.LoadInt64(&cp.embedCalls)

	// Invalidate.
	hash := embedding.CacheKey(text, "noop-v1")
	if err := svc.Invalidate(ctx, hash); err != nil {
		t.Fatal(err)
	}

	// Re-embed: should call provider again.
	if _, err := svc.Embed(ctx, []string{text}); err != nil {
		t.Fatal(err)
	}
	after := atomic.LoadInt64(&cp.embedCalls)
	if after <= before {
		t.Errorf("expected additional provider call after invalidation; before=%d after=%d", before, after)
	}
}
