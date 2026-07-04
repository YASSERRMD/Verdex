package backupdr_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/backupdr"
)

func TestNewEngine_NilStoreRejected(t *testing.T) {
	t.Parallel()
	policies := backupdr.NewInMemoryPolicyRepository()
	records := backupdr.NewInMemoryRecordRepository()
	drills := backupdr.NewInMemoryDrillRepository()
	targets := backupdr.NewInMemoryTargetRepository()

	tests := []struct {
		name string
		p    backupdr.PolicyRepository
		r    backupdr.RecordRepository
		d    backupdr.DrillRepository
		tg   backupdr.TargetRepository
	}{
		{"nil policies", nil, records, drills, targets},
		{"nil records", policies, nil, drills, targets},
		{"nil drills", policies, records, nil, targets},
		{"nil targets", policies, records, drills, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := backupdr.NewEngine(tt.p, tt.r, tt.d, tt.tg, nil)
			if !errors.Is(err, backupdr.ErrNilStore) {
				t.Fatalf("NewEngine() error = %v, want ErrNilStore", err)
			}
		})
	}
}

func TestEngine_SetPolicy_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	auditor := auditorUser(tenantID) // view-only, no manage

	_, err := engine.SetPolicy(ctxWithUser(auditor), tenantID, backupdr.BackupPolicy{
		Class: backupdr.DataClassCaseData, Frequency: time.Hour, RetentionWindow: 24 * time.Hour,
	})
	if !errors.Is(err, backupdr.ErrForbidden) {
		t.Fatalf("SetPolicy() error = %v, want ErrForbidden for view-only actor", err)
	}
}

func TestEngine_SetPolicy_RequiresAuthentication(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	_, err := engine.SetPolicy(t.Context(), tenantID, backupdr.BackupPolicy{
		Class: backupdr.DataClassCaseData, Frequency: time.Hour, RetentionWindow: 24 * time.Hour,
	})
	if !errors.Is(err, backupdr.ErrUnauthenticated) {
		t.Fatalf("SetPolicy() error = %v, want ErrUnauthenticated for anonymous context", err)
	}
}

func TestEngine_SetPolicy_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.SetPolicy(ctxWithUser(adminA), tenantB, backupdr.BackupPolicy{
		Class: backupdr.DataClassCaseData, Frequency: time.Hour, RetentionWindow: 24 * time.Hour,
	})
	if !errors.Is(err, backupdr.ErrCrossTenantAccess) {
		t.Fatalf("SetPolicy() error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestEngine_SetPolicy_RoundTrip(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	p := setTestPolicy(t, engine, tenantID, backupdr.DataClassCaseData)
	if p.TenantID != tenantID {
		t.Fatalf("SetPolicy() TenantID = %v, want %v", p.TenantID, tenantID)
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatal("SetPolicy() did not populate timestamps")
	}

	got, err := engine.PolicyFor(ctxWithUser(auditorUser(tenantID)), tenantID, backupdr.DataClassCaseData)
	if err != nil {
		t.Fatalf("PolicyFor: %v", err)
	}
	if got.Class != backupdr.DataClassCaseData {
		t.Fatalf("PolicyFor().Class = %v, want %v", got.Class, backupdr.DataClassCaseData)
	}
}

func TestEngine_SetPolicy_InvalidPolicyRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	_, err := engine.SetPolicy(ctxWithUser(admin), tenantID, backupdr.BackupPolicy{
		Class:     backupdr.DataClassCaseData,
		Frequency: 0, // invalid
	})
	if !errors.Is(err, backupdr.ErrInvalidPolicy) {
		t.Fatalf("SetPolicy() error = %v, want ErrInvalidPolicy", err)
	}
}

func TestEngine_ListPolicies(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	setTestPolicy(t, engine, tenantID, backupdr.DataClassCaseData)
	setTestPolicy(t, engine, tenantID, backupdr.DataClassAuditLog)

	list, err := engine.ListPolicies(ctxWithUser(auditorUser(tenantID)), tenantID)
	if err != nil {
		t.Fatalf("ListPolicies: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListPolicies() returned %d policies, want 2", len(list))
	}
}

func TestEngine_RecordBackup_RoundTrip(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	takenAt := time.Now().Add(-time.Hour)

	rec := recordTestBackup(t, engine, tenantID, backupdr.DataClassCaseData, takenAt)
	if rec.ID == uuid.Nil {
		t.Fatal("RecordBackup() did not assign an ID")
	}
	if rec.Status != backupdr.BackupStatusSucceeded {
		t.Fatalf("RecordBackup().Status = %v, want %v", rec.Status, backupdr.BackupStatusSucceeded)
	}

	got, err := engine.ListBackupRecords(ctxWithUser(auditorUser(tenantID)), tenantID)
	if err != nil {
		t.Fatalf("ListBackupRecords: %v", err)
	}
	if len(got) != 1 || got[0].ID != rec.ID {
		t.Fatalf("ListBackupRecords() = %v, want exactly rec", got)
	}
}

