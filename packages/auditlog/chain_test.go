package auditlog_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestStore_Append_BuildsValidChain(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()

	e1 := mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))
	e2 := mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))
	e3 := mustAppend(t, store, newEvent(tenantID, "user-2", "case.signoff", auditlog.KindSignoff))

	if e1.PrevHash != "" {
		t.Fatalf("first event PrevHash = %q, want empty", e1.PrevHash)
	}
	if e2.PrevHash != e1.ChainHash {
		t.Fatalf("second event PrevHash = %q, want %q", e2.PrevHash, e1.ChainHash)
	}
	if e3.PrevHash != e2.ChainHash {
		t.Fatalf("third event PrevHash = %q, want %q", e3.PrevHash, e2.ChainHash)
	}

	valid, brokenAt, err := auditlog.VerifyGenesisChain([]auditlog.Event{e1, e2, e3})
	if err != nil || !valid || brokenAt != -1 {
		t.Fatalf("VerifyGenesisChain: valid=%v brokenAt=%d err=%v, want valid", valid, brokenAt, err)
	}
}

func TestVerifyChain_DetectsTamperedField(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()

	e1 := mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))
	e2 := mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))
	e3 := mustAppend(t, store, newEvent(tenantID, "user-2", "case.signoff", auditlog.KindSignoff))

	// Tamper with e2's Detail after the fact, without recomputing its
	// ChainHash -- exactly what an attacker directly modifying a
	// database row would do.
	events := []auditlog.Event{e1, e2, e3}
	events[1].Detail = "tampered"

	valid, brokenAt, err := auditlog.VerifyChain(events)
	if valid {
		t.Fatalf("VerifyChain reported valid=true for a tampered chain")
	}
	if brokenAt != 1 {
		t.Fatalf("brokenAt = %d, want 1", brokenAt)
	}
	if !errors.Is(err, auditlog.ErrChainBroken) {
		t.Fatalf("err = %v, want ErrChainBroken", err)
	}

	// The event *after* the tampered one should still verify in
	// isolation up to the point of divergence -- but because e3's
	// PrevHash depends on e2's real ChainHash (not the tampered
	// Detail), the break is detected exactly at index 1, not 2 or later.
}

func TestVerifyChain_DetectsTamperedChainHash(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()

	e1 := mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))
	e2 := mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))

	events := []auditlog.Event{e1, e2}
	events[0].ChainHash = "0000000000000000000000000000000000000000000000000000000000000000"

	valid, brokenAt, err := auditlog.VerifyChain(events)
	if valid {
		t.Fatalf("VerifyChain reported valid=true after ChainHash tampering")
	}
	if brokenAt != 0 {
		t.Fatalf("brokenAt = %d, want 0", brokenAt)
	}
	if !errors.Is(err, auditlog.ErrChainBroken) {
		t.Fatalf("err = %v, want ErrChainBroken", err)
	}
}

func TestVerifyChain_DetectsDeletedEvent(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()

	e1 := mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))
	_ = mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))
	e3 := mustAppend(t, store, newEvent(tenantID, "user-2", "case.signoff", auditlog.KindSignoff))

	// Silently drop the middle event, as if a row had been deleted
	// directly against the database outside of Store.Purge.
	tampered := []auditlog.Event{e1, e3}

	valid, brokenAt, err := auditlog.VerifyChain(tampered)
	if valid {
		t.Fatalf("VerifyChain reported valid=true after an event was removed")
	}
	if brokenAt != 1 {
		t.Fatalf("brokenAt = %d, want 1", brokenAt)
	}
	if !errors.Is(err, auditlog.ErrChainBroken) {
		t.Fatalf("err = %v, want ErrChainBroken", err)
	}
}

func TestVerifyChain_EmptyIsValid(t *testing.T) {
	valid, brokenAt, err := auditlog.VerifyChain(nil)
	if !valid || brokenAt != -1 || err != nil {
		t.Fatalf("VerifyChain(nil) = valid=%v brokenAt=%d err=%v, want valid empty chain", valid, brokenAt, err)
	}
}

func TestStore_VerifyTenantChain_RequiresAuditRead(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()
	mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))

	advocate := newTestUser(tenantID, identity.RoleAdvocate)
	_, _, err := store.VerifyTenantChain(ctxWithUser(advocate), tenantID)
	if !errors.Is(err, auditlog.ErrForbidden) {
		t.Fatalf("VerifyTenantChain as advocate: err = %v, want ErrForbidden", err)
	}

	auditor := newTestUser(tenantID, identity.RoleAuditor)
	valid, brokenAt, err := store.VerifyTenantChain(ctxWithUser(auditor), tenantID)
	if err != nil || !valid || brokenAt != -1 {
		t.Fatalf("VerifyTenantChain as auditor: valid=%v brokenAt=%d err=%v", valid, brokenAt, err)
	}
}

func TestStore_VerifyTenantChain_RequiresAuthentication(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()

	_, _, err := store.VerifyTenantChain(context.Background(), tenantID)
	if !errors.Is(err, auditlog.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}
