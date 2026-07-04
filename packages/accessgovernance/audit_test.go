package accessgovernance_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestEngine_Evaluate_IsAlwaysAudited proves every Evaluate call --
// allowed or denied -- is recorded via packages/auditlog.Store (task
// 6), by inspecting the same store the Engine's AuditSink writes to.
func TestEngine_Evaluate_IsAlwaysAudited(t *testing.T) {
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	advocate := newTestUser(tenantID, identity.RoleAdvocate)
	auditor := newTestUser(tenantID, identity.RoleAuditor) // holds PermAuditRead

	if _, err := engine.Evaluate(ctxWithUser(advocate), accessgovernance.Request{
		ActorUserID: advocate.ID,
		ActorRoles:  advocate.Roles,
		TenantID:    tenantID,
		Action:      "case:delete", // denied: advocate has no such policy/grant
	}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	events, err := auditStore.Query(ctxWithUser(auditor), tenantID, auditlog.Filter{Action: "access_governance.evaluate"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("audit trail has %d access_governance.evaluate events, want 1", len(events))
	}
	if events[0].Outcome != "denied" {
		t.Fatalf("audit event outcome = %q, want %q", events[0].Outcome, "denied")
	}
}

// TestAuditSink_PrivilegedActivity_SurfacesElevationsOnly proves task
// 6: PrivilegedActivity surfaces elevation events specifically,
// distinct from ordinary Evaluate events recorded in the same store.
func TestAuditSink_PrivilegedActivity_SurfacesElevationsOnly(t *testing.T) {
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)

	// An ordinary Evaluate call (not privileged).
	if _, err := engine.Evaluate(ctxWithUser(admin), accessgovernance.Request{
		ActorUserID: admin.ID,
		ActorRoles:  admin.Roles,
		TenantID:    tenantID,
		Action:      "case:view",
	}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	// A privileged JIT elevation.
	if _, err := engine.Elevate(ctxWithUser(admin), tenantID, admin.ID, "case:delete", uuid.Nil, "incident response", time.Hour); err != nil {
		t.Fatalf("Elevate: %v", err)
	}

	sink, err := accessgovernance.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("NewAuditSink: %v", err)
	}
	events, err := sink.PrivilegedActivity(ctxWithUser(admin), tenantID, accessgovernance.PrivilegedFilter{})
	if err != nil {
		t.Fatalf("PrivilegedActivity: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("PrivilegedActivity() returned %d events, want exactly 1 (the elevation)", len(events))
	}
	if events[0].Action != "access_governance.elevate" {
		t.Fatalf("PrivilegedActivity() event action = %q, want access_governance.elevate", events[0].Action)
	}
	if events[0].Outcome != "granted" {
		t.Fatalf("PrivilegedActivity() event outcome = %q, want granted", events[0].Outcome)
	}
}

// TestAuditSink_PrivilegedActivity_TenantIsolated proves a tenant's
// privileged-activity query never surfaces another tenant's
// elevation events.
func TestAuditSink_PrivilegedActivity_TenantIsolated(t *testing.T) {
	_, auditStore, tenantA := newTestEngineWithAudit(t)
	tenantB := uuid.New()
	adminA := newTestUser(tenantA, identity.RoleAdmin)
	adminB := newTestUser(tenantB, identity.RoleAdmin)

	sink, err := accessgovernance.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("NewAuditSink: %v", err)
	}
	if _, err := sink.RecordElevate(ctxWithUser(adminA), tenantA, adminA.ID, "case:delete", "tenant A elevation", nil); err != nil {
		t.Fatalf("RecordElevate (A): %v", err)
	}
	if _, err := sink.RecordElevate(ctxWithUser(adminB), tenantB, adminB.ID, "case:delete", "tenant B elevation", nil); err != nil {
		t.Fatalf("RecordElevate (B): %v", err)
	}

	eventsA, err := sink.PrivilegedActivity(ctxWithUser(adminA), tenantA, accessgovernance.PrivilegedFilter{})
	if err != nil {
		t.Fatalf("PrivilegedActivity (A): %v", err)
	}
	if len(eventsA) != 1 {
		t.Fatalf("tenant A privileged activity = %d events, want 1", len(eventsA))
	}

	eventsB, err := sink.PrivilegedActivity(ctxWithUser(adminB), tenantB, accessgovernance.PrivilegedFilter{})
	if err != nil {
		t.Fatalf("PrivilegedActivity (B): %v", err)
	}
	if len(eventsB) != 1 {
		t.Fatalf("tenant B privileged activity = %d events, want 1", len(eventsB))
	}
}
