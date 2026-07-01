// Package embedding provides a provider-agnostic embedding service for the
// Verdex judicial reasoning platform.
//
// The service wraps any [provider.LLMProvider] that supports the Embed
// operation and adds:
//   - Content-addressed caching via [Cache] to avoid redundant round-trips.
//   - Configurable batching so large input sets are split into chunks the
//     upstream provider can handle.
//   - Text chunking with configurable max-token size and overlap so long
//     documents can be embedded without hitting model token limits.
//   - Version tracking via [VersionRegistry] so callers can detect when the
//     underlying model changes and trigger re-embedding workflows.
//   - Structured metrics via [MetricsSink] for observability.
//
// Quick start:
//
//	cache := embedding.NewInMemoryCache()
//	reg   := embedding.NewInMemoryVersionRegistry()
//	svc   := embedding.NewEmbeddingService(myProvider, cache,
//	            embedding.WithBatchSize(32),
//	            embedding.WithVersionRegistry(reg),
//	         )
//	texts, err := svc.Embed(ctx, []string{"legal clause one", "legal clause two"})
package embedding
