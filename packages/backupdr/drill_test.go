package backupdr_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/backupdr"
)

func TestDrillOutcome_IsValid(t *testing.T) {
	t.Parallel()
	valid := []backupdr.DrillOutcome{
		backupdr.DrillOutcomeSuccess,
		backupdr.DrillOutcomeFailure,
		backupdr.DrillOutcomePartial,
	}
	for _, o := range valid {
		if !o.IsValid() {
			t.Errorf("DrillOutcome(%q).IsValid() = false, want true", o)
		}
	}
	if backupdr.DrillOutcome("bogus").IsValid() {
		t.Error("DrillOutcome(\"bogus\").IsValid() = true, want false")
	}
}

func TestRestoreDrill_Validate(t *testing.T) {
	t.Parallel()

	base := func() backupdr.RestoreDrill {
		return backupdr.RestoreDrill{
			TenantID:   uuid.New(),
			Class:      backupdr.DataClassCaseData,
			RecordID:   uuid.New(),
			ExecutedAt: time.Now(),
			Outcome:    backupdr.DrillOutcomeSuccess,
			Duration:   time.Minute,
		}
	}

	valid := base()
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid drill: Validate() = %v, want nil", err)
	}

	tests := []struct {
		name    string
		mutate  func(*backupdr.RestoreDrill)
		wantErr error
	}{
		{"empty tenant", func(d *backupdr.RestoreDrill) { d.TenantID = uuid.Nil }, backupdr.ErrEmptyTenantID},
		{"invalid class", func(d *backupdr.RestoreDrill) { d.Class = "bogus" }, backupdr.ErrInvalidDataClass},
		{"invalid outcome", func(d *backupdr.RestoreDrill) { d.Outcome = "bogus" }, backupdr.ErrInvalidDrill},
		{"zero executed at", func(d *backupdr.RestoreDrill) { d.ExecutedAt = time.Time{} }, backupdr.ErrInvalidDrill},
		{"negative duration", func(d *backupdr.RestoreDrill) { d.Duration = -time.Minute }, backupdr.ErrInvalidDrill},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := base()
			tt.mutate(&d)
			if err := d.Validate(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want error wrapping %v", err, tt.wantErr)
			}
		})
	}
}

func TestRestoreDrill_Validate_NilReceiver(t *testing.T) {
	t.Parallel()
	var d *backupdr.RestoreDrill
	if err := d.Validate(); !errors.Is(err, backupdr.ErrInvalidDrill) {
		t.Fatalf("nil *RestoreDrill.Validate() = %v, want ErrInvalidDrill", err)
	}
}

// TestEngine_RunDrill_Success proves a drill against a succeeded
// BackupRecord with a matching recomputed hash resolves to
// DrillOutcomeSuccess and is persisted with a real Duration.
func TestEngine_RunDrill_Success(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rec := recordTestBackup(t, engine, tenantID, backupdr.DataClassCaseData, time.Now().Add(-time.Hour))
	admin := adminUser(tenantID)

	drill, err := engine.RunDrill(ctxWithUser(admin), tenantID, rec.ID, rec.IntegrityHash, "quarterly game day")
	if err != nil {
		t.Fatalf("RunDrill: %v", err)
	}
	if drill.Outcome != backupdr.DrillOutcomeSuccess {
		t.Fatalf("RunDrill().Outcome = %v, want %v", drill.Outcome, backupdr.DrillOutcomeSuccess)
	}
	if drill.RecordID != rec.ID {
		t.Fatalf("RunDrill().RecordID = %v, want %v", drill.RecordID, rec.ID)
	}
	if drill.Class != rec.Class {
		t.Fatalf("RunDrill().Class = %v, want %v", drill.Class, rec.Class)
	}
	if !strings.Contains(drill.Notes, "quarterly game day") {
		t.Fatalf("RunDrill().Notes = %q, want it to contain the caller-supplied note", drill.Notes)
	}
	if drill.Executor != admin.ID {
		t.Fatalf("RunDrill().Executor = %v, want %v", drill.Executor, admin.ID)
	}

	// The drill must actually be persisted, not just returned.
	list, err := engine.ListDrills(ctxWithUser(auditorUser(tenantID)), tenantID)
	if err != nil {
		t.Fatalf("ListDrills: %v", err)
	}
	if len(list) != 1 || list[0].ID != drill.ID {
		t.Fatalf("ListDrills() = %v, want exactly the recorded drill", list)
	}
}

// TestEngine_RunDrill_FailsAgainstFailedBackup proves a drill against
// a BackupRecord whose Status is not Succeeded resolves to
// DrillOutcomeFailure -- the drill cannot restore from something that
// was never a valid backup, regardless of what hash is supplied.
func TestEngine_RunDrill_FailsAgainstFailedBackup(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	rec, err := engine.RecordBackup(ctxWithUser(admin), tenantID, backupdr.BackupRecord{
		Class:     backupdr.DataClassCaseData,
		TakenAt:   time.Now(),
		Location:  backupdr.LocationPrimaryRegion,
		Reference: "broken-backup",
		Status:    backupdr.BackupStatusFailed,
	})
	if err != nil {
		t.Fatalf("RecordBackup: %v", err)
	}

	drill, err := engine.RunDrill(ctxWithUser(admin), tenantID, rec.ID, "any-hash", "")
	if err != nil {
		t.Fatalf("RunDrill: %v", err)
	}
	if drill.Outcome != backupdr.DrillOutcomeFailure {
		t.Fatalf("RunDrill().Outcome = %v, want %v for a non-succeeded source record", drill.Outcome, backupdr.DrillOutcomeFailure)
	}
}

