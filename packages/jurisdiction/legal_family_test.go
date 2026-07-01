package jurisdiction_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/jurisdiction"
)

func TestLegalFamily_IsValid(t *testing.T) {
	t.Parallel()

	valid := []jurisdiction.LegalFamily{
		jurisdiction.LegalFamilyCommonLaw,
		jurisdiction.LegalFamilyCivilLaw,
		jurisdiction.LegalFamilyMixed,
		jurisdiction.LegalFamilyIslamicLaw,
	}
	for _, lf := range valid {
		lf := lf
		t.Run(string(lf)+"_valid", func(t *testing.T) {
			t.Parallel()
			if !lf.IsValid() {
				t.Errorf("expected %q to be valid", lf)
			}
		})
	}

	invalid := []jurisdiction.LegalFamily{
		"",
		"custom",
		"COMMON_LAW",
		"Civil Law",
	}
	for _, lf := range invalid {
		lf := lf
		t.Run(string(lf)+"_invalid", func(t *testing.T) {
			t.Parallel()
			if lf.IsValid() {
				t.Errorf("expected %q to be invalid", lf)
			}
		})
	}
}

func TestLegalFamily_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		lf   jurisdiction.LegalFamily
		want string
	}{
		{jurisdiction.LegalFamilyCommonLaw, "Common Law"},
		{jurisdiction.LegalFamilyCivilLaw, "Civil Law"},
		{jurisdiction.LegalFamilyMixed, "Mixed Legal System"},
		{jurisdiction.LegalFamilyIslamicLaw, "Islamic Law (Shari'a)"},
		{"unknown_family", "unknown(unknown_family)"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.lf), func(t *testing.T) {
			t.Parallel()
			got := tc.lf.String()
			if got != tc.want {
				t.Errorf("LegalFamily(%q).String() = %q; want %q", tc.lf, got, tc.want)
			}
		})
	}
}

func TestCourtLevel_IsValid(t *testing.T) {
	t.Parallel()

	valid := []jurisdiction.CourtLevel{
		jurisdiction.CourtLevelSupreme,
		jurisdiction.CourtLevelAppellate,
		jurisdiction.CourtLevelHigh,
		jurisdiction.CourtLevelDistrict,
		jurisdiction.CourtLevelMagistrate,
		jurisdiction.CourtLevelSpecial,
	}
	for _, cl := range valid {
		cl := cl
		t.Run(string(cl)+"_valid", func(t *testing.T) {
			t.Parallel()
			if !cl.IsValid() {
				t.Errorf("expected %q to be valid", cl)
			}
		})
	}

	invalid := []jurisdiction.CourtLevel{
		"",
		"federal",
		"SUPREME",
		"High Court",
	}
	for _, cl := range invalid {
		cl := cl
		t.Run(string(cl)+"_invalid", func(t *testing.T) {
			t.Parallel()
			if cl.IsValid() {
				t.Errorf("expected %q to be invalid", cl)
			}
		})
	}
}

func TestCourtLevel_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cl   jurisdiction.CourtLevel
		want string
	}{
		{jurisdiction.CourtLevelSupreme, "Supreme Court"},
		{jurisdiction.CourtLevelAppellate, "Appellate Court"},
		{jurisdiction.CourtLevelHigh, "High Court"},
		{jurisdiction.CourtLevelDistrict, "District Court"},
		{jurisdiction.CourtLevelMagistrate, "Magistrate Court"},
		{jurisdiction.CourtLevelSpecial, "Special / Tribunal"},
		{"mystery_level", "unknown(mystery_level)"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.cl), func(t *testing.T) {
			t.Parallel()
			got := tc.cl.String()
			if got != tc.want {
				t.Errorf("CourtLevel(%q).String() = %q; want %q", tc.cl, got, tc.want)
			}
		})
	}
}
