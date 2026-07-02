package vectorindex

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/embedding"
)

// EmbedLeaves computes an embedding for every leaf's Text via svc.Embed,
// returning one VectorRecord per leaf in the same order. Embedding
// generation always goes through packages/embedding's EmbeddingService
// interface — this package never calls an LLM provider directly (see
// doc.go).
//
// Returns ErrNilEmbeddingService if svc is nil. An empty leaves slice
// returns an empty, non-nil slice and a nil error without calling svc.
func EmbedLeaves(ctx context.Context, svc embedding.EmbeddingService, leaves []IndexableLeaf) ([]VectorRecord, error) {
	if svc == nil {
		return nil, ErrNilEmbeddingService
	}
	if len(leaves) == 0 {
		return []VectorRecord{}, nil
	}

	texts := make([]string, len(leaves))
	for i, leaf := range leaves {
		texts[i] = leaf.Text
	}

	embedded, err := svc.Embed(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("vectorindex: embed leaves: %w", err)
	}
	if len(embedded) != len(leaves) {
		return nil, fmt.Errorf("vectorindex: embed leaves: expected %d results, got %d", len(leaves), len(embedded))
	}

	records := make([]VectorRecord, len(leaves))
	for i, leaf := range leaves {
		records[i] = recordFromLeaf(leaf, embedded[i])
	}
	return records, nil
}

// recordFromLeaf combines an IndexableLeaf with the embedding.EmbeddedText
// computed from its Text into a stored VectorRecord.
func recordFromLeaf(leaf IndexableLeaf, embedded embedding.EmbeddedText) VectorRecord {
	return VectorRecord{
		ID:               leaf.ID,
		NodeType:         leaf.NodeType,
		CaseID:           leaf.CaseID,
		JurisdictionCode: leaf.JurisdictionCode,
		CategoryCode:     leaf.CategoryCode,
		PartyID:          leaf.PartyID,
		Text:             leaf.Text,
		Vector:           embedded.Vector,
		SourceSpans:      leaf.SourceSpans,
		ModelID:          embedded.ModelID,
		ProviderID:       embedded.ProviderID,
		UpdatedAt:        embedded.CreatedAt,
	}
}
