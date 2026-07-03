package caselifecycle_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestBulkTransition_ReportsMixedSuccessAndFailure(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	okCase := seedCase(t, repo, tenantID, actor.ID) // draft -> active is legal
	staleCase := seedCase(t, repo, tenantID, actor.ID)
	advanceTo(t, ctx, repo, tenantID, staleCase.ID, caselifecycle.StateClosed) // closed -> active is illegal via plain Transition
	missingID := uuid.New()

	results, err := caselifecycle.BulkTransition(ctx, repo, caselifecycle.BulkTransitionInput{
		TenantID: tenantID,
		CaseIDs:  []uuid.UUID{okCase.ID, staleCase.ID, missingID},
		ToState:  caselifecycle.StateActive,
		Reason:   "bulk activation sweep",
	})
	if err != nil {
		t.Fatalf("BulkTransition top-level error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if !results[0].Succeeded() {
		t.Errorf("expected okCase to succeed, got err %v", results[0].Err)
	}
	if results[0].Case == nil || results[0].Case.State != caselifecycle.StateActive {
		t.Errorf("expected okCase to end in StateActive, got %+v", results[0].Case)
	}

	if results[1].Succeeded() {
		t.Error("expected staleCase (closed) to fail bulk transition to active via plain Transition")
	}
	if !errors.Is(results[1].Err, caselifecycle.ErrIllegalTransition) {
		t.Errorf("expected ErrIllegalTransition for staleCase, got %v", results[1].Err)
	}

	if results[2].Succeeded() {
		t.Error("expected missing case id to fail")
	}
	if !errors.Is(results[2].Err, caselifecycle.ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing case, got %v", results[2].Err)
	}

	// The successful case must actually be persisted, and the failed
	// ones must be unaffected by the other entries' failures.
	persisted, err := repo.Get(ctx, tenantID, okCase.ID)
	if err != nil {
		t.Fatalf("Get okCase: %v", err)
	}
	if persisted.State != caselifecycle.StateActive {
		t.Errorf("persisted okCase.State = %s, want %s", persisted.State, caselifecycle.StateActive)
	}

	stillClosed, err := repo.Get(ctx, tenantID, staleCase.ID)
	if err != nil {
		t.Fatalf("Get staleCase: %v", err)
	}
	if stillClosed.State != caselifecycle.StateClosed {
		t.Errorf("staleCase.State = %s, want unchanged %s", stillClosed.State, caselifecycle.StateClosed)
	}
}

func TestBulkTransition_SucceededAndFailedHelpers(t *testing.T) {
	results := []caselifecycle.BulkResult{
		{CaseID: uuid.New(), Case: &caselifecycle.Case{}, Err: nil},
		{CaseID: uuid.New(), Err: caselifecycle.ErrNotFound},
	}
	ok := caselifecycle.SucceededCaseIDs(results)
	if len(ok) != 1 || ok[0] != results[0].CaseID {
		t.Errorf("SucceededCaseIDs = %v, want [%v]", ok, results[0].CaseID)
	}
	failed := caselifecycle.FailedResults(results)
	if len(failed) != 1 || failed[0].CaseID != results[1].CaseID {
		t.Errorf("FailedResults = %v, want [%v]", failed, results[1])
	}
}

func TestBulkSetMetadata_ReportsMixedSuccessAndFailure(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c1 := seedCase(t, repo, tenantID, actor.ID)
	c2 := seedCase(t, repo, tenantID, actor.ID)
	missingID := uuid.New()

	results, err := caselifecycle.BulkSetMetadata(ctx, repo, caselifecycle.BulkMetadataUpdateInput{
		TenantID: tenantID,
		CaseIDs:  []uuid.UUID{c1.ID, c2.ID, missingID},
		Values:   map[string]string{"batch": "true"},
	})
	if err != nil {
		t.Fatalf("BulkSetMetadata top-level error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results[:2] {
		if !r.Succeeded() {
			t.Errorf("result[%d] expected success, got err %v", i, r.Err)
		}
		if r.Case.Metadata["batch"] != "true" {
			t.Errorf("result[%d] Metadata[batch] = %q, want %q", i, r.Case.Metadata["batch"], "true")
		}
	}
	if results[2].Succeeded() {
		t.Error("expected missing case id to fail")
	}
	if !errors.Is(results[2].Err, caselifecycle.ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing case, got %v", results[2].Err)
	}
}

func TestBulkTransition_NilRepositoryRejected(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	_, err := caselifecycle.BulkTransition(ctx, nil, caselifecycle.BulkTransitionInput{
		TenantID: tenantID,
		CaseIDs:  []uuid.UUID{uuid.New()},
		ToState:  caselifecycle.StateActive,
	})
	if !errors.Is(err, caselifecycle.ErrNilRepository) {
		t.Fatalf("expected ErrNilRepository, got %v", err)
	}
}
