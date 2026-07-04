package compliance_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/compliance"
)

func TestProfile_Validate(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		p := compliance.Profile{TenantID: tenantID, Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection}}
		if err := p.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		var p *compliance.Profile
		if err := p.Validate(); !errors.Is(err, compliance.ErrInvalidProfile) {
			t.Errorf("Validate() = %v, want ErrInvalidProfile", err)
		}
	})

	t.Run("empty tenant", func(t *testing.T) {
		t.Parallel()
		p := compliance.Profile{}
		if err := p.Validate(); !errors.Is(err, compliance.ErrEmptyTenantID) {
			t.Errorf("Validate() = %v, want ErrEmptyTenantID", err)
		}
	})

	t.Run("invalid framework", func(t *testing.T) {
		t.Parallel()
		p := compliance.Profile{TenantID: tenantID, Frameworks: []compliance.Framework{""}}
		if err := p.Validate(); !errors.Is(err, compliance.ErrInvalidFramework) {
			t.Errorf("Validate() = %v, want ErrInvalidFramework", err)
		}
	})
}

func uaeControl() compliance.Control {
	return compliance.Control{ID: uuid.New(), Code: "UAE-X", Title: "UAE fixture control", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
}

func jrhControl() compliance.Control {
	return compliance.Control{ID: uuid.New(), Code: "JRH-X", Title: "JRH fixture control", Framework: compliance.FrameworkJudicialRecordsHandling, Category: compliance.CategoryRecordRetention}
}

// TestApplicableControls_NilProfile_EveryControlApplies proves the
// permissive default: a nil Profile (no profile set yet) means every
// catalogued control applies.
func TestApplicableControls_NilProfile_EveryControlApplies(t *testing.T) {
	t.Parallel()
	catalogue := []compliance.Control{uaeControl(), jrhControl()}

	applicable := compliance.ApplicableControls(catalogue, nil)
	if len(applicable) != 2 {
		t.Fatalf("ApplicableControls(nil) = %d controls, want 2", len(applicable))
	}
}

// TestApplicableControls_EmptyFrameworks_EveryControlApplies mirrors
// the nil case for a non-nil Profile with an empty Frameworks list.
func TestApplicableControls_EmptyFrameworks_EveryControlApplies(t *testing.T) {
	t.Parallel()
	catalogue := []compliance.Control{uaeControl(), jrhControl()}
	profile := &compliance.Profile{TenantID: uuid.New()}

	applicable := compliance.ApplicableControls(catalogue, profile)
	if len(applicable) != 2 {
		t.Fatalf("ApplicableControls(empty Frameworks) = %d controls, want 2", len(applicable))
	}
}

// TestApplicableControls_SelectedFrameworkOnly proves a Profile
// naming one Framework filters the catalogue down to only that
// framework's controls.
func TestApplicableControls_SelectedFrameworkOnly(t *testing.T) {
	t.Parallel()
	uae := uaeControl()
	jrh := jrhControl()
	catalogue := []compliance.Control{uae, jrh}
	profile := &compliance.Profile{
		TenantID:   uuid.New(),
		Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection},
	}

	applicable := compliance.ApplicableControls(catalogue, profile)
	if len(applicable) != 1 || applicable[0].ID != uae.ID {
		t.Fatalf("ApplicableControls() = %v, want exactly the UAE control", applicable)
	}
}

// TestApplicableControls_ExcludedControlIDRemoved proves
// ExcludedControlIDs removes a specific control even though its
// framework is selected.
func TestApplicableControls_ExcludedControlIDRemoved(t *testing.T) {
	t.Parallel()
	uae1 := uaeControl()
	uae2 := uaeControl()
	catalogue := []compliance.Control{uae1, uae2}
	profile := &compliance.Profile{
		TenantID:           uuid.New(),
		Frameworks:         []compliance.Framework{compliance.FrameworkUAEDataProtection},
		ExcludedControlIDs: []uuid.UUID{uae1.ID},
	}

	applicable := compliance.ApplicableControls(catalogue, profile)
	if len(applicable) != 1 || applicable[0].ID != uae2.ID {
		t.Fatalf("ApplicableControls() = %v, want exactly uae2", applicable)
	}
}

// TestEngine_SetProfile_RequiresManagePermission proves an auditor
// cannot set a tenant's compliance Profile.
func TestEngine_SetProfile_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	auditor := auditorUser(tenantID)

	_, err := engine.SetProfile(ctxWithUser(auditor), tenantID, compliance.Profile{Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection}})
	if !errors.Is(err, compliance.ErrForbidden) {
		t.Fatalf("SetProfile() error = %v, want ErrForbidden", err)
	}
}

// TestEngine_SetProfile_SucceedsAndIsGettable exercises task 7's full
// write+read round trip.
func TestEngine_SetProfile_SucceedsAndIsGettable(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	set, err := engine.SetProfile(ctxWithUser(admin), tenantID, compliance.Profile{
		Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection, compliance.FrameworkJudicialRecordsHandling},
	})
	if err != nil {
		t.Fatalf("SetProfile: %v", err)
	}
	if set.SetBy != admin.ID {
		t.Fatalf("set.SetBy = %v, want %v", set.SetBy, admin.ID)
	}

	got, err := engine.GetProfile(ctxWithUser(auditorUser(tenantID)), tenantID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(got.Frameworks) != 2 {
		t.Fatalf("got.Frameworks = %v, want 2 entries", got.Frameworks)
	}
}

// TestEngine_GetProfile_NotFoundBeforeSet proves GetProfile surfaces
// ErrProfileNotFound for a tenant that has never called SetProfile.
func TestEngine_GetProfile_NotFoundBeforeSet(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	_, err := engine.GetProfile(ctxWithUser(auditorUser(tenantID)), tenantID)
	if !errors.Is(err, compliance.ErrProfileNotFound) {
		t.Fatalf("GetProfile() error = %v, want ErrProfileNotFound", err)
	}
}

// TestEngine_RunGapAnalysis_RespectsProfileScoping proves
// RunGapAnalysis actually narrows to the tenant's selected framework
// once a Profile has been set, rather than always evaluating every
// catalogued control.
func TestEngine_RunGapAnalysis_RespectsProfileScoping(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	uae, err := engine.RegisterControl(ctxWithUser(admin), uaeControl())
	if err != nil {
		t.Fatalf("RegisterControl (uae): %v", err)
	}
	if _, err := engine.RegisterControl(ctxWithUser(admin), jrhControl()); err != nil {
		t.Fatalf("RegisterControl (jrh): %v", err)
	}

	if _, err := engine.SetProfile(ctxWithUser(admin), tenantID, compliance.Profile{
		Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection},
	}); err != nil {
		t.Fatalf("SetProfile: %v", err)
	}

	report, err := engine.RunGapAnalysis(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("RunGapAnalysis: %v", err)
	}
	if len(report.Results) != 1 || report.Results[0].Control.ID != uae.ID {
		t.Fatalf("RunGapAnalysis() = %v, want exactly the UAE control", report.Results)
	}
}
