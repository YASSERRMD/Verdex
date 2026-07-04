package localization_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/localization"
)

func TestDirectionFor(t *testing.T) {
	cases := []struct {
		locale localization.Locale
		want   localization.Direction
	}{
		{localization.LocaleArabic, localization.DirectionRTL},
		{localization.LocaleUrdu, localization.DirectionRTL},
		{localization.LocaleTamil, localization.DirectionLTR},
		{localization.LocaleEnglish, localization.DirectionLTR},
		{localization.Locale("xx-unrecognized"), localization.DirectionLTR},
	}
	for _, tc := range cases {
		if got := localization.DirectionFor(tc.locale); got != tc.want {
			t.Errorf("DirectionFor(%q) = %q, want %q", tc.locale, got, tc.want)
		}
	}
}

func TestIsRTL(t *testing.T) {
	if !localization.IsRTL(localization.LocaleArabic) {
		t.Errorf("IsRTL(ar) = false, want true")
	}
	if !localization.IsRTL(localization.LocaleUrdu) {
		t.Errorf("IsRTL(ur) = false, want true")
	}
	if localization.IsRTL(localization.LocaleEnglish) {
		t.Errorf("IsRTL(en) = true, want false")
	}
	if localization.IsRTL(localization.LocaleTamil) {
		t.Errorf("IsRTL(ta) = true, want false")
	}
}

func TestLookupLocaleInfo(t *testing.T) {
	li, ok := localization.LookupLocaleInfo(localization.LocaleArabic)
	if !ok {
		t.Fatalf("LookupLocaleInfo(ar) not found")
	}
	if li.DisplayName != "Arabic" {
		t.Errorf("DisplayName = %q, want Arabic", li.DisplayName)
	}
	if li.Direction != localization.DirectionRTL {
		t.Errorf("Direction = %q, want rtl", li.Direction)
	}
	if li.NativeName != "العربية" {
		t.Errorf("NativeName = %q, want العربية", li.NativeName)
	}

	if _, ok := localization.LookupLocaleInfo(localization.Locale("zz")); ok {
		t.Errorf("LookupLocaleInfo(zz) found, want not found")
	}
}

func TestSupportedLocalesMatchesFrontendOrder(t *testing.T) {
	// Mirrors apps/web's LanguageStep.tsx LANGUAGE_OPTIONS order
	// exactly (Arabic, Urdu, Tamil, English) so the two never drift.
	want := []localization.Locale{
		localization.LocaleArabic,
		localization.LocaleUrdu,
		localization.LocaleTamil,
		localization.LocaleEnglish,
	}
	if len(localization.SupportedLocales) != len(want) {
		t.Fatalf("len(SupportedLocales) = %d, want %d", len(localization.SupportedLocales), len(want))
	}
	for i, li := range localization.SupportedLocales {
		if li.Locale != want[i] {
			t.Errorf("SupportedLocales[%d].Locale = %q, want %q", i, li.Locale, want[i])
		}
	}
}

func TestDirectionIsValid(t *testing.T) {
	if !localization.DirectionLTR.IsValid() {
		t.Errorf("DirectionLTR.IsValid() = false")
	}
	if !localization.DirectionRTL.IsValid() {
		t.Errorf("DirectionRTL.IsValid() = false")
	}
	if localization.Direction("diagonal").IsValid() {
		t.Errorf("Direction(diagonal).IsValid() = true, want false")
	}
}

func TestLocaleIsValid(t *testing.T) {
	if !localization.LocaleEnglish.IsValid() {
		t.Errorf("LocaleEnglish.IsValid() = false")
	}
	if localization.Locale("").IsValid() {
		t.Errorf(`Locale("").IsValid() = true, want false`)
	}
	if localization.Locale("   ").IsValid() {
		t.Errorf(`Locale("   ").IsValid() = true, want false`)
	}
}
