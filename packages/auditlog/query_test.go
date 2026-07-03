package auditlog_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestStore_Query_FiltersByActorCaseKindAction(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()
	caseA := uuid.New()
	caseB := uuid.New()

	ev1 := newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess)
	ev1.CaseID = caseA
	mustAppend(t, store, ev1)

	ev2 := newEvent(tenantID, "user-2", "case.viewed", auditlog.KindDataAccess)
	ev2.CaseID = caseB
	mustAppend(t, store, ev2)

	ev3 := newEvent(tenantID, "user-1", "case.signoff", auditlog.KindSignoff)
	ev3.CaseID = caseA
	mustAppend(t, store, ev3)

	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	// Filter by actor.
	got, err := store.Query(ctx, tenantID, auditlog.Filter{Actor: "user-1"})
	if err != nil {
		t.Fatalf("Query by actor: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Query by actor: got %d events, want 2", len(got))
	}

	// Filter by case.
	got, err = store.Query(ctx, tenantID, auditlog.Filter{CaseID: caseB})
	if err != nil {
		t.Fatalf("Query by case: %v", err)
	}
	if len(got) != 1 || got[0].Actor != "user-2" {
		t.Fatalf("Query by case: got %+v, want single user-2 event", got)
	}

	// Filter by kind.
	got, err = store.Query(ctx, tenantID, auditlog.Filter{Kinds: []auditlog.Kind{auditlog.KindSignoff}})
	if err != nil {
		t.Fatalf("Query by kind: %v", err)
	}
	if len(got) != 1 || got[0].Action != "case.signoff" {
		t.Fatalf("Query by kind: got %+v, want single signoff event", got)
	}

	// Filter by action.
	got, err = store.Query(ctx, tenantID, auditlog.Filter{Action: "case.viewed"})
	if err != nil {
		t.Fatalf("Query by action: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Query by action: got %d events, want 2", len(got))
	}
}

func TestStore_Query_FiltersByTimeRange(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()

	early := newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess)
	early.Time = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	mustAppend(t, store, early)

	late := newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess)
	late.Time = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mustAppend(t, store, late)

	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	got, err := store.Query(ctx, tenantID, auditlog.Filter{Since: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Query since: %v", err)
	}
	if len(got) != 1 || !got[0].Time.Equal(late.Time) {
		t.Fatalf("Query since: got %+v, want only the late event", got)
	}

	got, err = store.Query(ctx, tenantID, auditlog.Filter{Until: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Query until: %v", err)
	}
	if len(got) != 1 || !got[0].Time.Equal(early.Time) {
		t.Fatalf("Query until: got %+v, want only the early event", got)
	}
}

func TestStore_Query_RequiresAuditReadPermission(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()
	mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))

	cases := []struct {
		name    string
		roles   []identity.Role
		wantErr error
	}{
		{"unauthenticated", nil, auditlog.ErrUnauthenticated},
		{"advocate lacks PermAuditRead", []identity.Role{identity.RoleAdvocate}, auditlog.ErrForbidden},
		{"clerk lacks PermAuditRead", []identity.Role{identity.RoleClerk}, auditlog.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.roles != nil {
				ctx = ctxWithUser(newTestUser(tenantID, tc.roles...))
			}
			_, err := store.Query(ctx, tenantID, auditlog.Filter{})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Query: err = %v, want %v", err, tc.wantErr)
			}
		})
	}

	allowed := []identity.Role{identity.RoleJudge, identity.RoleAdmin, identity.RoleAuditor}
	for _, role := range allowed {
		t.Run(string(role)+" allowed", func(t *testing.T) {
			ctx := ctxWithUser(newTestUser(tenantID, role))
			_, err := store.Query(ctx, tenantID, auditlog.Filter{})
			if err != nil {
				t.Fatalf("Query as %s: unexpected error %v", role, err)
			}
		})
	}
}

func TestStore_Query_EnforcesTenantIsolation(t *testing.T) {
	store := newTestStore(t)
	tenantA := uuid.New()
	tenantB := uuid.New()

	mustAppend(t, store, newEvent(tenantA, "user-1", "case.viewed", auditlog.KindDataAccess))
	mustAppend(t, store, newEvent(tenantB, "user-2", "case.viewed", auditlog.KindDataAccess))

	// An auditor from tenant A must never see tenant B's events, and
	// must be rejected outright when asking for tenant B's data (not
	// silently given an empty result that could be confused with "no
	// events exist").
	auditorA := newTestUser(tenantA, identity.RoleAuditor)
	_, err := store.Query(ctxWithUser(auditorA), tenantB, auditlog.Filter{})
	if !errors.Is(err, auditlog.ErrCrossTenantAccess) {
		t.Fatalf("cross-tenant Query: err = %v, want ErrCrossTenantAccess", err)
	}

	got, err := store.Query(ctxWithUser(auditorA), tenantA, auditlog.Filter{})
	if err != nil {
		t.Fatalf("same-tenant Query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("same-tenant Query: got %d events, want 1", len(got))
	}
}
