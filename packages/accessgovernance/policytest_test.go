package accessgovernance_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestTestPolicy_PredictsAllowAndDenyScenarios proves task 8: a real,
// working dry-run harness that predicts allow/deny for constructed
// scenarios against a candidate Policy, without ever activating it
// through a PolicyRepository.
func TestTestPolicy_PredictsAllowAndDenyScenarios(t *testing.T) {
	tenantID := uuid.New()
	candidate := accessgovernance.Policy{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     "candidate-judge-signoff",
		Active:   false, // TestPolicy should still evaluate it
		Rules: []accessgovernance.PolicyRule{
			{
				Roles:         []identity.Role{identity.RoleJudge},
				Actions:       []accessgovernance.Action{"case:sign_off"},
				Jurisdictions: []string{"dubai"},
				Effect:        accessgovernance.EffectAllow,
			},
		},
	}

	scenarios := []accessgovernance.Scenario{
		{
			Name: "judge signs off in dubai",
			Request: accessgovernance.Request{
				ActorUserID:    uuid.New(),
				ActorRoles:     []identity.Role{identity.RoleJudge},
				TenantID:       tenantID,
				Action:         "case:sign_off",
				JurisdictionID: "dubai",
			},
			WantEffect: accessgovernance.EffectAllow,
		},
		{
			Name: "judge signs off outside dubai",
			Request: accessgovernance.Request{
				ActorUserID:    uuid.New(),
				ActorRoles:     []identity.Role{identity.RoleJudge},
				TenantID:       tenantID,
				Action:         "case:sign_off",
				JurisdictionID: "abu-dhabi",
			},
			WantEffect: accessgovernance.EffectDeny,
		},
		{
			Name: "clerk cannot sign off",
			Request: accessgovernance.Request{
				ActorUserID:    uuid.New(),
				ActorRoles:     []identity.Role{identity.RoleClerk},
				TenantID:       tenantID,
				Action:         "case:sign_off",
				JurisdictionID: "dubai",
			},
			WantEffect: accessgovernance.EffectDeny,
		},
	}

	results, err := accessgovernance.TestPolicy(t.Context(), candidate, scenarios)
	if err != nil {
		t.Fatalf("TestPolicy: %v", err)
	}
	if !results.AllPassed() {
		t.Fatalf("TestPolicy() results = %+v, want all scenarios to pass", results)
	}
	if results.Total != 3 || results.Passed != 3 || results.Failed != 0 {
		t.Fatalf("TestPolicy() totals = %+v, want Total=3 Passed=3 Failed=0", results)
	}
}

// TestTestPolicy_ReportsMismatchAsFailed proves TestPolicy actually
// evaluates the policy rather than trivially agreeing with every
// expectation: a scenario whose WantEffect contradicts what the
// policy produces is reported as failed, not silently passed.
func TestTestPolicy_ReportsMismatchAsFailed(t *testing.T) {
	tenantID := uuid.New()
	candidate := accessgovernance.Policy{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     "candidate",
		Rules: []accessgovernance.PolicyRule{
			{
				Roles:   []identity.Role{identity.RoleAdmin},
				Actions: []accessgovernance.Action{"case:delete"},
				Effect:  accessgovernance.EffectAllow,
			},
		},
	}

	scenarios := []accessgovernance.Scenario{
		{
			Name: "admin delete, wrongly expected to be denied",
			Request: accessgovernance.Request{
				ActorUserID: uuid.New(),
				ActorRoles:  []identity.Role{identity.RoleAdmin},
				TenantID:    tenantID,
				Action:      "case:delete",
			},
			WantEffect: accessgovernance.EffectDeny,
		},
	}

	results, err := accessgovernance.TestPolicy(t.Context(), candidate, scenarios)
	if err != nil {
		t.Fatalf("TestPolicy: %v", err)
	}
	if results.AllPassed() {
		t.Fatal("TestPolicy() should report the mismatched scenario as failed")
	}
	if results.Failed != 1 {
		t.Fatalf("results.Failed = %d, want 1", results.Failed)
	}
	if results.Cases[0].Got != accessgovernance.EffectAllow {
		t.Fatalf("results.Cases[0].Got = %v, want EffectAllow", results.Cases[0].Got)
	}
}

// TestTestPolicy_RejectsInvalidPolicy proves TestPolicy validates the
// candidate Policy before dry-running anything against it.
func TestTestPolicy_RejectsInvalidPolicy(t *testing.T) {
	_, err := accessgovernance.TestPolicy(t.Context(), accessgovernance.Policy{}, nil)
	if err == nil {
		t.Fatal("TestPolicy() with an empty/invalid policy should error")
	}
}

// TestTestPolicy_TimeOfDayScenario exercises the time-of-day
// dimension through the harness end to end.
func TestTestPolicy_TimeOfDayScenario(t *testing.T) {
	tenantID := uuid.New()
	candidate := accessgovernance.Policy{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     "business-hours-only",
		Rules: []accessgovernance.PolicyRule{
			{
				Roles:      []identity.Role{identity.RoleClerk},
				Actions:    []accessgovernance.Action{"hearing:schedule"},
				TimeWindow: accessgovernance.TimeWindow{Enabled: true, StartHour: 9, EndHour: 17},
				Effect:     accessgovernance.EffectAllow,
			},
		},
	}

	scenarios := []accessgovernance.Scenario{
		{
			Name: "during business hours",
			Request: accessgovernance.Request{
				ActorUserID: uuid.New(),
				ActorRoles:  []identity.Role{identity.RoleClerk},
				TenantID:    tenantID,
				Action:      "hearing:schedule",
				Now:         time.Date(2026, 1, 5, 11, 0, 0, 0, time.UTC),
			},
			WantEffect: accessgovernance.EffectAllow,
		},
		{
			Name: "after hours",
			Request: accessgovernance.Request{
				ActorUserID: uuid.New(),
				ActorRoles:  []identity.Role{identity.RoleClerk},
				TenantID:    tenantID,
				Action:      "hearing:schedule",
				Now:         time.Date(2026, 1, 5, 23, 0, 0, 0, time.UTC),
			},
			WantEffect: accessgovernance.EffectDeny,
		},
	}

	results, err := accessgovernance.TestPolicy(t.Context(), candidate, scenarios)
	if err != nil {
		t.Fatalf("TestPolicy: %v", err)
	}
	if !results.AllPassed() {
		t.Fatalf("TestPolicy() results = %+v, want all scenarios to pass", results)
	}
}
