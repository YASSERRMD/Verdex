package multilingual_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/multilingual"
)

func TestRuleBasedTokenizer_ProducesNonEmptyTokens(t *testing.T) {
	tz := multilingual.RuleBasedTokenizer{}

	tests := []struct {
		name string
		text string
		lang multilingual.Language
	}{
		{"english", "The court ruled in favor of the appellant.", multilingual.LanguageEnglish},
		{"arabic", "قررت المحكمة الابتدائية رفض الدعوى", multilingual.LanguageArabic},
		{"urdu", "عدالت نے درخواست مسترد کر دی", multilingual.LanguageUrdu},
		{"tamil", "நீதிமன்றம் மனுவை நிராகரித்தது", multilingual.LanguageTamil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tz.Tokenize(tt.text, tt.lang)
			if len(tokens) == 0 {
				t.Fatalf("Tokenize(%q) returned no tokens", tt.text)
			}
			for i, tok := range tokens {
				if tok == "" {
					t.Errorf("token[%d] is empty", i)
				}
			}
		})
	}
}

func TestRuleBasedTokenizer_EnglishKeepsApostropheAndHyphen(t *testing.T) {
	tz := multilingual.RuleBasedTokenizer{}
	tokens := tz.Tokenize("petitioner's co-accused fled.", multilingual.LanguageEnglish)

	want := []string{"petitioner's", "co-accused", "fled"}
	if len(tokens) != len(want) {
		t.Fatalf("Tokenize() = %v, want %v", tokens, want)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i], want[i])
		}
	}
}

func TestRuleBasedTokenizer_ArabicSplitsOnWhitespace(t *testing.T) {
	tz := multilingual.RuleBasedTokenizer{}
	tokens := tz.Tokenize("قررت المحكمة الابتدائية", multilingual.LanguageArabic)

	if len(tokens) != 3 {
		t.Fatalf("Tokenize() returned %d tokens, want 3: %v", len(tokens), tokens)
	}
}

func TestRuleBasedTokenizer_TamilPreservesCombiningMarks(t *testing.T) {
	tz := multilingual.RuleBasedTokenizer{}
	// நீதிமன்றம் is a single Tamil word with combining vowel signs; it must
	// remain a single token.
	tokens := tz.Tokenize("நீதிமன்றம் மனு", multilingual.LanguageTamil)

	if len(tokens) != 2 {
		t.Fatalf("Tokenize() returned %d tokens, want 2: %v", len(tokens), tokens)
	}
	if tokens[0] != "நீதிமன்றம்" {
		t.Errorf("token[0] = %q, want %q (combining marks must stay attached)", tokens[0], "நீதிமன்றம்")
	}
}

func TestRuleBasedTokenizer_EmptyText(t *testing.T) {
	tz := multilingual.RuleBasedTokenizer{}

	tokens := tz.Tokenize("", multilingual.LanguageEnglish)
	if tokens == nil {
		t.Fatalf("Tokenize(\"\") returned nil, want non-nil empty slice")
	}
	if len(tokens) != 0 {
		t.Errorf("Tokenize(\"\") = %v, want empty", tokens)
	}

	tokens = tz.Tokenize("   ", multilingual.LanguageEnglish)
	if len(tokens) != 0 {
		t.Errorf("Tokenize(whitespace) = %v, want empty", tokens)
	}
}

func TestRuleBasedTokenizer_UrduRetainsJoiningMarks(t *testing.T) {
	tz := multilingual.RuleBasedTokenizer{}
	// Includes a ZWNJ (U+200C) inside a word, which should not split the
	// token.
	text := "کر‌دیا گیا"
	tokens := tz.Tokenize(text, multilingual.LanguageUrdu)

	if len(tokens) != 2 {
		t.Fatalf("Tokenize() returned %d tokens, want 2: %v", len(tokens), tokens)
	}
}
