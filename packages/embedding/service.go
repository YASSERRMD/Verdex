package embedding

import "context"

// EmbeddingService is the top-level contract for computing, caching, and
// versioning embeddings in Verdex.
//
// All implementations MUST be safe for concurrent use.
type EmbeddingService interface {
	// Embed returns an [EmbeddedText] for each element of texts in the same
	// order.  Results that exist in the cache are returned without calling
	// the upstream provider.  New embeddings are stored in the cache before
	// being returned.
	//
	// Texts that exceed the provider's token limit should be pre-split with
	// [EmbedChunked].
	Embed(ctx context.Context, texts []string) ([]EmbeddedText, error)

	// EmbedChunked splits text according to cfg and then calls [Embed] on
	// each chunk.  The returned slice contains one [EmbeddedText] per chunk
	// in document order.
	EmbedChunked(ctx context.Context, text string, cfg ChunkConfig) ([]EmbeddedText, error)

	// Invalidate removes the cached embedding for the given contentHash so
	// that the next [Embed] call recomputes it.  It is a no-op when the
	// hash is absent from the cache.
	Invalidate(ctx context.Context, contentHash string) error

	// ModelVersion returns a human-readable string that uniquely identifies
	// the current embedding model and provider, e.g. "openai/text-embedding-3-small/v1".
	// Callers can persist this value and compare it on subsequent runs to
	// detect when re-embedding is required.
	ModelVersion() string
}
