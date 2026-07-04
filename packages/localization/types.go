package localization

import "strings"

// Direction is a locale's writing direction, the Go-side half of task
// 3's RTL support. This mirrors packages/multilingual's RTL posture
// (Arabic-script text, covering both Arabic and Urdu, is
// right-to-left) but is a static property of a known Locale here, not a
// per-text-span runtime classification -- see doc.go for why this
// package does not import packages/multilingual to compute it.
type Direction string

const (
	// DirectionLTR is left-to-right (English, Tamil).
	DirectionLTR Direction = "ltr"

	// DirectionRTL is right-to-left (Arabic, Urdu).
	DirectionRTL Direction = "rtl"
)

// IsValid reports whether d is one of the two recognized directions.
func (d Direction) IsValid() bool {
	switch d {
	case DirectionLTR, DirectionRTL:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (d Direction) String() string { return string(d) }

// Locale identifies one of this platform's supported UI/output
// languages by its ISO 639-1 code. Locale is deliberately keyed the
// same way packages/jurisdiction.Jurisdiction.Languages already is
// ("ar", "ur", "ta", "en", ...) so a caller can derive a deployment's
// required locale set from a jurisdiction.Jurisdiction lookup without
// this package importing packages/jurisdiction or inventing a second
// mapping (see doc.go).
//
// Locale is an open string type rather than a closed enum for the same
// reason packages/compliance.Framework is open: a Catalog can carry
// entries for a locale beyond the four seeded here (SeedCatalog) without
// a code change, even though LocaleInfo (below) only describes the
// four this phase ships real translations for.
type Locale string

const (
	// LocaleEnglish is English -- also this package's fallback locale
	// (task 8): every seeded key has a complete English translation,
	// and Translate falls back to it when the requested locale is
	// missing a key.
	LocaleEnglish Locale = "en"

	// LocaleArabic is Arabic.
	LocaleArabic Locale = "ar"

	// LocaleUrdu is Urdu.
	LocaleUrdu Locale = "ur"

	// LocaleTamil is Tamil.
	LocaleTamil Locale = "ta"
)

// IsValid reports whether l is non-blank. Like Framework.IsValid in
// packages/compliance, "valid" here means structurally well-formed, not
// "one of the four seeded constants" -- a deployment-specific Catalog
// may register additional locales.
func (l Locale) IsValid() bool {
	return strings.TrimSpace(string(l)) != ""
}

// String satisfies fmt.Stringer.
func (l Locale) String() string { return string(l) }

// LocaleInfo describes one of the four locales this phase ships
// translations, direction metadata, and BCP-47 tags for.
type LocaleInfo struct {
	// Locale is the ISO 639-1 code.
	Locale Locale

	// DisplayName is the locale's English name (e.g. "Arabic").
	DisplayName string

	// NativeName is the locale's self-endonym (e.g. "العربية"),
	// mirroring apps/web's LanguageStep.tsx nativeLabel values exactly
	// so the Go seed data and the existing frontend copy never drift
	// apart.
	NativeName string

	// Direction is this locale's writing direction.
	Direction Direction

	// BCP47Tag is the locale's BCP 47 language tag, used to build a
	// golang.org/x/text/language.Tag for date/number formatting (see
	// format.go).
	BCP47Tag string
}

// SupportedLocales is the ordered, complete set of LocaleInfo this
// phase ships, in the same order apps/web's LanguageStep.tsx
// LANGUAGE_OPTIONS lists them (Arabic, Urdu, Tamil, English).
var SupportedLocales = []LocaleInfo{
	{
		Locale:      LocaleArabic,
		DisplayName: "Arabic",
		NativeName:  "العربية",
		Direction:   DirectionRTL,
		BCP47Tag:    "ar",
	},
	{
		Locale:      LocaleUrdu,
		DisplayName: "Urdu",
		NativeName:  "اردو",
		Direction:   DirectionRTL,
		BCP47Tag:    "ur",
	},
	{
		Locale:      LocaleTamil,
		DisplayName: "Tamil",
		NativeName:  "தமிழ்",
		Direction:   DirectionLTR,
		BCP47Tag:    "ta",
	},
	{
		Locale:      LocaleEnglish,
		DisplayName: "English",
		NativeName:  "English",
		Direction:   DirectionLTR,
		BCP47Tag:    "en",
	},
}

// localeInfoByCode indexes SupportedLocales by Locale for O(1) lookup,
// built once at init.
var localeInfoByCode = func() map[Locale]LocaleInfo {
	m := make(map[Locale]LocaleInfo, len(SupportedLocales))
	for _, li := range SupportedLocales {
		m[li.Locale] = li
	}
	return m
}()

// LookupLocaleInfo returns the LocaleInfo for l and true if l is one of
// the four seeded SupportedLocales, or a zero LocaleInfo and false
// otherwise.
func LookupLocaleInfo(l Locale) (LocaleInfo, bool) {
	li, ok := localeInfoByCode[l]
	return li, ok
}

// DirectionFor returns the Direction for l, defaulting to DirectionLTR
// for any locale not in SupportedLocales (an unrecognized locale is
// assumed left-to-right, the more common case, rather than erroring --
// mirroring how packages/multilingual.IsRTLScript returns false, not an
// error, for a script it does not track).
func DirectionFor(l Locale) Direction {
	if li, ok := LookupLocaleInfo(l); ok {
		return li.Direction
	}
	return DirectionLTR
}

// IsRTL reports whether l is a right-to-left locale.
func IsRTL(l Locale) bool {
	return DirectionFor(l) == DirectionRTL
}
