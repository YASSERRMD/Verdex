package embedding

import "time"

// EmbeddingVector is a dense numerical representation of a piece of text
// produced by an embedding model.  Cosine similarity is the standard distance
// metric.
type EmbeddingVector []float64

// EmbeddedText couples the original text with its computed vector and the
// metadata needed to understand which model produced it.
type EmbeddedText struct {
	// ContentHash is the cache key derived from the text and model.
	// See CacheKey for the derivation.
	ContentHash string

	// Text is the original input string that was embedded.
	Text string

	// Vector is the embedding produced by the model.
	Vector EmbeddingVector

	// Dimensions is len(Vector).  Stored explicitly so callers can validate
	// without allocating a slice copy.
	Dimensions int

	// ModelID identifies the embedding model (e.g. "text-embedding-3-small").
	ModelID string

	// ProviderID identifies the LLM provider (e.g. "openai").
	ProviderID string

	// Version is the monotonically increasing embedding schema version at the
	// time of creation.  A change in Version indicates the vector space has
	// shifted and downstream indices should be rebuilt.
	Version int

	// CreatedAt is the UTC wall-clock time at which the embedding was
	// computed or retrieved from cache.
	CreatedAt time.Time
}

// ChunkConfig controls how EmbedChunked splits a long document into
// independently-embeddable segments.
type ChunkConfig struct {
	// MaxTokens is the maximum number of whitespace-delimited tokens
	// (words) per chunk.  Must be > 0.
	MaxTokens int

	// Overlap is the number of tokens from the end of each chunk that are
	// repeated at the start of the following chunk.  Useful for sliding-
	// window retrieval.  Must be >= 0 and < MaxTokens.
	Overlap int

	// SplitOn is the preferred boundary character sequence to split on when
	// possible.  Common values are "\n\n" (paragraph), ". " (sentence), or
	// "" (any whitespace).  The chunker falls back to hard splits at
	// MaxTokens when no such boundary is found.
	SplitOn string
}
