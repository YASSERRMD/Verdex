package securitytesting_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

// TestInMemoryFindingRepository_TenantIsolated proves the repository
// layer itself never leaks a tenant's findings into another tenant's
// List/Get results, independent of the Engine authorization layer
// above it.
func TestInMemoryFindingRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := securitytesting.NewInMemoryFindingRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	fA := &securitytesting.Finding{
		ID: uuid.New(), TenantID: tenantA, Title: "tenant A finding",
		Category: securitytesting.CategoryAbuseCase, Severity: securitytesting.SeverityLow,
		SourceScenario: "fixture", Status: securitytesting.FindingOpen,
	}
	fB := &securitytesting.Finding{
		ID: uuid.New(), TenantID: tenantB, Title: "tenant B finding",
		Category: securitytesting.CategoryAbuseCase, Severity: securitytesting.SeverityLow,
		SourceScenario: "fixture", Status: securitytesting.FindingOpen,
	}
	if err := repo.Create(t.Context(), tenantA, fA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, fB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != fA.ID {
		t.Fatalf("ListAll(tenantA) = %v, want exactly fA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, fB.ID); !errors.Is(err, securitytesting.ErrFindingNotFound) {
		t.Fatalf("Get(tenantA, fB.ID) error = %v, want ErrFindingNotFound", err)
	}

	// A cross-tenant Update must also be rejected, not silently
	// succeed against the wrong tenant's record.
	fBCrossTenantAttempt := *fB
	fBCrossTenantAttempt.Title = "attacker-modified title"
	if err := repo.Update(t.Context(), tenantA, &fBCrossTenantAttempt); !errors.Is(err, securitytesting.ErrCrossTenantAccess) {
		t.Fatalf("Update(tenantA, fB) error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestInMemoryFindingRepository_ListByStatus proves ListByStatus
// filters both by tenant and by status simultaneously.
func TestInMemoryFindingRepository_ListByStatus(t *testing.T) {
	t.Parallel()
	repo := securitytesting.NewInMemoryFindingRepository()
	tenantID := uuid.New()

	open := &securitytesting.Finding{
		ID: uuid.New(), TenantID: tenantID, Title: "open one",
		Category: securitytesting.CategoryAbuseCase, Severity: securitytesting.SeverityLow,
		SourceScenario: "fixture", Status: securitytesting.FindingOpen,
	}
	fixed := &securitytesting.Finding{
		ID: uuid.New(), TenantID: tenantID, Title: "fixed one",
		Category: securitytesting.CategoryAbuseCase, Severity: securitytesting.SeverityLow,
		SourceScenario: "fixture", Status: securitytesting.FindingVerifiedFixed,
	}
	if err := repo.Create(t.Context(), tenantID, open); err != nil {
		t.Fatalf("Create (open): %v", err)
	}
	if err := repo.Create(t.Context(), tenantID, fixed); err != nil {
		t.Fatalf("Create (fixed): %v", err)
	}

	openList, err := repo.ListByStatus(t.Context(), tenantID, securitytesting.FindingOpen)
	if err != nil {
		t.Fatalf("ListByStatus(FindingOpen): %v", err)
	}
	if len(openList) != 1 || openList[0].ID != open.ID {
		t.Fatalf("ListByStatus(FindingOpen) = %v, want exactly [open]", openList)
	}
}

// TestInMemoryRunRecordRepository_TenantIsolated mirrors the same
// guarantee for RunRecords.
func TestInMemoryRunRecordRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := securitytesting.NewInMemoryRunRecordRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	rrA := &securitytesting.RunRecord{
		ID: uuid.New(), TenantID: tenantA, ScenarioName: "scenario-a",
		ScenarioCategory: securitytesting.CategoryRegression,
		Result:           securitytesting.Result{Outcome: securitytesting.OutcomePassed, Detail: "ok"},
	}
	rrB := &securitytesting.RunRecord{
		ID: uuid.New(), TenantID: tenantB, ScenarioName: "scenario-b",
		ScenarioCategory: securitytesting.CategoryRegression,
		Result:           securitytesting.Result{Outcome: securitytesting.OutcomePassed, Detail: "ok"},
	}
	if err := repo.Create(t.Context(), tenantA, rrA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, rrB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != rrA.ID {
		t.Fatalf("ListAll(tenantA) = %v, want exactly rrA", listA)
	}
}

// TestInMemoryRunRecordRepository_RejectsReplay is the storage-layer
// counterpart of abusecase_suite.go's ScenarioAuditReplayRejected,
// directly unit-testing the guard that scenario exercises end-to-end.
func TestInMemoryRunRecordRepository_RejectsReplay(t *testing.T) {
	t.Parallel()
	repo := securitytesting.NewInMemoryRunRecordRepository()
	tenantID := uuid.New()
	id := uuid.New()

	first := &securitytesting.RunRecord{
		ID: id, TenantID: tenantID, ScenarioName: "x",
		ScenarioCategory: securitytesting.CategoryRegression,
		Result:           securitytesting.Result{Outcome: securitytesting.OutcomePassed, Detail: "first"},
	}
	if err := repo.Create(t.Context(), tenantID, first); err != nil {
		t.Fatalf("Create (first): %v", err)
	}

	replay := &securitytesting.RunRecord{
		ID: id, TenantID: tenantID, ScenarioName: "x",
		ScenarioCategory: securitytesting.CategoryRegression,
		Result:           securitytesting.Result{Outcome: securitytesting.OutcomePassed, Detail: "replayed"},
	}
	if err := repo.Create(t.Context(), tenantID, replay); !errors.Is(err, securitytesting.ErrDuplicateRunRecord) {
		t.Fatalf("Create (replay) error = %v, want ErrDuplicateRunRecord", err)
	}

	stored, err := repo.Get(t.Context(), tenantID, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Result.Detail != "first" {
		t.Errorf("stored Detail = %q, want unchanged %q after a rejected replay", stored.Result.Detail, "first")
	}
}
