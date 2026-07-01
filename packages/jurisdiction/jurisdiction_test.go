package jurisdiction_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/jurisdiction"
)

// validJurisdiction returns a Jurisdiction that passes Validate() so
// individual test cases can mutate one field at a time.
func validJurisdiction() jurisdiction.Jurisdiction {
	return jurisdiction.Jurisdiction{
		ID:          uuid.New(),
		CountryCode: "AE",
		CountryName: "United Arab Emirates",
		CourtLevel:  jurisdiction.CourtLevelSupreme,
		CourtName:   "Federal Supreme Court of the UAE",
		LegalFamily: jurisdiction.LegalFamilyMixed,
		Languages:   []string{"ar"},
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

func TestValidate_ValidJurisdiction(t *testing.T) {
	t.Parallel()
	j := validJurisdiction()
	if err := jurisdiction.Validate(j); err != nil {
		t.Fatalf("Validate() returned unexpected error: %v", err)
	}
}

func TestValidate_CountryCodeErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code string
	}{
		{"empty", ""},
		{"too_short", "A"},
		{"too_long", "ARE"},
		{"lowercase", "ae"},
		{"mixed_case", "Ae"},
		{"digits", "12"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			j := validJurisdiction()
			j.CountryCode = tc.code
			err := jurisdiction.Validate(j)
			if err == nil {
				t.Fatalf("Validate() expected error for country code %q, got nil", tc.code)
			}
			if !errors.Is(err, jurisdiction.ErrCountryCodeInvalid) {
				t.Errorf("expected ErrCountryCodeInvalid; got %v", err)
			}
		})
	}
}

func TestValidate_EmptyCountryName(t *testing.T) {
	t.Parallel()
	j := validJurisdiction()
	j.CountryName = "   "
	err := jurisdiction.Validate(j)
	if err == nil {
		t.Fatal("expected error for blank country name, got nil")
	}
	if !errors.Is(err, jurisdiction.ErrInvalidJurisdiction) {
		t.Errorf("expected ErrInvalidJurisdiction; got %v", err)
	}
}

func TestValidate_EmptyCourtName(t *testing.T) {
	t.Parallel()
	j := validJurisdiction()
	j.CourtName = ""
	err := jurisdiction.Validate(j)
	if err == nil {
		t.Fatal("expected error for empty court name, got nil")
	}
	if !errors.Is(err, jurisdiction.ErrInvalidJurisdiction) {
		t.Errorf("expected ErrInvalidJurisdiction; got %v", err)
	}
}

func TestValidate_InvalidCourtLevel(t *testing.T) {
	t.Parallel()
	j := validJurisdiction()
	j.CourtLevel = "unknown_level"
	err := jurisdiction.Validate(j)
	if err == nil {
		t.Fatal("expected error for invalid court level, got nil")
	}
	if !errors.Is(err, jurisdiction.ErrInvalidJurisdiction) {
		t.Errorf("expected ErrInvalidJurisdiction; got %v", err)
	}
}

func TestValidate_InvalidLegalFamily(t *testing.T) {
	t.Parallel()
	j := validJurisdiction()
	j.LegalFamily = "roman_law"
	err := jurisdiction.Validate(j)
	if err == nil {
		t.Fatal("expected error for invalid legal family, got nil")
	}
	if !errors.Is(err, jurisdiction.ErrInvalidJurisdiction) {
		t.Errorf("expected ErrInvalidJurisdiction; got %v", err)
	}
}

func TestValidate_LanguageErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		langs []string
	}{
		{"nil", nil},
		{"empty_slice", []string{}},
		{"blank_entry", []string{""}},
		{"too_long", []string{"eng"}},
		{"too_short", []string{"e"}},
		{"uppercase", []string{"AR"}},
		{"digit_code", []string{"a1"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			j := validJurisdiction()
			j.Languages = tc.langs
			err := jurisdiction.Validate(j)
			if err == nil {
				t.Fatalf("Validate() expected error for languages %v, got nil", tc.langs)
			}
			if !errors.Is(err, jurisdiction.ErrInvalidJurisdiction) {
				t.Errorf("expected ErrInvalidJurisdiction; got %v", err)
			}
		})
	}
}

func TestValidate_MultipleLanguages(t *testing.T) {
	t.Parallel()
	j := validJurisdiction()
	j.Languages = []string{"ar", "en", "ur"}
	if err := jurisdiction.Validate(j); err != nil {
		t.Fatalf("Validate() unexpected error for multiple languages: %v", err)
	}
}
