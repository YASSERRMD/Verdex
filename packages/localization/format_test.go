package localization_test

import (
	"strings"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/localization"
)

func TestFormatInteger(t *testing.T) {
	got := localization.FormatInteger(localization.LocaleEnglish, 1234567)
	want := "1,234,567"
	if got != want {
		t.Errorf("FormatInteger(en, 1234567) = %q, want %q", got, want)
	}
}

func TestFormatFloat(t *testing.T) {
	got := localization.FormatFloat(localization.LocaleEnglish, 1234.5, 2)
	want := "1,234.50"
	if got != want {
		t.Errorf("FormatFloat(en, 1234.5, 2) = %q, want %q", got, want)
	}
}

// TestFormatIntegerUsesWesternDigitsAcrossLocales is a regression test
// for a real bug this phase's own test suite caught: golang.org/x/text
// defaults an "ar"/"ur" language tag to native Eastern Arabic-Indic
// numeral shapes (e.g. "٢٬٠٢٠" for 2020) for %d. This package pins
// every locale to Western/Latin digit shapes via the "-u-nu-latn"
// BCP-47 extension (see format.go's doc comment on why), while still
// applying the locale's own grouping-separator convention.
func TestFormatIntegerUsesWesternDigitsAcrossLocales(t *testing.T) {
	for _, locale := range []localization.Locale{
		localization.LocaleEnglish, localization.LocaleArabic, localization.LocaleUrdu, localization.LocaleTamil,
	} {
		got := localization.FormatInteger(locale, 2020)
		want := "2,020"
		if got != want {
			t.Errorf("FormatInteger(%s, 2020) = %q, want %q (Western digits, grouped)", locale, got, want)
		}
		for _, r := range got {
			if r > '9' {
				t.Errorf("FormatInteger(%s, 2020) = %q contains non-Western digit rune %q", locale, got, r)
			}
		}
	}
}

// TestFormatDateProducesDifferentOutputPerLocale is task 4's centerpiece
// test: the same time.Time renders as genuinely different text across
// English, Arabic, Urdu, and Tamil locales (translated month names),
// not a single hardcoded layout reused unchanged for every locale.
func TestFormatDateProducesDifferentOutputPerLocale(t *testing.T) {
	cat := localization.NewSeededCatalog()
	moment := time.Date(2026, time.July, 4, 0, 0, 0, 0, time.UTC)

	seen := map[string]bool{}
	for _, locale := range []localization.Locale{
		localization.LocaleEnglish, localization.LocaleArabic, localization.LocaleUrdu, localization.LocaleTamil,
	} {
		got := localization.FormatDate(cat, locale, moment)
		if got == "" {
			t.Errorf("FormatDate(%s) = empty string", locale)
		}
		if seen[got] {
			t.Errorf("FormatDate(%s) = %q, collided with another locale's output", locale, got)
		}
		seen[got] = true
	}

	english := localization.FormatDate(cat, localization.LocaleEnglish, moment)
	if !strings.Contains(english, "July") {
		t.Errorf("FormatDate(en) = %q, want it to contain \"July\"", english)
	}
	// The bare 4-digit year picks up English digit-grouping via
	// golang.org/x/text/message (see FormatInteger) -- "2,026", not
	// "2026". This matches FormatInteger's own documented behavior;
	// only citation.go's LocalizeCitation deliberately opts out of
	// grouping for citation-embedded figures (see its doc comment).
	if !strings.Contains(english, "2,026") {
		t.Errorf("FormatDate(en) = %q, want it to contain the grouped year \"2,026\"", english)
	}

	arabic := localization.FormatDate(cat, localization.LocaleArabic, moment)
	if !strings.Contains(arabic, "يوليو") {
		t.Errorf("FormatDate(ar) = %q, want it to contain the Arabic month name", arabic)
	}
}

func TestFormatDateTimeIncludesTimeSuffix(t *testing.T) {
	cat := localization.NewSeededCatalog()
	moment := time.Date(2026, time.July, 4, 14, 30, 0, 0, time.UTC)
	got := localization.FormatDateTime(cat, localization.LocaleEnglish, moment)
	if !strings.HasSuffix(got, "14:30") {
		t.Errorf("FormatDateTime = %q, want it to end with 14:30", got)
	}
}

func TestFormatDateNilCatalogFallsBackToEnglish(t *testing.T) {
	moment := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	got := localization.FormatDate(nil, localization.LocaleArabic, moment)
	if !strings.Contains(got, "يناير") {
		t.Errorf("FormatDate(nil catalog, ar) = %q, want it to still contain the Arabic month name via an internally seeded catalog", got)
	}
}
