package multilingual

import "strings"

// LegalTermNormalizer canonicalizes variant spellings, abbreviations, and
// transliteration variants of legal vocabulary to a single canonical form
// per language, so downstream segmentation and reasoning treat equivalent
// terms consistently (e.g. "FIR" and "First Information Report" should
// match the same canonical concept).
//
// The mapping dictionary is pluggable per language: callers supply their
// own jurisdiction-specific vocabulary via NewLegalTermNormalizer or
// AddTerm rather than this package hardcoding a fixed legal glossary,
// since legal terminology is jurisdiction- and language-specific and out
// of scope for a general-purpose normalization package.
type LegalTermNormalizer struct {
	// terms maps Language -> lowercased variant term -> canonical form.
	terms map[Language]map[string]string
}

// NewLegalTermNormalizer returns a LegalTermNormalizer seeded with seed, a
// small worked-example dictionary demonstrating the mapping shape. seed may
// be nil, in which case the normalizer starts empty. Keys of the inner map
// are matched case-insensitively.
func NewLegalTermNormalizer(seed map[Language]map[string]string) *LegalTermNormalizer {
	n := &LegalTermNormalizer{terms: map[Language]map[string]string{}}
	for lang, mapping := range seed {
		for variant, canonical := range mapping {
			n.AddTerm(lang, variant, canonical)
		}
	}
	return n
}

// DefaultLegalTermSeed returns a small, illustrative set of legal-term
// mappings per language. It is intentionally minimal — real jurisdictions
// are expected to supply their own comprehensive glossaries — but gives
// NewLegalTermNormalizer(nil) callers and tests a concrete, working
// example without depending on external data.
func DefaultLegalTermSeed() map[Language]map[string]string {
	return map[Language]map[string]string{
		LanguageEnglish: {
			"fir":                        "First Information Report",
			"first information report":   "First Information Report",
			"ipc":                        "Indian Penal Code",
			"indian penal code":          "Indian Penal Code",
			"petitioner's counsel":       "counsel for petitioner",
			"counsel for the petitioner": "counsel for petitioner",
		},
		LanguageArabic: {
			"محكمة ابتدائية":  "المحكمة الابتدائية",
			"محكمة الاستئناف": "محكمة الاستئناف",
		},
		LanguageUrdu: {
			"ایف آئی آر":    "ابتدائی اطلاعاتی رپورٹ",
			"ابتدائی رپورٹ": "ابتدائی اطلاعاتی رپورٹ",
		},
		LanguageTamil: {
			"நீதிமன்றம்":       "நீதிமன்றம்",
			"கீழமை நீதிமன்றம்": "கீழமை நீதிமன்றம்",
		},
	}
}

// AddTerm registers variant as mapping to canonical for lang. variant is
// matched case-insensitively; canonical is stored verbatim.
func (n *LegalTermNormalizer) AddTerm(lang Language, variant, canonical string) {
	if n.terms[lang] == nil {
		n.terms[lang] = map[string]string{}
	}
	n.terms[lang][strings.ToLower(strings.TrimSpace(variant))] = canonical
}

// Normalize returns the canonical form of term for lang if a mapping
// exists (matched case-insensitively, ignoring surrounding whitespace);
// otherwise it returns term unchanged.
func (n *LegalTermNormalizer) Normalize(lang Language, term string) string {
	mapping, ok := n.terms[lang]
	if !ok {
		return term
	}
	if canonical, ok := mapping[strings.ToLower(strings.TrimSpace(term))]; ok {
		return canonical
	}
	return term
}

// NormalizeText scans text for any known variant term (for lang) as a
// case-insensitive substring and replaces every occurrence with its
// canonical form. Longer variants are replaced before shorter ones so that
// a longer phrase is not partially shadowed by a shorter substring match
// (e.g. "first information report" is matched before any shorter variant
// that might be its substring).
func (n *LegalTermNormalizer) NormalizeText(lang Language, text string) string {
	mapping, ok := n.terms[lang]
	if !ok || text == "" {
		return text
	}

	variants := make([]string, 0, len(mapping))
	for v := range mapping {
		variants = append(variants, v)
	}
	// Longest-first so multi-word variants win over shorter overlapping
	// ones.
	for i := 0; i < len(variants); i++ {
		for j := i + 1; j < len(variants); j++ {
			if len(variants[j]) > len(variants[i]) {
				variants[i], variants[j] = variants[j], variants[i]
			}
		}
	}

	result := text
	lowerResult := strings.ToLower(result)
	for _, variant := range variants {
		canonical := mapping[variant]
		for {
			idx := strings.Index(lowerResult, variant)
			if idx == -1 {
				break
			}
			result = result[:idx] + canonical + result[idx+len(variant):]
			lowerResult = strings.ToLower(result)
		}
	}
	return result
}
