package multilingual

import (
	"strings"
	"unicode"
)

// Tokenizer splits text into word-level tokens using rules appropriate to a
// given Language. Word-boundary conventions differ meaningfully across the
// scripts Verdex supports: Latin-script English uses whitespace and
// punctuation as separators; Tamil uses whitespace with orthographic
// syllables (grapheme clusters composed of a consonant plus dependent
// vowel signs) that must not be split apart; Arabic and Urdu use
// whitespace but must additionally treat joining marks (tatweel, zero-width
// joiner/non-joiner) as part of the token they modify rather than as
// separators.
//
// Implementations MUST be deterministic and rule-based; no ML model is
// used anywhere in this package.
type Tokenizer interface {
	// Tokenize splits text into tokens according to lang's word-boundary
	// rules. Returns an empty (non-nil) slice for empty/whitespace-only
	// text.
	Tokenize(text string, lang Language) []string
}

// RuleBasedTokenizer is the default Tokenizer. It applies a single set of
// deterministic rules that adapt per language:
//
//   - Whitespace always separates tokens.
//   - For LanguageArabic and LanguageUrdu, the tatweel character (ARABIC
//     TATWEEL, U+0640) and zero-width joiner/non-joiner (U+200C, U+200D)
//     do not separate tokens: they are retained as part of the
//     surrounding token, since they modify letter joining rather than
//     marking a word boundary.
//   - For LanguageTamil, combining vowel signs and virama (the Tamil
//     "pulli") do not separate tokens, keeping orthographic syllables
//     intact within a single token.
//   - For LanguageEnglish (and any other/unknown language), standard
//     Unicode punctuation and symbols split tokens, except for the
//     in-word apostrophe/hyphen ('  and -), which are retained (so
//     "petitioner's" and "co-accused" remain single tokens).
//
// RuleBasedTokenizer never discards characters: every non-whitespace,
// non-splitting rune from the input is retained inside some token.
type RuleBasedTokenizer struct{}

// Tokenize implements Tokenizer.
func (RuleBasedTokenizer) Tokenize(text string, lang Language) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}

	isSeparator := separatorFunc(lang)

	tokens := []string{}
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	for _, r := range text {
		if isSeparator(r) {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	flush()

	return tokens
}

// separatorFunc returns the rune predicate used to split tokens for lang.
func separatorFunc(lang Language) func(rune) bool {
	switch lang {
	case LanguageArabic, LanguageUrdu:
		return func(r rune) bool {
			if r == 0x0640 || r == 0x200C || r == 0x200D { // tatweel, ZWNJ, ZWJ
				return false
			}
			return unicode.IsSpace(r) || (unicode.IsPunct(r) && r != '\'' && r != '-') || unicode.IsSymbol(r)
		}
	case LanguageTamil:
		return func(r rune) bool {
			if unicode.In(r, unicode.Mn) { // combining marks / vowel signs / virama
				return false
			}
			return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
		}
	default: // LanguageEnglish and unknown
		return func(r rune) bool {
			if r == '\'' || r == '-' {
				return false
			}
			return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
		}
	}
}
