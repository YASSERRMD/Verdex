package pilot_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/pilot"
)

// properlyLabeledDraftAnalysis is a realistic, correctly-labeled
// draft-analysis opinion fixture: it reasons about the pilot case
// without ever using verdict/directive language, and ends with the
// mandatory non-binding disclaimer via guardrail.RequireDisclaimer --
// exactly what a compliant pilot-case opinion should look like.
var properlyLabeledDraftAnalysis = guardrail.RequireDisclaimer(
	`Draft analysis: based on the submitted contract and correspondence,
the claimant's assertion of breach appears reasonably well-grounded in
the identified clauses. The respondent's force-majeure argument raises
a plausible counter-consideration that a qualified legal professional
should weigh against the specific contractual language and applicable
precedent.`,
)

// verdictWordedFixture is a deliberately verdict-worded fixture --
// exactly the kind of output the non-binding-workflow guardrail exists
// to catch before it reaches a human, a filing, or a downstream
// system. This mirrors Phase 086's security-testing harness
// principle: a test proving a check actually rejects a bad input, not
// merely a comment asserting compliance holds.
const verdictWordedFixture = "The court hereby rules that the defendant is guilty and orders immediate payment of damages."

func TestEngine_ValidateNonBindingCompliance_FlagsVerdictLanguage(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	admin := adminUser(te.tenantID)

	result, err := te.engine.ValidateNonBindingCompliance(ctxWithUser(admin), te.tenantID, pc.ID, verdictWordedFixture)
	if err != nil {
		t.Fatalf("ValidateNonBindingCompliance: %v", err)
	}
	if result.Passed {
		t.Fatal("Passed should be false: the fixture is deliberately verdict-worded")
	}
	if result.FailureReason == "" {
		t.Fatal("FailureReason should explain why the check failed")
	}
}

func TestEngine_ValidateNonBindingCompliance_PassesProperlyLabeledDraft(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	admin := adminUser(te.tenantID)

	result, err := te.engine.ValidateNonBindingCompliance(ctxWithUser(admin), te.tenantID, pc.ID, properlyLabeledDraftAnalysis)
	if err != nil {
		t.Fatalf("ValidateNonBindingCompliance: %v", err)
	}
	if !result.Passed {
		t.Fatalf("Passed should be true for a properly labeled draft analysis, got FailureReason=%q", result.FailureReason)
	}
	if result.FailureReason != "" {
		t.Fatalf("FailureReason should be empty on pass, got %q", result.FailureReason)
	}
}

func TestEngine_ValidateNonBindingCompliance_FlagsMissingDisclaimer(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	admin := adminUser(te.tenantID)

	// Same reasoning-quality text as properlyLabeledDraftAnalysis, but
	// with the mandatory disclaimer stripped off, proving this check
	// verifies disclaimer presence independently of verdict-language
	// rejection.
	noDisclaimer := "Draft analysis: the claimant's assertion of breach appears reasonably well-grounded in the identified clauses."

	result, err := te.engine.ValidateNonBindingCompliance(ctxWithUser(admin), te.tenantID, pc.ID, noDisclaimer)
	if err != nil {
		t.Fatalf("ValidateNonBindingCompliance: %v", err)
	}
	if result.Passed {
		t.Fatal("Passed should be false: disclaimer is missing")
	}
}

func TestEngine_ValidateNonBindingCompliance_RejectsEmptyText(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	admin := adminUser(te.tenantID)

	_, err := te.engine.ValidateNonBindingCompliance(ctxWithUser(admin), te.tenantID, pc.ID, "")
	if !errors.Is(err, pilot.ErrEmptyOpinionText) {
		t.Fatalf("error = %v, want ErrEmptyOpinionText", err)
	}
}

func TestEngine_ValidateNonBindingCompliance_RequiresExistingCase(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	admin := adminUser(te.tenantID)

	_, err := te.engine.ValidateNonBindingCompliance(ctxWithUser(admin), te.tenantID, uuid.New(), properlyLabeledDraftAnalysis)
	if !errors.Is(err, pilot.ErrCaseNotFound) {
		t.Fatalf("error = %v, want ErrCaseNotFound", err)
	}
}
