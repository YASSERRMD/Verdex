package pii_test

import (
	"context"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/pii"
)

func TestRedactor_Redact_NeverLeavesRawPII(t *testing.T) {
	tests := []struct {
		name string
		text string
		mode pii.RedactionMode
	}{
		{"redact email", "Contact jane.doe@example.com now.", pii.ModeRedact},
		{"pseudonymize email", "Contact jane.doe@example.com now.", pii.ModePseudonymize},
		{"irreversible redact email", "Contact jane.doe@example.com now.", pii.ModeIrreversibleRedact},
		{"redact phone", "Call 555-123-4567 immediately.", pii.ModeRedact},
		{"pseudonymize phone", "Call 555-123-4567 immediately.", pii.ModePseudonymize},
	}

	d := pii.NewRuleBasedDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := d.Detect(context.Background(), tt.text)
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			matches = pii.ClassifyMatches(matches)
			if len(matches) == 0 {
				t.Fatalf("Detect(%q) found no matches", tt.text)
			}

			pmap := pii.NewPseudonymMap(pii.AccessPolicyFunc(func(context.Context, string) bool { return true }))
			redactor := pii.NewRedactor(tt.mode, pmap)

			result, err := redactor.Redact(tt.text, matches)
			if err != nil {
				t.Fatalf("Redact() error = %v", err)
			}

			for _, m := range matches {
				if strings.Contains(result.Text, m.Text) {
					t.Errorf("Redact() output still contains raw PII %q: %q", m.Text, result.Text)
				}
			}
		})
	}
}

func TestRedactor_Redact_ModeRedact_UsesCategoryPlaceholder(t *testing.T) {
	text := "Email me at jane.doe@example.com."
	needle := "jane.doe@example.com"
	start := len([]rune("Email me at "))
	end := start + len([]rune(needle))
	matches := pii.ClassifyMatches([]pii.PIIMatch{
		{Start: start, End: end, Text: needle, Pattern: "email"},
	})

	redactor := pii.NewRedactor(pii.ModeRedact, nil)
	result, err := redactor.Redact(text, matches)
	if err != nil {
		t.Fatalf("Redact() error = %v", err)
	}

	want := "Email me at [REDACTED:contact]."
	if result.Text != want {
		t.Errorf("Redact() = %q, want %q", result.Text, want)
	}
}

func TestRedactor_Redact_ModePseudonymize_StableTokenPerEntity(t *testing.T) {
	text := "Jane Doe called. Later, Jane Doe called again."
	// Two occurrences of the same name should receive the same token.
	matches := pii.ClassifyMatches([]pii.PIIMatch{
		{Start: 0, End: 8, Text: "Jane Doe", Category: pii.CategoryName},
		{Start: 25, End: 33, Text: "Jane Doe", Category: pii.CategoryName},
	})

	pmap := pii.NewPseudonymMap(nil)
	redactor := pii.NewRedactor(pii.ModePseudonymize, pmap)

	result, err := redactor.Redact(text, matches)
	if err != nil {
		t.Fatalf("Redact() error = %v", err)
	}

	if len(result.Applied) != 2 {
		t.Fatalf("len(Applied) = %d, want 2", len(result.Applied))
	}
	if result.Applied[0].Token == "" {
		t.Fatalf("first applied token is empty")
	}
	if result.Applied[0].Token != result.Applied[1].Token {
		t.Errorf("same entity got different tokens: %q vs %q", result.Applied[0].Token, result.Applied[1].Token)
	}
	if !strings.HasPrefix(result.Applied[0].Token, "PERSON_") {
		t.Errorf("token = %q, want PERSON_* prefix", result.Applied[0].Token)
	}
}

func TestRedactor_Redact_PseudonymizeWithoutMapReturnsError(t *testing.T) {
	matches := []pii.PIIMatch{{Start: 0, End: 4, Text: "abcd", Category: pii.CategoryOther}}
	redactor := pii.NewRedactor(pii.ModePseudonymize, nil)

	_, err := redactor.Redact("abcd", matches)
	if err == nil {
		t.Fatal("Redact() error = nil, want error for missing PseudonymMap")
	}
}

func TestRedactor_Redact_NoMatches(t *testing.T) {
	redactor := pii.NewRedactor(pii.ModeRedact, nil)
	result, err := redactor.Redact("no pii here", nil)
	if err != nil {
		t.Fatalf("Redact() error = %v", err)
	}
	if result.Text != "no pii here" {
		t.Errorf("Redact() = %q, want unchanged text", result.Text)
	}
}

func TestRedactionMode_IsReversible(t *testing.T) {
	tests := []struct {
		mode pii.RedactionMode
		want bool
	}{
		{pii.ModeRedact, false},
		{pii.ModePseudonymize, true},
		{pii.ModeIrreversibleRedact, false},
	}
	for _, tt := range tests {
		if got := tt.mode.IsReversible(); got != tt.want {
			t.Errorf("%v.IsReversible() = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestRedactor_Redact_ModeByCategoryOverride(t *testing.T) {
	text := "Jane Doe, jane.doe@example.com"
	nameEnd := len([]rune("Jane Doe"))
	emailStart := len([]rune("Jane Doe, "))
	emailEnd := len([]rune(text))
	matches := pii.ClassifyMatches([]pii.PIIMatch{
		{Start: 0, End: nameEnd, Text: "Jane Doe", Category: pii.CategoryName},
		{Start: emailStart, End: emailEnd, Text: "jane.doe@example.com", Category: pii.CategoryContact},
	})

	pmap := pii.NewPseudonymMap(nil)
	redactor := pii.NewRedactor(pii.ModeRedact, pmap)
	redactor.ModeByCategory = map[pii.PIICategory]pii.RedactionMode{
		pii.CategoryName: pii.ModePseudonymize,
	}

	result, err := redactor.Redact(text, matches)
	if err != nil {
		t.Fatalf("Redact() error = %v", err)
	}

	if !strings.HasPrefix(result.Applied[0].Token, "PERSON_") {
		t.Errorf("name category applied = %+v, want pseudonymized with PERSON_ token", result.Applied[0])
	}
	if result.Applied[1].Mode != pii.ModeRedact {
		t.Errorf("contact category mode = %v, want ModeRedact (default)", result.Applied[1].Mode)
	}
	if !strings.Contains(result.Text, "[REDACTED:contact]") {
		t.Errorf("Redact() = %q, want [REDACTED:contact] placeholder for email", result.Text)
	}
}
