package localization_test

import (
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/localization"
)

// TestLocalizeCitationUsesCommonLawFormatter asserts LocalizeCitation
// calls through to the supplied citation.Formatter (here,
// citation.CommonLawFormatter) rather than reimplementing any
// citation-style logic itself -- the precedent citation renders in
// exactly the style packages/citation/formatter.go's own
// formatCommonLawCase produces, for a left-to-right target locale
// (no bidi wrapping applied).
func TestLocalizeCitationUsesCommonLawFormatter(t *testing.T) {
	in := citation.FormatInput{
		Origin:      citation.OriginPrecedent,
		CaseName:    "Smith v Jones",
		RawCitation: "[2020] UKSC 1",
	}
	got, err := localization.LocalizeCitation(citation.CommonLawFormatter, localization.LocaleEnglish, in)
	if err != nil {
		t.Fatalf("LocalizeCitation error: %v", err)
	}
	want := "Smith v Jones [2020] UKSC 1"
	if got != want {
		t.Errorf("LocalizeCitation(en) = %q, want %q (unchanged, no bidi wrapping for an LTR locale)", got, want)
	}
}

// TestLocalizeCitationCivilLawStatute covers the statute-citation path
// through CivilLawFormatter, confirming LocalizeCitation is
// formatter-agnostic (task 4's "reference packages/citation's
// formatting-per-jurisdiction concept rather than duplicating it").
func TestLocalizeCitationCivilLawStatute(t *testing.T) {
	in := citation.FormatInput{
		Origin:  citation.OriginStatute,
		Act:     "Code Civil",
		Section: "5",
	}
	got, err := localization.LocalizeCitation(citation.CivilLawFormatter, localization.LocaleEnglish, in)
	if err != nil {
		t.Fatalf("LocalizeCitation error: %v", err)
	}
	want := "Art. 5 Code Civil"
	if got != want {
		t.Errorf("LocalizeCitation(civil law statute) = %q, want %q", got, want)
	}
}

// TestLocalizeCitationDoesNotGroupEmbeddedNumerals is a regression
// test for a real design flaw an earlier version of this function had
// (re-rendering citation-embedded digits through locale-aware
// thousands-grouping): a citation year or docket number must render
// exactly as the underlying citation.Formatter produced it --
// "[2020] UKSC 12345", never "[2,020] UKSC 12,345" -- since
// comma-grouping is not a real legal-citation convention in any
// locale this package supports. See citation.go's doc comment.
func TestLocalizeCitationDoesNotGroupEmbeddedNumerals(t *testing.T) {
	in := citation.FormatInput{
		Origin:      citation.OriginPrecedent,
		CaseName:    "Doe v Roe",
		RawCitation: "[2020] UKSC 12345",
	}
	for _, locale := range []localization.Locale{
		localization.LocaleEnglish, localization.LocaleArabic, localization.LocaleUrdu, localization.LocaleTamil,
	} {
		got, err := localization.LocalizeCitation(citation.CommonLawFormatter, locale, in)
		if err != nil {
			t.Fatalf("LocalizeCitation(%s) error: %v", locale, err)
		}
		if !strings.Contains(got, "[2020] UKSC 12345") {
			t.Errorf("LocalizeCitation(%s) = %q, want it to contain the ungrouped citation text unchanged", locale, got)
		}
	}
}

// TestLocalizeCitationWrapsInBidiControlsForRTLLocale asserts a
// citation embedded in a right-to-left target locale (Arabic, Urdu)
// is wrapped with explicit LRE...PDF bidi embedding controls so it
// renders left-to-right within RTL surrounding prose, while an
// LTR-locale citation (English, Tamil) is left completely unwrapped.
func TestLocalizeCitationWrapsInBidiControlsForRTLLocale(t *testing.T) {
	in := citation.FormatInput{
		Origin:      citation.OriginPrecedent,
		CaseName:    "Smith v Jones",
		RawCitation: "[2020] UKSC 1",
	}
	const (
		lre = "\u202a" // LEFT-TO-RIGHT EMBEDDING
		pdf = "\u202c" // POP DIRECTIONAL FORMATTING
	)

	for _, locale := range []localization.Locale{localization.LocaleArabic, localization.LocaleUrdu} {
		got, err := localization.LocalizeCitation(citation.CommonLawFormatter, locale, in)
		if err != nil {
			t.Fatalf("LocalizeCitation(%s) error: %v", locale, err)
		}
		want := lre + "Smith v Jones [2020] UKSC 1" + pdf
		if got != want {
			t.Errorf("LocalizeCitation(%s) = %q, want bidi-wrapped %q", locale, got, want)
		}
	}

	for _, locale := range []localization.Locale{localization.LocaleEnglish, localization.LocaleTamil} {
		got, err := localization.LocalizeCitation(citation.CommonLawFormatter, locale, in)
		if err != nil {
			t.Fatalf("LocalizeCitation(%s) error: %v", locale, err)
		}
		want := "Smith v Jones [2020] UKSC 1"
		if got != want {
			t.Errorf("LocalizeCitation(%s) = %q, want unwrapped %q", locale, got, want)
		}
	}
}

// TestLocalizeCitationNilFormatter covers the defensive nil-formatter
// error path.
func TestLocalizeCitationNilFormatter(t *testing.T) {
	_, err := localization.LocalizeCitation(nil, localization.LocaleEnglish, citation.FormatInput{})
	if err != localization.ErrNilFormatter {
		t.Errorf("LocalizeCitation(nil formatter) error = %v, want ErrNilFormatter", err)
	}
}

// TestLocalizeCitationEmptyResultNotWrapped covers the edge case where
// the underlying Formatter produces an empty string (e.g. a fully
// blank FormatInput) -- LocalizeCitation must not wrap an empty
// citation in bidi controls, which would render a visible-but-blank
// pair of control characters.
func TestLocalizeCitationEmptyResultNotWrapped(t *testing.T) {
	got, err := localization.LocalizeCitation(citation.CommonLawFormatter, localization.LocaleArabic, citation.FormatInput{})
	if err != nil {
		t.Fatalf("LocalizeCitation error: %v", err)
	}
	if got != "" {
		t.Errorf("LocalizeCitation(empty input) = %q, want empty string", got)
	}
}
