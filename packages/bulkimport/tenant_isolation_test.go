package bulkimport_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/bulkimport"
)

// TestEngine_RegisterJob_CrossTenantRejected proves an admin
// authenticated against tenant A can never register an ImportJob
// scoped to tenant B.
func TestEngine_RegisterJob_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.RegisterJob(ctxWithUser(adminA), tenantB, "cross tenant corpus", 1)
	if !errors.Is(err, bulkimport.ErrCrossTenantAccess) {
		t.Fatalf("RegisterJob() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_RunBatch_CrossTenantRejected mirrors the same guarantee
// for RunBatch.
func TestEngine_RunBatch_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)
	adminB := adminUser(tenantB)

	job, err := engine.RegisterJob(ctxWithUser(adminB), tenantB, "tenant b corpus", 1)
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(1))
	_, err = engine.RunBatch(ctxWithUser(adminA), tenantB, job.ID, source, 4)
	if !errors.Is(err, bulkimport.ErrCrossTenantAccess) {
		t.Fatalf("RunBatch() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_Rollback_CrossTenantRejected mirrors the same guarantee
// for Rollback.
func TestEngine_Rollback_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.Rollback(ctxWithUser(adminA), tenantB, uuid.New())
	if !errors.Is(err, bulkimport.ErrCrossTenantAccess) {
		t.Fatalf("Rollback() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestInMemoryJobRepository_TenantIsolated proves the repository layer
// itself never leaks a tenant's jobs into another tenant's List/Get
// results, independent of the Engine authorization layer above it.
func TestInMemoryJobRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := bulkimport.NewInMemoryJobRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	jobA := &bulkimport.ImportJob{
		ID: uuid.New(), TenantID: tenantA, SourceDescription: "corpus a",
		Status: bulkimport.StatusPending,
	}
	jobB := &bulkimport.ImportJob{
		ID: uuid.New(), TenantID: tenantB, SourceDescription: "corpus b",
		Status: bulkimport.StatusPending,
	}

	if err := repo.Create(t.Context(), tenantA, jobA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, jobB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.List(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("List (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != jobA.ID {
		t.Fatalf("List(tenantA) = %v, want exactly jobA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, jobB.ID); !errors.Is(err, bulkimport.ErrJobNotFound) {
		t.Fatalf("Get(tenantA, jobB.ID) error = %v, want ErrJobNotFound", err)
	}
}

// TestInMemoryRecordRepository_TenantIsolated mirrors the same
// guarantee for ImportRecord, including FindByDedupKey never matching
// across a tenant boundary even when both tenants happen to import an
// identically-keyed record.
func TestInMemoryRecordRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := bulkimport.NewInMemoryRecordRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()
	jobA := uuid.New()
	jobB := uuid.New()

	sameDedupKey := bulkimport.ComputeDedupKey("CASE-1", "j1", []string{"Jane Doe"})

	recA := &bulkimport.ImportRecord{
		ID: uuid.New(), TenantID: tenantA, JobID: jobA, DedupKey: sameDedupKey,
		ValidationStatus: bulkimport.ValidationPassed, Outcome: bulkimport.OutcomeImported,
	}
	recB := &bulkimport.ImportRecord{
		ID: uuid.New(), TenantID: tenantB, JobID: jobB, DedupKey: sameDedupKey,
		ValidationStatus: bulkimport.ValidationPassed, Outcome: bulkimport.OutcomeImported,
	}

	if err := repo.Create(t.Context(), tenantA, recA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, recB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listB, err := repo.ListForJob(t.Context(), tenantB, jobB)
	if err != nil {
		t.Fatalf("ListForJob (B): %v", err)
	}
	if len(listB) != 1 || listB[0].ID != recB.ID {
		t.Fatalf("ListForJob(tenantB, jobB) = %v, want exactly recB", listB)
	}

	if _, err := repo.Get(t.Context(), tenantB, recA.ID); !errors.Is(err, bulkimport.ErrRecordNotFound) {
		t.Fatalf("Get(tenantB, recA.ID) error = %v, want ErrRecordNotFound", err)
	}

	// Tenant B's FindByDedupKey scoped to tenant A's job must not
	// return tenant A's record, even though the DedupKey matches.
	if _, err := repo.FindByDedupKey(t.Context(), tenantB, jobA, sameDedupKey); !errors.Is(err, bulkimport.ErrRecordNotFound) {
		t.Fatalf("FindByDedupKey(tenantB, jobA, ...) error = %v, want ErrRecordNotFound", err)
	}

	found, err := repo.FindByDedupKey(t.Context(), tenantB, jobB, sameDedupKey)
	if err != nil {
		t.Fatalf("FindByDedupKey(tenantB, jobB, ...): %v", err)
	}
	if found.ID != recB.ID {
		t.Fatalf("FindByDedupKey(tenantB, jobB, ...) = %v, want recB", found)
	}
}
