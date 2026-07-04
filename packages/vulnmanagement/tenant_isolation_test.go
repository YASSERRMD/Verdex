package vulnmanagement_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

// TestInMemoryFindingRepository_TenantIsolated proves the repository
// layer itself never leaks a tenant's findings into another tenant's
// List/Get results, independent of the Engine authorization layer
// above it.
func TestInMemoryFindingRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := vulnmanagement.NewInMemoryFindingRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	fA := &vulnmanagement.Finding{
		ID: uuid.New(), TenantID: tenantA, Source: vulnmanagement.ScannerSourceSCA,
		Package: "pkg-a", Severity: vulnmanagement.SeverityHigh, AdvisoryID: "CVE-A",
		Title: "Finding A", Status: vulnmanagement.StatusOpen, DiscoveredAt: time.Now(),
	}
	fB := &vulnmanagement.Finding{
		ID: uuid.New(), TenantID: tenantB, Source: vulnmanagement.ScannerSourceSCA,
		Package: "pkg-b", Severity: vulnmanagement.SeverityHigh, AdvisoryID: "CVE-B",
		Title: "Finding B", Status: vulnmanagement.StatusOpen, DiscoveredAt: time.Now(),
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

	if _, err := repo.Get(t.Context(), tenantA, fB.ID); !errors.Is(err, vulnmanagement.ErrFindingNotFound) {
		t.Fatalf("Get(tenantA, fB.ID) error = %v, want ErrFindingNotFound", err)
	}

	// Update scoped to the wrong tenant must not succeed either: fB
	// already carries TenantID=tenantB, so attempting to Update it
	// under tenantA's scope is rejected as a cross-tenant write before
	// the repository even looks the record up.
	fB.Title = "Hijacked"
	if err := repo.Update(t.Context(), tenantA, fB); !errors.Is(err, vulnmanagement.ErrCrossTenantAccess) {
		t.Fatalf("Update(tenantA, fB) error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestInMemoryTriageRepository_TenantIsolated mirrors the same
// guarantee for triage decisions.
func TestInMemoryTriageRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := vulnmanagement.NewInMemoryTriageRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()
	findingID := uuid.New()

	dA := &vulnmanagement.TriageDecision{
		ID: uuid.New(), TenantID: tenantA, FindingID: findingID,
		FromStatus: vulnmanagement.StatusOpen, ToStatus: vulnmanagement.StatusTriaged,
		Notes: "tenant A decision", Actor: uuid.New(), DecidedAt: time.Now(),
	}
	dB := &vulnmanagement.TriageDecision{
		ID: uuid.New(), TenantID: tenantB, FindingID: findingID,
		FromStatus: vulnmanagement.StatusOpen, ToStatus: vulnmanagement.StatusTriaged,
		Notes: "tenant B decision", Actor: uuid.New(), DecidedAt: time.Now(),
	}
	if err := repo.Create(t.Context(), tenantA, dA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, dB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListForFinding(t.Context(), tenantA, findingID)
	if err != nil {
		t.Fatalf("ListForFinding (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != dA.ID {
		t.Fatalf("ListForFinding(tenantA) = %v, want exactly dA", listA)
	}

	allA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(allA) != 1 {
		t.Fatalf("ListAll(tenantA) len = %d, want 1", len(allA))
	}
}

// TestEngine_Triage_CrossTenantFindingNotVisible proves that Triage
// scoped to tenantA can never reach a Finding recorded for tenantB,
// even when the caller happens to know its ID.
func TestEngine_Triage_CrossTenantFindingNotVisible(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()

	adminB := adminUser(tenantB)
	fB, err := engine.RecordFinding(ctxWithUser(adminB), tenantB, newTestFinding(tenantB))
	if err != nil {
		t.Fatalf("RecordFinding(tenantB): %v", err)
	}

	adminA := adminUser(tenantA)
	_, err = engine.Triage(ctxWithUser(adminA), tenantA, fB.ID, vulnmanagement.StatusTriaged, "attempted cross-tenant triage", uuid.Nil)
	if !errors.Is(err, vulnmanagement.ErrFindingNotFound) {
		t.Fatalf("Triage(cross-tenant finding) error = %v, want ErrFindingNotFound", err)
	}
}
