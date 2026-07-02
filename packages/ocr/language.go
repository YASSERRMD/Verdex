package ocr

// LanguageHint biases an OCRProvider toward a specific ISO 639-1 language
// code (e.g. "ar", "en", "ta", "ur"). The zero value ("") means "no hint /
// let the provider auto-detect".
type LanguageHint string

// Script identifies a writing system an OCR provider may need to render or
// recognize glyphs for. Verdex jurisdictions commonly span multiple scripts
// within a single document set (e.g. an Arabic-script judgment with an
// English-script exhibit), so OCR must be able to declare and detect more
// than one script per request.
type Script string

const (
	// ScriptLatin covers English, French, and other Latin-alphabet scripts.
	ScriptLatin Script = "latin"
	// ScriptArabic covers Arabic, and Arabic-script Urdu/Persian text (see
	// ScriptUrdu for the Urdu-specific hint when script disambiguation
	// matters to a provider).
	ScriptArabic Script = "arabic"
	// ScriptTamil covers Tamil-script text.
	ScriptTamil Script = "tamil"
	// ScriptUrdu covers Urdu text, which is Arabic-script but has distinct
	// shaping/ligature behaviour that some OCR backends model separately
	// from generic Arabic.
	ScriptUrdu Script = "urdu"
)

// LanguageSet is a minimal, dependency-free description of the language(s)
// used in a jurisdiction's documents. It intentionally mirrors the subset of
// packages/jurisdiction.Jurisdiction that this package needs
// (Languages []string, ordered by precedence) without importing that
// module, so packages/ocr has no hard dependency on packages/jurisdiction.
//
// Callers that already have a *jurisdiction.Jurisdiction can construct a
// LanguageSet with:
//
//	ocr.LanguageSet{Codes: j.Languages}
type LanguageSet struct {
	// Codes lists ISO 639-1 language codes in order of precedence (the
	// first code is the primary/official language of the document set).
	Codes []string
}

// PrimaryHint returns the LanguageHint derived from the first code in the
// set. It returns the zero LanguageHint ("") if the set is empty.
func (ls LanguageSet) PrimaryHint() LanguageHint {
	if len(ls.Codes) == 0 {
		return ""
	}
	return LanguageHint(ls.Codes[0])
}

// DeriveLanguageHint returns a LanguageHint for the given jurisdiction
// language codes, preferring preferredCode when it is present in codes (so a
// caller can request e.g. the uploader's UI language when it is one of the
// jurisdiction's official languages). If preferredCode is empty or not
// present, the first entry in codes (the jurisdiction's primary language) is
// used. Returns "" if codes is empty.
func DeriveLanguageHint(codes []string, preferredCode string) LanguageHint {
	if preferredCode != "" {
		for _, c := range codes {
			if c == preferredCode {
				return LanguageHint(preferredCode)
			}
		}
	}
	if len(codes) == 0 {
		return ""
	}
	return LanguageHint(codes[0])
}

// scriptsByLanguage maps well-known ISO 639-1 codes to their default
// script. This is a small, deliberately incomplete lookup covering the
// scripts Verdex jurisdictions are known to require; unrecognized codes
// resolve to no script (an empty Script).
var scriptsByLanguage = map[string]Script{
	"en": ScriptLatin,
	"fr": ScriptLatin,
	"ar": ScriptArabic,
	"ur": ScriptUrdu,
	"ta": ScriptTamil,
}

// ScriptForLanguage returns the default Script associated with the given
// ISO 639-1 language code, or "" if the code is unrecognized.
func ScriptForLanguage(code string) Script {
	return scriptsByLanguage[code]
}

// MultiScriptSupport describes which writing systems an OCR request or
// provider must be able to handle simultaneously (e.g. a bilingual judgment
// with an Arabic body and Latin-script citations).
type MultiScriptSupport struct {
	// Scripts lists every Script that may appear in the document set, in no
	// particular order.
	Scripts []Script
}

// HasScript reports whether s is present in m.Scripts.
func (m MultiScriptSupport) HasScript(s Script) bool {
	for _, existing := range m.Scripts {
		if existing == s {
			return true
		}
	}
	return false
}

// IsMultiScript reports whether m declares more than one distinct script.
func (m MultiScriptSupport) IsMultiScript() bool {
	return len(m.Scripts) > 1
}

// DeriveMultiScriptSupport returns the MultiScriptSupport implied by a set
// of ISO 639-1 language codes, deduplicating scripts and skipping
// unrecognized codes.
func DeriveMultiScriptSupport(codes []string) MultiScriptSupport {
	seen := make(map[Script]bool)
	var scripts []Script
	for _, c := range codes {
		s := ScriptForLanguage(c)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		scripts = append(scripts, s)
	}
	return MultiScriptSupport{Scripts: scripts}
}
