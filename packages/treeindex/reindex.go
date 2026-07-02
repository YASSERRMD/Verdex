package treeindex

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// ReindexOnRevision rebuilds the PathIndex for the case identified by
// revision, in response to a new irac.TreeRevision being produced (e.g.
// by packages/treeassembly whenever a case's tree is reassembled). This
// gives treeindex the same tree-revision-driven maintenance hook
// packages/vectorindex's own ReindexOnRevision provides, so a caller
// wiring both packages into a shared "tree changed" event can treat them
// uniformly.
//
// Like vectorindex's ReindexOnRevision, this is a full rebuild rather than
// a delta: irac.TreeRevision carries no node/edge list, only a pointer
// ("case X has a new snapshot"), so RebuildCase's full-rebuild tradeoff
// (see its doc comment) is inherited here unchanged.
//
// Returns ErrEmptyCaseID if revision.CaseID is empty.
func ReindexOnRevision(ctx context.Context, idx *Indexer, revision irac.TreeRevision) error {
	if revision.CaseID == "" {
		return ErrEmptyCaseID
	}

	if err := idx.RebuildCase(ctx, revision.CaseID); err != nil {
		return fmt.Errorf("treeindex: reindex on revision %d for case %q: %w", revision.RevisionNumber, revision.CaseID, err)
	}
	return nil
}
