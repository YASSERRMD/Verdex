package precedent

import (
	"context"
	"strings"

	"github.com/YASSERRMD/verdex/packages/embedding"
)

// EmbeddedPrecedent bundles a HierarchyRule with the embeddings computed
// for its holding+ratio text. A precedent's combined text may span
// multiple chunks (see embedding.EmbeddingService.EmbedChunked), so
// Embeddings is a slice rather than a single vector — retrieval consumers
// match query embeddings against every chunk and aggregate at query time.
type EmbeddedPrecedent struct {
	HierarchyRule
	Embeddings []embedding.EmbeddedText
}

// EmbedOptions configures EmbedPrecedents.
type EmbedOptions struct {
	// ChunkConfig controls how each precedent's text is split before
	// embedding. If zero-valued (ChunkConfig.MaxTokens <= 0),
	// embedding.EmbedChunked applies its own default chunking.
	ChunkConfig embedding.ChunkConfig
}

// embeddableText returns the text EmbedPrecedents sends to
// EmbeddingService.EmbedChunked for r: the precedent's Holding and
// RatioDecidendi concatenated, since retrieval over precedents should
// match on the court's determination and its reasoning rather than the
// full (often much longer, and noisier) judgment text.
func embeddableText(r HierarchyRule) string {
	return strings.TrimSpace(strings.TrimSpace(r.Holding) + " " + strings.TrimSpace(r.RatioDecidendi))
}

// EmbedPrecedents computes embeddings for every rule's holding+ratio text
// via svc's EmbedChunked, using the existing embedding.EmbeddingService —
// this package never computes or calls out to an embedding model
// directly. Returns the input rules wrapped as EmbeddedPrecedent, in the
// same order.
//
// Rules whose combined holding+ratio text is empty (after trimming), or
// when svc is nil, are skipped for embedding (no EmbedChunked call is
// made for them) but are still included in the returned slice with a nil
// Embeddings field, mirroring packages/statute's EmbedRules convention.
//
// Returns the first error encountered from svc.EmbedChunked, if any,
// alongside the partial results computed so far.
func EmbedPrecedents(ctx context.Context, svc embedding.EmbeddingService, rules []HierarchyRule, opts EmbedOptions) ([]EmbeddedPrecedent, error) {
	out := make([]EmbeddedPrecedent, 0, len(rules))
	for _, r := range rules {
		text := embeddableText(r)
		if text == "" || svc == nil {
			out = append(out, EmbeddedPrecedent{HierarchyRule: r})
			continue
		}

		vectors, err := svc.EmbedChunked(ctx, text, opts.ChunkConfig)
		if err != nil {
			out = append(out, EmbeddedPrecedent{HierarchyRule: r})
			return out, err
		}
		out = append(out, EmbeddedPrecedent{HierarchyRule: r, Embeddings: vectors})
	}
	return out, nil
}
