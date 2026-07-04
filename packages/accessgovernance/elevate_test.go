package accessgovernance_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestEngine_Elevate_RequiresJustification proves Elevate mirrors
// packages/keymanagement's break-glass justification requirement.
func TestEngine_Elevate_RequiresJustification(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)

	_, err := engine.Elevate(ctxWithUser(admin), tenantID, admin.ID, "case:delete", uuid.Nil, "   ", 0)
	if !errors.Is(err, accessgovernance.ErrJustificationRequired) {
		t.Fatalf("Elevate() blank justification error = %v, want ErrJustificationRequired", err)
	}
}

// TestEngine_Elevate_GrantsTemporaryAccess proves a successful
// Elevate call produces a Grant that Evaluate honors for the elevated
// action, even though the actor holds no ordinary permission for it.
func TestEngine_Elevate_GrantsTemporaryAccess(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	clerk := newTestUser(tenantID, identity.RoleClerk) // does not hold case:delete

	grant, err := engine.Elevate(ctxWithUser(admin), tenantID, clerk.ID, "case:delete", uuid.Nil, "emergency docket cleanup INC-1", time.Hour)
	if err != nil {
		t.Fatalf("Elevate: %v", err)
	}
	if grant.ExpiresAt.Before(grant.GrantedAt) {
		t.Fatal("grant.ExpiresAt is before grant.GrantedAt")
	}

	dec, err := engine.Evaluate(ctxWithUser(clerk), accessgovernance.Request{
		ActorUserID: clerk.ID,
		ActorRoles:  clerk.Roles,
		TenantID:    tenantID,
		Action:      "case:delete",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed() {
		t.Fatalf("Evaluate() with active elevation grant = %v, want Allow", dec.Effect)
	}
	if dec.MatchedGrantID != grant.ID {
		t.Errorf("Decision.MatchedGrantID = %v, want %v", dec.MatchedGrantID, grant.ID)
	}
}

// TestEngine_Elevate_ExpiredGrantRejected proves the mandatory-expiry
// half of task 3: once ExpiresAt has passed, Evaluate no longer
// honors the grant -- checked purely at evaluation time, no
// background job involved.
func TestEngine_Elevate_ExpiredGrantRejected(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	clerk := newTestUser(tenantID, identity.RoleClerk)

	grant, err := engine.Elevate(ctxWithUser(admin), tenantID, clerk.ID, "case:delete", uuid.Nil, "temporary access", time.Minute)
	if err != nil {
		t.Fatalf("Elevate: %v", err)
	}

	dec, err := engine.Evaluate(ctxWithUser(clerk), accessgovernance.Request{
		ActorUserID: clerk.ID,
		ActorRoles:  clerk.Roles,
		TenantID:    tenantID,
		Action:      "case:delete",
		Now:         grant.ExpiresAt.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed() {
		t.Fatal("Evaluate() with an expired elevation grant should deny")
	}
}

// TestEngine_Elevate_DefaultTTLAppliedWhenZero proves a zero ttl uses
// DefaultElevationTTL rather than producing an immediately-expired
// grant.
func TestEngine_Elevate_DefaultTTLAppliedWhenZero(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)

	grant, err := engine.Elevate(ctxWithUser(admin), tenantID, admin.ID, "case:delete", uuid.Nil, "default ttl check", 0)
	if err != nil {
		t.Fatalf("Elevate: %v", err)
	}
	if got := grant.ExpiresAt.Sub(grant.GrantedAt); got != accessgovernance.DefaultElevationTTL {
		t.Fatalf("grant TTL = %v, want %v", got, accessgovernance.DefaultElevationTTL)
	}
}

// TestEngine_Elevate_RequiresManagePermission proves a plain actor
// without managePermission cannot self-elevate.
func TestEngine_Elevate_RequiresManagePermission(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	advocate := newTestUser(tenantID, identity.RoleAdvocate)

	_, err := engine.Elevate(ctxWithUser(advocate), tenantID, advocate.ID, "case:delete", uuid.Nil, "trying to self elevate", time.Hour)
	if !errors.Is(err, accessgovernance.ErrForbidden) {
		t.Fatalf("Elevate() by advocate error = %v, want ErrForbidden", err)
	}
}
