package backupdr_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/backupdr"
)

func TestDataClass_IsValid(t *testing.T) {
	t.Parallel()

	valid := []backupdr.DataClass{
		backupdr.DataClassCaseData,
		backupdr.DataClassCorpusPrecedent,
		backupdr.DataClassAuditLog,
		backupdr.DataClassConfig,
	}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("DataClass(%q).IsValid() = false, want true", c)
		}
	}

	if backupdr.DataClass("bogus").IsValid() {
		t.Error("DataClass(\"bogus\").IsValid() = true, want false")
	}
	if backupdr.DataClass("").IsValid() {
		t.Error("DataClass(\"\").IsValid() = true, want false")
	}
}

func TestBackupLocation_IsValid(t *testing.T) {
	t.Parallel()

	valid := []backupdr.BackupLocation{
		backupdr.LocationPrimaryRegion,
		backupdr.LocationCrossRegion,
		backupdr.LocationOffline,
	}
	for _, l := range valid {
		if !l.IsValid() {
			t.Errorf("BackupLocation(%q).IsValid() = false, want true", l)
		}
	}
	if backupdr.BackupLocation("bogus").IsValid() {
		t.Error("BackupLocation(\"bogus\").IsValid() = true, want false")
	}
}

func TestBackupPolicy_Validate(t *testing.T) {
	t.Parallel()

	base := func() backupdr.BackupPolicy {
		return backupdr.BackupPolicy{
			TenantID:        uuid.New(),
			Class:           backupdr.DataClassCaseData,
			Frequency:       time.Hour,
			RetentionWindow: 24 * time.Hour,
		}
	}

	valid := base()
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid policy: Validate() = %v, want nil", err)
	}

	tests := []struct {
		name    string
		mutate  func(*backupdr.BackupPolicy)
		wantErr error
	}{
		{
			name:    "empty tenant id",
			mutate:  func(p *backupdr.BackupPolicy) { p.TenantID = uuid.Nil },
			wantErr: backupdr.ErrEmptyTenantID,
		},
		{
			name:    "invalid data class",
			mutate:  func(p *backupdr.BackupPolicy) { p.Class = "bogus" },
			wantErr: backupdr.ErrInvalidDataClass,
		},
		{
			name:    "zero frequency",
			mutate:  func(p *backupdr.BackupPolicy) { p.Frequency = 0 },
			wantErr: backupdr.ErrInvalidPolicy,
		},
		{
			name:    "negative frequency",
			mutate:  func(p *backupdr.BackupPolicy) { p.Frequency = -time.Hour },
			wantErr: backupdr.ErrInvalidPolicy,
		},
		{
			name:    "zero retention",
			mutate:  func(p *backupdr.BackupPolicy) { p.RetentionWindow = 0 },
			wantErr: backupdr.ErrInvalidPolicy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := base()
			tt.mutate(&p)
			err := p.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want error wrapping %v", err, tt.wantErr)
			}
		})
	}
}

func TestBackupPolicy_Validate_NilReceiver(t *testing.T) {
	t.Parallel()
	var p *backupdr.BackupPolicy
	if err := p.Validate(); !errors.Is(err, backupdr.ErrInvalidPolicy) {
		t.Fatalf("nil *BackupPolicy.Validate() = %v, want ErrInvalidPolicy", err)
	}
}

func TestBackupRecord_Validate(t *testing.T) {
	t.Parallel()

	base := func() backupdr.BackupRecord {
		return backupdr.BackupRecord{
			ID:            uuid.New(),
			TenantID:      uuid.New(),
			Class:         backupdr.DataClassCaseData,
			TakenAt:       time.Now(),
			Location:      backupdr.LocationPrimaryRegion,
			Reference:     "s3://bucket/key",
			IntegrityHash: "deadbeef",
			Status:        backupdr.BackupStatusSucceeded,
		}
	}

	valid := base()
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid record: Validate() = %v, want nil", err)
	}

	tests := []struct {
		name    string
		mutate  func(*backupdr.BackupRecord)
		wantErr error
	}{
		{
			name:    "empty tenant id",
			mutate:  func(r *backupdr.BackupRecord) { r.TenantID = uuid.Nil },
			wantErr: backupdr.ErrEmptyTenantID,
		},
		{
			name:    "invalid data class",
			mutate:  func(r *backupdr.BackupRecord) { r.Class = "bogus" },
			wantErr: backupdr.ErrInvalidDataClass,
		},
		{
			name:    "invalid location",
			mutate:  func(r *backupdr.BackupRecord) { r.Location = "bogus" },
			wantErr: backupdr.ErrInvalidRecord,
		},
		{
			name:    "blank reference",
			mutate:  func(r *backupdr.BackupRecord) { r.Reference = "   " },
			wantErr: backupdr.ErrInvalidRecord,
		},
		{
			name:    "invalid status",
			mutate:  func(r *backupdr.BackupRecord) { r.Status = "bogus" },
			wantErr: backupdr.ErrInvalidRecord,
		},
		{
			name: "succeeded without integrity hash",
			mutate: func(r *backupdr.BackupRecord) {
				r.Status = backupdr.BackupStatusSucceeded
				r.IntegrityHash = ""
			},
			wantErr: backupdr.ErrInvalidRecord,
		},
		{
			name:    "zero taken at",
			mutate:  func(r *backupdr.BackupRecord) { r.TakenAt = time.Time{} },
			wantErr: backupdr.ErrInvalidRecord,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := base()
			tt.mutate(&r)
			err := r.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want error wrapping %v", err, tt.wantErr)
			}
		})
	}
}

// TestBackupRecord_Validate_FailedWithoutHash proves a
// BackupStatusFailed record is allowed to have a blank IntegrityHash
// -- a failed backup never produced verifiable bytes, so there is
// nothing to hash yet, unlike a BackupStatusSucceeded record.
func TestBackupRecord_Validate_FailedWithoutHash(t *testing.T) {
	t.Parallel()
	r := backupdr.BackupRecord{
		TenantID:  uuid.New(),
		Class:     backupdr.DataClassCaseData,
		TakenAt:   time.Now(),
		Location:  backupdr.LocationPrimaryRegion,
		Reference: "s3://bucket/key",
		Status:    backupdr.BackupStatusFailed,
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("failed record without hash: Validate() = %v, want nil", err)
	}
}
