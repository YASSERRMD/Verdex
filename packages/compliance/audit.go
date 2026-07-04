package compliance

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/privacy/audit.go and packages/accessgovernance/audit.go.
const (
	auditActionControlRegister = "compliance.control_register"
	auditActionEvidenceAdd     = "compliance.evidence_add"
	auditActionProfileSet      = "compliance.profile_set"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/privacy and packages/accessgovernance's
// systemActor idiom.
const systemActor = "system:compliance"

// AuditSink records every control registration, evidence addition, and
// compliance profile change via packages/auditlog.Store, reusing the
// existing hash-chained, queryable audit trail rather than a second
// table -- exactly the composition pattern packages/privacy's and
// packages/accessgovernance's own AuditSink established.
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

// RecordControlRegister appends an Event describing a Control being
// registered into the catalogue. Unlike the other Record* methods,
// this is recorded under systemActor's tenant-agnostic scope
// (uuid.Nil) whenever no concrete tenantID is available, since a
// Control is shared catalogue data rather than a per-tenant record --
// callers that register a control while acting on behalf of a specific
// tenant pass that tenantID for a more useful audit trail.
func (s *AuditSink) RecordControlRegister(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, c Control, registerErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "registered"
	detail := fmt.Sprintf("code=%s framework=%s category=%s", c.Code, c.Framework, c.Category)
	if registerErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, registerErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionControlRegister
	ev.Target = c.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordControlRegister", err)
	}
	return appended, nil
}

// RecordEvidenceAdd appends an Event describing a ControlEvidence
// record being added for tenantID.
func (s *AuditSink) RecordEvidenceAdd(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, ev ControlEvidence, addErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "recorded"
	detail := fmt.Sprintf("control=%s kind=%s reference=%s", ev.ControlID, ev.Kind, ev.Reference)
	if addErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, addErr.Error())
	}

	event := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	event.Actor = actorFor(actorUserID)
	event.Action = auditActionEvidenceAdd
	event.Target = ev.ID.String()
	event.Outcome = outcome

	appended, err := s.store.Append(ctx, event)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordEvidenceAdd", err)
	}
	return appended, nil
}

// RecordProfileSet appends an Event describing a tenant's
// ComplianceProfile being set or replaced.
func (s *AuditSink) RecordProfileSet(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, p ComplianceProfile, setErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "set"
	detail := fmt.Sprintf("frameworks=%d excluded_controls=%d", len(p.Frameworks), len(p.ExcludedControlIDs))
	if setErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, setErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionProfileSet
	ev.Target = tenantID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordProfileSet", err)
	}
	return appended, nil
}

// ComplianceActivity surfaces every compliance-related audit event for
// tenantID matching filter, queried through
// packages/auditlog.Store's own PermAuditRead-gated Query -- this
// package never bypasses that authorization check. This is the
// read-side counterpart the compliance dashboard uses to show recent
// catalogue/evidence/profile activity alongside the gap-analysis
// numbers.
func (s *AuditSink) ComplianceActivity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("ComplianceActivity", err)
	}
	return events, nil
}
