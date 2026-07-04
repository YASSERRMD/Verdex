package accessgovernance_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestEngine_GrantCaseAccess_OverridesDefaultScoping proves a CaseGrant
// can allow an actor who holds no ordinary role-level permission for
// the action (e.g. an external reviewer) to access one specific case
// (task 2).
func TestEngine_GrantCaseAccess_OverridesDefaultScoping(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	// externalReviewer holds no role at all -- an ordinary
	// identity.HasPermission check would always fail for them.
	externalReviewer := newTestUser(tenantID)
	caseID := uuid.New()

	_, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: externalReviewer.ID,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GrantCaseAccess: %v", err)
	}

	dec, err := engine.Evaluate(ctxWithUser(externalReviewer), accessgovernance.Request{
		ActorUserID: externalReviewer.ID,
		TenantID:    tenantID,
		CaseID:      caseID,
		Action:      "case:view",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed() {
		t.Fatalf("Evaluate() with case grant = %v, want Allow", dec.Effect)
	}
	if dec.MatchedGrantID == uuid.Nil {
		t.Error("Decision.MatchedGrantID should be set when a case grant produced the decision")
	}

	// A different case (no grant) must still deny.
	otherCase := uuid.New()
	dec, err = engine.Evaluate(ctxWithUser(externalReviewer), accessgovernance.Request{
		ActorUserID: externalReviewer.ID,
		TenantID:    tenantID,
		CaseID:      otherCase,
		Action:      "case:view",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed() {
		t.Fatal("Evaluate() on a case with no grant should deny")
	}
}

// TestEngine_GrantCaseAccess_CanRestrictBeyondRole proves a Deny
// CaseGrant narrows access below what a role would otherwise permit
// (task 2's "or restricting it").
func TestEngine_GrantCaseAccess_CanRestrictBeyondRole(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	advocate := newTestUser(tenantID, identity.RoleAdvocate)
	caseID := uuid.New()

	// Ordinarily an advocate has case:view via the role matrix; a
	// Policy granting it broadly is set up here to isolate the
	// restriction behavior to the CaseGrant itself.
	policy := activePolicy(tenantID, accessgovernance.PolicyRule{
		Roles:   []identity.Role{identity.RoleAdvocate},
		Actions: []accessgovernance.Action{"case:view"},
		Effect:  accessgovernance.EffectAllow,
	})
	if _, err := engine.CreatePolicy(ctxWithUser(admin), tenantID, policy); err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	if _, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: advocate.ID,
		Deny:          true,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("GrantCaseAccess (deny): %v", err)
	}

	dec, err := engine.Evaluate(ctxWithUser(advocate), accessgovernance.Request{
		ActorUserID: advocate.ID,
		ActorRoles:  advocate.Roles,
		TenantID:    tenantID,
		CaseID:      caseID,
		Action:      "case:view",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed() {
		t.Fatal("Evaluate() should deny once a restricting CaseGrant applies to this case")
	}

	// The same advocate on a different case (no restriction) still
	// gets the policy's Allow.
	otherCase := uuid.New()
	dec, err = engine.Evaluate(ctxWithUser(advocate), accessgovernance.Request{
		ActorUserID: advocate.ID,
		ActorRoles:  advocate.Roles,
		TenantID:    tenantID,
		CaseID:      otherCase,
		Action:      "case:view",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed() {
		t.Fatal("Evaluate() on an unrestricted case should still allow via policy")
	}
}

// TestEngine_GrantCaseAccess_ExpiredGrantRejected proves an expired
// CaseGrant no longer applies at evaluation time.
func TestEngine_GrantCaseAccess_ExpiredGrantRejected(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	reviewer := newTestUser(tenantID)
	caseID := uuid.New()

	if _, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: reviewer.ID,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("GrantCaseAccess: %v", err)
	}

	dec, err := engine.Evaluate(ctxWithUser(reviewer), accessgovernance.Request{
		ActorUserID: reviewer.ID,
		TenantID:    tenantID,
		CaseID:      caseID,
		Action:      "case:view",
		Now:         time.Now().Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed() {
		t.Fatal("Evaluate() with an expired CaseGrant should deny")
	}
}

// TestEngine_RevokeCaseAccess_TakesEffectImmediately proves an
// explicitly revoked CaseGrant stops applying even before its natural
// expiry.
func TestEngine_RevokeCaseAccess_TakesEffectImmediately(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	reviewer := newTestUser(tenantID)
	caseID := uuid.New()

	grant, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: reviewer.ID,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GrantCaseAccess: %v", err)
	}

	if err := engine.RevokeCaseAccess(ctxWithUser(admin), tenantID, grant.ID); err != nil {
		t.Fatalf("RevokeCaseAccess: %v", err)
	}

	dec, err := engine.Evaluate(ctxWithUser(reviewer), accessgovernance.Request{
		ActorUserID: reviewer.ID,
		TenantID:    tenantID,
		CaseID:      caseID,
		Action:      "case:view",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed() {
		t.Fatal("Evaluate() after RevokeCaseAccess should deny")
	}
}
