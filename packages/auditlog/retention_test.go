package auditlog_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestStore_Purge_RemovesOnlyEventsBeyondRetentionWindow(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()

	old := newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess)
	old.Time = time.Now().Add(-100 * 24 * time.Hour)
	mustAppend(t, store, old)

	recent := newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess)
	recent.Time = time.Now().Add(-1 * time.Hour)
	mustAppend(t, store, recent)

	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	removed, err := store.Purge(ctx, tenantID, auditlog.RetentionPolicy{Window: 30 * 24 * time.Hour})
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if removed != 1 {
		t.Fatalf("Purge removed %d events, want 1", removed)
	}

	remaining, err := store.Query(ctx, tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("Query after purge: %v", err)
	}
	// The recent event survives, plus the KindSystem "audit.purged"
	// event Purge itself appends.
	if len(remaining) != 2 {
		t.Fatalf("Query after purge: got %d events, want 2 (recent + purge record)", len(remaining))
	}

	var sawRecent, sawPurgeRecord bool
	for _, e := range remaining {
		if e.Kind == auditlog.KindSystem && e.Action == "audit.purged" {
			sawPurgeRecord = true
		}
		if e.Time.Equal(recent.Time) {
			sawRecent = true
		}
	}
	if !sawRecent {
		t.Fatalf("recent event did not survive purge: %+v", remaining)
	}
	if !sawPurgeRecord {
		t.Fatalf("purge did not record its own audit.purged system event: %+v", remaining)
	}
}

func TestStore_Purge_PreservesChainIntegrityOfSurvivors(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()

	old := newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess)
	old.Time = time.Now().Add(-100 * 24 * time.Hour)
	mustAppend(t, store, old)

	recent1 := newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess)
	recent1.Time = time.Now().Add(-2 * time.Hour)
	survivorFirst := mustAppend(t, store, recent1)

	recent2 := newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess)
	recent2.Time = time.Now().Add(-1 * time.Hour)
	survivorSecond := mustAppend(t, store, recent2)

	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	if _, err := store.Purge(ctx, tenantID, auditlog.RetentionPolicy{Window: 30 * 24 * time.Hour}); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	all, err := store.Query(ctx, tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("Query after purge: %v", err)
	}

	// The surviving prefix (recent1, recent2) must still form an
	// internally consistent chain: recent2.PrevHash must still equal
	// recent1's real ChainHash, since Purge never mutates a surviving
	// row.
	var gotFirst, gotSecond *auditlog.Event
	for i := range all {
		if all[i].ID == survivorFirst.ID {
			gotFirst = &all[i]
		}
		if all[i].ID == survivorSecond.ID {
			gotSecond = &all[i]
		}
	}
	if gotFirst == nil || gotSecond == nil {
		t.Fatalf("survivors missing from post-purge query: %+v", all)
	}
	if gotFirst.ChainHash != survivorFirst.ChainHash {
		t.Fatalf("surviving event's ChainHash mutated by purge: got %q, want %q", gotFirst.ChainHash, survivorFirst.ChainHash)
	}
	if gotSecond.PrevHash != gotFirst.ChainHash {
		t.Fatalf("surviving chain link broken: gotSecond.PrevHash = %q, want %q", gotSecond.PrevHash, gotFirst.ChainHash)
	}

	valid, brokenAt, err := auditlog.VerifyChain([]auditlog.Event{*gotFirst, *gotSecond})
	if err != nil || !valid || brokenAt != -1 {
		t.Fatalf("VerifyChain over survivors: valid=%v brokenAt=%d err=%v", valid, brokenAt, err)
	}
}

func TestStore_Purge_RequiresAuditReadPermission(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()
	mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))

	advocate := newTestUser(tenantID, identity.RoleAdvocate)
	_, err := store.Purge(ctxWithUser(advocate), tenantID, auditlog.RetentionPolicy{Window: time.Hour})
	if !errors.Is(err, auditlog.ErrForbidden) {
		t.Fatalf("Purge as advocate: err = %v, want ErrForbidden", err)
	}
}

func TestRetentionPolicy_Validate_RejectsNonPositiveWindow(t *testing.T) {
	cases := []auditlog.RetentionPolicy{
		{Window: 0},
		{Window: -time.Hour},
	}
	for _, p := range cases {
		if err := p.Validate(); !errors.Is(err, auditlog.ErrInvalidRetention) {
			t.Fatalf("Validate(%+v) = %v, want ErrInvalidRetention", p, err)
		}
	}
}
