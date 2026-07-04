package backupdr_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/backupdr"
)

func TestTarget_Validate(t *testing.T) {
	t.Parallel()

	base := func() backupdr.Target {
		return backupdr.Target{
			TenantID: uuid.New(),
			Class:    backupdr.DataClassCaseData,
			RPO:      time.Hour,
			RTO:      4 * time.Hour,
		}
	}

	if err := base().Validate(); err != nil {
		t.Fatalf("valid target: Validate() = %v, want nil", err)
	}

	tests := []struct {
		name    string
		mutate  func(*backupdr.Target)
		wantErr error
	}{
		{"empty tenant", func(target *backupdr.Target) { target.TenantID = uuid.Nil }, backupdr.ErrEmptyTenantID},
		{"invalid class", func(target *backupdr.Target) { target.Class = "bogus" }, backupdr.ErrInvalidDataClass},
		{"zero rpo", func(target *backupdr.Target) { target.RPO = 0 }, backupdr.ErrInvalidTarget},
		{"negative rpo", func(target *backupdr.Target) { target.RPO = -time.Minute }, backupdr.ErrInvalidTarget},
		{"zero rto", func(target *backupdr.Target) { target.RTO = 0 }, backupdr.ErrInvalidTarget},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := base()
			tt.mutate(&target)
			if err := target.Validate(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want error wrapping %v", err, tt.wantErr)
			}
		})
	}
}

// TestEvaluateRPO_Met proves a backup younger than the RPO passes.
func TestEvaluateRPO_Met(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	target := backupdr.Target{TenantID: tenantID, Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: 4 * time.Hour}
	asOf := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	record := backupdr.BackupRecord{Class: backupdr.DataClassCaseData, TakenAt: asOf.Add(-30 * time.Minute)}

	eval, err := backupdr.EvaluateRPO(record, target, asOf)
	if err != nil {
		t.Fatalf("EvaluateRPO: %v", err)
	}
	if !eval.Met {
		t.Fatalf("EvaluateRPO().Met = false, want true (age %v <= RPO %v)", eval.Age, target.RPO)
	}
	if eval.Age != 30*time.Minute {
		t.Fatalf("EvaluateRPO().Age = %v, want 30m", eval.Age)
	}
}

// TestEvaluateRPO_NotMet proves a backup older than the RPO fails --
// the explicit failing case the design guidance calls for.
func TestEvaluateRPO_NotMet(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	target := backupdr.Target{TenantID: tenantID, Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: 4 * time.Hour}
	asOf := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	record := backupdr.BackupRecord{Class: backupdr.DataClassCaseData, TakenAt: asOf.Add(-2 * time.Hour)}

	eval, err := backupdr.EvaluateRPO(record, target, asOf)
	if err != nil {
		t.Fatalf("EvaluateRPO: %v", err)
	}
	if eval.Met {
		t.Fatalf("EvaluateRPO().Met = true, want false (age %v > RPO %v)", eval.Age, target.RPO)
	}
}

// TestEvaluateRPO_ExactlyAtBoundary proves an age exactly equal to the
// RPO is still considered met (<=, not <).
func TestEvaluateRPO_ExactlyAtBoundary(t *testing.T) {
	t.Parallel()
	target := backupdr.Target{TenantID: uuid.New(), Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: 4 * time.Hour}
	asOf := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	record := backupdr.BackupRecord{Class: backupdr.DataClassCaseData, TakenAt: asOf.Add(-time.Hour)}

	eval, err := backupdr.EvaluateRPO(record, target, asOf)
	if err != nil {
		t.Fatalf("EvaluateRPO: %v", err)
	}
	if !eval.Met {
		t.Fatal("EvaluateRPO().Met = false, want true at exact RPO boundary")
	}
}

// TestEvaluateRPO_ClassMismatch proves EvaluateRPO refuses to evaluate
// a record against a Target registered for a different DataClass.
func TestEvaluateRPO_ClassMismatch(t *testing.T) {
	t.Parallel()
	target := backupdr.Target{TenantID: uuid.New(), Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: time.Hour}
	record := backupdr.BackupRecord{Class: backupdr.DataClassConfig, TakenAt: time.Now()}

	_, err := backupdr.EvaluateRPO(record, target, time.Now())
	if !errors.Is(err, backupdr.ErrInvalidTarget) {
		t.Fatalf("EvaluateRPO() error = %v, want ErrInvalidTarget for class mismatch", err)
	}
}

// TestEvaluateRTO_Met proves a drill finishing within the RTO passes.
func TestEvaluateRTO_Met(t *testing.T) {
	t.Parallel()
	target := backupdr.Target{TenantID: uuid.New(), Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: 4 * time.Hour}
	drill := backupdr.RestoreDrill{Class: backupdr.DataClassCaseData, Duration: 2 * time.Hour}

	eval, err := backupdr.EvaluateRTO(drill, target)
	if err != nil {
		t.Fatalf("EvaluateRTO: %v", err)
	}
	if !eval.Met {
		t.Fatalf("EvaluateRTO().Met = false, want true (duration %v <= RTO %v)", drill.Duration, target.RTO)
	}
}

// TestEvaluateRTO_NotMet proves a drill exceeding the RTO fails -- the
// explicit failing case the design guidance calls for.
func TestEvaluateRTO_NotMet(t *testing.T) {
	t.Parallel()
	target := backupdr.Target{TenantID: uuid.New(), Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: 4 * time.Hour}
	drill := backupdr.RestoreDrill{Class: backupdr.DataClassCaseData, Duration: 6 * time.Hour}

	eval, err := backupdr.EvaluateRTO(drill, target)
	if err != nil {
		t.Fatalf("EvaluateRTO: %v", err)
	}
	if eval.Met {
		t.Fatalf("EvaluateRTO().Met = true, want false (duration %v > RTO %v)", drill.Duration, target.RTO)
	}
}

// TestEvaluateRTO_ClassMismatch proves EvaluateRTO refuses to evaluate
// a drill against a Target registered for a different DataClass.
func TestEvaluateRTO_ClassMismatch(t *testing.T) {
	t.Parallel()
	target := backupdr.Target{TenantID: uuid.New(), Class: backupdr.DataClassCaseData, RPO: time.Hour, RTO: time.Hour}
	drill := backupdr.RestoreDrill{Class: backupdr.DataClassAuditLog, Duration: time.Minute}

	_, err := backupdr.EvaluateRTO(drill, target)
	if !errors.Is(err, backupdr.ErrInvalidTarget) {
		t.Fatalf("EvaluateRTO() error = %v, want ErrInvalidTarget for class mismatch", err)
	}
}
