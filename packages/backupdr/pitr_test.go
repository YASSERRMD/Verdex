package backupdr_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/backupdr"
)

func mustRecord(tenantID uuid.UUID, class backupdr.DataClass, takenAt time.Time, status backupdr.BackupStatus) backupdr.BackupRecord {
	return backupdr.BackupRecord{
		ID:            uuid.New(),
		TenantID:      tenantID,
		Class:         class,
		TakenAt:       takenAt,
		Location:      backupdr.LocationPrimaryRegion,
		Reference:     "ref",
		IntegrityHash: "hash",
		Status:        status,
	}
}

// TestResolveRecoveryPoint_SelectsNearestAtOrBefore proves
// ResolveRecoveryPoint picks the latest succeeded record that is
// still at-or-before the requested instant, not simply the most
// recent record overall.
func TestResolveRecoveryPoint_SelectsNearestAtOrBefore(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	class := backupdr.DataClassCaseData
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	oldest := mustRecord(tenantID, class, base, backupdr.BackupStatusSucceeded)
	middle := mustRecord(tenantID, class, base.Add(2*time.Hour), backupdr.BackupStatusSucceeded)
	newest := mustRecord(tenantID, class, base.Add(4*time.Hour), backupdr.BackupStatusSucceeded)
	future := mustRecord(tenantID, class, base.Add(10*time.Hour), backupdr.BackupStatusSucceeded)

	records := []backupdr.BackupRecord{oldest, newest, middle, future}

	requestedAt := base.Add(3 * time.Hour) // between middle and newest
	point, err := backupdr.ResolveRecoveryPoint(records, tenantID, class, requestedAt)
	if err != nil {
		t.Fatalf("ResolveRecoveryPoint: %v", err)
	}
	if point.Record.ID != middle.ID {
		t.Fatalf("resolved record = %v, want middle (%v)", point.Record.ID, middle.ID)
	}
	wantAge := requestedAt.Sub(middle.TakenAt)
	if point.AgeAtRequest != wantAge {
		t.Fatalf("AgeAtRequest = %v, want %v", point.AgeAtRequest, wantAge)
	}
	if point.TenantID != tenantID || point.Class != class || !point.RequestedAt.Equal(requestedAt) {
		t.Fatalf("RecoveryPoint envelope fields incorrect: %+v", point)
	}
}

// TestResolveRecoveryPoint_ExactMatch proves a record taken exactly at
// the requested instant is itself eligible (at-or-before is
// inclusive), with zero AgeAtRequest.
func TestResolveRecoveryPoint_ExactMatch(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	class := backupdr.DataClassCaseData
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	exact := mustRecord(tenantID, class, at, backupdr.BackupStatusSucceeded)
	records := []backupdr.BackupRecord{exact}

	point, err := backupdr.ResolveRecoveryPoint(records, tenantID, class, at)
	if err != nil {
		t.Fatalf("ResolveRecoveryPoint: %v", err)
	}
	if point.Record.ID != exact.ID {
		t.Fatalf("resolved record = %v, want exact (%v)", point.Record.ID, exact.ID)
	}
	if point.AgeAtRequest != 0 {
		t.Fatalf("AgeAtRequest = %v, want 0", point.AgeAtRequest)
	}
}

// TestResolveRecoveryPoint_IgnoresOtherTenantAndClass proves
// ResolveRecoveryPoint filters strictly by tenant and class, never
// leaking a match across either boundary.
func TestResolveRecoveryPoint_IgnoresOtherTenantAndClass(t *testing.T) {
	t.Parallel()
	tenantA := uuid.New()
	tenantB := uuid.New()
	class := backupdr.DataClassCaseData
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	wrongTenant := mustRecord(tenantB, class, at, backupdr.BackupStatusSucceeded)
	wrongClass := mustRecord(tenantA, backupdr.DataClassConfig, at, backupdr.BackupStatusSucceeded)
	records := []backupdr.BackupRecord{wrongTenant, wrongClass}

	_, err := backupdr.ResolveRecoveryPoint(records, tenantA, class, at.Add(time.Hour))
	if !errors.Is(err, backupdr.ErrNoRecoveryPoint) {
		t.Fatalf("ResolveRecoveryPoint() error = %v, want ErrNoRecoveryPoint", err)
	}
}

// TestResolveRecoveryPoint_IgnoresUnsucceededRecords proves a failed or
// still-verifying BackupRecord is never eligible as a recovery point,
// even if it is the only record on file and its TakenAt otherwise
// qualifies.
func TestResolveRecoveryPoint_IgnoresUnsucceededRecords(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	class := backupdr.DataClassCaseData
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	failed := mustRecord(tenantID, class, at, backupdr.BackupStatusFailed)
	verifying := mustRecord(tenantID, class, at, backupdr.BackupStatusVerifying)
	records := []backupdr.BackupRecord{failed, verifying}

	_, err := backupdr.ResolveRecoveryPoint(records, tenantID, class, at.Add(time.Hour))
	if !errors.Is(err, backupdr.ErrNoRecoveryPoint) {
		t.Fatalf("ResolveRecoveryPoint() error = %v, want ErrNoRecoveryPoint", err)
	}
}

// TestResolveRecoveryPoint_NoRecordsBeforeRequestedAt proves that when
// every record on file postdates the requested instant,
// ResolveRecoveryPoint reports ErrNoRecoveryPoint rather than
// incorrectly selecting a future backup.
func TestResolveRecoveryPoint_NoRecordsBeforeRequestedAt(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	class := backupdr.DataClassCaseData
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	future := mustRecord(tenantID, class, base.Add(time.Hour), backupdr.BackupStatusSucceeded)
	records := []backupdr.BackupRecord{future}

	_, err := backupdr.ResolveRecoveryPoint(records, tenantID, class, base)
	if !errors.Is(err, backupdr.ErrNoRecoveryPoint) {
		t.Fatalf("ResolveRecoveryPoint() error = %v, want ErrNoRecoveryPoint", err)
	}
}

// TestResolveRecoveryPoint_EmptyRecords proves an empty input slice
// resolves to ErrNoRecoveryPoint rather than panicking.
func TestResolveRecoveryPoint_EmptyRecords(t *testing.T) {
	t.Parallel()
	_, err := backupdr.ResolveRecoveryPoint(nil, uuid.New(), backupdr.DataClassCaseData, time.Now())
	if !errors.Is(err, backupdr.ErrNoRecoveryPoint) {
		t.Fatalf("ResolveRecoveryPoint(nil) error = %v, want ErrNoRecoveryPoint", err)
	}
}
