package pii_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/pii"
)

func TestStorageGuard_WriteGuarded_RedactsBeforeAtRest(t *testing.T) {
	guard := pii.NewStorageGuard(pii.StoragePolicy{
		Detector: pii.NewRuleBasedDetector(),
		Mode:     pii.ModeRedact,
	})

	result, err := guard.WriteGuarded(context.Background(), "Contact Jane Doe at jane.doe@example.com.")
	if err != nil {
		t.Fatalf("WriteGuarded() error = %v", err)
	}
	if strings.Contains(result.Text, "jane.doe@example.com") {
		t.Errorf("WriteGuarded() text still contains raw email: %q", result.Text)
	}
	if len(result.Matches) == 0 {
		t.Error("WriteGuarded() found no matches, expected at least the email")
	}
}

func TestStorageGuard_WriteGuarded_RejectsConfiguredCategories(t *testing.T) {
	guard := pii.NewStorageGuard(pii.StoragePolicy{
		Detector:         pii.NewRuleBasedDetector(),
		Mode:             pii.ModeRedact,
		RejectCategories: map[pii.PIICategory]bool{pii.CategoryContact: true},
	})

	result, err := guard.WriteGuarded(context.Background(), "Contact Jane Doe at jane.doe@example.com.")
	if !errors.Is(err, pii.ErrPolicyViolation) {
		t.Fatalf("WriteGuarded() error = %v, want ErrPolicyViolation", err)
	}
	if result.Text != "" {
		t.Errorf("WriteGuarded() on rejected write returned non-empty text: %q", result.Text)
	}
}

func TestStorageGuard_WriteGuarded_AllowsCleanText(t *testing.T) {
	guard := pii.NewStorageGuard(pii.StoragePolicy{
		Detector: pii.NewRuleBasedDetector(),
		Mode:     pii.ModeRedact,
	})

	result, err := guard.WriteGuarded(context.Background(), "The hearing is scheduled for next Tuesday.")
	if err != nil {
		t.Fatalf("WriteGuarded() error = %v", err)
	}
	if result.Text != "The hearing is scheduled for next Tuesday." {
		t.Errorf("WriteGuarded() = %q, want unchanged text", result.Text)
	}
}

func TestStorageGuard_WriteGuarded_AppliesJurisdictionOverride(t *testing.T) {
	rules := pii.NewJurisdictionPIIRules(pii.CategoryRule{})
	rules.SetRule("AE", pii.CategoryContact, pii.CategoryRule{RequiredMode: pii.ModeIrreversibleRedact})

	pmap := pii.NewPseudonymMap(nil)
	guard := pii.NewStorageGuard(pii.StoragePolicy{
		Detector:          pii.NewRuleBasedDetector(),
		Mode:              pii.ModePseudonymize, // package-wide default
		Pseudonyms:        pmap,
		JurisdictionRules: rules,
		JurisdictionCode:  "AE",
	})

	result, err := guard.WriteGuarded(context.Background(), "Email jane.doe@example.com for records.")
	if err != nil {
		t.Fatalf("WriteGuarded() error = %v", err)
	}

	for _, applied := range result.Redaction.Applied {
		if applied.Match.Category == pii.CategoryContact && applied.Mode != pii.ModeIrreversibleRedact {
			t.Errorf("contact category mode = %v, want jurisdiction-required ModeIrreversibleRedact", applied.Mode)
		}
	}
}

func TestStorageGuard_WriteGuarded_RequiresDetector(t *testing.T) {
	guard := pii.NewStorageGuard(pii.StoragePolicy{})

	_, err := guard.WriteGuarded(context.Background(), "some text")
	if !errors.Is(err, pii.ErrInvalidRequest) {
		t.Fatalf("WriteGuarded() error = %v, want ErrInvalidRequest", err)
	}
}
