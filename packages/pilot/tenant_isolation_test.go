package pilot_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/pilot"
)

// TestInMemoryDeploymentRepository_TenantIsolated proves the
// repository layer itself never leaks a tenant's deployments into
// another tenant's List/Get results, independent of the Engine
// authorization layer above it.
func TestInMemoryDeploymentRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := pilot.NewInMemoryDeploymentRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	dA := &pilot.PilotDeployment{
		ID: uuid.New(), TenantID: tenantA, Name: "Pilot A", JurisdictionCode: "AE-DXB-COMM",
		Status: pilot.DeploymentStatusProvisioning, StartDate: time.Now(),
	}
	dB := &pilot.PilotDeployment{
		ID: uuid.New(), TenantID: tenantB, Name: "Pilot B", JurisdictionCode: "AE-AUH-COMM",
		Status: pilot.DeploymentStatusProvisioning, StartDate: time.Now(),
	}
	if err := repo.Create(t.Context(), tenantA, dA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, dB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != dA.ID {
		t.Fatalf("ListAll(tenantA) = %v, want exactly dA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, dB.ID); !errors.Is(err, pilot.ErrDeploymentNotFound) {
		t.Fatalf("Get(tenantA, dB.ID) error = %v, want ErrDeploymentNotFound", err)
	}
}

// TestInMemoryFindingRepository_TenantIsolated mirrors the same
// guarantee for pilot findings.
func TestInMemoryFindingRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := pilot.NewInMemoryFindingRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()
	deploymentID := uuid.New()

	fA := &pilot.PilotFinding{
		ID: uuid.New(), TenantID: tenantA, DeploymentID: deploymentID,
		SourceFeedbackIDs: []uuid.UUID{uuid.New()}, Title: "Finding A",
		Priority: pilot.PriorityHigh, Status: pilot.FindingStatusOpen, DiscoveredAt: time.Now(),
	}
	fB := &pilot.PilotFinding{
		ID: uuid.New(), TenantID: tenantB, DeploymentID: deploymentID,
		SourceFeedbackIDs: []uuid.UUID{uuid.New()}, Title: "Finding B",
		Priority: pilot.PriorityLow, Status: pilot.FindingStatusOpen, DiscoveredAt: time.Now(),
	}
	if err := repo.Create(t.Context(), tenantA, fA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, fB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListForDeployment(t.Context(), tenantA, deploymentID)
	if err != nil {
		t.Fatalf("ListForDeployment (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != fA.ID {
		t.Fatalf("ListForDeployment(tenantA) = %v, want exactly fA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, fB.ID); !errors.Is(err, pilot.ErrFindingNotFound) {
		t.Fatalf("Get(tenantA, fB.ID) error = %v, want ErrFindingNotFound", err)
	}
}

// TestEngine_GetDeployment_CrossTenantRejected proves an admin
// authenticated against tenant A can never read tenant B's
// PilotDeployment.
func TestEngine_GetDeployment_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	tenantB := uuid.New()
	adminB := adminUser(tenantB)

	_, err := te.engine.GetDeployment(ctxWithUser(adminB), tenantB, d.ID)
	// The deployment simply does not exist for tenantB's scope, so this
	// resolves to ErrDeploymentNotFound (not ErrCrossTenantAccess) --
	// what matters is that no data from tenant A is ever returned.
	if !errors.Is(err, pilot.ErrDeploymentNotFound) {
		t.Fatalf("GetDeployment cross-tenant error = %v, want ErrDeploymentNotFound", err)
	}
}

// TestEngine_ProvisionDeployment_CrossTenantRejected proves an admin
// authenticated against tenant A can never provision a deployment
// under tenant B's ID.
func TestEngine_ProvisionDeployment_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(te.tenantID)

	_, err := te.engine.ProvisionDeployment(ctxWithUser(adminA), tenantB, pilot.PilotDeployment{
		Name:             "Should be rejected",
		JurisdictionCode: "AE-DXB-COMM",
		StartDate:        time.Now(),
	})
	if !errors.Is(err, pilot.ErrCrossTenantAccess) {
		t.Fatalf("ProvisionDeployment cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_ListFindingsForDeployment_TenantsDoNotLeak proves that
// tenant B's finding list never reflects tenant A's findings, even
// though both tenants create findings under deployments with
// independently-scoped IDs.
func TestEngine_ListFindingsForDeployment_TenantsDoNotLeak(t *testing.T) {
	t.Parallel()
	teA := newTestEngine(t)
	dA := provisionAndActivate(t, teA)
	pcA := assignTestCase(t, teA, dA.ID)
	entryA := submitTestFeedback(t, teA, pcA.ID)
	recordTestFinding(t, teA, dA.ID, entryA.ID)

	teB := newTestEngine(t)
	dB := provisionAndActivate(t, teB)
	adminB := adminUser(teB.tenantID)

	listB, err := teB.engine.ListFindingsForDeployment(ctxWithUser(adminB), teB.tenantID, dB.ID)
	if err != nil {
		t.Fatalf("ListFindingsForDeployment (B): %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("ListFindingsForDeployment(B) = %v, want empty (tenant A's finding must not leak)", listB)
	}
}

// TestEngine_SubmitFeedback_CrossTenantRejected proves an admin
// authenticated against tenant A can never submit feedback scoped to
// tenant B.
func TestEngine_SubmitFeedback_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	tenantB := uuid.New()
	adminA := adminUser(te.tenantID)

	_, err := te.engine.SubmitFeedback(ctxWithUser(adminA), tenantB, pilot.FeedbackEntry{
		PilotCaseID: pc.ID,
		Ratings:     []pilot.DimensionRating{{Dimension: pilot.DimensionGrounding, Score: 0.5}},
		Trust:       pilot.TrustModerate,
	})
	if !errors.Is(err, pilot.ErrCrossTenantAccess) {
		t.Fatalf("SubmitFeedback cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}
