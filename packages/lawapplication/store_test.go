package lawapplication_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

func TestInMemoryRepository_SaveAndGet(t *testing.T) {
	repo := lawapplication.NewInMemoryRepository()
	ctx := context.Background()

	result := lawapplication.Result{CaseID: "case-1"}
	if err := repo.Save(ctx, result); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.Get(ctx, "case-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CaseID != "case-1" {
		t.Errorf("CaseID = %q, want case-1", got.CaseID)
	}
}

func TestInMemoryRepository_SaveEmptyCaseID(t *testing.T) {
	repo := lawapplication.NewInMemoryRepository()
	err := repo.Save(context.Background(), lawapplication.Result{})
	if !errors.Is(err, lawapplication.ErrEmptyCaseID) {
		t.Errorf("err = %v, want ErrEmptyCaseID", err)
	}
}

func TestInMemoryRepository_GetNotFound(t *testing.T) {
	repo := lawapplication.NewInMemoryRepository()
	_, err := repo.Get(context.Background(), "case-missing")
	if !errors.Is(err, lawapplication.ErrResultNotFound) {
		t.Errorf("err = %v, want ErrResultNotFound", err)
	}
}

func TestInMemoryRepository_GetEmptyCaseID(t *testing.T) {
	repo := lawapplication.NewInMemoryRepository()
	_, err := repo.Get(context.Background(), "")
	if !errors.Is(err, lawapplication.ErrEmptyCaseID) {
		t.Errorf("err = %v, want ErrEmptyCaseID", err)
	}
}

func TestInMemoryRepository_SaveOverwrites(t *testing.T) {
	repo := lawapplication.NewInMemoryRepository()
	ctx := context.Background()

	if err := repo.Save(ctx, lawapplication.Result{CaseID: "case-1", IssueApplications: nil}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.Save(ctx, lawapplication.Result{CaseID: "case-1", IssueApplications: []lawapplication.IssueApplication{{IssueNodeID: "issue-1"}}}); err != nil {
		t.Fatalf("Save (overwrite): %v", err)
	}

	got, err := repo.Get(ctx, "case-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.IssueApplications) != 1 {
		t.Errorf("len(IssueApplications) = %d, want 1 (overwrite should replace)", len(got.IssueApplications))
	}
	if repo.Len() != 1 {
		t.Errorf("Len() = %d, want 1", repo.Len())
	}
}

func TestInMemoryRepository_DeleteByCase(t *testing.T) {
	repo := lawapplication.NewInMemoryRepository()
	ctx := context.Background()

	if err := repo.Save(ctx, lawapplication.Result{CaseID: "case-1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.DeleteByCase(ctx, "case-1"); err != nil {
		t.Fatalf("DeleteByCase: %v", err)
	}
	if _, err := repo.Get(ctx, "case-1"); !errors.Is(err, lawapplication.ErrResultNotFound) {
		t.Errorf("Get after delete = %v, want ErrResultNotFound", err)
	}
}

func TestInMemoryRepository_DeleteByCaseUnknownIsNotError(t *testing.T) {
	repo := lawapplication.NewInMemoryRepository()
	if err := repo.DeleteByCase(context.Background(), "case-missing"); err != nil {
		t.Errorf("DeleteByCase for unknown case = %v, want nil", err)
	}
}

func TestInMemoryRepository_DeleteByCaseEmptyCaseID(t *testing.T) {
	repo := lawapplication.NewInMemoryRepository()
	err := repo.DeleteByCase(context.Background(), "")
	if !errors.Is(err, lawapplication.ErrEmptyCaseID) {
		t.Errorf("err = %v, want ErrEmptyCaseID", err)
	}
}