func TestEngine_RecordBackup_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.RecordBackup(ctxWithUser(adminA), tenantB, backupdr.BackupRecord{
		Class: backupdr.DataClassCaseData, Location: backupdr.LocationPrimaryRegion,
		Reference: "ref", IntegrityHash: "hash", Status: backupdr.BackupStatusSucceeded,
	})
	if !errors.Is(err, backupdr.ErrCrossTenantAccess) {
		t.Fatalf("RecordBackup() error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestEngine_RecordBackup_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	auditor := auditorUser(tenantID)

	_, err := engine.RecordBackup(ctxWithUser(auditor), tenantID, backupdr.BackupRecord{
		Class: backupdr.DataClassCaseData, Location: backupdr.LocationPrimaryRegion,
		Reference: "ref", IntegrityHash: "hash", Status: backupdr.BackupStatusSucceeded,
	})
	if !errors.Is(err, backupdr.ErrForbidden) {
		t.Fatalf("RecordBackup() error = %v, want ErrForbidden for view-only actor", err)
	}
}

// TestEngine_FindRecoveryPoint_EndToEnd proves the engine-level PITR
// entry point resolves through real records recorded via RecordBackup,
// not just the package-level function in isolation.
func TestEngine_FindRecoveryPoint_EndToEnd(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	base := time.Now().Add(-10 * time.Hour)

	older := recordTestBackup(t, engine, tenantID, backupdr.DataClassCaseData, base)
	newer := recordTestBackup(t, engine, tenantID, backupdr.DataClassCaseData, base.Add(2*time.Hour))

	point, err := engine.FindRecoveryPoint(ctxWithUser(auditorUser(tenantID)), tenantID, backupdr.DataClassCaseData, base.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("FindRecoveryPoint: %v", err)
	}
	if point.Record.ID != newer.ID {
		t.Fatalf("FindRecoveryPoint() resolved %v, want newer (%v)", point.Record.ID, newer.ID)
	}

	// Requesting a point before the older backup even existed must
	// fail -- there is nothing to recover from.
	_, err = engine.FindRecoveryPoint(ctxWithUser(auditorUser(tenantID)), tenantID, backupdr.DataClassCaseData, base.Add(-time.Hour))
	if !errors.Is(err, backupdr.ErrNoRecoveryPoint) {
		t.Fatalf("FindRecoveryPoint() error = %v, want ErrNoRecoveryPoint", err)
	}
	_ = older
}

func TestEngine_SetTarget_And_CheckRPO(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	_, err := engine.SetTarget(ctxWithUser(admin), tenantID, backupdr.Target{
		Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: 4 * time.Hour,
	})
	if err != nil {
		t.Fatalf("SetTarget: %v", err)
	}

	rec := recordTestBackup(t, engine, tenantID, backupdr.DataClassCaseData, time.Now().Add(-30*time.Minute))
	eval, err := engine.CheckRPO(ctxWithUser(auditorUser(tenantID)), tenantID, rec)
	if err != nil {
		t.Fatalf("CheckRPO: %v", err)
	}
	if !eval.Met {
		t.Fatal("CheckRPO().Met = false, want true for a 30m-old backup against a 1h RPO")
	}
}

func TestEngine_CheckRPO_NoTargetRegistered(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rec := recordTestBackup(t, engine, tenantID, backupdr.DataClassCaseData, time.Now())

	_, err := engine.CheckRPO(ctxWithUser(auditorUser(tenantID)), tenantID, rec)
	if !errors.Is(err, backupdr.ErrTargetNotFound) {
		t.Fatalf("CheckRPO() error = %v, want ErrTargetNotFound", err)
	}
}

// TestEngine_FindRecoveryPoint_AuditLogClass proves the engine's PITR
// path is not hardcoded to DataClassCaseData -- resolving a recovery
// point for DataClassAuditLog (Phase 077's own audit trail, itself in
// scope for backup per doc.go) works identically.
func TestEngine_FindRecoveryPoint_AuditLogClass(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	takenAt := time.Now().Add(-2 * time.Hour)
	rec := recordTestBackup(t, engine, tenantID, backupdr.DataClassAuditLog, takenAt)

	point, err := engine.FindRecoveryPoint(ctxWithUser(auditorUser(tenantID)), tenantID, backupdr.DataClassAuditLog, time.Now())
	if err != nil {
		t.Fatalf("FindRecoveryPoint: %v", err)
	}
	if point.Record.ID != rec.ID || point.Class != backupdr.DataClassAuditLog {
		t.Fatalf("FindRecoveryPoint() = %+v, want it to resolve rec under DataClassAuditLog", point)
	}
}

// TestEngine_SetPolicy_NeitherPermissionRejected proves an actor
// holding neither PermViewBackupDR nor PermManageBackupDR (e.g. a
// judge, whose role is unrelated to backup/DR administration) is
// rejected the same way a completely anonymous caller would be.
func TestEngine_SetPolicy_NeitherPermissionRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	judge := judgeUser(tenantID)

	_, err := engine.SetPolicy(ctxWithUser(judge), tenantID, backupdr.BackupPolicy{
		Class: backupdr.DataClassCaseData, Frequency: time.Hour, RetentionWindow: 24 * time.Hour,
	})
	if !errors.Is(err, backupdr.ErrForbidden) {
		t.Fatalf("SetPolicy() error = %v, want ErrForbidden for a judge (holds neither backupdr permission)", err)
	}

	if _, err := engine.ListPolicies(ctxWithUser(judge), tenantID); !errors.Is(err, backupdr.ErrForbidden) {
		t.Fatalf("ListPolicies() error = %v, want ErrForbidden for a judge (holds neither backupdr permission)", err)
	}
}
