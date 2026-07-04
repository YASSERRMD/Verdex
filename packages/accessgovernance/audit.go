package accessgovernance

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/dataresidency.AuditSink / packages/keymanagement's audit.go.
const (
	auditActionEvaluate = "access_governance.evaluate"
	auditActionElevate  = "access_governance.elevate"
	auditActionAttest   = "access_governance.attest"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the evaluating/eleveating/attesting caller,
// mirroring packages/dataresidency.AuditSink's "system:<pkg>" idiom
// for the small number of calls with no resolvable ctx actor.
const systemActor = "system:accessgovernance"

// AuditSink records every Evaluate/Elevate/Attest call via
// packages/auditlog.Store (task 6), reusing the existing hash-chained,
// queryable audit trail rather than a new logging channel. It mirrors
// packages/dataresidency.AuditSink and
// packages/keymanagement's recordAudit helper exactly.
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

// actorOrSystem returns actor.String() if non-nil, else systemActor.
func actorFor(actorUserID uuid.UUID) string {
	if actorUserID == uuid.Nil {
		return systemActor
	}
	return actorUserID.String()
}

// RecordEvaluate appends an Event describing the outcome of an
// Evaluate call.
func (s *AuditSink) RecordEvaluate(ctx context.Context, tenantID uuid.UUID, req Request, dec Decision) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "denied"
	if dec.Allowed() {
		outcome = "allowed"
	}
	detail := fmt.Sprintf("action=%s reason=%s", req.Action, dec.Reason)
	if req.CaseID != uuid.Nil {
		detail = fmt.Sprintf("%s case=%s", detail, req.CaseID)
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataAccess,
		CaseID:   req.CaseID,
		Detail:   detail,
	}
	ev.Actor = actorFor(req.ActorUserID)
	ev.Action = auditActionEvaluate
	ev.Target = string(req.Action)
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordEvaluate", err)
	}
	return appended, nil
}

// RecordElevate appends an Event describing an Elevate call --
// success or failure -- with the grant's justification attached,
// satisfying the "heavily audited" requirement this package inherits
// from packages/keymanagement's break-glass procedure.
func (s *AuditSink) RecordElevate(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, action Action, justification string, elevateErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "granted"
	detail := fmt.Sprintf("action=%s justification=%s", action, justification)
	if elevateErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, elevateErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionElevate
	ev.Target = string(action)
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordElevate", err)
	}
	return appended, nil
}

// RecordAttest appends an Event describing an Attest call: which
// review, who attested, and the decision reached.
func (s *AuditSink) RecordAttest(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, reviewID uuid.UUID, decision AttestationDecision, attestErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "recorded"
	detail := fmt.Sprintf("review=%s decision=%s", reviewID, decision)
	if attestErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, attestErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionAttest
	ev.Target = reviewID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordAttest", err)
	}
	return appended, nil
}

// PrivilegedActivity surfaces elevated/break-glass-style access events
// specifically (task 6): every access_governance.elevate event for
// tenantID matching filter, queried through packages/auditlog.Store's
// own PermAuditRead-gated Query -- this package never bypasses that
// authorization check.
func (s *AuditSink) PrivilegedActivity(ctx context.Context, tenantID uuid.UUID, filter PrivilegedFilter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, auditlog.Filter{
		Actor:  filter.Actor,
		CaseID: filter.CaseID,
		Action: auditActionElevate,
		Since:  filter.Since,
		Until:  filter.Until,
		Limit:  filter.Limit,
	})
	if err != nil {
		return nil, wrapf("PrivilegedActivity", err)
	}
	return events, nil
}
