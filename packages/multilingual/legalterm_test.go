package multilingual_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/multilingual"
)

func TestLegalTermNormalizer_Normalize(t *testing.T) {
	n := multilingual.NewLegalTermNormalizer(multilingual.DefaultLegalTermSeed())

	tests := []struct {
		name string
		lang multilingual.Language
		term string
		want string
	}{
		{"english-abbreviation", multilingual.LanguageEnglish, "FIR", "First Information Report"},
		{"english-case-insensitive", multilingual.LanguageEnglish, "fir", "First Information Report"},
		{"english-unmapped", multilingual.LanguageEnglish, "affidavit", "affidavit"},
		{"arabic-variant", multilingual.LanguageArabic, "محكمة ابتدائية", "المحكمة الابتدائية"},
		{"urdu-abbreviation", multilingual.LanguageUrdu, "ایف آئی آر", "ابتدائی اطلاعاتی رپورٹ"},
		{"tamil-term", multilingual.LanguageTamil, "கீழமை நீதிமன்றம்", "கீழமை நீதிமன்றம்"},
		{"unknown-language", multilingual.LanguageUnknown, "FIR", "FIR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := n.Normalize(tt.lang, tt.term); got != tt.want {
				t.Errorf("Normalize(%v, %q) = %q, want %q", tt.lang, tt.term, got, tt.want)
			}
		})
	}
}

func TestLegalTermNormalizer_NormalizeText(t *testing.T) {
	n := multilingual.NewLegalTermNormalizer(multilingual.DefaultLegalTermSeed())

	in := "The FIR was filed under the IPC before the magistrate."
	got := n.NormalizeText(multilingual.LanguageEnglish, in)

	want := "The First Information Report was filed under the Indian Penal Code before the magistrate."
	if got != want {
		t.Errorf("NormalizeText() = %q, want %q", got, want)
	}
}

func TestLegalTermNormalizer_NormalizeText_LongestVariantWins(t *testing.T) {
	n := multilingual.NewLegalTermNormalizer(nil)
	n.AddTerm(multilingual.LanguageEnglish, "counsel for the petitioner", "counsel for petitioner")
	n.AddTerm(multilingual.LanguageEnglish, "petitioner", "the petitioner")

	got := n.NormalizeText(multilingual.LanguageEnglish, "counsel for the petitioner appeared")
	want := "counsel for petitioner appeared"
	if got != want {
		t.Errorf("NormalizeText() = %q, want %q", got, want)
	}
}

func TestLegalTermNormalizer_AddTerm(t *testing.T) {
	n := multilingual.NewLegalTermNormalizer(nil)
	n.AddTerm(multilingual.LanguageEnglish, "  Learned Counsel  ", "counsel")

	if got := n.Normalize(multilingual.LanguageEnglish, "learned counsel"); got != "counsel" {
		t.Errorf("Normalize() = %q, want %q", got, "counsel")
	}
}

func TestLegalTermNormalizer_EmptySeed(t *testing.T) {
	n := multilingual.NewLegalTermNormalizer(nil)
	if got := n.Normalize(multilingual.LanguageEnglish, "FIR"); got != "FIR" {
		t.Errorf("Normalize() with empty seed = %q, want unchanged %q", got, "FIR")
	}
}
