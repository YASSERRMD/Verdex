package localization_test

import (
	"reflect"
	"testing"

	"github.com/YASSERRMD/verdex/packages/localization"
)

// TestTranslateFallback is task 8's centerpiece test: an intentionally
// incomplete locale (a French-ish "fr" Catalog with only a partial key
// set, standing in for "a real locale added later that starts
// incomplete") must fall back to English for a missing key, and that
// gap must be durably recorded for MissingKeys (task 7) to report --
// not merely silently swallowed.
func TestTranslateFallback(t *testing.T) {
	cat := localization.NewCatalog()
	cat.Set(localization.LocaleEnglish, "greeting.hello", "Hello")
	cat.Set(localization.LocaleEnglish, "greeting.goodbye", "Goodbye")
	incomplete := localization.Locale("fr")
	cat.Set(incomplete, "greeting.hello", "Bonjour")
	// Deliberately no "greeting.goodbye" for "fr".

	// A key present in the target locale is used as-is.
	if got := localization.Translate(cat, incomplete, "greeting.hello"); got != "Bonjour" {
		t.Errorf("Translate(fr, greeting.hello) = %q, want Bonjour", got)
	}

	// A key missing from the target locale falls back to English.
	if got := localization.Translate(cat, incomplete, "greeting.goodbye"); got != "Goodbye" {
		t.Errorf("Translate(fr, greeting.goodbye) = %q, want Goodbye (fallback)", got)
	}

	// The gap must be observable afterward via MissingKeys -- this is
	// what makes fallback "real logic" per the phase brief, not a
	// silent no-op.
	missing := cat.MissingKeys(incomplete)
	want := []string{"greeting.goodbye"}
	if !reflect.DeepEqual(missing, want) {
		t.Errorf("MissingKeys(fr) = %v, want %v", missing, want)
	}

	// A locale that was never queried has no reported gaps yet (gaps
	// are observed at Translate-time, not proactively computed) --
	// UntranslatedKeys is the proactive counterpart, tested
	// separately.
	if got := cat.MissingKeys(localization.LocaleEnglish); got != nil {
		t.Errorf("MissingKeys(en) = %v, want nil", got)
	}
}

// TestTranslateUnknownKeyEvenInFallback covers Translate's third
// resolution branch: a key missing even from FallbackLocale (an
// authoring bug) renders as a visually-obvious placeholder rather than
// panicking or silently returning an empty string.
func TestTranslateUnknownKeyEvenInFallback(t *testing.T) {
	cat := localization.NewCatalog()
	got := localization.Translate(cat, localization.LocaleArabic, "totally.unknown.key")
	want := "!(totally.unknown.key)!"
	if got != want {
		t.Errorf("Translate(unknown key) = %q, want %q", got, want)
	}
}

// TestTranslateNilCatalog covers the defensive nil-catalog path.
func TestTranslateNilCatalog(t *testing.T) {
	got := localization.Translate(nil, localization.LocaleEnglish, "any.key")
	want := "!(any.key)!"
	if got != want {
		t.Errorf("Translate(nil catalog) = %q, want %q", got, want)
	}
}

// TestMustTranslate covers the stricter variant: MustTranslate returns
// ErrUnknownKey for a key missing from fallback, but succeeds (using
// fallback) for a key merely missing from the target locale.
func TestMustTranslate(t *testing.T) {
	cat := localization.NewCatalog()
	cat.Set(localization.LocaleEnglish, "known.key", "Known")

	got, err := localization.MustTranslate(cat, localization.LocaleArabic, "known.key")
	if err != nil {
		t.Fatalf("MustTranslate(known key) unexpected error: %v", err)
	}
	if got != "Known" {
		t.Errorf("MustTranslate(known key) = %q, want Known (fallback)", got)
	}

	if _, err := localization.MustTranslate(cat, localization.LocaleArabic, "unknown.key"); err == nil {
		t.Errorf("MustTranslate(unknown key) = nil error, want ErrUnknownKey")
	}
}

