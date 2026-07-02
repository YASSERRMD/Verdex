package stt_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/stt"
)

func TestLanguageSet_PrimaryHint(t *testing.T) {
	tests := []struct {
		name  string
		codes []string
		want  stt.LanguageHint
	}{
		{"empty", nil, ""},
		{"single", []string{"ar"}, "ar"},
		{"multiple_takes_first", []string{"ar", "en"}, "ar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := stt.LanguageSet{Codes: tt.codes}
			if got := ls.PrimaryHint(); got != tt.want {
				t.Errorf("PrimaryHint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveLanguageHint(t *testing.T) {
	tests := []struct {
		name          string
		codes         []string
		preferredCode string
		want          stt.LanguageHint
	}{
		{"empty_codes", nil, "en", ""},
		{"no_preference_uses_first", []string{"ar", "en"}, "", "ar"},
		{"preference_present", []string{"ar", "en"}, "en", "en"},
		{"preference_absent_falls_back_to_first", []string{"ar", "en"}, "fr", "ar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stt.DeriveLanguageHint(tt.codes, tt.preferredCode)
			if got != tt.want {
				t.Errorf("DeriveLanguageHint(%v, %q) = %q, want %q", tt.codes, tt.preferredCode, got, tt.want)
			}
		})
	}
}
