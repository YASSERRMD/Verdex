package multilingual_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/multilingual"
)

func TestNoOpTranslator_ReturnsUnchanged(t *testing.T) {
	tr := multilingual.NoOpTranslator{}

	tests := []struct {
		name   string
		text   string
		source multilingual.Language
		target multilingual.Language
	}{
		{"english", "The court ruled in favor of the appellant.", multilingual.LanguageEnglish, multilingual.LanguageArabic},
		{"arabic", "قررت المحكمة", multilingual.LanguageArabic, multilingual.LanguageEnglish},
		{"urdu", "عدالت کا فیصلہ", multilingual.LanguageUrdu, multilingual.LanguageEnglish},
		{"tamil", "நீதிமன்ற தீர்ப்பு", multilingual.LanguageTamil, multilingual.LanguageEnglish},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tr.Translate(context.Background(), tt.text, tt.source, tt.target)
			if err != nil {
				t.Fatalf("Translate() unexpected error: %v", err)
			}
			if got != tt.text {
				t.Errorf("Translate() = %q, want unchanged %q", got, tt.text)
			}
		})
	}

	if tr.ID() != "noop" {
		t.Errorf("ID() = %q, want %q", tr.ID(), "noop")
	}
}

func TestTranslate_OriginalAlwaysPreserved(t *testing.T) {
	tests := []struct {
		name       string
		translator multilingual.Translator
		text       string
		source     multilingual.Language
		target     multilingual.Language
	}{
		{"noop-same-language", multilingual.NoOpTranslator{}, "hello", multilingual.LanguageEnglish, multilingual.LanguageEnglish},
		{"noop-cross-language", multilingual.NoOpTranslator{}, "hello", multilingual.LanguageEnglish, multilingual.LanguageArabic},
		{"nil-translator", nil, "hello", multilingual.LanguageEnglish, multilingual.LanguageArabic},
		{"failing-translator", failingTranslator{}, "hello", multilingual.LanguageEnglish, multilingual.LanguageArabic},
		{"real-translator", upperTranslator{}, "hello", multilingual.LanguageEnglish, multilingual.LanguageArabic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := multilingual.Translate(context.Background(), tt.translator, tt.text, tt.source, tt.target)
			if result.Original != tt.text {
				t.Errorf("Original = %q, want %q (original text must always be preserved)", result.Original, tt.text)
			}
		})
	}
}

func TestTranslate_SameLanguageSkipsTranslator(t *testing.T) {
	result, err := multilingual.Translate(context.Background(), upperTranslator{}, "hello", multilingual.LanguageEnglish, multilingual.LanguageEnglish)
	if err != nil {
		t.Fatalf("Translate() unexpected error: %v", err)
	}
	if result.Applied {
		t.Errorf("Applied = true, want false when source == target")
	}
	if result.Translated != "hello" {
		t.Errorf("Translated = %q, want unchanged %q", result.Translated, "hello")
	}
}

func TestTranslate_AppliesRealTranslator(t *testing.T) {
	result, err := multilingual.Translate(context.Background(), upperTranslator{}, "hello", multilingual.LanguageEnglish, multilingual.LanguageArabic)
	if err != nil {
		t.Fatalf("Translate() unexpected error: %v", err)
	}
	if !result.Applied {
		t.Errorf("Applied = false, want true")
	}
	if result.Translated != "HELLO" {
		t.Errorf("Translated = %q, want %q", result.Translated, "HELLO")
	}
	if result.Original != "hello" {
		t.Errorf("Original = %q, want %q", result.Original, "hello")
	}
	if result.TargetLanguage != multilingual.LanguageArabic {
		t.Errorf("TargetLanguage = %v, want LanguageArabic", result.TargetLanguage)
	}
}

func TestTranslate_ErrorPropagatesButPreservesOriginal(t *testing.T) {
	result, err := multilingual.Translate(context.Background(), failingTranslator{}, "hello", multilingual.LanguageEnglish, multilingual.LanguageArabic)
	if !errors.Is(err, errFailingTranslator) {
		t.Errorf("Translate() error = %v, want errFailingTranslator", err)
	}
	if result.Original != "hello" {
		t.Errorf("Original = %q, want %q", result.Original, "hello")
	}
	if result.Applied {
		t.Errorf("Applied = true, want false on failure")
	}
}

// upperTranslator is a test Translator that uppercases text.
type upperTranslator struct{}

func (upperTranslator) ID() string { return "upper" }
func (upperTranslator) Translate(_ context.Context, text string, _, _ multilingual.Language) (string, error) {
	upper := make([]byte, len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		upper[i] = c
	}
	return string(upper), nil
}

var errFailingTranslator = errors.New("translation backend unavailable")

// failingTranslator is a test Translator that always errors.
type failingTranslator struct{}

func (failingTranslator) ID() string { return "failing" }
func (failingTranslator) Translate(_ context.Context, _ string, _, _ multilingual.Language) (string, error) {
	return "", errFailingTranslator
}
