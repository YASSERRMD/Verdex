package compliance_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/compliance"
)

// TestInMemoryEvidenceRepository_TenantIsolated proves the repository
// layer itself never leaks a tenant's evidence into another tenant's
// List/Get results, independent of the Engine authorization layer
// above it.
func TestInMemoryEvidenceRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := compliance.NewInMemoryEvidenceRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()
	controlID := uuid.New()

	evA := &compliance.ControlEvidence{
		ID: uuid.New(), TenantID: tenantA, ControlID: controlID,
		Kind: compliance.EvidenceKindTestName, Reference: "TestA", CollectedAt: time.Now(),
	}
	evB := &compliance.ControlEvidence{
		ID: uuid.New(), TenantID: tenantB, ControlID: controlID,
		Kind: compliance.EvidenceKindTestName, Reference: "TestB", CollectedAt: time.Now(),
	}
	if err := repo.Create(t.Context(), tenantA, evA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, evB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != evA.ID {
		t.Fatalf("ListAll(tenantA) = %v, want exactly evA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, evB.ID); !errors.Is(err, compliance.ErrEvidenceNotFound) {
		t.Fatalf("Get(tenantA, evB.ID) error = %v, want ErrEvidenceNotFound", err)
	}
}

// TestInMemoryProfileRepository_TenantIsolated mirrors the same
// guarantee for compliance profiles.
func TestInMemoryProfileRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := compliance.NewInMemoryProfileRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	profileA := &compliance.Profile{TenantID: tenantA, Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection}}
	profileB := &compliance.Profile{TenantID: tenantB, Frameworks: []compliance.Framework{compliance.FrameworkJudicialRecordsHandling}}

	if err := repo.Set(t.Context(), tenantA, profileA); err != nil {
		t.Fatalf("Set (A): %v", err)
	}
	if err := repo.Set(t.Context(), tenantB, profileB); err != nil {
		t.Fatalf("Set (B): %v", err)
	}

	gotA, err := repo.Get(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("Get (A): %v", err)
	}
	if len(gotA.Frameworks) != 1 || gotA.Frameworks[0] != compliance.FrameworkUAEDataProtection {
		t.Fatalf("Get(tenantA).Frameworks = %v, want exactly [uae_data_protection]", gotA.Frameworks)
	}
}

// TestEngine_SetProfile_CrossTenantRejected proves an admin
// authenticated against tenant A can never set tenant B's compliance
// Profile.
func TestEngine_SetProfile_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.SetProfile(ctxWithUser(adminA), tenantB, compliance.Profile{
		Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection},
	})
	if !errors.Is(err, compliance.ErrCrossTenantAccess) {
		t.Fatalf("SetProfile() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_GetProfile_CrossTenantRejected mirrors the same
// guarantee for GetProfile.
func TestEngine_GetProfile_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.GetProfile(ctxWithUser(adminA), tenantB)
	if !errors.Is(err, compliance.ErrCrossTenantAccess) {
		t.Fatalf("GetProfile() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_ListEvidenceForControl_CrossTenantRejected mirrors the
// same guarantee for evidence listing.
func TestEngine_ListEvidenceForControl_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	control := registerTestControl(t, engine, tenantA)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.ListEvidenceForControl(ctxWithUser(adminA), tenantB, control.ID)
	if !errors.Is(err, compliance.ErrCrossTenantAccess) {
		t.Fatalf("ListEvidenceForControl() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_RunGapAnalysis_TenantsDoNotLeakEvidence proves that
// tenant B's gap analysis never reflects tenant A's evidence, even
// though both tenants may hold evidence against the same shared
// catalogued Control (compliance_controls carries no tenant_id --
// see ControlRepository's doc comment).
func TestEngine_RunGapAnalysis_TenantsDoNotLeakEvidence(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	adminA := adminUser(tenantA)
	control := registerTestControl(t, engine, tenantA)

	if _, err := engine.RecordEvidence(ctxWithUser(adminA), tenantA, compliance.ControlEvidence{
		ControlID: control.ID, Kind: compliance.EvidenceKindTestName, Reference: "TestA1",
	}); err != nil {
		t.Fatalf("RecordEvidence (A, kind 1): %v", err)
	}
	if _, err := engine.RecordEvidence(ctxWithUser(adminA), tenantA, compliance.ControlEvidence{
		ControlID: control.ID, Kind: compliance.EvidenceKindDocument, Reference: "doc/a.md",
	}); err != nil {
		t.Fatalf("RecordEvidence (A, kind 2): %v", err)
	}

	tenantB := uuid.New()
	adminB := adminUser(tenantB)

	reportB, err := engine.RunGapAnalysis(ctxWithUser(adminB), tenantB)
	if err != nil {
		t.Fatalf("RunGapAnalysis (B): %v", err)
	}
	if len(reportB.Results) != 1 {
		t.Fatalf("len(reportB.Results) = %d, want 1 (the shared catalogued control)", len(reportB.Results))
	}
	if reportB.Results[0].Status != compliance.StatusGap {
		t.Fatalf("reportB status = %q, want %q (tenant B has recorded no evidence of its own)", reportB.Results[0].Status, compliance.StatusGap)
	}

	reportA, err := engine.RunGapAnalysis(ctxWithUser(adminA), tenantA)
	if err != nil {
		t.Fatalf("RunGapAnalysis (A): %v", err)
	}
	if reportA.Results[0].Status != compliance.StatusSatisfied {
		t.Fatalf("reportA status = %q, want %q", reportA.Results[0].Status, compliance.StatusSatisfied)
	}
}
