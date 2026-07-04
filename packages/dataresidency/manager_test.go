package dataresidency_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func newTestManager(t *testing.T) *dataresidency.Manager {
	t.Helper()
	m, err := dataresidency.NewManager(dataresidency.NewInMemoryPolicyStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestManager_SetPolicy_RequiresAuthentication(t *testing.T) {
	m := newTestManager(t)
	policy := dataresidency.ResidencyPolicy{DeploymentID: uuid.New(), AllowedRegions: []string{"eu"}}

	err := m.SetPolicy(context.Background(), policy)
	if !errors.Is(err, dataresidency.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestManager_SetPolicy_RequiresManagePermission(t *testing.T) {
	m := newTestManager(t)
	tenantID := uuid.New()
	user := newTestUser(tenantID, identity.RoleAdvocate) // lacks PermManageSettings
	policy := dataresidency.ResidencyPolicy{DeploymentID: uuid.New(), AllowedRegions: []string{"eu"}}

	err := m.SetPolicy(ctxWithUser(user), policy)
	if !errors.Is(err, dataresidency.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestManager_SetPolicy_AllowsAdmin(t *testing.T) {
	m := newTestManager(t)
	tenantID := uuid.New()
	admin := newTestUser(tenantID, identity.RoleAdmin)
	deploymentID := uuid.New()
	policy := dataresidency.ResidencyPolicy{DeploymentID: deploymentID, AllowedRegions: []string{"eu"}}

	if err := m.SetPolicy(ctxWithUser(admin), policy); err != nil {
		t.Fatalf("SetPolicy as admin: %v", err)
	}

	got, err := m.Policy(context.Background(), deploymentID)
	if err != nil {
		t.Fatalf("Policy: %v", err)
	}
	if len(got.AllowedRegions) != 1 || got.AllowedRegions[0] != "eu" {
		t.Fatalf("unexpected policy round-trip: %+v", got)
	}
}

func TestManager_SetRegionPin_AllowsAdminAndRoundTrips(t *testing.T) {
	m := newTestManager(t)
	tenantID := uuid.New()
	admin := newTestUser(tenantID, identity.RoleAdmin)
	deploymentID := uuid.New()
	pin := dataresidency.RegionPin{DeploymentID: deploymentID, Region: "eu", HostPatterns: []string{"eu-west-1"}}

	if err := m.SetRegionPin(ctxWithUser(admin), pin); err != nil {
		t.Fatalf("SetRegionPin as admin: %v", err)
	}

	got, err := m.RegionPin(context.Background(), deploymentID)
	if err != nil {
		t.Fatalf("RegionPin: %v", err)
	}
	if got.Region != "eu" {
		t.Fatalf("unexpected region pin round-trip: %+v", got)
	}
}

func TestManager_SetPolicy_RejectsInvalidPolicy(t *testing.T) {
	m := newTestManager(t)
	tenantID := uuid.New()
	admin := newTestUser(tenantID, identity.RoleAdmin)

	err := m.SetPolicy(ctxWithUser(admin), dataresidency.ResidencyPolicy{})
	if err == nil {
		t.Fatal("expected error for policy with empty DeploymentID")
	}
}
