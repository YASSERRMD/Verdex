package setup_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/setup"
)

func TestSetupState_IsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state    setup.SetupState
		terminal bool
	}{
		{setup.StatePending, false},
		{setup.StateInProgress, false},
		{setup.StateJurisdictionSelected, false},
		{setup.StateCourtSelected, false},
		{setup.StateLanguageSelected, false},
		{setup.StateProviderConfigured, false},
		{setup.StateCompleted, false},
		{setup.StateLocked, true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.state), func(t *testing.T) {
			t.Parallel()
			if got := tc.state.IsTerminal(); got != tc.terminal {
				t.Errorf("state %q: IsTerminal() = %v, want %v", tc.state, got, tc.terminal)
			}
		})
	}
}

func TestSetupState_CanTransitionTo(t *testing.T) {
	t.Parallel()

	type tc struct {
		from  setup.SetupState
		to    setup.SetupState
		allow bool
	}

	tests := []tc{
		// Valid forward transitions.
		{setup.StatePending, setup.StateInProgress, true},
		{setup.StateInProgress, setup.StateJurisdictionSelected, true},
		{setup.StateJurisdictionSelected, setup.StateCourtSelected, true},
		{setup.StateCourtSelected, setup.StateLanguageSelected, true},
		{setup.StateLanguageSelected, setup.StateProviderConfigured, true},
		{setup.StateProviderConfigured, setup.StateCompleted, true},
		{setup.StateCompleted, setup.StateLocked, true},

		// Invalid – skip a step.
		{setup.StatePending, setup.StateJurisdictionSelected, false},
		{setup.StateInProgress, setup.StateCourtSelected, false},
		{setup.StateJurisdictionSelected, setup.StateLanguageSelected, false},
		{setup.StateCourtSelected, setup.StateProviderConfigured, false},
		{setup.StateLanguageSelected, setup.StateCompleted, false},
		{setup.StateProviderConfigured, setup.StateLocked, false},

		// Invalid – backwards transitions.
		{setup.StateInProgress, setup.StatePending, false},
		{setup.StateJurisdictionSelected, setup.StateInProgress, false},
		{setup.StateCourtSelected, setup.StateJurisdictionSelected, false},
		{setup.StateLanguageSelected, setup.StateCourtSelected, false},
		{setup.StateProviderConfigured, setup.StateLanguageSelected, false},
		{setup.StateCompleted, setup.StateProviderConfigured, false},

		// Terminal state – no outgoing transitions.
		{setup.StateLocked, setup.StateLocked, false},
		{setup.StateLocked, setup.StateCompleted, false},
		{setup.StateLocked, setup.StatePending, false},
	}

	for _, tc := range tests {
		tc := tc
		name := string(tc.from) + "→" + string(tc.to)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := tc.from.CanTransitionTo(tc.to)
			if got != tc.allow {
				t.Errorf("CanTransitionTo(%q → %q) = %v, want %v", tc.from, tc.to, got, tc.allow)
			}
		})
	}
}
