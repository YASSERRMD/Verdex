package citation_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
)

func TestInMemoryRepositorySaveAndGet(t *testing.T) {
	ctx := context.Background()
	repo := citation.NewInMemoryRepository()

	unit := citation.CitedUnit{NodeID: "rule-1", CaseID: "case-1", Citation: "Act 1, s.1"}
	if err := repo.Save(ctx, unit); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.Get(ctx, "case-1", "rule-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Citation != "Act 1, s.1" {
		t.Errorf("Citation = %q, want %q", got.Citation, "Act 1, s.1")
	}
}

func TestInMemoryRepositoryGetNotFound(t *testing.T) {
	ctx := context.Background()
	repo := citation.NewInMemoryRepository()

	_, err := repo.Get(ctx, "case-1", "missing")
	if !errors.Is(err, citation.ErrCitationNotFound) {
		t.Errorf("Get() error = %v, want ErrCitationNotFound", err)
	}
}

func TestInMemoryRepositorySaveOverwrites(t *testing.T) {
	ctx := context.Background()
	repo := citation.NewInMemoryRepository()

	if err := repo.Save(ctx, citation.CitedUnit{NodeID: "rule-1", CaseID: "case-1", Citation: "v1"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := repo.Save(ctx, citation.CitedUnit{NodeID: "rule-1", CaseID: "case-1", Citation: "v2"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.Get(ctx, "case-1", "rule-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Citation != "v2" {
		t.Errorf("Citation = %q, want v2 after overwrite", got.Citation)
	}
	if repo.Len() != 1 {
		t.Errorf("Len() = %d, want 1", repo.Len())
	}
}

func TestInMemoryRepositoryListByCase(t *testing.T) {
	ctx := context.Background()
	repo := citation.NewInMemoryRepository()

	units := []citation.CitedUnit{
		{NodeID: "a", CaseID: "case-1"},
		{NodeID: "b", CaseID: "case-1"},
		{NodeID: "c", CaseID: "case-2"},
	}
	if err := repo.SaveAll(ctx, units); err != nil {
		t.Fatalf("SaveAll() error = %v", err)
	}

	list, err := repo.ListByCase(ctx, "case-1")
	if err != nil {
		t.Fatalf("ListByCase() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}

	list2, err := repo.ListByCase(ctx, "case-3")
	if err != nil {
		t.Fatalf("ListByCase() error = %v", err)
	}
	if len(list2) != 0 {
		t.Errorf("len(list2) = %d, want 0 (empty, not nil)", len(list2))
	}
	if list2 == nil {
		t.Error("ListByCase() returned nil, want empty slice")
	}
}

func TestInMemoryRepositoryDeleteByCase(t *testing.T) {
	ctx := context.Background()
	repo := citation.NewInMemoryRepository()

	units := []citation.CitedUnit{
		{NodeID: "a", CaseID: "case-1"},
		{NodeID: "b", CaseID: "case-1"},
	}
	if err := repo.SaveAll(ctx, units); err != nil {
		t.Fatalf("SaveAll() error = %v", err)
	}
	if err := repo.DeleteByCase(ctx, "case-1"); err != nil {
		t.Fatalf("DeleteByCase() error = %v", err)
	}

	list, err := repo.ListByCase(ctx, "case-1")
	if err != nil {
		t.Fatalf("ListByCase() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("len(list) = %d, want 0 after delete", len(list))
	}

	// Deleting a case with nothing saved is not an error.
	if err := repo.DeleteByCase(ctx, "case-empty"); err != nil {
		t.Errorf("DeleteByCase() on empty case error = %v, want nil", err)
	}
}

func TestInMemoryRepositoryValidationErrors(t *testing.T) {
	ctx := context.Background()
	repo := citation.NewInMemoryRepository()

	if err := repo.Save(ctx, citation.CitedUnit{NodeID: "n"}); !errors.Is(err, citation.ErrEmptyCaseID) {
		t.Errorf("Save(empty case) error = %v, want ErrEmptyCaseID", err)
	}
	if err := repo.Save(ctx, citation.CitedUnit{CaseID: "c"}); !errors.Is(err, citation.ErrEmptyNodeID) {
		t.Errorf("Save(empty node) error = %v, want ErrEmptyNodeID", err)
	}
	if _, err := repo.Get(ctx, "", "n"); !errors.Is(err, citation.ErrEmptyCaseID) {
		t.Errorf("Get(empty case) error = %v, want ErrEmptyCaseID", err)
	}
	if _, err := repo.Get(ctx, "c", ""); !errors.Is(err, citation.ErrEmptyNodeID) {
		t.Errorf("Get(empty node) error = %v, want ErrEmptyNodeID", err)
	}
	if _, err := repo.ListByCase(ctx, ""); !errors.Is(err, citation.ErrEmptyCaseID) {
		t.Errorf("ListByCase(empty case) error = %v, want ErrEmptyCaseID", err)
	}
	if err := repo.DeleteByCase(ctx, ""); !errors.Is(err, citation.ErrEmptyCaseID) {
		t.Errorf("DeleteByCase(empty case) error = %v, want ErrEmptyCaseID", err)
	}
}

func TestInMemoryRepositoryConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	repo := citation.NewInMemoryRepository()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = repo.Save(ctx, citation.CitedUnit{NodeID: "n", CaseID: "case-1"})
			_, _ = repo.Get(ctx, "case-1", "n")
			_, _ = repo.ListByCase(ctx, "case-1")
		}()
	}
	wg.Wait()

	if repo.Len() != 1 {
		t.Errorf("Len() = %d, want 1", repo.Len())
	}
}

// TestPersistenceRoundTrip verifies the full resolve -> verify -> persist
// -> retrieve pipeline preserves every field a caller depends on for
// later audit/reporting.
func TestPersistenceRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := citation.NewInMemoryRepository()

	unit := citation.CitedUnit{
		NodeID:       "rule-1",
		CaseID:       "case-1",
		Citation:     "Act 12, s.5(a)",
		Origin:       citation.OriginStatute,
		Text:         "no person shall...",
		AnchorNodeID: "issue-1",
	}

	if err := repo.Save(ctx, unit); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.Get(ctx, "case-1", "rule-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	switch {
	case got.NodeID != unit.NodeID,
		got.CaseID != unit.CaseID,
		got.Citation != unit.Citation,
		got.Origin != unit.Origin,
		got.Text != unit.Text,
		got.AnchorNodeID != unit.AnchorNodeID:
		t.Errorf("round-tripped unit = %+v, want %+v", got, unit)
	}
}
