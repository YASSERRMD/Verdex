package keymanagement_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/keymanagement"
	"github.com/YASSERRMD/verdex/packages/observability"
)

// TestService_TenantIsolation_CurrentKeyNeverCrossesTenants is the
// real cross-tenant-leakage test task 4 asks for: two tenants each
// rotate their own key, and tenant B's CurrentKey must never resolve
// to tenant A's key material (or vice versa), even though both
// tenants share the same underlying Provider/Repository instances.
func TestService_TenantIsolation_CurrentKeyNeverCrossesTenants(t *testing.T) {
	repo := keymanagement.NewInMemoryRepository()
	provider := newInMemoryProvider(repo)

	tenantA := uuid.New()
	tenantB := uuid.New()
	adminA := newTestUser(tenantA, identity.RoleAdmin)
	adminB := newTestUser(tenantB, identity.RoleAdmin)

	auditRepo := keymanagement.NewInMemoryAuditRepository()
	recorder, err := keymanagement.NewAuditRecorder(observability.NewAuditLogger(discardWriter{}), auditRepo)
	if err != nil {
		t.Fatalf("NewAuditRecorder: %v", err)
	}
	svc, err := keymanagement.NewService(provider, repo, recorder, keymanagement.NewInMemoryBreakGlassStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if _, err := svc.Rotate(ctxWithUser(adminA), tenantA); err != nil {
		t.Fatalf("Rotate tenant A: %v", err)
	}
	if _, err := svc.Rotate(ctxWithUser(adminB), tenantB); err != nil {
		t.Fatalf("Rotate tenant B: %v", err)
	}

	keyA, err := svc.CurrentKey(ctxWithUser(adminA), tenantA)
	if err != nil {
		t.Fatalf("CurrentKey tenant A: %v", err)
	}
	keyB, err := svc.CurrentKey(ctxWithUser(adminB), tenantB)
	if err != nil {
		t.Fatalf("CurrentKey tenant B: %v", err)
	}

	if keyA.Metadata.ID == keyB.Metadata.ID {
		t.Fatal("tenant A and tenant B resolved to the same key ID")
	}
	if string(keyA.Material) == string(keyB.Material) {
		t.Fatal("tenant A and tenant B resolved to identical key material")
	}
	if keyA.Metadata.TenantID != tenantA {
		t.Fatalf("keyA.Metadata.TenantID = %v, want %v", keyA.Metadata.TenantID, tenantA)
	}
	if keyB.Metadata.TenantID != tenantB {
		t.Fatalf("keyB.Metadata.TenantID = %v, want %v", keyB.Metadata.TenantID, tenantB)
	}

	// Tenant A's admin, scoped to tenant A, must never be able to read
	// tenant B's key by passing tenant B's ID with tenant A's own
	// authenticated identity -- requireMatchingUserTenant must reject
	// this even before Provider/Repository are consulted.
	if _, err := svc.CurrentKey(ctxWithUser(adminA), tenantB); !errors.Is(err, keymanagement.ErrCrossTenantAccess) {
		t.Fatalf("cross-tenant CurrentKey error = %v, want ErrCrossTenantAccess", err)
	}

	// Tenant B's admin must not be able to fetch tenant A's specific
	// key ID by guessing it, because Repository.Get scopes by tenantID.
	if _, err := svc.Key(ctxWithUser(adminB), tenantB, keyA.Metadata.ID); err == nil {
		t.Fatal("tenant B resolved tenant A's key ID; want an error")
	}

	// ListKeys for tenant A must never include tenant B's key.
	listA, err := svc.ListKeys(ctxWithUser(adminA), tenantA, keymanagement.Filter{})
	if err != nil {
		t.Fatalf("ListKeys tenant A: %v", err)
	}
	for _, k := range listA {
		if k.ID == keyB.Metadata.ID {
			t.Fatal("tenant B's key leaked into tenant A's ListKeys result")
		}
	}
}

// TestInMemoryRepository_CrossTenantAccessRejected proves the
// repository layer itself refuses to persist a KeyMetadata whose
// TenantID does not match the scope's tenantID, mirroring
// packages/caseversioning's tenant_isolation_test.go direct-repository
// check.
func TestInMemoryRepository_CrossTenantAccessRejected(t *testing.T) {
	repo := keymanagement.NewInMemoryRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	mismatched := &keymanagement.KeyMetadata{
		ID:       "k1",
		TenantID: tenantA,
		Version:  1,
		State:    keymanagement.KeyStateActive,
	}
	if err := repo.Create(context.Background(), tenantB, mismatched); !errors.Is(err, keymanagement.ErrCrossTenantAccess) {
		t.Fatalf("Create() tenant mismatch error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestInMemoryRepository_GetActive_ScopedPerTenant proves
// GetActive never returns another tenant's Active key even when both
// tenants happen to be Active at the same time.
func TestInMemoryRepository_GetActive_ScopedPerTenant(t *testing.T) {
	ctx := context.Background()
	repo := keymanagement.NewInMemoryRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	if err := repo.Create(ctx, tenantA, &keymanagement.KeyMetadata{
		ID: "a-1", TenantID: tenantA, Version: 1, State: keymanagement.KeyStateActive,
	}); err != nil {
		t.Fatalf("Create tenant A: %v", err)
	}
	if err := repo.Create(ctx, tenantB, &keymanagement.KeyMetadata{
		ID: "b-1", TenantID: tenantB, Version: 1, State: keymanagement.KeyStateActive,
	}); err != nil {
		t.Fatalf("Create tenant B: %v", err)
	}

	activeA, err := repo.GetActive(ctx, tenantA)
	if err != nil {
		t.Fatalf("GetActive tenant A: %v", err)
	}
	if activeA.ID != "a-1" {
		t.Fatalf("GetActive tenant A = %q, want %q", activeA.ID, "a-1")
	}

	activeB, err := repo.GetActive(ctx, tenantB)
	if err != nil {
		t.Fatalf("GetActive tenant B: %v", err)
	}
	if activeB.ID != "b-1" {
		t.Fatalf("GetActive tenant B = %q, want %q", activeB.ID, "b-1")
	}
}
