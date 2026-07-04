package corpusupdater

import "context"

// Embedder is a small interface shaped like
// packages/embedding.EmbeddingService's own Embed method: given a
// batch of texts, return one opaque vector per text in the same order.
// Engine.ApplyAmendment calls Embed exactly once per changed rule/
// precedent after its text is written (task 4), so a corpus's
// retrieval index never drifts from its current text.
//
// This package deliberately does not import packages/embedding: a
// production caller wires an adapter around
// embedding.EmbeddingService.Embed (or EmbedChunked, chunking first)
// that satisfies this interface, keeping this package's dependency
// footprint thin exactly as packages/statute's EmbedRules keeps its
// own embedding call-out behind the same real interface rather than a
// reimplementation.
type Embedder interface {
	// Embed computes and stores (or otherwise records) an embedding for
	// each of texts, returning an error if any computation fails. The
	// returned vectors themselves are not consumed by this package --
	// Engine only needs to know embedding happened, not the resulting
	// values -- so implementations are free to return nil alongside a
	// nil error.
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}
