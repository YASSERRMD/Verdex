package multilingual_test

import (
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/multilingual"
)

func TestIsRTLScript(t *testing.T) {
	tests := []struct {
		script multilingual.Script
		want   bool
	}{
		{multilingual.ScriptArabic, true},
		{multilingual.ScriptLatin, false},
		{multilingual.ScriptTamil, false},
		{multilingual.ScriptUnknown, false},
	}
	for _, tt := range tests {
		if got := multilingual.IsRTLScript(tt.script); got != tt.want {
			t.Errorf("IsRTLScript(%v) = %v, want %v", tt.script, got, tt.want)
		}
	}
}

func TestDetectRTLRuns(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantAnyRTL  bool
		wantAllRTL  bool
		wantRunsMin int
	}{
		{"pure-english", "The court ruled in favor of the appellant.", false, false, 1},
		{"pure-arabic", "قررت المحكمة الابتدائية رفض الدعوى", true, true, 1},
		{"pure-urdu", "عدالت نے درخواست مسترد کر دی", true, true, 1},
		{"pure-tamil", "நீதிமன்றம் மனுவை நிராகரித்தது", false, false, 1},
		{"mixed-arabic-english", "The judgment states قررت المحكمة clearly.", true, false, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := multilingual.DetectRTLRuns(tt.text)
			if len(runs) < tt.wantRunsMin {
				t.Fatalf("DetectRTLRuns(%q) returned %d runs, want at least %d", tt.text, len(runs), tt.wantRunsMin)
			}

			anyRTL := false
			allRTL := true
			var reassembled strings.Builder
			for _, r := range runs {
				if r.IsRTL {
					anyRTL = true
				} else {
					allRTL = false
				}
				reassembled.WriteString(r.Text)
			}

			if anyRTL != tt.wantAnyRTL {
				t.Errorf("anyRTL = %v, want %v", anyRTL, tt.wantAnyRTL)
			}
			if tt.wantRunsMin == 1 && allRTL != tt.wantAllRTL {
				t.Errorf("allRTL = %v, want %v", allRTL, tt.wantAllRTL)
			}

			// Logical order must be preserved: concatenating the runs
			// reproduces the original text exactly.
			if reassembled.String() != tt.text {
				t.Errorf("reassembled runs = %q, want original %q (logical order not preserved)", reassembled.String(), tt.text)
			}
		})
	}
}

func TestDetectRTLRuns_EmptyText(t *testing.T) {
	runs := multilingual.DetectRTLRuns("")
	if runs != nil {
		t.Errorf("DetectRTLRuns(\"\") = %v, want nil", runs)
	}
}

func TestWrapWithBidiControls(t *testing.T) {
	rtl := multilingual.WrapWithBidiControls("قرار", true)
	if !strings.HasPrefix(rtl, string(multilingual.RLE)) || !strings.HasSuffix(rtl, string(multilingual.PDF)) {
		t.Errorf("WrapWithBidiControls(rtl) = %q, want RLE...PDF wrapping", rtl)
	}

	ltr := multilingual.WrapWithBidiControls("order", false)
	if !strings.HasPrefix(ltr, string(multilingual.LRE)) || !strings.HasSuffix(ltr, string(multilingual.PDF)) {
		t.Errorf("WrapWithBidiControls(ltr) = %q, want LRE...PDF wrapping", ltr)
	}

	if got := multilingual.WrapWithBidiControls("", true); got != "" {
		t.Errorf("WrapWithBidiControls(empty) = %q, want empty", got)
	}
}
