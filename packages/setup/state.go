package setup

// SetupState represents the current stage of a tenant's setup wizard.
type SetupState string

const (
	// StatePending is the initial state before any wizard interaction.
	StatePending SetupState = "pending"

	// StateInProgress indicates the wizard has been started but no step is
	// complete yet.
	StateInProgress SetupState = "in_progress"

	// StateJurisdictionSelected indicates the tenant has chosen a jurisdiction.
	StateJurisdictionSelected SetupState = "jurisdiction_selected"

	// StateCourtSelected indicates the court level has been chosen.
	StateCourtSelected SetupState = "court_selected"

	// StateLanguageSelected indicates one or more reasoning languages have been
	// chosen.
	StateLanguageSelected SetupState = "language_selected"

	// StateProviderConfigured indicates the AI inference provider stub has been
	// configured.
	StateProviderConfigured SetupState = "provider_configured"

	// StateCompleted indicates all required steps are done.  The wizard is
	// finalised but not yet locked against future modification.
	StateCompleted SetupState = "completed"

	// StateLocked is the terminal state.  The wizard is immutable; any further
	// step attempts return [ErrSetupLocked].
	StateLocked SetupState = "locked"
)

// SetupStep is a human-readable label for each transition in the wizard flow.
type SetupStep string

const (
	StepLabelStart             SetupStep = "start"
	StepLabelJurisdiction      SetupStep = "select_jurisdiction"
	StepLabelCourt             SetupStep = "select_court"
	StepLabelLanguages         SetupStep = "select_languages"
	StepLabelProviderConfigure SetupStep = "configure_provider"
	StepLabelComplete          SetupStep = "complete"
	StepLabelLock              SetupStep = "lock"
)

// validTransitions maps each state to the set of states it may transition into.
var validTransitions = map[SetupState][]SetupState{
	StatePending:              {StateInProgress},
	StateInProgress:           {StateJurisdictionSelected},
	StateJurisdictionSelected: {StateCourtSelected},
	StateCourtSelected:        {StateLanguageSelected},
	StateLanguageSelected:     {StateProviderConfigured},
	StateProviderConfigured:   {StateCompleted},
	StateCompleted:            {StateLocked},
	StateLocked:               {}, // terminal – no outgoing edges
}

// IsTerminal returns true when the state has no valid outgoing transitions.
func (s SetupState) IsTerminal() bool {
	targets, ok := validTransitions[s]
	return ok && len(targets) == 0
}

// CanTransitionTo reports whether transitioning from s to next is permitted by
// the wizard's state machine.
func (s SetupState) CanTransitionTo(next SetupState) bool {
	targets, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == next {
			return true
		}
	}
	return false
}
