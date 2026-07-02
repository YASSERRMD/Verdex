package vectorindex_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

func TestInMemoryVectorStore_UpsertValidation(t *testing.T) {
	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	if err := store.Upsert(ctx, vectorindex.VectorRecord{Vector: embedding.EmbeddingVector{1, 2}}); err != vectorindex.ErrEmptyRecordID {
		t.Errorf("expected ErrEmptyRecordID, got %v", err)
	}
	if err := store.Upsert(ctx, vectorindex.VectorRecord{ID: "a"}); err != vectorindex.ErrEmptyVector {
		t.Errorf("expected ErrEmptyVector, got %v", err)
	}
}

func TestInMemoryVectorStore_DimensionMismatch(t *testing.T) {
	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	if err := store.Upsert(ctx, vectorindex.VectorRecord{ID: "a", Vector: embedding.EmbeddingVector{1, 2, 3}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.Upsert(ctx, vectorindex.VectorRecord{ID: "b", Vector: embedding.EmbeddingVector{1, 2}}); err != vectorindex.ErrDimensionMismatch {
		t.Errorf("expected ErrDimensionMismatch, got %v", err)
	}

	_, err := store.Query(ctx, vectorindex.QueryRequest{Vector: embedding.EmbeddingVector{1, 2}})
	if err != vectorindex.ErrDimensionMismatch {
		t.Errorf("Query: expected ErrDimensionMismatch, got %v", err)
	}
}

// TestInMemoryVectorStore_RetrievalRecall seeds a known-similar and a
// known-dissimilar record relative to a query vector, and asserts the
// similar one ranks first — the core recall guarantee this package exists
// to provide.
func TestInMemoryVectorStore_RetrievalRecall(t *testing.T) {
	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	similar := vectorindex.VectorRecord{
		ID:       "similar",
		NodeType: irac.NodeFact,
		CaseID:   "case-1",
		Text:     "contract breach warranty",
		Vector:   embedding.EmbeddingVector{1, 1, 0, 0},
	}
	dissimilar := vectorindex.VectorRecord{
		ID:       "dissimilar",
		NodeType: irac.NodeFact,
		CaseID:   "case-1",
		Text:     "unrelated criminal procedure",
		Vector:   embedding.EmbeddingVector{0, 0, 1, 1},
	}
	orthogonalButCloser := vectorindex.VectorRecord{
		ID:       "somewhat-similar",
		NodeType: irac.NodeFact,
		CaseID:   "case-1",
		Text:     "contract only",
		Vector:   embedding.EmbeddingVector{1, 0, 0, 0},
	}

	for _, r := range []vectorindex.VectorRecord{similar, dissimilar, orthogonalButCloser} {
		if err := store.Upsert(ctx, r); err != nil {
			t.Fatalf("Upsert(%s): %v", r.ID, err)
		}
	}

	results, err := store.Query(ctx, vectorindex.QueryRequest{
		Vector: embedding.EmbeddingVector{1, 1, 0, 0}, // identical to "similar"
		TopK:   3,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Record.ID != "similar" {
		t.Fatalf("expected top result to be %q, got %q (score %f)", "similar", results[0].Record.ID, results[0].VectorScore)
	}
	if results[0].VectorScore < results[1].VectorScore || results[1].VectorScore < results[2].VectorScore {
		t.Errorf("expected descending scores, got %v, %v, %v", results[0].VectorScore, results[1].VectorScore, results[2].VectorScore)
	}
	if results[len(results)-1].Record.ID != "dissimilar" {
		t.Errorf("expected the least similar (%q) to rank last, got %q", "dissimilar", results[len(results)-1].Record.ID)
	}

	// GraphScore/CombinedScore are placeholders this package never
	// populates.
	for _, r := range results {
		if r.GraphScore != 0 {
			t.Errorf("expected GraphScore to be left at 0, got %f for %q", r.GraphScore, r.Record.ID)
		}
		if r.CombinedScore != 0 {
			t.Errorf("expected CombinedScore to be left at 0, got %f for %q", r.CombinedScore, r.Record.ID)
		}
	}
}

func TestInMemoryVectorStore_MetadataFiltering(t *testing.T) {
	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	records := []vectorindex.VectorRecord{
		{ID: "us-contract", CaseID: "case-1", JurisdictionCode: "us-ny", CategoryCode: "contract", PartyID: "plaintiff", Vector: embedding.EmbeddingVector{1, 0}},
		{ID: "uk-contract", CaseID: "case-1", JurisdictionCode: "uk", CategoryCode: "contract", PartyID: "plaintiff", Vector: embedding.EmbeddingVector{1, 0}},
		{ID: "us-tort", CaseID: "case-1", JurisdictionCode: "us-ny", CategoryCode: "tort", PartyID: "defendant", Vector: embedding.EmbeddingVector{1, 0}},
	}
	for _, r := range records {
		if err := store.Upsert(ctx, r); err != nil {
			t.Fatalf("Upsert(%s): %v", r.ID, err)
		}
	}

	// Filter by jurisdiction only.
	results, err := store.Query(ctx, vectorindex.QueryRequest{
		Vector: embedding.EmbeddingVector{1, 0},
		TopK:   10,
		Filter: vectorindex.MetadataFilter{JurisdictionCode: "us-ny"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 us-ny results, got %d", len(results))
	}

	// Filter by category AND party.
	results, err = store.Query(ctx, vectorindex.QueryRequest{
		Vector: embedding.EmbeddingVector{1, 0},
		TopK:   10,
		Filter: vectorindex.MetadataFilter{CategoryCode: "contract", PartyID: "plaintiff"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 contract/plaintiff results, got %d", len(results))
	}

	// Filter that matches nothing.
	results, err = store.Query(ctx, vectorindex.QueryRequest{
		Vector: embedding.EmbeddingVector{1, 0},
		TopK:   10,
		Filter: vectorindex.MetadataFilter{JurisdictionCode: "fr"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for a non-matching jurisdiction, got %d", len(results))
	}
}

func TestInMemoryVectorStore_CaseIDScoping(t *testing.T) {
	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	if err := store.Upsert(ctx, vectorindex.VectorRecord{ID: "a", CaseID: "case-1", Vector: embedding.EmbeddingVector{1, 0}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.Upsert(ctx, vectorindex.VectorRecord{ID: "b", CaseID: "case-2", Vector: embedding.EmbeddingVector{1, 0}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	results, err := store.Query(ctx, vectorindex.QueryRequest{
		Vector: embedding.EmbeddingVector{1, 0},
		TopK:   10,
		CaseID: "case-1",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 || results[0].Record.ID != "a" {
		t.Fatalf("expected only case-1's record, got %+v", results)
	}
}

func TestInMemoryVectorStore_TopKDefault(t *testing.T) {
	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{DefaultTopK: 2})
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		if err := store.Upsert(ctx, vectorindex.VectorRecord{ID: id, Vector: embedding.EmbeddingVector{1, 0}}); err != nil {
			t.Fatalf("Upsert(%s): %v", id, err)
		}
	}

	results, err := store.Query(ctx, vectorindex.QueryRequest{Vector: embedding.EmbeddingVector{1, 0}})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected DefaultTopK=2 results when TopK is unset, got %d", len(results))
	}
}

func TestInMemoryVectorStore_DeleteAndDeleteCase(t *testing.T) {
	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	if err := store.Upsert(ctx, vectorindex.VectorRecord{ID: "a", CaseID: "case-1", Vector: embedding.EmbeddingVector{1, 0}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.Upsert(ctx, vectorindex.VectorRecord{ID: "b", CaseID: "case-1", Vector: embedding.EmbeddingVector{1, 0}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.Upsert(ctx, vectorindex.VectorRecord{ID: "c", CaseID: "case-2", Vector: embedding.EmbeddingVector{1, 0}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if err := store.Delete(ctx, "a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if store.Len() != 2 {
		t.Fatalf("expected 2 records after Delete, got %d", store.Len())
	}

	// Deleting a non-existent id is not an error.
	if err := store.Delete(ctx, "does-not-exist"); err != nil {
		t.Fatalf("Delete of missing id: %v", err)
	}

	if err := store.DeleteCase(ctx, "case-1"); err != nil {
		t.Fatalf("DeleteCase: %v", err)
	}
	if store.Len() != 1 {
		t.Fatalf("expected 1 record after DeleteCase, got %d", store.Len())
	}

	if err := store.Delete(ctx, ""); err != vectorindex.ErrEmptyRecordID {
		t.Errorf("expected ErrEmptyRecordID, got %v", err)
	}
	if err := store.DeleteCase(ctx, ""); err != vectorindex.ErrEmptyCaseID {
		t.Errorf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestInMemoryVectorStore_QueryValidation(t *testing.T) {
	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	if _, err := store.Query(ctx, vectorindex.QueryRequest{}); err != vectorindex.ErrEmptyVector {
		t.Errorf("expected ErrEmptyVector, got %v", err)
	}
	if _, err := store.Query(ctx, vectorindex.QueryRequest{Vector: embedding.EmbeddingVector{1}, TopK: -1}); err != vectorindex.ErrInvalidTopK {
		t.Errorf("expected ErrInvalidTopK, got %v", err)
	}
}

func TestInMemoryVectorStore_Health(t *testing.T) {
	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	if err := vectorindex.HealthCheck(context.Background(), store); err != nil {
		t.Errorf("expected healthy in-memory store, got %v", err)
	}
}

func TestHealthCheck_NilStore(t *testing.T) {
	if err := vectorindex.HealthCheck(context.Background(), nil); err == nil {
		t.Error("expected an error for a nil store")
	}
}

func TestIndexConfig_WithDefaults(t *testing.T) {
	cfg := vectorindex.IndexConfig{}.WithDefaults()
	if cfg.Metric != vectorindex.MetricCosine {
		t.Errorf("Metric = %q, want %q", cfg.Metric, vectorindex.MetricCosine)
	}
	if cfg.DefaultTopK != vectorindex.DefaultTopKValue {
		t.Errorf("DefaultTopK = %d, want %d", cfg.DefaultTopK, vectorindex.DefaultTopKValue)
	}

	explicit := vectorindex.IndexConfig{Metric: vectorindex.MetricDotProduct, DefaultTopK: 5}.WithDefaults()
	if explicit.Metric != vectorindex.MetricDotProduct {
		t.Errorf("explicit Metric was overwritten: got %q", explicit.Metric)
	}
	if explicit.DefaultTopK != 5 {
		t.Errorf("explicit DefaultTopK was overwritten: got %d", explicit.DefaultTopK)
	}
}

func TestMetric_IsValid(t *testing.T) {
	for _, m := range []vectorindex.Metric{vectorindex.MetricCosine, vectorindex.MetricDotProduct, vectorindex.MetricEuclidean} {
		if !m.IsValid() {
			t.Errorf("expected %q to be valid", m)
		}
	}
	if vectorindex.Metric("bogus").IsValid() {
		t.Error("expected an unrecognized metric to be invalid")
	}
}

// ensure the fake embedding service used elsewhere in this package's tests
// satisfies embedding.EmbeddingService at compile time.
var _ embedding.EmbeddingService = (*fakeEmbeddingService)(nil)
