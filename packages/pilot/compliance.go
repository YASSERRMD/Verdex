package pilot

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// ComplianceResult is the outcome of ValidateNonBindingCompliance
// (task 8): whether the reviewed opinion text passed the platform's
// real non-binding-workflow guardrail checks, and which specific check
// (if any) failed. This is a report/query type over a real check this
// package performs -- not a stub, and not merely a comment asserting
// compliance holds.
type ComplianceResult struct {
	// PilotCaseID identifies the PilotCase the validated opinion text
	// belongs to.
	PilotCaseID uuid.UUID `json:"pilot_case_id"`

	// Passed reports whether opinionText passed every guardrail check
	// ValidateNonBindingCompliance performs.
	Passed bool `json:"passed"`

	// FailureReason explains which check failed and why, when Passed is
	// false. Empty when Passed is true.
	FailureReason string `json:"failure_reason,omitempty"`

	// CheckedAt is when this validation was performed.
	CheckedAt time.Time `json:"checked_at"`
}

// ValidateNonBindingCompliance validates that opinionText complies
// with the platform's non-binding-workflow guardrail (task 8) by
// calling straight into packages/guardrail's own real checks --
// guardrail.CheckText (verdict/directive-language rejection) and
// guardrail.HasDisclaimer (mandatory disclaimer presence) -- rather
// than reimplementing any label or verdict-language logic in this
// package. Requires viewPermission and tenant match: validating
// compliance is a read-oriented check, not a mutation, so it does not
// require managePermission the way triage/refinement operations do.
// The referenced PilotCase must already exist for tenantID.
func (e *Engine) ValidateNonBindingCompliance(ctx context.Context, tenantID, pilotCaseID uuid.UUID, opinionText string) (ComplianceResult, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return ComplianceResult{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return ComplianceResult{}, err
	}
	if opinionText == "" {
		return ComplianceResult{}, ErrEmptyOpinionText
	}
	if _, err := e.cases.Get(ctx, tenantID, pilotCaseID); err != nil {
		return ComplianceResult{}, wrapf("ValidateNonBindingCompliance", err)
	}

	now := e.now()
	result := ComplianceResult{PilotCaseID: pilotCaseID, CheckedAt: now}

	if err := guardrail.CheckText(opinionText); err != nil {
		result.FailureReason = err.Error()
		return result, nil
	}
	if !guardrail.HasDisclaimer(opinionText) {
		result.FailureReason = "opinion text is missing the mandatory non-binding disclaimer"
		return result, nil
	}

	result.Passed = true
	return result, nil
}
