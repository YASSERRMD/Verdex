package privacy_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/privacy"
)

// TestHasValidConsent_ActiveGrant proves a subject with a single,
// unwithdrawn ConsentRecord for a purpose has valid consent.
func TestHasValidConsent_ActiveGrant(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	records := []privacy.ConsentRecord{
		{SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent, GrantedAt: now.Add(-time.Hour)},
	}

	if !privacy.HasValidConsent(records, "subject-1", "case_analytics", now) {
		t.Fatal("HasValidConsent() = false, want true for an active, unwithdrawn grant")
	}
}

// TestHasValidConsent_WithdrawnNoConsent proves a subject with only a
// withdrawn record (no subsequent re-grant) has no valid consent --
// this is the exact scenario the brief calls out as "real logic, not
// a stub".
func TestHasValidConsent_WithdrawnNoConsent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	withdrawnAt := now.Add(-time.Hour)

	records := []privacy.ConsentRecord{
		{SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent, GrantedAt: now.Add(-48 * time.Hour), WithdrawnAt: &withdrawnAt},
	}

	if privacy.HasValidConsent(records, "subject-1", "case_analytics", now) {
		t.Fatal("HasValidConsent() = true, want false for a withdrawn record with no re-grant")
	}
}

// TestHasValidConsent_WithdrawnThenReGranted proves a subsequent
// ConsentRecord after a withdrawal correctly re-establishes valid
// consent.
func TestHasValidConsent_WithdrawnThenReGranted(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	withdrawnAt := now.Add(-48 * time.Hour)

	records := []privacy.ConsentRecord{
		{SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent, GrantedAt: now.Add(-72 * time.Hour), WithdrawnAt: &withdrawnAt},
		{SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent, GrantedAt: now.Add(-24 * time.Hour)},
	}

	if !privacy.HasValidConsent(records, "subject-1", "case_analytics", now) {
		t.Fatal("HasValidConsent() = false, want true after a re-grant following withdrawal")
	}
}

// TestHasValidConsent_DifferentPurpose proves a consent grant for one
// purpose does not satisfy a check for a different purpose.
func TestHasValidConsent_DifferentPurpose(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	records := []privacy.ConsentRecord{
		{SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent, GrantedAt: now.Add(-time.Hour)},
	}

	if privacy.HasValidConsent(records, "subject-1", "third_party_sharing", now) {
		t.Fatal("HasValidConsent() = true, want false for a different purpose")
	}
}

// TestHasValidConsent_FutureGrant proves a ConsentRecord whose
// GrantedAt is in the future is not yet active.
func TestHasValidConsent_FutureGrant(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	records := []privacy.ConsentRecord{
		{SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent, GrantedAt: now.Add(time.Hour)},
	}

	if privacy.HasValidConsent(records, "subject-1", "case_analytics", now) {
		t.Fatal("HasValidConsent() = true, want false for a not-yet-effective grant")
	}
}

func TestConsentRecord_Validate(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	granted := time.Now()
	withdrawnBeforeGrant := granted.Add(-time.Hour)

	tests := []struct {
		name    string
		record  privacy.ConsentRecord
		wantErr error
	}{
		{
			name:   "valid",
			record: privacy.ConsentRecord{TenantID: tenantID, SubjectID: "s1", Purpose: "p1", LegalBasis: privacy.BasisConsent, GrantedAt: granted},
		},
		{
			name:    "empty tenant",
			record:  privacy.ConsentRecord{SubjectID: "s1", Purpose: "p1", LegalBasis: privacy.BasisConsent, GrantedAt: granted},
			wantErr: privacy.ErrEmptyTenantID,
		},
		{
			name:    "blank subject",
			record:  privacy.ConsentRecord{TenantID: tenantID, SubjectID: " ", Purpose: "p1", LegalBasis: privacy.BasisConsent, GrantedAt: granted},
			wantErr: privacy.ErrInvalidConsentRecord,
		},
		{
			name:    "blank purpose",
			record:  privacy.ConsentRecord{TenantID: tenantID, SubjectID: "s1", Purpose: "", LegalBasis: privacy.BasisConsent, GrantedAt: granted},
			wantErr: privacy.ErrInvalidConsentRecord,
		},
		{
			name:    "invalid legal basis",
			record:  privacy.ConsentRecord{TenantID: tenantID, SubjectID: "s1", Purpose: "p1", LegalBasis: "bogus", GrantedAt: granted},
			wantErr: privacy.ErrInvalidConsentRecord,
		},
		{
			name:    "zero granted at",
			record:  privacy.ConsentRecord{TenantID: tenantID, SubjectID: "s1", Purpose: "p1", LegalBasis: privacy.BasisConsent},
			wantErr: privacy.ErrInvalidConsentRecord,
		},
		{
			name:    "withdrawn before granted",
			record:  privacy.ConsentRecord{TenantID: tenantID, SubjectID: "s1", Purpose: "p1", LegalBasis: privacy.BasisConsent, GrantedAt: granted, WithdrawnAt: &withdrawnBeforeGrant},
			wantErr: privacy.ErrInvalidConsentRecord,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := tt.record
			err := r.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestEngine_RecordConsent_WithdrawConsent_CheckConsent(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	c := privacy.ConsentRecord{
		SubjectID:  "subject-1",
		Purpose:    "case_analytics",
		LegalBasis: privacy.BasisConsent,
	}
	created, err := engine.RecordConsent(ctxWithUser(admin), tenantID, c)
	if err != nil {
		t.Fatalf("RecordConsent: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("RecordConsent did not assign an ID")
	}

	valid, err := engine.CheckConsent(ctxWithUser(admin), tenantID, "subject-1", "case_analytics")
	if err != nil {
		t.Fatalf("CheckConsent: %v", err)
	}
	if !valid {
		t.Fatal("CheckConsent() = false, want true immediately after RecordConsent")
	}

	if _, err := engine.WithdrawConsent(ctxWithUser(admin), tenantID, created.ID); err != nil {
		t.Fatalf("WithdrawConsent: %v", err)
	}

	valid, err = engine.CheckConsent(ctxWithUser(admin), tenantID, "subject-1", "case_analytics")
	if err != nil {
		t.Fatalf("CheckConsent after withdraw: %v", err)
	}
	if valid {
		t.Fatal("CheckConsent() = true, want false after WithdrawConsent")
	}
}

func TestEngine_WithdrawConsent_AlreadyWithdrawn(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	created, err := engine.RecordConsent(ctxWithUser(admin), tenantID, privacy.ConsentRecord{
		SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent,
	})
	if err != nil {
		t.Fatalf("RecordConsent: %v", err)
	}
	if _, err := engine.WithdrawConsent(ctxWithUser(admin), tenantID, created.ID); err != nil {
		t.Fatalf("first WithdrawConsent: %v", err)
	}

	_, err = engine.WithdrawConsent(ctxWithUser(admin), tenantID, created.ID)
	if !errors.Is(err, privacy.ErrConsentAlreadyWithdrawn) {
		t.Fatalf("second WithdrawConsent() error = %v, want ErrConsentAlreadyWithdrawn", err)
	}
}

func TestEngine_RecordConsent_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	auditor := auditorUser(tenantID)

	_, err := engine.RecordConsent(ctxWithUser(auditor), tenantID, privacy.ConsentRecord{
		SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent,
	})
	if !errors.Is(err, privacy.ErrForbidden) {
		t.Fatalf("RecordConsent() error = %v, want ErrForbidden", err)
	}
}
