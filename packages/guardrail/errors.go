package guardrail

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrMissingLabel is returned by RequireLabel/ValidateLabeled when a
	// reasoning output does not carry the mandatory Label
	// (draft_analysis).
	ErrMissingLabel = errors.New("guardrail: reasoning output missing mandatory draft_analysis label")

	// ErrVerdictLanguageDetected is returned by CheckText when the
	// supplied text contains verdict or directive phrasing.
	ErrVerdictLanguageDetected = errors.New("guardrail: text contains verdict or directive language")

	// ErrSignoffNotApproved is returned by CanFinalize when the
	// configured SignoffGate reports any status other than
	// SignoffApproved for the case.
	ErrSignoffNotApproved = errors.New("guardrail: case does not have an approved human sign-off")

	// ErrNilSignoffGate is returned by CanFinalize when called with a
	// nil SignoffGate — finalization must never proceed with no gate to
	// consult at all.
	ErrNilSignoffGate = errors.New("guardrail: signoff gate must not be nil")

	// ErrEmptyCaseID is returned by CanFinalize and NoSignoffRecordedGate
	// when called with an empty case ID.
	ErrEmptyCaseID = errors.New("guardrail: case id is required")
)
