package multilingual_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/multilingual"
)

func TestPassthroughTransliterator_NeverChangesText(t *testing.T) {
	tr := multilingual.PassthroughTransliterator{}

	tests := []struct {
		name       string
		text       string
		fromScript multilingual.Script
		toScript   multilingual.Script
	}{
		{"english", "The court of appeal", multilingual.ScriptLatin, multilingual.ScriptLatin},
		{"arabic", "قرار المحكمة", multilingual.ScriptArabic, multilingual.ScriptLatin},
		{"urdu", "عدالت کا فیصلہ", multilingual.ScriptArabic, multilingual.ScriptLatin},
		{"tamil", "நீதிமன்ற தீர்ப்பு", multilingual.ScriptTamil, multilingual.ScriptLatin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tr.Transliterate(tt.text, tt.fromScript, tt.toScript)
			if err != nil {
				t.Fatalf("Transliterate() unexpected error: %v", err)
			}
			if got != tt.text {
				t.Errorf("Transliterate() = %q, want unchanged %q", got, tt.text)
			}
		})
	}

	if tr.ID() != "passthrough" {
		t.Errorf("ID() = %q, want %q", tr.ID(), "passthrough")
	}
}

func TestExampleTamilTransliterator_MapsKnownCharacters(t *testing.T) {
	tr := multilingual.ExampleTamilTransliterator{}

	// 'அ' -> "a", 'ம' -> "ma" (both in exampleTamilToLatinMap).
	got, err := tr.Transliterate("அம", multilingual.ScriptTamil, multilingual.ScriptLatin)
	if err != nil {
		t.Fatalf("Transliterate() unexpected error: %v", err)
	}
	want := "ama"
	if got != want {
		t.Errorf("Transliterate() = %q, want %q", got, want)
	}
}

func TestExampleTamilTransliterator_PassesThroughUnmappedCharacters(t *testing.T) {
	tr := multilingual.ExampleTamilTransliterator{}

	// 'ஔ' is not in exampleTamilToLatinMap, so it must pass through
	// unchanged; 'அ' is mapped to "a".
	got, err := tr.Transliterate("அஔ", multilingual.ScriptTamil, multilingual.ScriptLatin)
	if err != nil {
		t.Fatalf("Transliterate() unexpected error: %v", err)
	}
	want := "a" + "ஔ"
	if got != want {
		t.Errorf("Transliterate() = %q, want %q", got, want)
	}
}

func TestExampleTamilTransliterator_UnsupportedScriptPair(t *testing.T) {
	tr := multilingual.ExampleTamilTransliterator{}

	got, err := tr.Transliterate("hello", multilingual.ScriptLatin, multilingual.ScriptArabic)
	if !errors.Is(err, multilingual.ErrUnsupportedScript) {
		t.Errorf("Transliterate() error = %v, want ErrUnsupportedScript", err)
	}
	if got != "hello" {
		t.Errorf("Transliterate() = %q, want unchanged input on unsupported pair", got)
	}
}
