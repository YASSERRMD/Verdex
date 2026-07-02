package pii_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/pii"
)

func TestPIIService_Process_DefaultDetectAndRedact(t *testing.T) {
	svc := pii.NewPIIService()

	result, err := svc.Process(context.Background(), pii.ProcessRequest{
		Text: "The plaintiff, Jane Doe, can be reached at jane.doe@example.com or 555-123-4567.",
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if strings.Contains(result.SanitizedText, "jane.doe@example.com") {
		t.Errorf("SanitizedText still contains raw email: %q", result.SanitizedText)
	}
	if strings.Contains(result.SanitizedText, "555-123-4567") {
		t.Errorf("SanitizedText still contains raw phone: %q", result.SanitizedText)
	}
	if len(result.Matches) < 2 {
		t.Errorf("len(Matches) = %d, want at least 2", len(result.Matches))
	}
	for _, m := range result.Matches {
		if m.Category == "" {
			t.Errorf("match %+v has no Category assigned", m)
		}
	}
}

func TestPIIService_Process_EmptyInput(t *testing.T) {
	svc := pii.NewPIIService()

	_, err := svc.Process(context.Background(), pii.ProcessRequest{Text: "   "})
	if !errors.Is(err, pii.ErrEmptyInput) {
		t.Fatalf("Process() error = %v, want ErrEmptyInput", err)
	}
}

func TestPIIService_Process_PseudonymizeThenReveal(t *testing.T) {
	pmap := pii.NewPseudonymMap(pii.AccessPolicyFunc(func(context.Context, string) bool { return true }))
	svc := &pii.PIIService{
		Detector:   pii.NewRuleBasedDetector(),
		Mode:       pii.ModePseudonymize,
		Pseudonyms: pmap,
	}

	result, err := svc.Process(context.Background(), pii.ProcessRequest{
		Text:  "Jane Doe filed the motion.",
		Actor: "clerk-1",
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(result.Redaction.Applied) == 0 {
		t.Fatal("Process() applied no redactions")
	}

	token := result.Redaction.Applied[0].Token
	if token == "" {
		t.Fatal("applied redaction has no token")
	}

	original, err := svc.Reveal(context.Background(), "clerk-1", "", token)
	if err != nil {
		t.Fatalf("Reveal() error = %v", err)
	}
	if original != "Jane Doe" {
		t.Errorf("Reveal() = %q, want %q", original, "Jane Doe")
	}
}

func TestPIIService_Process_JurisdictionRulesApplied(t *testing.T) {
	rules := pii.NewJurisdictionPIIRules(pii.CategoryRule{})
	rules.SetRule("AE", pii.CategoryContact, pii.CategoryRule{RequiredMode: pii.ModeIrreversibleRedact})

	svc := &pii.PIIService{
		Detector:          pii.NewRuleBasedDetector(),
		Mode:              pii.ModeRedact,
		JurisdictionRules: rules,
	}

	result, err := svc.Process(context.Background(), pii.ProcessRequest{
		Text:             "Email jane.doe@example.com for the docket.",
		JurisdictionCode: "AE",
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	found := false
	for _, applied := range result.Redaction.Applied {
		if applied.Match.Category == pii.CategoryContact {
			found = true
			if applied.Mode != pii.ModeIrreversibleRedact {
				t.Errorf("contact mode = %v, want ModeIrreversibleRedact per jurisdiction rule", applied.Mode)
			}
		}
	}
	if !found {
		t.Fatal("no contact-category match found/applied")
	}
}

func TestPIIService_Reveal_NoMapConfigured(t *testing.T) {
	svc := pii.NewPIIService()

	_, err := svc.Reveal(context.Background(), "actor", "", "PERSON_1")
	if !errors.Is(err, pii.ErrInvalidRequest) {
		t.Fatalf("Reveal() error = %v, want ErrInvalidRequest", err)
	}
}

func TestPIIService_Process_IrreversibleModeNeverRecoverable(t *testing.T) {
	pmap := pii.NewPseudonymMap(pii.AccessPolicyFunc(func(context.Context, string) bool { return true }))
	svc := &pii.PIIService{
		Detector:   pii.NewRuleBasedDetector(),
		Mode:       pii.ModeIrreversibleRedact,
		Pseudonyms: pmap,
	}

	result, err := svc.Process(context.Background(), pii.ProcessRequest{
		Text: "Jane Doe filed the motion.",
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if strings.Contains(result.SanitizedText, "Jane Doe") {
		t.Errorf("SanitizedText still contains raw name: %q", result.SanitizedText)
	}
	// Irreversible redaction produces no token and stores nothing in the
	// PseudonymMap.
	if pmap.Len() != 0 {
		t.Errorf("PseudonymMap.Len() = %d, want 0 (irreversible mode must store nothing)", pmap.Len())
	}
}
