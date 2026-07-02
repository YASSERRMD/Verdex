package knowledgeisolation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

func TestNewCaseScopedVectorStore_Validation(t *testing.T) {
	t.Parallel()

	if _, err := knowledgeisolation.NewCaseScopedVectorStore(nil, "case-a", nil); !errors.Is(err, knowledgeisolation.ErrNilStore) {
		t.Fatalf("expected ErrNilStore, got %v", err)
	}

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	if _, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "", nil); !errors.Is(err, knowledgeisolation.ErrEmptyCaseID) {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestCaseScopedVectorStore_Query_ForcesAuthorizedCaseID(t *testing.T) {
	t.Parallel()

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	recA := vectorRecordFor("fact-a", irac.NodeFact, "case-a")
	recB := vectorRecordFor("fact-b", irac.NodeFact, "case-b")
	if err := inner.Upsert(ctx, recA); err != nil {
		t.Fatalf("Upsert a: %v", err)
	}
	if err := inner.Upsert(ctx, recB); err != nil {
		t.Fatalf("Upsert b: %v", err)
	}

	store, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	// Even with no CaseID set in the request at all, results must be
	// scoped to case-a.
	results, err := store.Query(ctx, vectorindex.QueryRequest{Vector: embedding.EmbeddingVector{1, 0, 0}, TopK: 10})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 || results[0].Record.ID != "fact-a" {
		t.Fatalf("expected only fact-a, got %+v", results)
	}
}

func TestCaseScopedVectorStore_Upsert_AllowsSharedLaw(t *testing.T) {
	t.Parallel()

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	store, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	sharedRule := vectorRecordFor("rule-1", irac.NodeRule, "case-b")
	if err := store.Upsert(context.Background(), sharedRule); err != nil {
		t.Fatalf("expected shared-law upsert to succeed, got %v", err)
	}
}

func TestCaseScopedVectorStore_DeleteCase_RejectsForeignCase(t *testing.T) {
	t.Parallel()

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	store, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	err = store.DeleteCase(context.Background(), "case-b")
	if !errors.Is(err, knowledgeisolation.ErrCrossCaseAccess) {
		t.Fatalf("expected ErrCrossCaseAccess, got %v", err)
	}
}

func TestCaseScopedVectorStore_DeleteCase_OwnCase(t *testing.T) {
	t.Parallel()

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	store, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	if err := store.Upsert(context.Background(), vectorRecordFor("fact-1", irac.NodeFact, "case-a")); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.DeleteCase(context.Background(), "case-a"); err != nil {
		t.Fatalf("expected own-case DeleteCase to succeed, got %v", err)
	}
}

func TestCaseScopedVectorStore_DelegatesDeleteAndHealth(t *testing.T) {
	t.Parallel()

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	store, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	ctx := context.Background()
	if err := store.Upsert(ctx, vectorRecordFor("fact-1", irac.NodeFact, "case-a")); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := store.Delete(ctx, "fact-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := store.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestCaseScopedVectorStore_CaseID(t *testing.T) {
	t.Parallel()

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	store, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}
	if got := store.CaseID(); got != "case-a" {
		t.Fatalf("CaseID() = %q, want case-a", got)
	}
}
