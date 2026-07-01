package embedding

import "errors"

// Sentinel errors returned by the embedding package.  Callers should use
// errors.Is to test for these values.
var (
	// ErrEmbeddingFailed is returned when the upstream provider returns an
	// error during an embedding call.
	ErrEmbeddingFailed = errors.New("embedding: provider embedding call failed")

	// ErrCacheMiss is returned by [Cache.Get] when no entry exists for the
	// requested hash.
	ErrCacheMiss = errors.New("embedding: cache miss")

	// ErrTextTooLong is returned when a single text segment exceeds the
	// provider's token limit and cannot be split further.
	ErrTextTooLong = errors.New("embedding: text exceeds maximum token limit")

	// ErrEmptyInput is returned when an empty slice of texts or an empty
	// string is passed to [EmbeddingService.Embed] or
	// [EmbeddingService.EmbedChunked].
	ErrEmptyInput = errors.New("embedding: input must not be empty")

	// ErrProviderUnsupported is returned when the configured
	// [provider.LLMProvider] does not report [provider.TaskEmbed] in its
	// Capabilities.
	ErrProviderUnsupported = errors.New("embedding: provider does not support embedding")
)