// TestEngine_RunDrill_PartialOnIntegrityMismatch proves a drill
// against a succeeded BackupRecord whose recomputed hash does NOT
// match the stored IntegrityHash resolves to DrillOutcomePartial --
// the restore step itself would have "worked" but verification
// caught real data corruption.
func TestEngine_RunDrill_PartialOnIntegrityMismatch(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rec := recordTestBackup(t, engine, tenantID, backupdr.DataClassCaseData, time.Now().Add(-time.Hour))
	admin := adminUser(tenantID)

	wrongHash := backupdr.ComputeIntegrityHash([]byte("this is not the same content"))
	drill, err := engine.RunDrill(ctxWithUser(admin), tenantID, rec.ID, wrongHash, "")
	if err != nil {
		t.Fatalf("RunDrill: %v", err)
	}
	if drill.Outcome != backupdr.DrillOutcomePartial {
		t.Fatalf("RunDrill().Outcome = %v, want %v for an integrity mismatch", drill.Outcome, backupdr.DrillOutcomePartial)
	}
	if !strings.Contains(drill.Notes, "integrity verification failed") {
		t.Fatalf("RunDrill().Notes = %q, want it to explain the integrity failure", drill.Notes)
	}
}

func TestEngine_RunDrill_UnknownRecordRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	_, err := engine.RunDrill(ctxWithUser(admin), tenantID, uuid.New(), "hash", "")
	if !errors.Is(err, backupdr.ErrRecordNotFound) {
		t.Fatalf("RunDrill() error = %v, want ErrRecordNotFound", err)
	}
}

func TestEngine_RunDrill_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rec := recordTestBackup(t, engine, tenantID, backupdr.DataClassCaseData, time.Now())
	auditor := auditorUser(tenantID)

	_, err := engine.RunDrill(ctxWithUser(auditor), tenantID, rec.ID, rec.IntegrityHash, "")
	if !errors.Is(err, backupdr.ErrForbidden) {
		t.Fatalf("RunDrill() error = %v, want ErrForbidden for view-only actor", err)
	}
}

func TestEngine_RunDrill_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	rec := recordTestBackup(t, engine, tenantA, backupdr.DataClassCaseData, time.Now())
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.RunDrill(ctxWithUser(adminA), tenantB, rec.ID, rec.IntegrityHash, "")
	if !errors.Is(err, backupdr.ErrCrossTenantAccess) {
		t.Fatalf("RunDrill() error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_RunDrill_CrossTenantRecordIsolated proves a drill cannot
// be run against another tenant's BackupRecord even by ID guess --
// the record lookup itself is tenant-scoped.
func TestEngine_RunDrill_CrossTenantRecordIsolated(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	// Manually build a second engine sharing the same repositories so
	// tenantB's own admin can attempt to run a drill against tenantA's
	// record ID.
	recA := recordTestBackup(t, engine, tenantA, backupdr.DataClassCaseData, time.Now())
	adminB := adminUser(tenantB)

	_, err := engine.RunDrill(ctxWithUser(adminB), tenantB, recA.ID, recA.IntegrityHash, "")
	if !errors.Is(err, backupdr.ErrRecordNotFound) {
		t.Fatalf("RunDrill() error = %v, want ErrRecordNotFound (tenant B must not see tenant A's record)", err)
	}
}

// TestEngine_RunDrill_ThenCheckRTO exercises the full task-5-into-
// task-6 pipeline: a drill runs, its real (near-zero, since this is
// simulated) Duration is recorded, and CheckRTO evaluates that
// Duration against a registered Target.
func TestEngine_RunDrill_ThenCheckRTO(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)
	rec := recordTestBackup(t, engine, tenantID, backupdr.DataClassCaseData, time.Now().Add(-time.Hour))

	if _, err := engine.SetTarget(ctxWithUser(admin), tenantID, backupdr.Target{
		Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: 4 * time.Hour,
	}); err != nil {
		t.Fatalf("SetTarget: %v", err)
	}

	drill, err := engine.RunDrill(ctxWithUser(admin), tenantID, rec.ID, rec.IntegrityHash, "")
	if err != nil {
		t.Fatalf("RunDrill: %v", err)
	}

	eval, err := engine.CheckRTO(ctxWithUser(auditorUser(tenantID)), tenantID, drill)
	if err != nil {
		t.Fatalf("CheckRTO: %v", err)
	}
	if !eval.Met {
		t.Fatalf("CheckRTO().Met = false, want true for a near-instant simulated drill against a 4h RTO (duration=%v)", drill.Duration)
	}
}
