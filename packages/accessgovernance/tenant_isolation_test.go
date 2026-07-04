package accessgovernance_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestEngine_GrantCaseAccess_CrossTenantRejected proves an admin
// authenticated against tenant A can never create a CaseGrant scoped
// to tenant B.
func TestEngine_GrantCaseAccess_CrossTenantRejected(t *testing.T) {
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := newTestUser(tenantA, identity.RoleAdmin)

	_, err := engine.GrantCaseAccess(ctxWithUser(adminA), tenantB, accessgovernance.CaseGrant{
		CaseID:        uuid.New(),
		GranteeUserID: uuid.New(),
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(time.Hour),
	})
	if !errors.Is(err, accessgovernance.ErrCrossTenantAccess) {
		t.Fatalf("GrantCaseAccess() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_Elevate_CrossTenantRejected mirrors the same guarantee
// for Elevate.
func TestEngine_Elevate_CrossTenantRejected(t *testing.T) {
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := newTestUser(tenantA, identity.RoleAdmin)

	_, err := engine.Elevate(ctxWithUser(adminA), tenantB, uuid.New(), "case:delete", uuid.Nil, "cross tenant attempt", time.Hour)
	if !errors.Is(err, accessgovernance.ErrCrossTenantAccess) {
		t.Fatalf("Elevate() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_Certify_CrossTenantRejected mirrors the same guarantee
// for Certify.
func TestEngine_Certify_CrossTenantRejected(t *testing.T) {
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := newTestUser(tenantA, identity.RoleAdmin)

	_, err := engine.Certify(ctxWithUser(adminA), tenantB, accessgovernance.Period{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	})
	if !errors.Is(err, accessgovernance.ErrCrossTenantAccess) {
		t.Fatalf("Certify() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestInMemoryCaseGrantRepository_TenantIsolated proves the
// repository layer itself never leaks a tenant's grants into another
// tenant's ListAll/ListForCase/Get results, independent of the Engine
// authorization layer above it.
func TestInMemoryCaseGrantRepository_TenantIsolated(t *testing.T) {
	repo := accessgovernance.NewInMemoryCaseGrantRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()
	caseID := uuid.New()

	grantA := &accessgovernance.CaseGrant{
		ID:            uuid.New(),
		TenantID:      tenantA,
		CaseID:        caseID,
		GranteeUserID: uuid.New(),
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(time.Hour),
	}
	if err := repo.Create(t.Context(), tenantA, grantA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}

	grantB := &accessgovernance.CaseGrant{
		ID:            uuid.New(),
		TenantID:      tenantB,
		CaseID:        caseID,
		GranteeUserID: uuid.New(),
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(time.Hour),
	}
	if err := repo.Create(t.Context(), tenantB, grantB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	// tenant B must never see tenant A's grant, even for the same
	// CaseID.
	listB, err := repo.ListForCase(t.Context(), tenantB, caseID)
	if err != nil {
		t.Fatalf("ListForCase (B): %v", err)
	}
	if len(listB) != 1 || listB[0].ID != grantB.ID {
		t.Fatalf("ListForCase(tenantB) = %v, want exactly grantB", listB)
	}

	if _, err := repo.Get(t.Context(), tenantB, grantA.ID); !errors.Is(err, accessgovernance.ErrGrantNotFound) {
		t.Fatalf("Get(tenantB, grantA.ID) error = %v, want ErrGrantNotFound", err)
	}

	allA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(allA) != 1 || allA[0].ID != grantA.ID {
		t.Fatalf("ListAll(tenantA) = %v, want exactly grantA", allA)
	}
}

// TestInMemoryPolicyRepository_TenantIsolated proves the same
// guarantee for policies.
func TestInMemoryPolicyRepository_TenantIsolated(t *testing.T) {
	repo := accessgovernance.NewInMemoryPolicyRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	policyA := &accessgovernance.Policy{ID: uuid.New(), TenantID: tenantA, Name: "a", Active: true}
	policyB := &accessgovernance.Policy{ID: uuid.New(), TenantID: tenantB, Name: "b", Active: true}

	if err := repo.Create(t.Context(), tenantA, policyA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, policyB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.List(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("List (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != policyA.ID {
		t.Fatalf("List(tenantA) = %v, want exactly policyA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, policyB.ID); !errors.Is(err, accessgovernance.ErrPolicyNotFound) {
		t.Fatalf("Get(tenantA, policyB.ID) error = %v, want ErrPolicyNotFound", err)
	}
}
