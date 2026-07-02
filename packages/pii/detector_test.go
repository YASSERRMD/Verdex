package pii_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/pii"
)

func TestRuleBasedDetector_Detect_Recall(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantPattern string
	}{
		{"email", "Please contact John Smith at john.smith@example.com for details.", "email"},
		{"phone", "Call me at 555-123-4567 tomorrow.", "phone"},
		{"phone international", "Reach the office on +971 4 123 4567 during business hours.", "phone"},
		{"national id", "The applicant's SSN is 123-45-6789 on file.", "national_id"},
		{"long numeric id", "Emirates ID number 784199012345678 was presented.", "national_id"},
		{"address", "The defendant resides at 742 Evergreen Terrace.", "address"},
		{"person name with honorific", "The witness, Mr. Robert Cole, testified first.", "person_name_heuristic"},
		{"person name plain", "Jane Doe signed the affidavit.", "person_name_heuristic"},
	}

	d := pii.NewRuleBasedDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := d.Detect(context.Background(), tt.text)
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			found := false
			for _, m := range matches {
				if m.Pattern == tt.wantPattern {
					found = true
					if m.Text == "" {
						t.Errorf("match Text is empty for pattern %q", tt.wantPattern)
					}
					if m.Start < 0 || m.End <= m.Start {
						t.Errorf("match has invalid offsets: start=%d end=%d", m.Start, m.End)
					}
				}
			}
			if !found {
				t.Errorf("Detect(%q) did not find expected pattern %q; matches=%+v", tt.text, tt.wantPattern, matches)
			}
		})
	}
}

func TestRuleBasedDetector_Detect_Empty(t *testing.T) {
	d := pii.NewRuleBasedDetector()

	matches, err := d.Detect(context.Background(), "")
	if err != nil {
		t.Fatalf("Detect(\"\") error = %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("Detect(\"\") = %d matches, want 0", len(matches))
	}

	matches, err = d.Detect(context.Background(), "   \n\t  ")
	if err != nil {
		t.Fatalf("Detect(whitespace) error = %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("Detect(whitespace) = %d matches, want 0", len(matches))
	}
}

func TestRuleBasedDetector_Detect_NoOverlaps(t *testing.T) {
	d := pii.NewRuleBasedDetector()
	text := "Contact Jane Doe at jane.doe@example.com or 555-987-6543."

	matches, err := d.Detect(context.Background(), text)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(matches) < 2 {
		t.Fatalf("Detect() = %d matches, want at least 2 (name + email/phone)", len(matches))
	}

	for i := 1; i < len(matches); i++ {
		prev, cur := matches[i-1], matches[i]
		if cur.Start < prev.End {
			t.Errorf("matches overlap: prev=%+v cur=%+v", prev, cur)
		}
	}
}

func TestRuleBasedDetector_Detect_RuneOffsetsRoundTrip(t *testing.T) {
	d := pii.NewRuleBasedDetector()
	// Includes multi-byte runes before the PII so rune-vs-byte offset bugs
	// would surface as an incorrect Text slice.
	text := "Café notes: reach José at jose@example.com today."

	matches, err := d.Detect(context.Background(), text)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	runes := []rune(text)
	for _, m := range matches {
		got := string(runes[m.Start:m.End])
		if got != m.Text {
			t.Errorf("match offsets do not round-trip: slice(%d,%d)=%q, Text=%q", m.Start, m.End, got, m.Text)
		}
	}
}

// contextDetector is a minimal pluggable Detector implementation used to
// verify the interface is a genuine extension point (e.g. for a future NER
// model) independent of RuleBasedDetector.
type contextDetector struct {
	matches []pii.PIIMatch
}

func (c contextDetector) Detect(_ context.Context, _ string) ([]pii.PIIMatch, error) {
	return c.matches, nil
}

func TestDetector_InterfaceIsPluggable(t *testing.T) {
	var d pii.Detector = contextDetector{matches: []pii.PIIMatch{{Start: 0, End: 4, Text: "abcd", Pattern: "custom_ner"}}}

	matches, err := d.Detect(context.Background(), "abcd efgh")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(matches) != 1 || matches[0].Pattern != "custom_ner" {
		t.Errorf("Detect() = %+v, want single custom_ner match", matches)
	}
}
