package setup

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// transition attempts to move w.State to next, returning [ErrInvalidTransition]
// when the move is not permitted.
func transition(w *SetupWizard, next SetupState) error {
	if !w.State.CanTransitionTo(next) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, w.State, next)
	}
	w.State = next
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// StepSelectJurisdiction advances the wizard from [StateInProgress] to
// [StateJurisdictionSelected] and records the chosen jurisdiction UUID.
//
// Returns [ErrSetupLocked] if the wizard is already locked.
// Returns [ErrInvalidTransition] if the wizard is not in the expected state.
func StepSelectJurisdiction(w *SetupWizard, jurisdictionID uuid.UUID) error {
	if w.State == StateLocked {
		return ErrSetupLocked
	}
	if err := transition(w, StateJurisdictionSelected); err != nil {
		return err
	}
	jid := jurisdictionID
	w.JurisdictionID = &jid
	return nil
}

// StepSelectCourt advances the wizard from [StateJurisdictionSelected] to
// [StateCourtSelected] and records the chosen court level string (e.g.
// "supreme", "appellate", "trial").
func StepSelectCourt(w *SetupWizard, level string) error {
	if w.State == StateLocked {
		return ErrSetupLocked
	}
	if err := transition(w, StateCourtSelected); err != nil {
		return err
	}
	l := level
	w.CourtLevel = &l
	return nil
}

// StepSelectLanguages advances the wizard from [StateCourtSelected] to
// [StateLanguageSelected] and records the chosen BCP-47 language codes.
//
// Returns [ErrMissingLanguages] when langs is empty.
func StepSelectLanguages(w *SetupWizard, langs []string) error {
	if w.State == StateLocked {
		return ErrSetupLocked
	}
	if len(langs) == 0 {
		return ErrMissingLanguages
	}
	if err := transition(w, StateLanguageSelected); err != nil {
		return err
	}
	cp := make([]string, len(langs))
	copy(cp, langs)
	w.Languages = cp
	return nil
}

// StepConfigureProvider advances the wizard from [StateLanguageSelected] to
// [StateProviderConfigured] and records the provider stub.
func StepConfigureProvider(w *SetupWizard, cfg ProviderConfigStub) error {
	if w.State == StateLocked {
		return ErrSetupLocked
	}
	if err := transition(w, StateProviderConfigured); err != nil {
		return err
	}
	stub := cfg
	stub.ConfiguredAt = time.Now().UTC()
	w.ProviderConfig = &stub
	return nil
}

// StepComplete advances the wizard from [StateProviderConfigured] to
// [StateCompleted] and records the completion timestamp.
//
// Returns [ErrMissingJurisdiction] when no jurisdiction has been recorded.
// Returns [ErrMissingLanguages] when no languages have been recorded.
func StepComplete(w *SetupWizard) error {
	if w.State == StateLocked {
		return ErrSetupLocked
	}
	if w.JurisdictionID == nil {
		return ErrMissingJurisdiction
	}
	if len(w.Languages) == 0 {
		return ErrMissingLanguages
	}
	if err := transition(w, StateCompleted); err != nil {
		return err
	}
	now := time.Now().UTC()
	w.CompletedAt = &now
	return nil
}

// StepLock advances the wizard from [StateCompleted] to [StateLocked] and
// records the lock timestamp.  After this call, any step function returns
// [ErrSetupLocked].
func StepLock(w *SetupWizard) error {
	if w.State == StateLocked {
		return ErrSetupLocked
	}
	if err := transition(w, StateLocked); err != nil {
		return err
	}
	now := time.Now().UTC()
	w.LockedAt = &now
	return nil
}
