package multilingual

import "unicode"

// Script identifies the Unicode writing script a span of text is rendered
// in. Script is independent of Language: Urdu and Arabic share the Arabic
// script, for example, while a single document may mix scripts (e.g. an
// Arabic-script judgment citing an English-script statute).
//
// This mirrors the Script axis in packages/ocr's language.go, extended here
// with the language-detection layer OCR does not need.
type Script string

const (
	// ScriptUnknown is returned when no script could be determined, e.g.
	// for an empty string or text containing only characters with no
	// script association (digits, punctuation).
	ScriptUnknown Script = "unknown"

	// ScriptLatin covers the Latin alphabet used for English (and
	// transliterations).
	ScriptLatin Script = "latin"

	// ScriptArabic covers the Arabic script, shared by Arabic and Urdu
	// text (see Language for how the two are distinguished).
	ScriptArabic Script = "arabic"

	// ScriptTamil covers the Tamil script.
	ScriptTamil Script = "tamil"
)

// Language identifies a candidate natural language for a span of text.
type Language string

const (
	// LanguageUnknown is returned when no candidate language could be
	// determined.
	LanguageUnknown Language = "unknown"

	// LanguageEnglish is English (ISO 639-1 "en").
	LanguageEnglish Language = "en"

	// LanguageArabic is Arabic (ISO 639-1 "ar").
	LanguageArabic Language = "ar"

	// LanguageUrdu is Urdu (ISO 639-1 "ur").
	LanguageUrdu Language = "ur"

	// LanguageTamil is Tamil (ISO 639-1 "ta").
	LanguageTamil Language = "ta"
)

// urduOnlyRanges lists Unicode code points that appear in Urdu text using
// the Arabic script but are not part of standard Arabic orthography (extra
// letters built from Arabic base letters plus dots/marks for sounds Arabic
// does not have, e.g. retroflex consonants and "do-chashmi he"). Their
// presence is a strong, deterministic signal that Arabic-script text is
// Urdu rather than Arabic.
//
// This is intentionally a small, well-known subset (not exhaustive Urdu
// orthography) sufficient for deterministic disambiguation without an ML
// model.
var urduOnlyRanges = []struct{ lo, hi rune }{
	{0x0679, 0x0679}, // ARABIC LETTER TTEH (retroflex te)
	{0x0688, 0x0688}, // ARABIC LETTER DDAL (retroflex dal)
	{0x0691, 0x0691}, // ARABIC LETTER RREH (retroflex re)
	{0x0698, 0x0698}, // ARABIC LETTER JEH (zhe)
	{0x06A9, 0x06A9}, // ARABIC LETTER KEHEH (Urdu/Persian kaf)
	{0x06AF, 0x06AF}, // ARABIC LETTER GAF (ga)
	{0x06BA, 0x06BA}, // ARABIC LETTER NOON GHUNNA
	{0x06BE, 0x06BE}, // ARABIC LETTER HEH DOACHASHMEE
	{0x06C1, 0x06C2}, // ARABIC LETTER HEH GOAL (+ with hamza)
	{0x06CC, 0x06CC}, // ARABIC LETTER FARSI YEH (Urdu choti ye)
	{0x06D2, 0x06D2}, // ARABIC LETTER YEH BARREE (Urdu bari ye)
}

// DetectScript classifies the dominant Unicode script of text by counting
// code points that fall within each script's known Unicode block ranges and
// returning the script with the most matches. Digits, punctuation, and
// whitespace do not count toward any script. Ties are broken in the fixed
// precedence order Latin, Arabic, Tamil (matching the enumeration order of
// the Script constants) for determinism.
//
// DetectScript returns ScriptUnknown for empty text or text with no
// script-bearing characters.
func DetectScript(text string) Script {
	counts := scriptCounts(text)
	return dominantScript(counts)
}

// scriptCounts tallies code points per script.
func scriptCounts(text string) map[Script]int {
	counts := map[Script]int{}
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Latin):
			counts[ScriptLatin]++
		case unicode.In(r, unicode.Arabic):
			counts[ScriptArabic]++
		case unicode.In(r, unicode.Tamil):
			counts[ScriptTamil]++
		}
	}
	return counts
}

// dominantScript picks the script with the highest count, breaking ties by
// fixed precedence (Latin, Arabic, Tamil).
func dominantScript(counts map[Script]int) Script {
	precedence := []Script{ScriptLatin, ScriptArabic, ScriptTamil}

	best := ScriptUnknown
	bestCount := 0
	for _, s := range precedence {
		if c := counts[s]; c > bestCount {
			best = s
			bestCount = c
		}
	}
	return best
}

// DetectLanguage returns the most likely candidate Language for text using
// deterministic, code-point-range-based rules:
//
//  1. Detect the dominant Script via DetectScript.
//  2. ScriptLatin maps to LanguageEnglish.
//  3. ScriptTamil maps to LanguageTamil.
//  4. ScriptArabic is disambiguated between LanguageArabic and LanguageUrdu
//     by checking for Urdu-only letters (see urduOnlyRanges): their
//     presence indicates Urdu, their absence indicates Arabic.
//  5. ScriptUnknown maps to LanguageUnknown.
//
// No statistical or ML model is used; detection is a pure function of the
// code points present in text.
func DetectLanguage(text string) Language {
	switch DetectScript(text) {
	case ScriptLatin:
		return LanguageEnglish
	case ScriptTamil:
		return LanguageTamil
	case ScriptArabic:
		if containsUrduOnlyLetter(text) {
			return LanguageUrdu
		}
		return LanguageArabic
	default:
		return LanguageUnknown
	}
}

// containsUrduOnlyLetter reports whether text contains at least one code
// point from urduOnlyRanges.
func containsUrduOnlyLetter(text string) bool {
	for _, r := range text {
		for _, rg := range urduOnlyRanges {
			if r >= rg.lo && r <= rg.hi {
				return true
			}
		}
	}
	return false
}
