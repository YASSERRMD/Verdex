package localization_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/localization"
)

func TestPreferenceValidate(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	valid := &localization.Preference{TenantID: tenantID, UserID: userID, Locale: localization.LocaleArabic}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate(valid) unexpected error: %v", err)
	}

	if err := (&localization.Preference{UserID: userID, Locale: localization.LocaleArabic}).Validate(); err == nil {
		t.Errorf("Validate(no tenant) = nil, want error")
	}
	if err := (&localization.Preference{TenantID: tenantID, Locale: localization.LocaleArabic}).Validate(); err == nil {
		t.Errorf("Validate(no user) = nil, want error")
	}
	if err := (&localization.Preference{TenantID: tenantID, UserID: userID}).Validate(); err == nil {
		t.Errorf("Validate(no locale) = nil, want error")
	}
	var nilPref *localization.Preference
	if err := nilPref.Validate(); err == nil {
		t.Errorf("Validate(nil) = nil, want error")
	}
}
