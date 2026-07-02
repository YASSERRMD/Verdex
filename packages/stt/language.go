package stt

// LanguageHint biases an STTProvider toward a specific ISO 639-1 language
// code (e.g. "ar", "en", "ur"). The zero value ("") means "no hint / let the
// provider auto-detect".
type LanguageHint string

// LanguageSet is a minimal, dependency-free description of the language(s)
// spoken in a jurisdiction's proceedings. It intentionally mirrors the
// subset of packages/jurisdiction.Jurisdiction that this package needs
// (Languages []string, ordered by precedence) without importing that
// module, so packages/stt has no hard dependency on packages/jurisdiction.
//
// Callers that already have a *jurisdiction.Jurisdiction can construct a
// LanguageSet with:
//
//	stt.LanguageSet{Codes: j.Languages}
type LanguageSet struct {
	// Codes lists ISO 639-1 language codes in order of precedence (the
	// first code is the primary/official language of proceedings).
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
