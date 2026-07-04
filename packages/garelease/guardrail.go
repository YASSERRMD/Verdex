package garelease

import (
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// verdictLanguageFixture is a deliberately non-binding-guardrail-violating
// fixture: verdict/directive phrasing a real reasoning-output pipeline
// must never let through. VerifyGuardrails asserts
// guardrail.CheckText REJECTS this text -- if this fixture ever passed,
// the guardrail itself would have regressed, which is exactly what
// this dimension exists to catch before a release ships. Mirrors
// packages/pilot's compliance_test.go verdict-worded fixture and
// packages/securitytesting/selftest.go's
// VulnerableFixtureScenario/FixedFixtureScenario "harness that can
// fail" pattern.
const verdictLanguageFixture = "The court hereby ORDERS that the defendant shall pay damages of $50,000 within 30 days."

// properlyLabeledFixture is a properly-labeled, non-binding draft
// analysis: exactly the shape a real reasoning-output surface is
// expected to produce. VerifyGuardrails asserts this text -- once
// RequireDisclaimer has attached the mandatory disclaimer -- passes
// every guardrail check: CheckText does not flag it as verdict
// language, and HasDisclaimer confirms the mandatory disclaimer is
// present.
const properlyLabeledFixture = "Based on the evidence presented, the facts suggest a plausible claim for breach of contract, though additional discovery would help clarify the timeline of events."

// GuardrailCheckResult is the outcome of a single guardrail assertion
// VerifyGuardrails performed against one fixture, mirroring
// packages/iac.CheckResult's Kind/Name/Passed/Detail shape.
type GuardrailCheckResult struct {
	// Name identifies which specific guardrail assertion this result
	// reports (e.g. "verdict_language_rejected", "disclaimer_enforced").
	Name string `json:"name"`

	// Passed reports whether this specific assertion held.
	Passed bool `json:"passed"`

	// Detail explains the outcome, especially when Passed is false.
	Detail string `json:"detail,omitempty"`
}

// GuardrailVerification is the result of Engine.VerifyGuardrails: every
// individual GuardrailCheckResult plus an overall Passed aggregation,
// mirroring packages/iac.DeploymentVerificationReport's
// Passed()/Failures() shape.
type GuardrailVerification struct {
	Results     []GuardrailCheckResult `json:"results"`
	Passed      bool                   `json:"passed"`
	EvaluatedAt time.Time              `json:"evaluated_at"`
}

// Failures returns the subset of v.Results that did not pass.
func (v GuardrailVerification) Failures() []GuardrailCheckResult {
	out := make([]GuardrailCheckResult, 0)
	for _, r := range v.Results {
		if !r.Passed {
			out = append(out, r)
		}
	}
	return out
}

// VerifyGuardrails is task 6's guardrail half: a REAL harness making
// real calls into packages/guardrail's actual verdict-language/label
// functions against a deliberately-bad fixture (must fail) and a
// properly-labeled fixture (must pass) -- a harness that can fail, not
// a rubber stamp, mirroring packages/pilot.ValidateNonBindingCompliance's
// and packages/securitytesting/selftest.go's identical
// vulnerable-fixture/fixed-fixture proof pattern.
//
// Four assertions run, all four must hold for GuardrailVerification.Passed:
//
//  1. guardrail.CheckText(verdictLanguageFixture) returns a non-nil
//     error (the guardrail correctly rejects verdict/directive
//     phrasing).
//  2. guardrail.CheckText(properlyLabeledFixture) returns nil (the
//     guardrail does not false-positive on ordinary non-binding
//     analysis prose).
//  3. guardrail.HasDisclaimer on properlyLabeledFixture BEFORE
//     RequireDisclaimer is applied is false, and guardrail.RequireDisclaimer
//     followed by HasDisclaimer is true (disclaimer injection is real
//     and idempotent).
//  4. guardrail.ValidateLabeled against a value whose Label() is
//     guardrail.DraftAnalysisLabel passes, and against a value whose
//     Label() is some other string fails (the label check itself
//     discriminates, rather than always passing).
func (e *Engine) VerifyGuardrails() (GuardrailVerification, error) {
	now := e.now()
	results := make([]GuardrailCheckResult, 0, 4)

	// Assertion 1: the harness's own deliberately-bad fixture must be
	// rejected.
	if err := guardrail.CheckText(verdictLanguageFixture); err != nil {
		results = append(results, GuardrailCheckResult{
			Name:   "verdict_language_rejected",
			Passed: true,
			Detail: "deliberately verdict-worded fixture correctly rejected: " + err.Error(),
		})
	} else {
		results = append(results, GuardrailCheckResult{
			Name:   "verdict_language_rejected",
			Passed: false,
			Detail: "guardrail.CheckText did not reject a deliberately verdict-worded fixture -- the guardrail has regressed",
		})
	}

	// Assertion 2: the harness's own properly-labeled fixture must pass
	// through CheckText cleanly (no false positive on ordinary
	// non-binding analysis prose).
	if err := guardrail.CheckText(properlyLabeledFixture); err == nil {
		results = append(results, GuardrailCheckResult{
			Name:   "non_binding_prose_accepted",
			Passed: true,
			Detail: "properly-labeled non-binding analysis fixture correctly passed verdict-language check",
		})
	} else {
		results = append(results, GuardrailCheckResult{
			Name:   "non_binding_prose_accepted",
			Passed: false,
			Detail: "guardrail.CheckText false-positived on an ordinary non-binding analysis fixture: " + err.Error(),
		})
	}

	// Assertion 3: disclaimer enforcement is real (absent before, present
	// and idempotent after RequireDisclaimer).
	beforeHasDisclaimer := guardrail.HasDisclaimer(properlyLabeledFixture)
	disclaimed := guardrail.RequireDisclaimer(properlyLabeledFixture)
	afterHasDisclaimer := guardrail.HasDisclaimer(disclaimed)
	idempotent := guardrail.RequireDisclaimer(disclaimed) == disclaimed
	if !beforeHasDisclaimer && afterHasDisclaimer && idempotent {
		results = append(results, GuardrailCheckResult{
			Name:   "disclaimer_enforced",
			Passed: true,
			Detail: "RequireDisclaimer correctly attaches the mandatory disclaimer and is idempotent",
		})
	} else {
		results = append(results, GuardrailCheckResult{
			Name:   "disclaimer_enforced",
			Passed: false,
			Detail: fmt.Sprintf("disclaimer enforcement did not behave as expected (before=%v after=%v idempotent=%v)", beforeHasDisclaimer, afterHasDisclaimer, idempotent),
		})
	}

	// Assertion 4: label validation discriminates a correctly-labeled
	// value from an incorrectly-labeled one.
	goodLabelErr := guardrail.ValidateLabeled(fixedLabel(string(guardrail.DraftAnalysisLabel)))
	badLabelErr := guardrail.ValidateLabeled(fixedLabel("final_verdict"))
	if goodLabelErr == nil && badLabelErr != nil {
		results = append(results, GuardrailCheckResult{
			Name:   "label_discriminates",
			Passed: true,
			Detail: "ValidateLabeled passes the mandatory draft_analysis label and rejects an arbitrary label",
		})
	} else {
		results = append(results, GuardrailCheckResult{
			Name:   "label_discriminates",
			Passed: false,
			Detail: fmt.Sprintf("label validation did not discriminate correctly (good_label_err=%v bad_label_err=%v)", goodLabelErr, badLabelErr),
		})
	}

	passed := true
	for _, r := range results {
		if !r.Passed {
			passed = false
			break
		}
	}

	return GuardrailVerification{Results: results, Passed: passed, EvaluatedAt: now}, nil
}

// fixedLabel is a minimal guardrail.Labeled implementation returning a
// fixed string, used only by VerifyGuardrails's assertion 4 to exercise
// ValidateLabeled against both a correct and an incorrect label without
// constructing a real packages/irac.ConclusionNode.
type fixedLabel string

// Label implements guardrail.Labeled.
func (f fixedLabel) Label() string { return string(f) }

// checkGuardrailIntegrity adapts VerifyGuardrails into a ReadinessCheck
// for DimensionGuardrailIntegrity, called from CheckReadiness.
func (e *Engine) checkGuardrailIntegrity(now time.Time) (ReadinessCheck, error) {
	verification, err := e.VerifyGuardrails()
	if err != nil {
		return ReadinessCheck{}, wrapf("checkGuardrailIntegrity", err)
	}

	if verification.Passed {
		return ReadinessCheck{
			Dimension:   DimensionGuardrailIntegrity,
			Status:      CheckPassed,
			Detail:      fmt.Sprintf("all %d guardrail assertions held", len(verification.Results)),
			EvaluatedAt: now,
		}, nil
	}

	failures := verification.Failures()
	detail := fmt.Sprintf("%d of %d guardrail assertions failed:", len(failures), len(verification.Results))
	for _, f := range failures {
		detail = fmt.Sprintf("%s %s", detail, f.Name)
	}
	return ReadinessCheck{
		Dimension:   DimensionGuardrailIntegrity,
		Status:      CheckFailed,
		Detail:      detail,
		EvaluatedAt: now,
	}, nil
}
