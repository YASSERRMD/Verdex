package statute

import (
	"context"
	"strings"

	"github.com/YASSERRMD/verdex/packages/embedding"
)

// EmbeddedRule bundles an AmendedRule with the embeddings computed for
// its rule text. A rule's text may span multiple chunks (see
// embedding.EmbeddingService.EmbedChunked), so Embeddings is a slice
// rather than a single vector — retrieval consumers match query
// embeddings against every chunk and aggregate at query time.
type EmbeddedRule struct {
	AmendedRule
	Embeddings []embedding.EmbeddedText
}

// EmbedOptions configures EmbedRules.
type EmbedOptions struct {
	// ChunkConfig controls how each rule's text is split before
	// embedding. If zero-valued (ChunkConfig.MaxTokens <= 0),
	// embedding.EmbedChunked applies its own default chunking.
	ChunkConfig embedding.ChunkConfig
}

// EmbedRules computes embeddings for every rule's text via svc's
// EmbedChunked, using the existing embedding.EmbeddingService — this
// package never computes or calls out to an embedding model directly.
// Returns the input rules wrapped as EmbeddedRule, in the same order.
//
// Rules with empty (after trimming) text are skipped for embedding (no
// EmbedChunked call is made for them) but are still included in the
// returned slice with a nil Embeddings field, since an empty statute
// node's text is a data-quality issue for an earlier pipeline stage to
// catch, not a reason to fail the whole batch.
//
// Returns the first error encountered from svc.EmbedChunked, if any,
// alongside the partial results computed so far.
func EmbedRules(ctx context.Context, svc embedding.EmbeddingService, rules []AmendedRule, opts EmbedOptions) ([]EmbeddedRule, error) {
	out := make([]EmbeddedRule, 0, len(rules))
	for _, r := range rules {
		text := strings.TrimSpace(r.Node.Text)
		if text == "" || svc == nil {
			out = append(out, EmbeddedRule{AmendedRule: r})
			continue
		}

		vectors, err := svc.EmbedChunked(ctx, r.Node.Text, opts.ChunkConfig)
		if err != nil {
			out = append(out, EmbeddedRule{AmendedRule: r})
			return out, err
		}
		out = append(out, EmbeddedRule{AmendedRule: r, Embeddings: vectors})
	}
	return out, nil
}
