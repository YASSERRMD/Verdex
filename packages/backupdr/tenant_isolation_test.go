package backupdr_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/backupdr"
)

// TestEngine_ListBackupRecords_CrossTenantRejected proves an admin
// authenticated against tenant A can never list tenant B's backup
// records.
func TestEngine_ListBackupRecords_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.ListBackupRecords(ctxWithUser(adminA), tenantB)
	if !errors.Is(err, backupdr.ErrCrossTenantAccess) {
		t.Fatalf("ListBackupRecords() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_ListDrills_CrossTenantRejected mirrors the same guarantee
// for ListDrills.
func TestEngine_ListDrills_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.ListDrills(ctxWithUser(adminA), tenantB)
	if !errors.Is(err, backupdr.ErrCrossTenantAccess) {
		t.Fatalf("ListDrills() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_SetTarget_CrossTenantRejected mirrors the same guarantee
// for SetTarget.
func TestEngine_SetTarget_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.SetTarget(ctxWithUser(adminA), tenantB, backupdr.Target{
		Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: time.Hour,
	})
	if !errors.Is(err, backupdr.ErrCrossTenantAccess) {
		t.Fatalf("SetTarget() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestInMemoryPolicyRepository_TenantIsolated proves the repository
// layer itself never leaks a tenant's policies into another tenant's
// Get/ListAll results, independent of the Engine authorization layer
// above it.
func TestInMemoryPolicyRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := backupdr.NewInMemoryPolicyRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	policyA := &backupdr.BackupPolicy{
		TenantID: tenantA, Class: backupdr.DataClassCaseData,
		Frequency: time.Hour, RetentionWindow: 24 * time.Hour,
	}
	policyB := &backupdr.BackupPolicy{
		TenantID: tenantB, Class: backupdr.DataClassCaseData,
		Frequency: 2 * time.Hour, RetentionWindow: 48 * time.Hour,
	}

	if err := repo.Set(t.Context(), tenantA, policyA); err != nil {
		t.Fatalf("Set (A): %v", err)
	}
	if err := repo.Set(t.Context(), tenantB, policyB); err != nil {
		t.Fatalf("Set (B): %v", err)
	}

	listA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(listA) != 1 || listA[0].Frequency != time.Hour {
		t.Fatalf("ListAll(tenantA) = %v, want exactly policyA's frequency", listA)
	}

	// Same DataClass key, different tenant -- must resolve
	// independently, never tenant B's value under tenant A's lookup.
	gotA, err := repo.Get(t.Context(), tenantA, backupdr.DataClassCaseData)
	if err != nil {
		t.Fatalf("Get (A): %v", err)
	}
	if gotA.Frequency != time.Hour {
		t.Fatalf("Get(tenantA, DataClassCaseData).Frequency = %v, want 1h (policyA's, not policyB's)", gotA.Frequency)
	}
}

// TestInMemoryRecordRepository_TenantIsolated mirrors the same
// guarantee for backup records.
func TestInMemoryRecordRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := backupdr.NewInMemoryRecordRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	recA := &backupdr.BackupRecord{
		ID: uuid.New(), TenantID: tenantA, Class: backupdr.DataClassCaseData,
		TakenAt: time.Now(), Location: backupdr.LocationPrimaryRegion,
		Reference: "ref-a", IntegrityHash: "hash-a", Status: backupdr.BackupStatusSucceeded,
	}
	recB := &backupdr.BackupRecord{
		ID: uuid.New(), TenantID: tenantB, Class: backupdr.DataClassCaseData,
		TakenAt: time.Now(), Location: backupdr.LocationPrimaryRegion,
		Reference: "ref-b", IntegrityHash: "hash-b", Status: backupdr.BackupStatusSucceeded,
	}

	if err := repo.Create(t.Context(), tenantA, recA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, recB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != recA.ID {
		t.Fatalf("ListAll(tenantA) = %v, want exactly recA", listA)
	}
	for _, r := range listA {
		if r.IntegrityHash == "hash-b" {
			t.Fatal("tenant A's record list leaked tenant B's integrity hash")
		}
	}

	if _, err := repo.Get(t.Context(), tenantA, recB.ID); !errors.Is(err, backupdr.ErrRecordNotFound) {
		t.Fatalf("Get(tenantA, recB.ID) error = %v, want ErrRecordNotFound", err)
	}
}

// TestInMemoryDrillRepository_TenantIsolated mirrors the same
// guarantee for restore drills.
func TestInMemoryDrillRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := backupdr.NewInMemoryDrillRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	drillA := &backupdr.RestoreDrill{
		ID: uuid.New(), TenantID: tenantA, Class: backupdr.DataClassCaseData,
		RecordID: uuid.New(), ExecutedAt: time.Now(), Outcome: backupdr.DrillOutcomeSuccess,
	}
	drillB := &backupdr.RestoreDrill{
		ID: uuid.New(), TenantID: tenantB, Class: backupdr.DataClassCaseData,
		RecordID: uuid.New(), ExecutedAt: time.Now(), Outcome: backupdr.DrillOutcomeFailure,
	}

	if err := repo.Create(t.Context(), tenantA, drillA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, drillB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listB, err := repo.ListAll(t.Context(), tenantB)
	if err != nil {
		t.Fatalf("ListAll (B): %v", err)
	}
	if len(listB) != 1 || listB[0].ID != drillB.ID {
		t.Fatalf("ListAll(tenantB) = %v, want exactly drillB", listB)
	}

	if _, err := repo.Get(t.Context(), tenantB, drillA.ID); !errors.Is(err, backupdr.ErrDrillNotFound) {
		t.Fatalf("Get(tenantB, drillA.ID) error = %v, want ErrDrillNotFound", err)
	}
}

// TestInMemoryTargetRepository_TenantIsolated mirrors the same
// guarantee for RPO/RTO targets.
func TestInMemoryTargetRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := backupdr.NewInMemoryTargetRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	targetA := &backupdr.Target{TenantID: tenantA, Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: time.Hour}
	targetB := &backupdr.Target{TenantID: tenantB, Class: backupdr.DataClassCaseData, RPO: 6 * time.Hour, RTO: 6 * time.Hour}

	if err := repo.Set(t.Context(), tenantA, targetA); err != nil {
		t.Fatalf("Set (A): %v", err)
	}
	if err := repo.Set(t.Context(), tenantB, targetB); err != nil {
		t.Fatalf("Set (B): %v", err)
	}

	gotA, err := repo.Get(t.Context(), tenantA, backupdr.DataClassCaseData)
	if err != nil {
		t.Fatalf("Get (A): %v", err)
	}
	if gotA.RPO != time.Hour {
		t.Fatalf("Get(tenantA, DataClassCaseData).RPO = %v, want 1h (targetA's, not targetB's 6h)", gotA.RPO)
	}

	if _, err := repo.Get(t.Context(), uuid.New(), backupdr.DataClassCaseData); !errors.Is(err, backupdr.ErrTargetNotFound) {
		t.Fatalf("Get(unknown tenant) error = %v, want ErrTargetNotFound", err)
	}
}
