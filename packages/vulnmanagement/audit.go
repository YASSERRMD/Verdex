package vulnmanagement

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/compliance/audit.go and packages/threatmodel/audit.go.
const (
	auditActionFindingRecord = "vulnmanagement.finding_record"
	auditActionTriage        = "vulnmanagement.triage"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/compliance and packages/threatmodel's
// systemActor idiom.
const systemActor = "system:vulnmanagement"

// AuditSink records every finding recorded and every triage decision
// via packages/auditlog.Store, reusing the existing hash-chained,
// queryable audit trail rather than a second table -- exactly the
// composition pattern packages/compliance's and
// packages/threatmodel's own AuditSink established. No second audit
// table.
type AuditSink struct {
	store *auditlog.Store
}

// NewAuditSink builds an AuditSink backed by store. Returns
// ErrNilAuditSink if store is nil.
func NewAuditSink(store *auditlog.Store) (*AuditSink, error) {
	if store == nil {
		return nil, ErrNilAuditSink
	}
	return &AuditSink{store: store}, nil
}

// actorFor returns actorUserID.String() if non-nil, else systemActor.
func actorFor(actorUserID uuid.UUID) string {
	if actorUserID == uuid.Nil {
		return systemActor
	}
	return actorUserID.String()
}

// actorFromCtx resolves the actor's user ID from ctx if present,
// falling back to uuid.Nil (which actorFor renders as systemActor) when
// ctx carries no authenticated user -- used by the audit-on-failure
// paths, which must still record an event even when authorizeManage
// itself failed (e.g. ErrUnauthenticated).
func actorFromCtx(ctx context.Context) uuid.UUID {
	user, err := authorizeActor(ctx)
	if err != nil {
		return uuid.Nil
	}
	return user.ID
}

// RecordFindingRecord appends an Event describing a Finding being
// recorded into a tenant's finding set.
func (s *AuditSink) RecordFindingRecord(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, f Finding, recordErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "recorded"
	detail := fmt.Sprintf("source=%s package=%s severity=%s advisory=%s", f.Source, f.Package, f.Severity, f.AdvisoryID)
	if recordErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, recordErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionFindingRecord
	ev.Target = f.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordFindingRecord", err)
	}
	return appended, nil
}

// RecordTriage appends an Event describing a TriageDecision made
// against a Finding: who decided, what they decided, and why.
func (s *AuditSink) RecordTriage(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, d TriageDecision, triageErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "decided"
	detail := fmt.Sprintf("finding=%s from=%s to=%s notes=%s", d.FindingID, d.FromStatus, d.ToStatus, d.Notes)
	if triageErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, triageErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionTriage
	ev.Target = d.FindingID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordTriage", err)
	}
	return appended, nil
}

// FindingActivity surfaces every vulnerability-management audit event
// for tenantID matching filter, queried through
// packages/auditlog.Store's own PermAuditRead-gated Query -- this
// package never bypasses that authorization check.
func (s *AuditSink) FindingActivity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("FindingActivity", err)
	}
	return events, nil
}
