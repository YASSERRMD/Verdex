package evidenceweighing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
)

func TestInMemoryRepository_SaveGetRoundTrip(t *testing.T) {
	repo := evidenceweighing.NewInMemoryRepository()
	ctx := context.Background()

	result := evidenceweighing.Result{
		CaseID: "case-1",
		FactWeights: []evidenceweighing.FactWeight{
			{FactNodeID: "fact-1", Weight: 0.75, Rationale: "test rationale"},
		},
	}

	if err := repo.Save(ctx, result); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := repo.Get(ctx, "case-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.CaseID != "case-1" || len(got.FactWeights) != 1 || got.FactWeights[0].FactNodeID != "fact-1" {
		t.Errorf("round-tripped result mismatch: %+v", got)
	}
}

func TestInMemoryRepository_GetNotFound(t *testing.T) {
	repo := evidenceweighing.NewInMemoryRepository()

	_, err := repo.Get(context.Background(), "missing-case")
	if !errors.Is(err, evidenceweighing.ErrResultNotFound) {
		t.Errorf("err = %v, want ErrResultNotFound", err)
	}
}

func TestInMemoryRepository_SaveOverwrites(t *testing.T) {
	repo := evidenceweighing.NewInMemoryRepository()
	ctx := context.Background()

	first := evidenceweighing.Result{CaseID: "case-1", LegalFamily: evidenceweighing.CommonLawFamily}
	second := evidenceweighing.Result{CaseID: "case-1", LegalFamily: evidenceweighing.CivilLawFamily}

	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("Save first returned error: %v", err)
	}
	if err := repo.Save(ctx, second); err != nil {
		t.Fatalf("Save second returned error: %v", err)
	}

	got, err := repo.Get(ctx, "case-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.LegalFamily != evidenceweighing.CivilLawFamily {
		t.Errorf("LegalFamily = %q, want overwritten value %q", got.LegalFamily, evidenceweighing.CivilLawFamily)
	}
	if repo.Len() != 1 {
		t.Errorf("Len() = %d, want 1 (overwrite, not append)", repo.Len())
	}
}

func TestInMemoryRepository_DeleteByCase(t *testing.T) {
	repo := evidenceweighing.NewInMemoryRepository()
	ctx := context.Background()

	if err := repo.Save(ctx, evidenceweighing.Result{CaseID: "case-1"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := repo.DeleteByCase(ctx, "case-1"); err != nil {
		t.Fatalf("DeleteByCase returned error: %v", err)
	}

	_, err := repo.Get(ctx, "case-1")
	if !errors.Is(err, evidenceweighing.ErrResultNotFound) {
		t.Errorf("err = %v, want ErrResultNotFound after delete", err)
	}

	// Deleting a case with nothing stored is not an error.
	if err := repo.DeleteByCase(ctx, "never-existed"); err != nil {
		t.Errorf("DeleteByCase on unknown case returned error: %v", err)
	}
}

func TestInMemoryRepository_EmptyCaseID(t *testing.T) {
	repo := evidenceweighing.NewInMemoryRepository()
	ctx := context.Background()

	if err := repo.Save(ctx, evidenceweighing.Result{}); !errors.Is(err, evidenceweighing.ErrEmptyCaseID) {
		t.Errorf("Save with empty case id: err = %v, want ErrEmptyCaseID", err)
	}
	if _, err := repo.Get(ctx, ""); !errors.Is(err, evidenceweighing.ErrEmptyCaseID) {
		t.Errorf("Get with empty case id: err = %v, want ErrEmptyCaseID", err)
	}
	if err := repo.DeleteByCase(ctx, ""); !errors.Is(err, evidenceweighing.ErrEmptyCaseID) {
		t.Errorf("DeleteByCase with empty case id: err = %v, want ErrEmptyCaseID", err)
	}
}
