package corpusupdater_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/corpusupdater"
)

// TestEngine_CreateJob_CrossTenantRejected proves an admin
// authenticated against tenant A can never create a CorpusUpdateJob
// scoped to tenant B.
func TestEngine_CreateJob_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.CreateJob(ctxWithUser(adminA), tenantB, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if !errors.Is(err, corpusupdater.ErrCrossTenantAccess) {
		t.Fatalf("CreateJob() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_StageAmendment_CrossTenantRejected mirrors the same
// guarantee for StageAmendment.
func TestEngine_StageAmendment_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	job, err := engine.CreateJob(ctxWithUser(adminA), tenantA, corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
	})
	if err != nil {
		t.Fatalf("CreateJob() = %v", err)
	}

	_, err = engine.StageAmendment(ctxWithUser(adminA), tenantB, job.ID, corpusupdater.Amendment{
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		Citation:      "Some citation",
		EffectiveDate: time.Now(),
	})
	if !errors.Is(err, corpusupdater.ErrCrossTenantAccess) {
		t.Fatalf("StageAmendment() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_GetJob_CrossTenantRejected proves an admin authenticated
// against tenant A can never read a job by asking for tenant B's
// scope even when the tenantID argument mismatches their own context.
func TestEngine_GetJob_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.GetJob(ctxWithUser(adminA), tenantB, uuid.New())
	if !errors.Is(err, corpusupdater.ErrCrossTenantAccess) {
		t.Fatalf("GetJob() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_Rollback_CrossTenantRejected mirrors the same guarantee
// for Rollback.
func TestEngine_Rollback_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	err := engine.Rollback(ctxWithUser(adminA), tenantB, uuid.New(), newFakeTextStore())
	if !errors.Is(err, corpusupdater.ErrCrossTenantAccess) {
		t.Fatalf("Rollback() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestInMemoryJobRepository_TenantIsolated proves the repository layer
// itself never leaks a tenant's jobs into another tenant's List/Get
// results, independent of the Engine authorization layer above it.
func TestInMemoryJobRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := corpusupdater.NewInMemoryJobRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	jobA := &corpusupdater.CorpusUpdateJob{
		ID: uuid.New(), TenantID: tenantA, JurisdictionCode: "AE-DXB",
		TargetCorpus: corpusupdater.CorpusStatute, Status: corpusupdater.StatusPending,
	}
	if err := repo.Create(t.Context(), tenantA, jobA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}

	jobB := &corpusupdater.CorpusUpdateJob{
		ID: uuid.New(), TenantID: tenantB, JurisdictionCode: "AE-DXB",
		TargetCorpus: corpusupdater.CorpusStatute, Status: corpusupdater.StatusPending,
	}
	if err := repo.Create(t.Context(), tenantB, jobB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != jobA.ID {
		t.Fatalf("ListAll(tenantA) = %v, want exactly jobA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, jobB.ID); !errors.Is(err, corpusupdater.ErrJobNotFound) {
		t.Fatalf("Get(tenantA, jobB.ID) error = %v, want ErrJobNotFound", err)
	}
}

// TestInMemoryAmendmentRepository_TenantIsolated mirrors the same
// guarantee for amendments.
func TestInMemoryAmendmentRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := corpusupdater.NewInMemoryAmendmentRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()
	jobID := uuid.New()

	amendmentA := &corpusupdater.Amendment{
		ID: uuid.New(), TenantID: tenantA, JobID: jobID, TargetCorpus: corpusupdater.CorpusStatute,
		TargetID: "rule-1", ChangeType: corpusupdater.ChangeTypeAmend, Citation: "Citation A",
		EffectiveDate: time.Now(),
	}
	if err := repo.Create(t.Context(), tenantA, amendmentA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}

	amendmentB := &corpusupdater.Amendment{
		ID: uuid.New(), TenantID: tenantB, JobID: jobID, TargetCorpus: corpusupdater.CorpusStatute,
		TargetID: "rule-1", ChangeType: corpusupdater.ChangeTypeAmend, Citation: "Citation B",
		EffectiveDate: time.Now(),
	}
	if err := repo.Create(t.Context(), tenantB, amendmentB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListForJob(t.Context(), tenantA, jobID)
	if err != nil {
		t.Fatalf("ListForJob (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != amendmentA.ID {
		t.Fatalf("ListForJob(tenantA, jobID) = %v, want exactly amendmentA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, amendmentB.ID); !errors.Is(err, corpusupdater.ErrAmendmentNotFound) {
		t.Fatalf("Get(tenantA, amendmentB.ID) error = %v, want ErrAmendmentNotFound", err)
	}
}
