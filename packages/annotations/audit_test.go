package annotations_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/annotations"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestService_AuditTrail_CapturesEveryAction proves task 7's contract:
// every create/edit/delete/resolve(/reopen) call appends exactly one
// AuditRecord, in order, queryable via AuditTrail.
func TestService_AuditTrail_CapturesEveryAction(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	// RoleAdmin holds both PermEditCase and PermAuditRead, so a single
	// actor can perform every action and then read the trail back.
	admin := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(admin)

	a, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "initial note",
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.UpdateBody(ctx, tenantID, a.ID, "edited note"); err != nil {
		t.Fatalf("UpdateBody: %v", err)
	}
	if _, err := svc.Resolve(ctx, tenantID, a.ID); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := svc.Reopen(ctx, tenantID, a.ID); err != nil {
		t.Fatalf("Reopen: %v", err)
	}

	trail, err := svc.AuditTrail(ctx, tenantID, a.ID)
	if err != nil {
		t.Fatalf("AuditTrail: %v", err)
	}

	wantVerbs := []annotations.AuditVerb{
		annotations.AuditCreated,
		annotations.AuditEdited,
		annotations.AuditResolved,
		annotations.AuditReopened,
	}
	if len(trail) != len(wantVerbs) {
		t.Fatalf("len(trail) = %d, want %d: %+v", len(trail), len(wantVerbs), trail)
	}
	for i, rec := range trail {
		if rec.Verb != wantVerbs[i] {
			t.Fatalf("trail[%d].Verb = %s, want %s", i, rec.Verb, wantVerbs[i])
		}
		if rec.Actor != admin.ID {
			t.Fatalf("trail[%d].Actor = %s, want %s", i, rec.Actor, admin.ID)
		}
		if rec.AnnotationID != a.ID {
			t.Fatalf("trail[%d].AnnotationID = %s, want %s", i, rec.AnnotationID, a.ID)
		}
	}

	// Delete is audited too, and the record survives the annotation's
	// own deletion (no FK back to the annotations table).
	if err := svc.Delete(ctx, tenantID, a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	trailAfterDelete, err := svc.AuditTrail(ctx, tenantID, a.ID)
	if err != nil {
		t.Fatalf("AuditTrail after delete: %v", err)
	}
	if len(trailAfterDelete) != len(wantVerbs)+1 {
		t.Fatalf("len(trailAfterDelete) = %d, want %d", len(trailAfterDelete), len(wantVerbs)+1)
	}
	if trailAfterDelete[len(trailAfterDelete)-1].Verb != annotations.AuditDeleted {
		t.Fatalf("last verb = %s, want %s", trailAfterDelete[len(trailAfterDelete)-1].Verb, annotations.AuditDeleted)
	}
}

// TestService_AuditTrail_RequiresAuditPermission proves that reading
// the audit trail requires identity.PermAuditRead in addition to
// identity.PermViewCase — RoleClerk holds view but not audit, so it
// must be forbidden here even though it can read the annotation
// itself.
func TestService_AuditTrail_RequiresAuditPermission(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	clerk := newTestUser(tenantID, identity.RoleClerk)

	a, err := svc.Create(ctxWithUser(admin), tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "note",
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.AuditTrail(ctxWithUser(clerk), tenantID, a.ID); !errors.Is(err, annotations.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}
