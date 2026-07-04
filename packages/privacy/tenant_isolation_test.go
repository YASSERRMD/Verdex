package privacy_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/privacy"
)

// TestEngine_RegisterInventoryEntry_CrossTenantRejected proves an
// admin authenticated against tenant A can never register a
// DataInventoryEntry scoped to tenant B.
func TestEngine_RegisterInventoryEntry_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.RegisterInventoryEntry(ctxWithUser(adminA), tenantB, privacy.DataInventoryEntry{
		Category:        privacy.CategoryCaseParty,
		SourceTag:       "case.parties",
		Sensitivity:     privacy.SensitivityHigh,
		LegalBasis:      privacy.BasisPublicTask,
		RetentionPeriod: time.Hour,
	})
	if !errors.Is(err, privacy.ErrCrossTenantAccess) {
		t.Fatalf("RegisterInventoryEntry() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_SubmitSAR_CrossTenantRejected mirrors the same guarantee
// for SubmitSAR.
func TestEngine_SubmitSAR_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.SubmitSAR(ctxWithUser(adminA), tenantB, privacy.SubjectAccessRequest{SubjectID: "subject-1"})
	if !errors.Is(err, privacy.ErrCrossTenantAccess) {
		t.Fatalf("SubmitSAR() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_SubmitErasureRequest_CrossTenantRejected mirrors the same
// guarantee for SubmitErasureRequest.
func TestEngine_SubmitErasureRequest_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.SubmitErasureRequest(ctxWithUser(adminA), tenantB, privacy.ErasureRequest{
		SubjectID: "subject-1",
		Category:  privacy.CategoryCaseParty,
		SourceTag: "case.parties",
	})
	if !errors.Is(err, privacy.ErrCrossTenantAccess) {
		t.Fatalf("SubmitErasureRequest() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_RecordConsent_CrossTenantRejected mirrors the same
// guarantee for RecordConsent.
func TestEngine_RecordConsent_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.RecordConsent(ctxWithUser(adminA), tenantB, privacy.ConsentRecord{
		SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent,
	})
	if !errors.Is(err, privacy.ErrCrossTenantAccess) {
		t.Fatalf("RecordConsent() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestInMemoryInventoryRepository_TenantIsolated proves the
// repository layer itself never leaks a tenant's inventory entries
// into another tenant's List/Get results, independent of the Engine
// authorization layer above it.
func TestInMemoryInventoryRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := privacy.NewInMemoryInventoryRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	entryA := &privacy.DataInventoryEntry{
		ID: uuid.New(), TenantID: tenantA, Category: privacy.CategoryCaseParty,
		SourceTag: "case.parties", Sensitivity: privacy.SensitivityHigh,
		LegalBasis: privacy.BasisPublicTask, RetentionPeriod: time.Hour,
	}
	if err := repo.Create(t.Context(), tenantA, entryA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}

	entryB := &privacy.DataInventoryEntry{
		ID: uuid.New(), TenantID: tenantB, Category: privacy.CategoryCaseParty,
		SourceTag: "case.parties", Sensitivity: privacy.SensitivityHigh,
		LegalBasis: privacy.BasisPublicTask, RetentionPeriod: time.Hour,
	}
	if err := repo.Create(t.Context(), tenantB, entryB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.List(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("List (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != entryA.ID {
		t.Fatalf("List(tenantA) = %v, want exactly entryA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, entryB.ID); !errors.Is(err, privacy.ErrInventoryEntryNotFound) {
		t.Fatalf("Get(tenantA, entryB.ID) error = %v, want ErrInventoryEntryNotFound", err)
	}
}

// TestInMemoryConsentRepository_TenantIsolated mirrors the same
// guarantee for consent records.
func TestInMemoryConsentRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := privacy.NewInMemoryConsentRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	recordA := &privacy.ConsentRecord{ID: uuid.New(), TenantID: tenantA, SubjectID: "s1", Purpose: "p1", LegalBasis: privacy.BasisConsent, GrantedAt: time.Now()}
	recordB := &privacy.ConsentRecord{ID: uuid.New(), TenantID: tenantB, SubjectID: "s1", Purpose: "p1", LegalBasis: privacy.BasisConsent, GrantedAt: time.Now()}

	if err := repo.Create(t.Context(), tenantA, recordA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, recordB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	// Same SubjectID, different tenant -- tenant B must never see
	// tenant A's consent record.
	listB, err := repo.ListForSubject(t.Context(), tenantB, "s1")
	if err != nil {
		t.Fatalf("ListForSubject (B): %v", err)
	}
	if len(listB) != 1 || listB[0].ID != recordB.ID {
		t.Fatalf("ListForSubject(tenantB, s1) = %v, want exactly recordB", listB)
	}

	if _, err := repo.Get(t.Context(), tenantB, recordA.ID); !errors.Is(err, privacy.ErrConsentNotFound) {
		t.Fatalf("Get(tenantB, recordA.ID) error = %v, want ErrConsentNotFound", err)
	}
}

// TestInMemorySARRepository_TenantIsolated mirrors the same guarantee
// for subject access requests.
func TestInMemorySARRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := privacy.NewInMemorySARRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	sarA := &privacy.SubjectAccessRequest{ID: uuid.New(), TenantID: tenantA, SubjectID: "s1", Status: privacy.SARStatusReceived, ReceivedAt: time.Now()}
	sarB := &privacy.SubjectAccessRequest{ID: uuid.New(), TenantID: tenantB, SubjectID: "s1", Status: privacy.SARStatusReceived, ReceivedAt: time.Now()}

	if err := repo.Create(t.Context(), tenantA, sarA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, sarB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != sarA.ID {
		t.Fatalf("ListAll(tenantA) = %v, want exactly sarA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, sarB.ID); !errors.Is(err, privacy.ErrSARNotFound) {
		t.Fatalf("Get(tenantA, sarB.ID) error = %v, want ErrSARNotFound", err)
	}
}

// TestInMemoryErasureRepository_TenantIsolated mirrors the same
// guarantee for erasure requests -- proving even the provenance
// fields never leak across a tenant boundary.
func TestInMemoryErasureRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := privacy.NewInMemoryErasureRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	reqA := &privacy.ErasureRequest{
		ID: uuid.New(), TenantID: tenantA, SubjectID: "s1", Category: privacy.CategoryCaseParty,
		SourceTag: "case.parties", Status: privacy.ErasureStatusReceived, RequestedAt: time.Now(),
		ProvenanceRecordID: uuid.New(), ProvenanceHash: "tenant-a-hash",
	}
	reqB := &privacy.ErasureRequest{
		ID: uuid.New(), TenantID: tenantB, SubjectID: "s1", Category: privacy.CategoryCaseParty,
		SourceTag: "case.parties", Status: privacy.ErasureStatusReceived, RequestedAt: time.Now(),
	}

	if err := repo.Create(t.Context(), tenantA, reqA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, reqB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listB, err := repo.ListAll(t.Context(), tenantB)
	if err != nil {
		t.Fatalf("ListAll (B): %v", err)
	}
	if len(listB) != 1 || listB[0].ID != reqB.ID {
		t.Fatalf("ListAll(tenantB) = %v, want exactly reqB", listB)
	}
	for _, r := range listB {
		if r.ProvenanceHash == "tenant-a-hash" {
			t.Fatalf("tenant B's erasure list leaked tenant A's provenance hash")
		}
	}

	if _, err := repo.Get(t.Context(), tenantB, reqA.ID); !errors.Is(err, privacy.ErrErasureNotFound) {
		t.Fatalf("Get(tenantB, reqA.ID) error = %v, want ErrErasureNotFound", err)
	}
}
