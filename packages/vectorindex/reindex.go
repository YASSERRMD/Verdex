package vectorindex

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// ReindexOnRevision recomputes and upserts the vector index for the case
// identified by revision, in response to a new irac.TreeRevision being
// produced (e.g. by packages/treeassembly whenever a case's tree is
// reassembled).
//
// # Full re-embed, not delta-aware, for v1
//
// irac.TreeRevision (packages/irac/version.go) intentionally carries no
// node list — it is a lightweight pointer identifying "case X now has a
// new tree snapshot", not the snapshot's content. Because this package
// reads tree content through graph.GraphStore rather than holding its own
// copy, the only information ReindexOnRevision has to work with is "case X
// changed" — it cannot diff revision N against revision N-1 to find which
// specific leaves actually changed without either (a) this package taking
// on a dependency on packages/treeassembly's Tree/SnapshotStore to fetch
// both revisions' content, or (b) GraphStore exposing a revision-scoped
// read (it does not; GraphStore.Traverse always reads the current state).
//
// v1 therefore re-projects and re-embeds every current leaf in the case on
// every revision (delegating to IndexingService.IndexCase), rather than
// computing a delta. This is the same tradeoff packages/embedding's own
// Cache existing makes viable: EmbeddingService.Embed only recomputes a
// vector for text whose ContentHash is not already cached, so a full
// re-embed of an unchanged leaf is a cache hit, not a wasted provider call.
// A future revision-diffing optimization (comparing revision N's node set
// against N-1's, e.g. once packages/treeassembly's SnapshotStore is
// plumbed through here) can narrow this to a true delta without changing
// this function's signature or the VectorStore/EmbeddingService contracts
// it composes. See doc/vector-index.md.
//
// Returns ErrEmptyCaseID if revision.CaseID is empty. Returns the number
// of leaves re-indexed.
func ReindexOnRevision(ctx context.Context, svc IndexingService, revision irac.TreeRevision) (int, error) {
	if revision.CaseID == "" {
		return 0, ErrEmptyCaseID
	}

	n, err := svc.IndexCase(ctx, revision.CaseID)
	if err != nil {
		return 0, fmt.Errorf("vectorindex: reindex on revision %d for case %q: %w", revision.RevisionNumber, revision.CaseID, err)
	}
	return n, nil
}
