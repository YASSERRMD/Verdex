package multilingual_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/multilingual"
)

func TestNormalizeUnicode_Idempotent(t *testing.T) {
	tests := []struct {
		name string
		text string
		form multilingual.NormalizeForm
	}{
		{"english", "The petitioner's counsel appeared.", multilingual.FormNFC},
		{"arabic", "قررت المحكمة الابتدائية رفض الدعوى", multilingual.FormNFC},
		{"urdu", "عدالت نے درخواست مسترد کر دی", multilingual.FormNFC},
		{"tamil", "நீதிமன்றம் மனுவை நிராகரித்தது", multilingual.FormNFC},
		{"english-nfkc", "The petitioner's counsel appeared.", multilingual.FormNFKC},
		{"arabic-nfkc", "قررت المحكمة الابتدائية رفض الدعوى", multilingual.FormNFKC},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			once := multilingual.NormalizeUnicode(tt.text, tt.form)
			twice := multilingual.NormalizeUnicode(once, tt.form)
			if once != twice {
				t.Errorf("NormalizeUnicode not idempotent: once=%q twice=%q", once, twice)
			}
		})
	}
}

func TestNormalizeUnicode_CollapsesWhitespace(t *testing.T) {
	in := "hello   \t\n  world\r\n"
	got := multilingual.NormalizeUnicode(in, multilingual.FormNFC)
	want := "hello world"
	if got != want {
		t.Errorf("NormalizeUnicode() = %q, want %q", got, want)
	}
}

func TestNormalizeUnicode_StripsControlChars(t *testing.T) {
	in := "hello\x00\x01world"
	got := multilingual.NormalizeUnicode(in, multilingual.FormNFC)
	want := "helloworld"
	if got != want {
		t.Errorf("NormalizeUnicode() = %q, want %q", got, want)
	}
}

func TestNormalizeUnicode_DefaultsToNFC(t *testing.T) {
	in := "café"
	got := multilingual.NormalizeUnicode(in, "")
	want := multilingual.NormalizeUnicode(in, multilingual.FormNFC)
	if got != want {
		t.Errorf("NormalizeUnicode() with empty form = %q, want NFC result %q", got, want)
	}
}

func TestNormalizeUnicode_NFKCFoldsCompatibilityForms(t *testing.T) {
	// U+FF21 FULLWIDTH LATIN CAPITAL LETTER A should fold to "A" under NFKC
	// but not under NFC.
	in := "Ａ"
	nfkc := multilingual.NormalizeUnicode(in, multilingual.FormNFKC)
	if nfkc != "A" {
		t.Errorf("NFKC fullwidth fold = %q, want %q", nfkc, "A")
	}
	nfc := multilingual.NormalizeUnicode(in, multilingual.FormNFC)
	if nfc == "A" {
		t.Errorf("NFC unexpectedly folded fullwidth character")
	}
}

func TestNormalizeUnicode_TrimsLeadingTrailingWhitespace(t *testing.T) {
	in := "   padded text   "
	got := multilingual.NormalizeUnicode(in, multilingual.FormNFC)
	want := "padded text"
	if got != want {
		t.Errorf("NormalizeUnicode() = %q, want %q", got, want)
	}
}
