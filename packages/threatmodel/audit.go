package threatmodel

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/privacy/audit.go and packages/compliance/audit.go.
const auditActionMitigationTransition = "threatmodel.mitigation_transition"

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/privacy and packages/compliance's
// systemActor idiom.
const systemActor = "system:threatmodel"

// AuditSink records every mitigation status transition via
// packages/auditlog.Store, reusing the existing hash-chained,
// queryable audit trail rather than a second table -- exactly the
// composition pattern packages/privacy's and packages/compliance's own
// AuditSink established. See doc.go's persistence discussion for why
// this is the only durable record this package keeps: the threat
// catalogue itself is versioned-in-code data, but a mitigation's
// status transition over time is a genuine operational fact worth a
// durable, auditable history.
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

// RecordMitigationTransition appends an Event describing a
// Mitigation's status being transitioned. tenantID identifies the
// tenant on whose behalf the transition was requested -- a threat
// catalogue entry is shared reference data (see doc.go), but *who
// requested this status change, on behalf of which tenant's
// deployment* is exactly the kind of operational fact this package's
// audit trail exists to capture.
func (s *AuditSink) RecordMitigationTransition(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, mitigationID uuid.UUID, from, to MitigationStatus, transitionErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "transitioned"
	detail := fmt.Sprintf("mitigation=%s from=%s to=%s", mitigationID, from, to)
	if transitionErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, transitionErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionMitigationTransition
	ev.Target = mitigationID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordMitigationTransition", err)
	}
	return appended, nil
}

// MitigationHistory surfaces every mitigation-transition audit event
// for tenantID matching filter, queried through
// packages/auditlog.Store's own PermAuditRead-gated Query -- this
// package never bypasses that authorization check. This is the
// read-side counterpart callers use to reconstruct a mitigation's
// full status-transition history over time.
func (s *AuditSink) MitigationHistory(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("MitigationHistory", err)
	}
	return events, nil
}
