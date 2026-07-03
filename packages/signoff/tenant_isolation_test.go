package signoff_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/signoff"
)

func TestInMemoryRepository_TenantAIsolatedFromTenantB(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	caseID := uuid.New()
	ctx := context.Background()

	repo := signoff.NewInMemoryRepository()
	rec := &signoff.SignoffRecord{
		ID:          uuid.New(),
		CaseID:      caseID,
		TenantID:    tenantA,
		Status:      guardrail.SignoffApproved,
		CaseVersion: 1,
	}
	if err := repo.Upsert(ctx, tenantA, rec); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Tenant B must not be able to see tenant A's sign-off record.
	if _, err := repo.Get(ctx, tenantB, caseID); !errors.Is(err, signoff.ErrNotFound) {
		t.Fatalf("expected ErrNotFound fetching tenant A's record under tenant B's scope, got %v", err)
	}

	// Tenant A can see its own record.
	got, err := repo.Get(ctx, tenantA, caseID)
	if err != nil {
		t.Fatalf("Get under tenant A: %v", err)
	}
	if got.Status != guardrail.SignoffApproved {
		t.Fatalf("Status = %v, want Approved", got.Status)
	}
}

func TestInMemoryRepository_UpsertRejectsMismatchedTenant(t *testing.T) {
	scopeTenant := uuid.New()
	otherTenant := uuid.New()
	ctx := context.Background()

	repo := signoff.NewInMemoryRepository()
	rec := &signoff.SignoffRecord{
		ID:       uuid.New(),
		CaseID:   uuid.New(),
		TenantID: otherTenant,
		Status:   guardrail.SignoffPending,
	}

	err := repo.Upsert(ctx, scopeTenant, rec)
	if !errors.Is(err, signoff.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess, got %v", err)
	}
}

func TestInMemoryRepository_ListAuditScopedToTenant(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	caseID := uuid.New()
	ctx := context.Background()

	repo := signoff.NewInMemoryRepository()
	entry := &signoff.AuditEntry{
		ID:         uuid.New(),
		CaseID:     caseID,
		TenantID:   tenantA,
		FromStatus: guardrail.SignoffPending,
		ToStatus:   guardrail.SignoffApproved,
		Source:     signoff.DecisionSourceReviewer,
	}
	if err := repo.AppendAudit(ctx, tenantA, entry); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}

	listB, err := repo.ListAudit(ctx, tenantB, caseID)
	if err != nil {
		t.Fatalf("ListAudit under tenant B: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("expected 0 audit entries visible to tenant B, got %d", len(listB))
	}

	listA, err := repo.ListAudit(ctx, tenantA, caseID)
	if err != nil {
		t.Fatalf("ListAudit under tenant A: %v", err)
	}
	if len(listA) != 1 {
		t.Fatalf("expected 1 audit entry visible to tenant A, got %d", len(listA))
	}
}

// TestService_TenantIsolation proves that the Service layer refuses
// to leak or accept a sign-off decision across tenant boundaries: an
// actor from tenant B cannot approve a case whose SignoffRecord (once
// it exists) belongs to tenant A's scope, because Approve upserts
// scoped strictly to the tenantID passed in DecisionInput and the
// underlying repository enforces requireMatchingTenant.
func TestService_TenantIsolation_RecordsDoNotCrossTenants(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	caseID := uuid.New()

	repo := signoff.NewInMemoryRepository()
	reader := newFakeCaseVersionReader()
	reader.set(caseID, 1)
	svc, err := signoff.NewService(repo, reader, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	judgeA := newTestUser(tenantA, identity.RoleJudge)
	ctxA := ctxWithUser(judgeA)
	if _, err := svc.Approve(ctxA, signoff.DecisionInput{
		TenantID:        tenantA,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		t.Fatalf("Approve under tenant A: %v", err)
	}

	// Tenant B's view of the same case ID must not see tenant A's
	// approval: Get (with view permission) must not find it.
	judgeB := newTestUser(tenantB, identity.RoleJudge)
	ctxB := ctxWithUser(judgeB)
	recB, err := svc.Get(ctxB, tenantB, caseID)
	if err != nil {
		t.Fatalf("Get under tenant B: %v", err)
	}
	if recB.Status != guardrail.SignoffPending {
		t.Fatalf("expected tenant B to see a synthetic Pending record (no cross-tenant leak), got %v", recB.Status)
	}
}
