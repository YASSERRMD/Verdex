package accessgovernance_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestEngine_Evaluate_AllowsMatchingPolicyRule covers the basic
// attribute-policy allow path: a rule scoped to a role and action
// matches and Evaluate returns Allow.
func TestEngine_Evaluate_AllowsMatchingPolicyRule(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	policy := activePolicy(tenantID, accessgovernance.PolicyRule{
		Roles:   []identity.Role{identity.RoleAdmin},
		Actions: []accessgovernance.Action{"case:view"},
		Effect:  accessgovernance.EffectAllow,
	})
	if _, err := engine.CreatePolicy(ctx, tenantID, policy); err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	dec, err := engine.Evaluate(ctx, accessgovernance.Request{
		ActorUserID: admin.ID,
		ActorRoles:  admin.Roles,
		TenantID:    tenantID,
		Action:      "case:view",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed() {
		t.Fatalf("Evaluate() effect = %v, want Allow", dec.Effect)
	}
}

// TestEngine_Evaluate_DeniesWhenNoRuleMatches proves Evaluate fails
// closed: if no active Policy rule matches, the request is denied.
func TestEngine_Evaluate_DeniesWhenNoRuleMatches(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	advocate := newTestUser(tenantID, identity.RoleAdvocate)
	ctx := ctxWithUser(advocate)

	dec, err := engine.Evaluate(ctx, accessgovernance.Request{
		ActorUserID: advocate.ID,
		ActorRoles:  advocate.Roles,
		TenantID:    tenantID,
		Action:      "case:delete",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed() {
		t.Fatal("Evaluate() with no matching policy should deny, got allow")
	}
}

// TestEngine_Evaluate_ExplicitDenyRuleWins proves a Deny-effect rule
// takes precedence when it's the first match.
func TestEngine_Evaluate_ExplicitDenyRuleWins(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	clerk := newTestUser(tenantID, identity.RoleClerk)

	policy := activePolicy(tenantID,
		accessgovernance.PolicyRule{
			Roles:   []identity.Role{identity.RoleClerk},
			Actions: []accessgovernance.Action{"case:view"},
			Effect:  accessgovernance.EffectDeny,
		},
	)
	if _, err := engine.CreatePolicy(ctxWithUser(admin), tenantID, policy); err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	dec, err := engine.Evaluate(ctxWithUser(clerk), accessgovernance.Request{
		ActorUserID: clerk.ID,
		ActorRoles:  clerk.Roles,
		TenantID:    tenantID,
		Action:      "case:view",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed() {
		t.Fatal("Evaluate() should deny under explicit deny rule")
	}
}

// TestEngine_Evaluate_JurisdictionScopedRule proves attribute
// evaluation reasons over the resource's jurisdiction, not just role.
func TestEngine_Evaluate_JurisdictionScopedRule(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	policy := activePolicy(tenantID, accessgovernance.PolicyRule{
		Roles:         []identity.Role{identity.RoleJudge},
		Actions:       []accessgovernance.Action{"case:sign_off"},
		Jurisdictions: []string{"dubai"},
		Effect:        accessgovernance.EffectAllow,
	})
	if _, err := engine.CreatePolicy(ctxWithUser(admin), tenantID, policy); err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	allowed, err := engine.Evaluate(ctx, accessgovernance.Request{
		ActorUserID:    judge.ID,
		ActorRoles:     judge.Roles,
		TenantID:       tenantID,
		Action:         "case:sign_off",
		JurisdictionID: "dubai",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !allowed.Allowed() {
		t.Fatal("Evaluate() in matching jurisdiction should allow")
	}

	denied, err := engine.Evaluate(ctx, accessgovernance.Request{
		ActorUserID:    judge.ID,
		ActorRoles:     judge.Roles,
		TenantID:       tenantID,
		Action:         "case:sign_off",
		JurisdictionID: "abu-dhabi",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if denied.Allowed() {
		t.Fatal("Evaluate() in non-matching jurisdiction should deny")
	}
}

// TestEngine_Evaluate_TimeOfDayWindow proves a PolicyRule's TimeWindow
// is actually consulted (task 1's "time-of-day/expiry" attribute).
func TestEngine_Evaluate_TimeOfDayWindow(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	policy := activePolicy(tenantID, accessgovernance.PolicyRule{
		Roles:   []identity.Role{identity.RoleAdmin},
		Actions: []accessgovernance.Action{"settings:manage"},
		TimeWindow: accessgovernance.TimeWindow{
			Enabled:   true,
			StartHour: 9,
			EndHour:   17,
		},
		Effect: accessgovernance.EffectAllow,
	})
	if _, err := engine.CreatePolicy(ctx, tenantID, policy); err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	inWindow := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	dec, err := engine.Evaluate(ctx, accessgovernance.Request{
		ActorUserID: admin.ID,
		ActorRoles:  admin.Roles,
		TenantID:    tenantID,
		Action:      "settings:manage",
		Now:         inWindow,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed() {
		t.Fatal("Evaluate() within time window should allow")
	}

	outOfWindow := time.Date(2026, 7, 4, 22, 0, 0, 0, time.UTC)
	dec, err = engine.Evaluate(ctx, accessgovernance.Request{
		ActorUserID: admin.ID,
		ActorRoles:  admin.Roles,
		TenantID:    tenantID,
		Action:      "settings:manage",
		Now:         outOfWindow,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed() {
		t.Fatal("Evaluate() outside time window should deny")
	}
}

// TestEngine_Evaluate_RequiresAuthenticatedActor proves Evaluate
// rejects a context with no identity.User.
func TestEngine_Evaluate_RequiresAuthenticatedActor(t *testing.T) {
	engine, tenantID := newTestEngine(t)

	_, err := engine.Evaluate(t.Context(), accessgovernance.Request{
		ActorUserID: uuid.New(),
		TenantID:    tenantID,
		Action:      "case:view",
	})
	if !errors.Is(err, accessgovernance.ErrUnauthenticated) {
		t.Fatalf("Evaluate() error = %v, want ErrUnauthenticated", err)
	}
}

// TestEngine_Evaluate_CrossTenantRequestRejected proves an actor from
// tenant A can never evaluate a Request scoped to tenant B, even one
// they construct themselves.
func TestEngine_Evaluate_CrossTenantRequestRejected(t *testing.T) {
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	admin := newTestUser(tenantA, identity.RoleAdmin)

	_, err := engine.Evaluate(ctxWithUser(admin), accessgovernance.Request{
		ActorUserID: admin.ID,
		TenantID:    tenantB,
		Action:      "case:view",
	})
	if !errors.Is(err, accessgovernance.ErrCrossTenantAccess) {
		t.Fatalf("Evaluate() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}
