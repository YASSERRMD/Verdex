package prompts_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

// minimalTemplate returns a simple PromptTemplate suitable for testing.
func minimalTemplate() prompts.PromptTemplate {
	return prompts.PromptTemplate{
		ID:      "test.render",
		Name:    "Test Render",
		Version: 1,
		Body:    `Hello, {{index . "name"}}! Case: {{index . "case_ref"}}.`,
		Variables: []prompts.VariableSpec{
			{Name: "name", Required: true, Sanitize: true, MaxLen: 64},
			{Name: "case_ref", Required: true, Sanitize: false, MaxLen: 32},
		},
	}
}

func TestRender_AllVars_Success(t *testing.T) {
	tmpl := minimalTemplate()
	vars := map[string]string{
		"name":     "Alice",
		"case_ref": "CASE-2024-001",
	}

	out, err := prompts.Render(&tmpl, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected output to contain %q, got: %s", "Alice", out)
	}
	if !strings.Contains(out, "CASE-2024-001") {
		t.Errorf("expected output to contain %q, got: %s", "CASE-2024-001", out)
	}
}

func TestRender_MissingRequiredVar_Error(t *testing.T) {
	tmpl := minimalTemplate()
	// Omit "name" which is required.
	vars := map[string]string{
		"case_ref": "CASE-2024-002",
	}

	_, err := prompts.Render(&tmpl, vars)
	if err == nil {
		t.Fatal("expected error for missing required variable, got nil")
	}
	if !errors.Is(err, prompts.ErrMissingVariable) {
		t.Errorf("expected ErrMissingVariable, got: %v", err)
	}
}

func TestRender_InjectionAttemptInVar_Error(t *testing.T) {
	tmpl := minimalTemplate()
	vars := map[string]string{
		"name":     `{{.Secret}}`, // injection attempt
		"case_ref": "CASE-2024-003",
	}

	_, err := prompts.Render(&tmpl, vars)
	if err == nil {
		t.Fatal("expected error for injection attempt, got nil")
	}
	if !errors.Is(err, prompts.ErrInjectionAttempt) {
		t.Errorf("expected ErrInjectionAttempt, got: %v", err)
	}
}

func TestRender_NonBindingLabel_DisclaimerAppended(t *testing.T) {
	tmpl := prompts.PromptTemplate{
		ID:      "test.nonbinding",
		Name:    "Test NonBinding",
		Version: 1,
		Body:    `Analysis: {{index . "content"}}`,
		Variables: []prompts.VariableSpec{
			{Name: "content", Required: true, Sanitize: true, MaxLen: 1024},
		},
		NonBindingLabel: true,
	}

	vars := map[string]string{"content": "some legal analysis"}

	out, err := prompts.Render(&tmpl, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "DISCLAIMER") {
		t.Errorf("expected non-binding disclaimer in output, got:\n%s", out)
	}
	if !strings.Contains(out, "NOT constitute legal advice") {
		t.Errorf("expected disclaimer text in output, got:\n%s", out)
	}
}

func TestRender_NoNonBindingLabel_NoDisclaimer(t *testing.T) {
	tmpl := prompts.PromptTemplate{
		ID:      "test.binding",
		Name:    "Test Binding",
		Version: 1,
		Body:    `{{index . "content"}}`,
		Variables: []prompts.VariableSpec{
			{Name: "content", Required: true, Sanitize: true, MaxLen: 1024},
		},
		NonBindingLabel: false,
	}

	vars := map[string]string{"content": "some content"}

	out, err := prompts.Render(&tmpl, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "DISCLAIMER") {
		t.Errorf("expected no disclaimer when NonBindingLabel=false, got:\n%s", out)
	}
}

func TestRender_VarTooLong_Error(t *testing.T) {
	tmpl := prompts.PromptTemplate{
		ID:      "test.maxlen",
		Name:    "Test MaxLen",
		Version: 1,
		Body:    `{{index . "short"}}`,
		Variables: []prompts.VariableSpec{
			{Name: "short", Required: true, Sanitize: true, MaxLen: 5},
		},
	}

	vars := map[string]string{"short": "this is way too long"}

	_, err := prompts.Render(&tmpl, vars)
	if err == nil {
		t.Fatal("expected error for variable too long, got nil")
	}
	if !errors.Is(err, prompts.ErrVariableTooLong) {
		t.Errorf("expected ErrVariableTooLong, got: %v", err)
	}
}

func TestRender_NilTemplate_Error(t *testing.T) {
	_, err := prompts.Render(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil template")
	}
	if !errors.Is(err, prompts.ErrInvalidTemplate) {
		t.Errorf("expected ErrInvalidTemplate, got: %v", err)
	}
}

func TestRender_ControlCharsStripped(t *testing.T) {
	tmpl := minimalTemplate()
	vars := map[string]string{
		"name":     "Bob\x00\x01\x02",
		"case_ref": "CASE-X",
	}

	out, err := prompts.Render(&tmpl, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.ContainsAny(out, "\x00\x01\x02") {
		t.Error("expected control characters to be stripped from output")
	}
	if !strings.Contains(out, "Bob") {
		t.Errorf("expected 'Bob' in output after stripping control chars, got: %s", out)
	}
}
