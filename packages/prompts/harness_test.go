package prompts_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

func harnessTemplate() prompts.PromptTemplate {
	return prompts.PromptTemplate{
		ID:      "test.harness",
		Name:    "Harness Test Template",
		Version: 1,
		Body: `You are analysing case {{index . "case_id"}}.
Claimant: {{index . "claimant"}}
Jurisdiction: {{index . "jurisdiction"}}`,
		Variables: []prompts.VariableSpec{
			{Name: "case_id", Required: true, Sanitize: true, MaxLen: 64},
			{Name: "claimant", Required: true, Sanitize: true, MaxLen: 256},
			{Name: "jurisdiction", Required: true, Sanitize: true, MaxLen: 256},
		},
		NonBindingLabel: false,
	}
}

func TestHarness_RunCase_Success(t *testing.T) {
	h := prompts.TestHarness{}
	tmpl := harnessTemplate()

	vars := map[string]string{
		"case_id":      "CASE-2024-XYZ",
		"claimant":     "Smith Enterprises Ltd",
		"jurisdiction": "DIFC Courts",
	}

	expected := []string{
		"CASE-2024-XYZ",
		"Smith Enterprises Ltd",
		"DIFC Courts",
	}

	if err := h.RunCase(&tmpl, vars, expected); err != nil {
		t.Errorf("RunCase should have passed, got: %v", err)
	}
}

func TestHarness_RunCase_MissingSubstring(t *testing.T) {
	h := prompts.TestHarness{}
	tmpl := harnessTemplate()

	vars := map[string]string{
		"case_id":      "CASE-2024-ABC",
		"claimant":     "Jones Corp",
		"jurisdiction": "UAE Federal Court",
	}

	// "XYZ" won't be in the rendered output.
	expected := []string{"Jones Corp", "XYZ-not-present"}

	err := h.RunCase(&tmpl, vars, expected)
	if err == nil {
		t.Error("expected RunCase to fail when substring is missing, got nil")
	}
}

func TestHarness_RunCase_RenderError(t *testing.T) {
	h := prompts.TestHarness{}
	tmpl := harnessTemplate()

	// Missing required vars — Render should fail.
	vars := map[string]string{}

	err := h.RunCase(&tmpl, vars, []string{"anything"})
	if err == nil {
		t.Error("expected RunCase to fail on render error, got nil")
	}
}

func TestHarness_RunCase_NilTemplate(t *testing.T) {
	h := prompts.TestHarness{}
	err := h.RunCase(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil template, got nil")
	}
}

func TestHarness_FailsOn_ReturnsTrue_OnError(t *testing.T) {
	h := prompts.TestHarness{}
	tmpl := harnessTemplate()

	// Omit required vars — should fail.
	vars := map[string]string{}

	failed, err := h.FailsOn(&tmpl, vars)
	if err != nil {
		t.Fatalf("FailsOn returned internal error: %v", err)
	}
	if !failed {
		t.Error("expected FailsOn to return true when render fails")
	}
}

func TestHarness_FailsOn_ReturnsFalse_OnSuccess(t *testing.T) {
	h := prompts.TestHarness{}
	tmpl := harnessTemplate()

	vars := map[string]string{
		"case_id":      "CASE-999",
		"claimant":     "Doe LLC",
		"jurisdiction": "ADGM",
	}

	failed, err := h.FailsOn(&tmpl, vars)
	if err != nil {
		t.Fatalf("FailsOn returned internal error: %v", err)
	}
	if failed {
		t.Error("expected FailsOn to return false when render succeeds")
	}
}

func TestHarness_FailsOn_InjectionAttempt(t *testing.T) {
	h := prompts.TestHarness{}
	tmpl := harnessTemplate()

	vars := map[string]string{
		"case_id":      `{{.Injected}}`,
		"claimant":     "Attacker Inc",
		"jurisdiction": "Rogue Court",
	}

	failed, err := h.FailsOn(&tmpl, vars)
	if err != nil {
		t.Fatalf("FailsOn returned internal error: %v", err)
	}
	if !failed {
		t.Error("expected FailsOn to return true when injection attempt is made")
	}
}

func TestHarness_RunCase_WithNonBindingDisclaimer(t *testing.T) {
	h := prompts.TestHarness{}
	tmpl := prompts.PromptTemplate{
		ID:      "test.harness.nonbinding",
		Name:    "Harness NonBinding Test",
		Version: 1,
		Body:    `Analysis for {{index . "subject"}}.`,
		Variables: []prompts.VariableSpec{
			{Name: "subject", Required: true, Sanitize: true, MaxLen: 128},
		},
		NonBindingLabel: true,
	}

	vars := map[string]string{"subject": "Contract Breach Case"}

	expected := []string{
		"Contract Breach Case",
		"DISCLAIMER",
		"NOT constitute legal advice",

	}

	if err := h.RunCase(&tmpl, vars, expected); err != nil {
		t.Errorf("RunCase should have passed with disclaimer substrings, got: %v", err)
	}
}
