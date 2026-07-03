package keymanagement_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/keymanagement"
)

func TestService_GrantBreakGlass_RequiresBreakGlassPermission(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	keyID, err := svc.Rotate(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// RoleAuditor holds PermViewKeys but not PermBreakGlassKeys.
	auditor := newTestUser(tenantID, identity.RoleAuditor)
	if _, err := svc.GrantBreakGlass(ctxWithUser(auditor), tenantID, keyID, "investigating incident 123", 0); !errors.Is(err, keymanagement.ErrForbidden) {
		t.Fatalf("GrantBreakGlass() as auditor error = %v, want ErrForbidden", err)
	}
}

func TestService_GrantBreakGlass_RequiresJustification(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	keyID, err := svc.Rotate(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	if _, err := svc.GrantBreakGlass(ctxWithUser(admin), tenantID, keyID, "   ", 0); !errors.Is(err, keymanagement.ErrJustificationRequired) {
		t.Fatalf("GrantBreakGlass() blank justification error = %v, want ErrJustificationRequired", err)
	}
}

func TestService_GrantAndUseBreakGlass_Success(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	keyID, err := svc.Rotate(ctx, tenantID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	grant, err := svc.GrantBreakGlass(ctx, tenantID, keyID, "auditor requested emergency access for incident INC-42", time.Hour)
	if err != nil {
		t.Fatalf("GrantBreakGlass() error = %v, want nil", err)
	}
	if grant.Justification == "" {
		t.Fatal("grant.Justification is empty")
	}
	if grant.ExpiresAt.Before(grant.GrantedAt) {
		t.Fatal("grant.ExpiresAt is before grant.GrantedAt")
	}

	material, err := svc.UseBreakGlass(ctx, tenantID, grant.ID)
	if err != nil {
		t.Fatalf("UseBreakGlass() error = %v, want nil", err)
	}
	if material.Metadata.ID != keyID {
		t.Fatalf("UseBreakGlass() key ID = %q, want %q", material.Metadata.ID, keyID)
	}
	if len(material.Material) == 0 {
		t.Fatal("UseBreakGlass() returned empty key material")
	}

	// Every grant and use is heavily audited (task 6): both actions
	// must appear in the tenant's audit history, carrying the
	// justification.
	history, err := svc.AuditHistory(ctx, tenantID, 0)
	if err != nil {
		t.Fatalf("AuditHistory: %v", err)
	}
	var sawGrant, sawUse bool
	for _, e := range history {
		if e.Action == keymanagement.AuditActionBreakGlassGrant && e.Outcome == keymanagement.AuditOutcomeSuccess {
			sawGrant = true
			if e.Justification == "" {
				t.Error("break-glass grant audit entry missing justification")
			}
		}
		if e.Action == keymanagement.AuditActionBreakGlassUse && e.Outcome == keymanagement.AuditOutcomeSuccess {
			sawUse = true
			if e.Justification == "" {
				t.Error("break-glass use audit entry missing justification")
			}
		}
	}
	if !sawGrant {
		t.Error("audit history missing successful AuditActionBreakGlassGrant entry")
	}
	if !sawUse {
		t.Error("audit history missing successful AuditActionBreakGlassUse entry")
	}
}

// TestService_UseBreakGlass_ExpiredGrantRejected proves the
// "time-bound" half of task 6: a grant issued with a TTL in the past
// (simulated here via a negative ttl, which GrantBreakGlass converts
// to DefaultBreakGlassTTL only when ttl <= 0 -- so this test instead
// constructs an already-expired grant directly against the store to
// exercise UseBreakGlass's expiry check deterministically).
func TestService_UseBreakGlass_ExpiredGrantRejected(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	keyID, err := svc.Rotate(ctx, tenantID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	grant, err := svc.GrantBreakGlass(ctx, tenantID, keyID, "will expire immediately", time.Nanosecond)
	if err != nil {
		t.Fatalf("GrantBreakGlass: %v", err)
	}

	time.Sleep(2 * time.Millisecond)

	if _, err := svc.UseBreakGlass(ctx, tenantID, grant.ID); !errors.Is(err, keymanagement.ErrBreakGlassExpired) {
		t.Fatalf("UseBreakGlass() expired grant error = %v, want ErrBreakGlassExpired", err)
	}
}

func TestService_UseBreakGlass_UnknownGrantRejected(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)

	if _, err := svc.UseBreakGlass(ctxWithUser(admin), tenantID, uuid.New()); !errors.Is(err, keymanagement.ErrBreakGlassNotFound) {
		t.Fatalf("UseBreakGlass() unknown grant error = %v, want ErrBreakGlassNotFound", err)
	}
}

func TestService_UseBreakGlass_WrongUserRejected(t *testing.T) {
	svc, _, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	otherAdmin := newTestUser(tenantID, identity.RoleAdmin)

	keyID, err := svc.Rotate(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	grant, err := svc.GrantBreakGlass(ctxWithUser(admin), tenantID, keyID, "for admin's own use only", time.Hour)
	if err != nil {
		t.Fatalf("GrantBreakGlass: %v", err)
	}

	if _, err := svc.UseBreakGlass(ctxWithUser(otherAdmin), tenantID, grant.ID); !errors.Is(err, keymanagement.ErrForbidden) {
		t.Fatalf("UseBreakGlass() by a different user error = %v, want ErrForbidden", err)
	}
}
