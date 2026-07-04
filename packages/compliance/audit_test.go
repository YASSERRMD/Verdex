package compliance_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/compliance"
)

// TestEngine_RegisterControl_RecordsAuditEvent proves task 7's audit
// composition: registering a control appends an Event to the shared
// packages/auditlog.Store, not a second table.
func TestEngine_RegisterControl_RecordsAuditEvent(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	admin := adminUser(tenantID)

	if _, err := engine.RegisterControl(ctxWithUser(admin), validControl()); err != nil {
		t.Fatalf("RegisterControl: %v", err)
	}

	events, err := auditStore.Query(ctxWithUser(auditorUser(tenantID)), tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("auditStore.Query: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Outcome != "registered" {
		t.Fatalf("events[0].Outcome = %q, want %q", events[0].Outcome, "registered")
	}
}

// TestEngine_RecordEvidence_RecordsAuditEventEvenOnFailure proves the
// "audited regardless of outcome" discipline this package inherits
// from packages/privacy and packages/accessgovernance: a denied
// RecordEvidence call (missing permission) is still recorded.
func TestEngine_RecordEvidence_RecordsAuditEventEvenOnFailure(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	control := registerTestControl(t, engine, tenantID)
	auditor := auditorUser(tenantID)

	_, err := engine.RecordEvidence(ctxWithUser(auditor), tenantID, validEvidence(tenantID, control.ID))
	if err == nil {
		t.Fatal("RecordEvidence() with auditor role = nil error, want ErrForbidden")
	}

	events, err := auditStore.Query(ctxWithUser(adminUser(tenantID)), tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("auditStore.Query: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Outcome == "denied" {
			found = true
		}
	}
	if !found {
		t.Fatalf("events = %v, want at least one denied RecordEvidence event", events)
	}
}

// TestEngine_SetProfile_RecordsAuditEvent proves task 7's profile
// changes are also recorded via the shared audit trail.
func TestEngine_SetProfile_RecordsAuditEvent(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	admin := adminUser(tenantID)

	if _, err := engine.SetProfile(ctxWithUser(admin), tenantID, compliance.Profile{
		Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection},
	}); err != nil {
		t.Fatalf("SetProfile: %v", err)
	}

	events, err := auditStore.Query(ctxWithUser(auditorUser(tenantID)), tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("auditStore.Query: %v", err)
	}
	if len(events) != 1 || events[0].Outcome != "set" {
		t.Fatalf("events = %v, want exactly one 'set' event", events)
	}
}

// TestAuditSink_ComplianceActivity proves the compliance dashboard's
// read-side wrapper around packages/auditlog.Store.Query surfaces the
// same events RegisterControl/SetProfile append -- the read-side
// counterpart a compliance dashboard would use to show recent
// catalogue/evidence/profile activity.
func TestAuditSink_ComplianceActivity(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	admin := adminUser(tenantID)

	if _, err := engine.SetProfile(ctxWithUser(admin), tenantID, compliance.Profile{
		Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection},
	}); err != nil {
		t.Fatalf("SetProfile: %v", err)
	}

	sink, err := compliance.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("NewAuditSink: %v", err)
	}
	events, err := sink.ComplianceActivity(ctxWithUser(auditorUser(tenantID)), tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("ComplianceActivity: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
}

// TestAuditSink_ComplianceActivity_NilSink proves the nil-receiver
// guard surfaces ErrNilAuditSink rather than panicking.
func TestAuditSink_ComplianceActivity_NilSink(t *testing.T) {
	t.Parallel()
	var sink *compliance.AuditSink
	if _, err := sink.ComplianceActivity(t.Context(), uuid.New(), auditlog.Filter{}); err == nil {
		t.Fatal("ComplianceActivity() on nil sink = nil error, want ErrNilAuditSink")
	}
}
