package ocr_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/ocr"
)

func TestLanguageSet_PrimaryHint(t *testing.T) {
	tests := []struct {
		name  string
		codes []string
		want  ocr.LanguageHint
	}{
		{"empty", nil, ""},
		{"single", []string{"ar"}, "ar"},
		{"multiple_takes_first", []string{"ar", "en"}, "ar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := ocr.LanguageSet{Codes: tt.codes}
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
		want          ocr.LanguageHint
	}{
		{"empty_codes", nil, "en", ""},
		{"no_preference_uses_first", []string{"ar", "en"}, "", "ar"},
		{"preference_present", []string{"ar", "en"}, "en", "en"},
		{"preference_absent_falls_back_to_first", []string{"ar", "en"}, "fr", "ar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ocr.DeriveLanguageHint(tt.codes, tt.preferredCode)
			if got != tt.want {
				t.Errorf("DeriveLanguageHint(%v, %q) = %q, want %q", tt.codes, tt.preferredCode, got, tt.want)
			}
		})
	}
}

func TestScriptForLanguage(t *testing.T) {
	tests := []struct {
		code string
		want ocr.Script
	}{
		{"en", ocr.ScriptLatin},
		{"fr", ocr.ScriptLatin},
		{"ar", ocr.ScriptArabic},
		{"ur", ocr.ScriptUrdu},
		{"ta", ocr.ScriptTamil},
		{"zz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := ocr.ScriptForLanguage(tt.code); got != tt.want {
				t.Errorf("ScriptForLanguage(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestDeriveMultiScriptSupport(t *testing.T) {
	tests := []struct {
		name        string
		codes       []string
		wantScripts []ocr.Script
		wantMulti   bool
	}{
		{"empty", nil, nil, false},
		{"single_language", []string{"en"}, []ocr.Script{ocr.ScriptLatin}, false},
		{"two_scripts", []string{"ar", "en"}, []ocr.Script{ocr.ScriptArabic, ocr.ScriptLatin}, true},
		{"dedupes_same_script", []string{"en", "fr"}, []ocr.Script{ocr.ScriptLatin}, false},
		{"unknown_code_skipped", []string{"zz", "ta"}, []ocr.Script{ocr.ScriptTamil}, false},
		{"three_scripts", []string{"ar", "en", "ta"}, []ocr.Script{ocr.ScriptArabic, ocr.ScriptLatin, ocr.ScriptTamil}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ocr.DeriveMultiScriptSupport(tt.codes)
			if len(m.Scripts) != len(tt.wantScripts) {
				t.Fatalf("Scripts = %v, want %v", m.Scripts, tt.wantScripts)
			}
			for i, s := range tt.wantScripts {
				if m.Scripts[i] != s {
					t.Errorf("Scripts[%d] = %q, want %q", i, m.Scripts[i], s)
				}
			}
			if m.IsMultiScript() != tt.wantMulti {
				t.Errorf("IsMultiScript() = %v, want %v", m.IsMultiScript(), tt.wantMulti)
			}
		})
	}
}

func TestMultiScriptSupport_HasScript(t *testing.T) {
	m := ocr.MultiScriptSupport{Scripts: []ocr.Script{ocr.ScriptArabic, ocr.ScriptLatin}}

	if !m.HasScript(ocr.ScriptArabic) {
		t.Error("HasScript(ScriptArabic) = false, want true")
	}
	if m.HasScript(ocr.ScriptTamil) {
		t.Error("HasScript(ScriptTamil) = true, want false")
	}
}
