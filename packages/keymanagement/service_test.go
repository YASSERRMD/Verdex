package keymanagement_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/keymanagement"
)

func TestService_Rotate_RequiresAuthentication(t *testing.T) {
	svc, _, tenantID := newTestService(t)

	if _, err := svc.Rotate(context.Background(), tenantID); !errors.Is(err, keymanagement.ErrUnauthenticated) {
		t.Fatalf("Rotate() unauthenticated error = %v, want ErrUnauthenticated", err)
	}
}

func TestService_Rotate_RejectsUnauthorizedRole(t *testing.T) {
	svc, _, tenantID := newTestService(t)

	// RoleAdvocate holds no key-management permission at all.
	user := newTestUser(tenantID, identity.RoleAdvocate)
	if _, err := svc.Rotate(ctxWithUser(user), tenantID); !errors.Is(err, keymanagement.ErrForbidden) {
		t.Fatalf("Rotate() unauthorized role error = %v, want ErrForbidden", err)
	}
}

func TestService_Rotate_RejectsAuditorViewOnlyRole(t *testing.T) {
	svc, _, tenantID := newTestService(t)

	// RoleAuditor holds PermViewKeys but not PermManageKeys.
	user := newTestUser(tenantID, identity.RoleAuditor)
	if _, err := svc.Rotate(ctxWithUser(user), tenantID); !errors.Is(err, keymanagement.ErrForbidden) {
		t.Fatalf("Rotate() as auditor error = %v, want ErrForbidden", err)
	}
}

func TestService_Rotate_AllowsAdmin(t *testing.T) {
	svc, repo, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)

	keyID, err := svc.Rotate(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("Rotate() as admin error = %v, want nil", err)
	}
	if keyID == "" {
		t.Fatal("Rotate() returned empty key ID")
	}

	active, err := repo.GetActive(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("repo.GetActive: %v", err)
	}
	if active.ID != keyID {
		t.Fatalf("GetActive().ID = %q, want %q", active.ID, keyID)
	}
}

// TestService_Rotate_MarksPriorVersionRetired proves task 3: rotation
// creates a new Active key version while the prior Active version
// becomes Retired (not deleted, not forgotten) -- still resolvable,
// matching Phase 075's rotation-preserves-old-key-decryptability
// expectation at the metadata layer.
func TestService_Rotate_MarksPriorVersionRetired(t *testing.T) {
	svc, repo, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	firstID, err := svc.Rotate(ctx, tenantID)
	if err != nil {
		t.Fatalf("first Rotate() error = %v, want nil", err)
	}

	secondID, err := svc.Rotate(ctx, tenantID)
	if err != nil {
		t.Fatalf("second Rotate() error = %v, want nil", err)
	}
	if firstID == secondID {
		t.Fatal("second Rotate() returned the same key ID as the first")
	}

	first, err := repo.Get(context.Background(), tenantID, firstID)
	if err != nil {
		t.Fatalf("repo.Get(firstID): %v", err)
	}
	if first.State != keymanagement.KeyStateRetired {
		t.Fatalf("first key State = %q, want %q", first.State, keymanagement.KeyStateRetired)
	}

	second, err := repo.Get(context.Background(), tenantID, secondID)
	if err != nil {
		t.Fatalf("repo.Get(secondID): %v", err)
	}
	if second.State != keymanagement.KeyStateActive {
		t.Fatalf("second key State = %q, want %q", second.State, keymanagement.KeyStateActive)
	}
}

func TestService_Revoke_RequiresManagePermission(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	keyID, err := svc.Rotate(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	viewer := newTestUser(tenantID, identity.RoleAuditor)
	if err := svc.Revoke(ctxWithUser(viewer), tenantID, keyID); !errors.Is(err, keymanagement.ErrForbidden) {
		t.Fatalf("Revoke() as auditor error = %v, want ErrForbidden", err)
	}

	if err := svc.Revoke(ctxWithUser(admin), tenantID, keyID); err != nil {
		t.Fatalf("Revoke() as admin error = %v, want nil", err)
	}
}

func TestService_ListKeys_RequiresViewPermission(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	if _, err := svc.Rotate(ctxWithUser(admin), tenantID); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	noPerm := newTestUser(tenantID, identity.RoleAdvocate)
	if _, err := svc.ListKeys(ctxWithUser(noPerm), tenantID, keymanagement.Filter{}); !errors.Is(err, keymanagement.ErrForbidden) {
		t.Fatalf("ListKeys() unauthorized error = %v, want ErrForbidden", err)
	}

	viewer := newTestUser(tenantID, identity.RoleAuditor)
	list, err := svc.ListKeys(ctxWithUser(viewer), tenantID, keymanagement.Filter{})
	if err != nil {
		t.Fatalf("ListKeys() as auditor error = %v, want nil", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListKeys() returned %d keys, want 1", len(list))
	}
}

func TestService_CurrentKey_AuditsAccess(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	if _, err := svc.Rotate(ctx, tenantID); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if _, err := svc.CurrentKey(ctx, tenantID); err != nil {
		t.Fatalf("CurrentKey() error = %v, want nil", err)
	}

	history, err := svc.AuditHistory(ctx, tenantID, 0)
	if err != nil {
		t.Fatalf("AuditHistory: %v", err)
	}

	var sawCurrentKey, sawRotate bool
	for _, e := range history {
		if e.Action == keymanagement.AuditActionCurrentKey {
			sawCurrentKey = true
		}
		if e.Action == keymanagement.AuditActionRotate {
			sawRotate = true
		}
		if e.Outcome != keymanagement.AuditOutcomeSuccess {
			t.Errorf("audit entry %v has outcome %v, want success", e.Action, e.Outcome)
		}
	}
	if !sawCurrentKey {
		t.Error("audit history missing AuditActionCurrentKey entry")
	}
	if !sawRotate {
		t.Error("audit history missing AuditActionRotate entry")
	}
}

func TestService_CurrentKey_AuditsDeniedAccess(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	if _, err := svc.Rotate(ctxWithUser(admin), tenantID); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	noPerm := newTestUser(tenantID, identity.RoleAdvocate)
	if _, err := svc.CurrentKey(ctxWithUser(noPerm), tenantID); !errors.Is(err, keymanagement.ErrForbidden) {
		t.Fatalf("CurrentKey() unauthorized error = %v, want ErrForbidden", err)
	}

	history, err := svc.AuditHistory(ctxWithUser(admin), tenantID, 0)
	if err != nil {
		t.Fatalf("AuditHistory: %v", err)
	}
	var sawDenied bool
	for _, e := range history {
		if e.Action == keymanagement.AuditActionCurrentKey && e.Outcome == keymanagement.AuditOutcomeDenied {
			sawDenied = true
		}
	}
	if !sawDenied {
		t.Error("audit history missing a denied AuditActionCurrentKey entry")
	}
}
