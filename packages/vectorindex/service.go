package vectorindex

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/graph"
)

// IndexingService orchestrates the full pipeline this package exists to
// run: project a case's indexable leaves out of a GraphStore, embed them
// via an EmbeddingService, and upsert the resulting VectorRecords into a
// VectorStore. This mirrors packages/treeassembly's TreeAssemblyService
// orchestration pattern — a single entry point wiring together this
// package's otherwise independently usable pieces (ProjectLeaves,
// EmbedLeaves, VectorStore.Upsert).
//
// A tree should generally pass packages/treevalidation's CanFinalize gate
// before being indexed for retrieval — an unvalidated tree may contain
// orphaned or unsupported claims that are misleading to surface in a
// semantic-recall result. This package does not enforce that gate itself:
// indexing is infrastructure, not a workflow decision, so CanFinalize
// remains a caller concern (see doc/vector-index.md).
type IndexingService struct {
	// Graph is the source of truth for tree content. Required.
	Graph graph.GraphStore

	// Embeddings computes vectors for leaf text. Required.
	Embeddings embedding.EmbeddingService

	// Vectors is the destination store for computed VectorRecords.
	// Required.
	Vectors VectorStore

	// ProjectionOptions supplies the metadata-filter values ProjectLeaves
	// cannot derive from a GraphStore's base irac.Node shape (see
	// projection.go).
	ProjectionOptions ProjectionOptions
}

// validate checks that every required dependency is set.
func (s IndexingService) validate() error {
	if s.Graph == nil {
		return ErrNilGraphStore
	}
	if s.Embeddings == nil {
		return ErrNilEmbeddingService
	}
	if s.Vectors == nil {
		return ErrNilVectorStore
	}
	return nil
}

// IndexCase runs the full project -> embed -> upsert pipeline for caseID:
// it projects caseID's indexable leaves from s.Graph, embeds their text via
// s.Embeddings, and upserts the resulting records into s.Vectors. Returns
// the number of leaves indexed.
//
// IndexCase always re-embeds and re-upserts every leaf currently in the
// tree (a full re-index, not a delta) — see ReindexOnRevision for the
// tree-revision-aware entry point, and doc/vector-index.md for the
// documented full-vs-delta tradeoff this phase accepts for v1.
func (s IndexingService) IndexCase(ctx context.Context, caseID string) (int, error) {
	if err := s.validate(); err != nil {
		return 0, err
	}
	if caseID == "" {
		return 0, ErrEmptyCaseID
	}

	leaves, err := ProjectLeaves(ctx, s.Graph, caseID, s.ProjectionOptions)
	if err != nil {
		return 0, fmt.Errorf("vectorindex: index case %q: project leaves: %w", caseID, err)
	}

	records, err := EmbedLeaves(ctx, s.Embeddings, leaves)
	if err != nil {
		return 0, fmt.Errorf("vectorindex: index case %q: embed leaves: %w", caseID, err)
	}

	for _, record := range records {
		if err := s.Vectors.Upsert(ctx, record); err != nil {
			return 0, fmt.Errorf("vectorindex: index case %q: upsert record %q: %w", caseID, record.ID, err)
		}
	}

	return len(records), nil
}