// TestTranslateWithArgs covers fmt.Sprintf-style argument
// interpolation, and that a template with no args and a literal "%"
// sign is left untouched.
func TestTranslateWithArgs(t *testing.T) {
	cat := localization.NewCatalog()
	cat.Set(localization.LocaleEnglish, "greeting.named", "Hello, %s!")
	cat.Set(localization.LocaleEnglish, "literal.percent", "100% complete")

	if got := localization.Translate(cat, localization.LocaleEnglish, "greeting.named", "Judge Smith"); got != "Hello, Judge Smith!" {
		t.Errorf("Translate with args = %q, want %q", got, "Hello, Judge Smith!")
	}
	if got := localization.Translate(cat, localization.LocaleEnglish, "literal.percent"); got != "100% complete" {
		t.Errorf("Translate no-args literal percent = %q, want unchanged", got)
	}
}

// TestUntranslatedKeysAndCoverage covers the proactive
// translation-management report (task 7), independent of whether
// Translate has ever been called.
func TestUntranslatedKeysAndCoverage(t *testing.T) {
	cat := localization.NewCatalog()
	cat.Set(localization.LocaleEnglish, "a", "A")
	cat.Set(localization.LocaleEnglish, "b", "B")
	cat.Set(localization.LocaleEnglish, "c", "C")
	cat.Set(localization.LocaleArabic, "a", "أ")
	// "b" and "c" deliberately untranslated for Arabic.

	untranslated := cat.UntranslatedKeys(localization.LocaleArabic)
	want := []string{"b", "c"}
	if !reflect.DeepEqual(untranslated, want) {
		t.Errorf("UntranslatedKeys(ar) = %v, want %v", untranslated, want)
	}

	if got := cat.CoveragePercent(localization.LocaleArabic); got != 33 {
		t.Errorf("CoveragePercent(ar) = %d, want 33", got)
	}
	if got := cat.CoveragePercent(localization.LocaleEnglish); got != 100 {
		t.Errorf("CoveragePercent(en) = %d, want 100", got)
	}
}

// TestSeededCatalogHasNoGapsAcrossFourLocales asserts SeedCatalog's own
// real translation data has complete en/ar/ur/ta coverage for every
// seeded key -- the seed data itself must not have the very gaps this
// package's fallback logic is designed to paper over.
func TestSeededCatalogHasNoGapsAcrossFourLocales(t *testing.T) {
	cat := localization.NewSeededCatalog()
	keys := cat.AllKeys()
	if len(keys) == 0 {
		t.Fatalf("SeedCatalog produced no keys")
	}
	for _, locale := range []localization.Locale{
		localization.LocaleArabic, localization.LocaleUrdu, localization.LocaleTamil,
	} {
		if got := cat.CoveragePercent(locale); got != 100 {
			t.Errorf("CoveragePercent(%s) = %d, want 100 (seed data must be complete)", locale, got)
		}
		if untranslated := cat.UntranslatedKeys(locale); len(untranslated) != 0 {
			t.Errorf("UntranslatedKeys(%s) = %v, want none", locale, untranslated)
		}
	}
}

// TestCaseStatusKeysMirrorFrontend asserts the case_status.* keys this
// package seeds match apps/web's CaseState values exactly, so a caller
// can look up "case_status."+state directly for every real CaseState.
func TestCaseStatusKeysMirrorFrontend(t *testing.T) {
	cat := localization.NewSeededCatalog()
	states := []string{"draft", "active", "under_review", "closed", "archived"}
	for _, s := range states {
		key := "case_status." + s
		got, err := localization.MustTranslate(cat, localization.LocaleEnglish, key)
		if err != nil {
			t.Errorf("MustTranslate(%s) error: %v", key, err)
		}
		if got == "" {
			t.Errorf("MustTranslate(%s) = empty string", key)
		}
	}
}

// TestMergeAddsAdditionalTranslations covers Merge's role as the
// deployment-specific "layer on top of the seeded catalogue" extension
// point.
func TestMergeAddsAdditionalTranslations(t *testing.T) {
	cat := localization.NewSeededCatalog()
	cat.Merge([]localization.CatalogEntry{
		{Locale: localization.LocaleEnglish, Key: "custom.greeting", Value: "Welcome"},
		{Locale: localization.LocaleArabic, Key: "custom.greeting", Value: "أهلاً"},
	})

	if got := localization.Translate(cat, localization.LocaleArabic, "custom.greeting"); got != "أهلاً" {
		t.Errorf("Translate(custom.greeting, ar) = %q, want أهلاً", got)
	}
}
