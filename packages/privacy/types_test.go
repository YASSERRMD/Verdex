package privacy_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/privacy"
)

func validInventoryEntry(tenantID uuid.UUID) privacy.DataInventoryEntry {
	return privacy.DataInventoryEntry{
		ID:              uuid.New(),
		TenantID:        tenantID,
		Category:        privacy.CategoryCaseParty,
		SourceTag:       "case.parties",
		Sensitivity:     privacy.SensitivityHigh,
		LegalBasis:      privacy.BasisPublicTask,
		RetentionPeriod: 24 * time.Hour,
	}
}

func TestDataInventoryEntry_Validate(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()

	tests := []struct {
		name    string
		mutate  func(*privacy.DataInventoryEntry)
		wantErr error
	}{
		{name: "valid", mutate: func(*privacy.DataInventoryEntry) {}, wantErr: nil},
		{name: "empty tenant", mutate: func(e *privacy.DataInventoryEntry) { e.TenantID = uuid.Nil }, wantErr: privacy.ErrEmptyTenantID},
		{name: "invalid category", mutate: func(e *privacy.DataInventoryEntry) { e.Category = "bogus" }, wantErr: privacy.ErrInvalidDataCategory},
		{name: "blank source tag", mutate: func(e *privacy.DataInventoryEntry) { e.SourceTag = "  " }, wantErr: privacy.ErrInvalidInventoryEntry},
		{name: "invalid sensitivity", mutate: func(e *privacy.DataInventoryEntry) { e.Sensitivity = "bogus" }, wantErr: privacy.ErrInvalidSensitivity},
		{name: "invalid legal basis", mutate: func(e *privacy.DataInventoryEntry) { e.LegalBasis = "bogus" }, wantErr: privacy.ErrInvalidInventoryEntry},
		{name: "zero retention", mutate: func(e *privacy.DataInventoryEntry) { e.RetentionPeriod = 0 }, wantErr: privacy.ErrInvalidInventoryEntry},
		{name: "negative retention", mutate: func(e *privacy.DataInventoryEntry) { e.RetentionPeriod = -time.Hour }, wantErr: privacy.ErrInvalidInventoryEntry},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := validInventoryEntry(tenantID)
			tt.mutate(&entry)
			err := entry.Validate()
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

func TestDataCategory_IsValid(t *testing.T) {
	t.Parallel()

	valid := []privacy.DataCategory{
		privacy.CategoryIdentity, privacy.CategoryCaseParty, privacy.CategoryContact,
		privacy.CategoryIdentifier, privacy.CategoryFinancial, privacy.CategoryTranscript,
		privacy.CategoryBehavioral, privacy.CategoryOther,
	}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("DataCategory(%q).IsValid() = false, want true", c)
		}
	}
	if privacy.DataCategory("bogus").IsValid() {
		t.Error(`DataCategory("bogus").IsValid() = true, want false`)
	}
}

func TestSensitivity_IsValid(t *testing.T) {
	t.Parallel()

	valid := []privacy.Sensitivity{privacy.SensitivityLow, privacy.SensitivityMedium, privacy.SensitivityHigh, privacy.SensitivityCritical}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("Sensitivity(%q).IsValid() = false, want true", s)
		}
	}
	if privacy.Sensitivity("bogus").IsValid() {
		t.Error(`Sensitivity("bogus").IsValid() = true, want false`)
	}
}

func TestLegalBasis_IsValid(t *testing.T) {
	t.Parallel()

	valid := []privacy.LegalBasis{
		privacy.BasisConsent, privacy.BasisContract, privacy.BasisLegalObligation,
		privacy.BasisLegitimateInterest, privacy.BasisPublicTask, privacy.BasisVitalInterest,
	}
	for _, b := range valid {
		if !b.IsValid() {
			t.Errorf("LegalBasis(%q).IsValid() = false, want true", b)
		}
	}
	if privacy.LegalBasis("bogus").IsValid() {
		t.Error(`LegalBasis("bogus").IsValid() = true, want false`)
	}
}

func TestEngine_RegisterInventoryEntry_And_List(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	entry := validInventoryEntry(uuid.Nil) // TenantID set by RegisterInventoryEntry
	created, err := engine.RegisterInventoryEntry(ctxWithUser(admin), tenantID, entry)
	if err != nil {
		t.Fatalf("RegisterInventoryEntry: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("RegisterInventoryEntry did not assign an ID")
	}
	if created.TenantID != tenantID {
		t.Fatalf("created.TenantID = %v, want %v", created.TenantID, tenantID)
	}
	if created.CreatedBy != admin.ID {
		t.Fatalf("created.CreatedBy = %v, want %v", created.CreatedBy, admin.ID)
	}

	list, err := engine.ListInventory(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("ListInventory: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("ListInventory = %v, want exactly the created entry", list)
	}
}

func TestEngine_RegisterInventoryEntry_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	// RoleAuditor holds PermViewPrivacy but not PermManagePrivacy.
	auditor := auditorUser(tenantID)

	_, err := engine.RegisterInventoryEntry(ctxWithUser(auditor), tenantID, validInventoryEntry(tenantID))
	if !errors.Is(err, privacy.ErrForbidden) {
		t.Fatalf("RegisterInventoryEntry() error = %v, want ErrForbidden", err)
	}
}

func TestEngine_ListInventory_AuditorCanView(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)
	auditor := auditorUser(tenantID)

	if _, err := engine.RegisterInventoryEntry(ctxWithUser(admin), tenantID, validInventoryEntry(tenantID)); err != nil {
		t.Fatalf("RegisterInventoryEntry: %v", err)
	}

	list, err := engine.ListInventory(ctxWithUser(auditor), tenantID)
	if err != nil {
		t.Fatalf("ListInventory (auditor) = %v, want nil error", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListInventory (auditor) = %d entries, want 1", len(list))
	}
}
