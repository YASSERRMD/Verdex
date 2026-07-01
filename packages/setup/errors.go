package setup

import "errors"

// Domain errors for the setup wizard.  All public errors are sentinel values
// that can be matched with [errors.Is].
var (
	// ErrSetupNotFound is returned when no wizard record exists for a tenant.
	ErrSetupNotFound = errors.New("setup: wizard not found")

	// ErrSetupLocked is returned when an operation is attempted on a wizard
	// that has already reached [StateLocked].
	ErrSetupLocked = errors.New("setup: wizard is locked and cannot be modified")

	// ErrSetupAlreadyComplete is returned when StartSetup or a step function is
	// called on a wizard that has already completed.
	ErrSetupAlreadyComplete = errors.New("setup: wizard has already been completed")

	// ErrInvalidTransition is returned (wrapped) when a requested state
	// transition is not permitted by the state machine.
	ErrInvalidTransition = errors.New("setup: invalid state transition")

	// ErrMissingJurisdiction is returned when StepComplete is called before a
	// jurisdiction has been selected.
	ErrMissingJurisdiction = errors.New("setup: jurisdiction must be selected before completing setup")

	// ErrMissingLanguages is returned when StepSelectLanguages is called with
	// an empty slice, or when StepComplete is called before any language has
	// been selected.
	ErrMissingLanguages = errors.New("setup: at least one language must be selected")
)
