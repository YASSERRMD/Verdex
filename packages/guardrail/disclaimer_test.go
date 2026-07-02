package guardrail_test

import (
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

func TestRequireDisclaimerAppends(t *testing.T) {
	text := "The evidence favors the plaintiff on this issue."
	out := guardrail.RequireDisclaimer(text)

	if !strings.HasPrefix(out, text) {
		t.Fatalf("RequireDisclaimer output does not start with original text: %q", out)
	}
	if !strings.Contains(out, "DRAFT ANALYSIS") {
		t.Fatalf("RequireDisclaimer output missing disclaimer marker: %q", out)
	}
	if !guardrail.HasDisclaimer(out) {
		t.Fatal("HasDisclaimer(RequireDisclaimer(text)) = false, want true")
	}
}

func TestRequireDisclaimerIdempotent(t *testing.T) {
	text := "The evidence favors the plaintiff on this issue."
	once := guardrail.RequireDisclaimer(text)
	twice := guardrail.RequireDisclaimer(once)

	if once != twice {
		t.Fatalf("RequireDisclaimer is not idempotent:\nonce:  %q\ntwice: %q", once, twice)
	}
}

func TestEnsureDisclaimerMatchesRequireDisclaimer(t *testing.T) {
	text := "The rule, if applied, would favor the defendant."
	if got, want := guardrail.EnsureDisclaimer(text), guardrail.RequireDisclaimer(text); got != want {
		t.Fatalf("EnsureDisclaimer(%q) = %q, want %q (must match RequireDisclaimer)", text, got, want)
	}
}

func TestHasDisclaimerFalseForPlainText(t *testing.T) {
	if guardrail.HasDisclaimer("plain text with no disclaimer") {
		t.Fatal("HasDisclaimer(plain text) = true, want false")
	}
}

func TestRequireDisclaimerEmptyText(t *testing.T) {
	out := guardrail.RequireDisclaimer("")
	if !guardrail.HasDisclaimer(out) {
		t.Fatal("RequireDisclaimer(\"\") did not attach the disclaimer")
	}
	// Idempotent even from empty.
	if again := guardrail.RequireDisclaimer(out); again != out {
		t.Fatalf("RequireDisclaimer not idempotent from empty start:\nfirst: %q\nagain: %q", out, again)
	}
}
