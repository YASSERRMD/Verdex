package multilingual_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/multilingual"
)

func TestDetectScript(t *testing.T) {
	tests := []struct {
		name string
		text string
		want multilingual.Script
	}{
		{"english", "The court has jurisdiction over this matter.", multilingual.ScriptLatin},
		{"arabic", "قررت المحكمة الابتدائية رفض الدعوى", multilingual.ScriptArabic},
		{"urdu", "عدالت نے درخواست مسترد کر دی", multilingual.ScriptArabic},
		{"tamil", "நீதிமன்றம் மனுவை நிராகரித்தது", multilingual.ScriptTamil},
		{"empty", "", multilingual.ScriptUnknown},
		{"digits-only", "12345", multilingual.ScriptUnknown},
		{"punctuation-only", "!!! ,,, ...", multilingual.ScriptUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := multilingual.DetectScript(tt.text); got != tt.want {
				t.Errorf("DetectScript(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name string
		text string
		want multilingual.Language
	}{
		{"english", "The petitioner filed an appeal before the High Court.", multilingual.LanguageEnglish},
		{"arabic", "قررت المحكمة الابتدائية رفض الدعوى وإلزام المدعي بالرسوم", multilingual.LanguageArabic},
		{"urdu", "عدالت نے درخواست مسترد کر دی اور ہرجانہ عائد کیا گیا", multilingual.LanguageUrdu},
		{"tamil", "நீதிமன்றம் மனுவை நிராகரித்து செலவினங்களை விதித்தது", multilingual.LanguageTamil},
		{"empty", "", multilingual.LanguageUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := multilingual.DetectLanguage(tt.text); got != tt.want {
				t.Errorf("DetectLanguage(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectLanguage_UrduVsArabicDisambiguation(t *testing.T) {
	// Contains ARABIC LETTER GAF (U+06AF), an Urdu-only letter.
	urdu := "گاؤں کی عدالت"
	if got := multilingual.DetectLanguage(urdu); got != multilingual.LanguageUrdu {
		t.Errorf("DetectLanguage(%q) = %v, want LanguageUrdu", urdu, got)
	}

	// Standard Arabic orthography without Urdu-only letters.
	arabic := "قرار المحكمة"
	if got := multilingual.DetectLanguage(arabic); got != multilingual.LanguageArabic {
		t.Errorf("DetectLanguage(%q) = %v, want LanguageArabic", arabic, got)
	}
}
