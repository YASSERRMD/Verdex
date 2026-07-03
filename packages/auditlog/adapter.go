package auditlog

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/signoff"
)

// signoffAuditAction is the stable verb-phrase recorded for every
// Event projected from a signoff.AuditEntry, following the
// "<noun>.<verb>" convention observability.AuditEvent.Action
// documents and packages/caselifecycle.auditAction already uses for
// "case.transitioned".
const signoffAuditAction = "case.signoff"

// FromSignoffEntry projects a packages/signoff.AuditEntry (task 3:
// reasoning/sign-off decisions — approvals, rejections, and automatic
// re-review reversions) into an Event, without requiring any change to
// packages/signoff itself. Actor is the empty string (rather than
// "00000000-0000-0000-0000-000000000000") when e.Actor is uuid.Nil —
// i.e. a system-triggered re-review entry — since Event.Validate
// requires a non-blank Actor and packages/signoff.DecisionSourceReReview
// entries are attributed to the system, not a human.
func FromSignoffEntry(e *signoff.AuditEntry) Event {
	if e == nil {
		return Event{}
	}

	actor := e.Actor.String()
	if e.Actor.String() == "00000000-0000-0000-0000-000000000000" {
		actor = "system:" + string(e.Source)
	}

	ev := Event{
		TenantID: e.TenantID,
		CaseID:   e.CaseID,
		Kind:     KindSignoff,
		Detail:   e.Notes,
	}
	ev.Time = e.OccurredAt
	ev.Actor = actor
	ev.Action = signoffAuditAction
	ev.Target = e.ToStatus.String()
	ev.Outcome = "success"
	return ev
}

// SignoffAuditSink implements packages/signoff's audit-append pattern
// as a thin wrapper: any code path that already has both a
// signoff.Repository and an auditlog.Store can call RecordSignoffEntry
// after (or in place of, if only durable audit history is needed)
// calling repo.AppendAudit, so sign-off decisions and re-review
// triggers flow into the centralized, hash-chained, queryable audit
// trail — the concrete, tested proof (task 3, task 9) that this
// package is a real durable sink packages/signoff can write into, not
// a parallel unused system.
type SignoffAuditSink struct {
	store *Store
}

// NewSignoffAuditSink builds a SignoffAuditSink backed by store.
// Returns ErrNilRepository if store is nil.
func NewSignoffAuditSink(store *Store) (*SignoffAuditSink, error) {
	if store == nil {
		return nil, ErrNilRepository
	}
	return &SignoffAuditSink{store: store}, nil
}

// RecordSignoffEntry projects entry via FromSignoffEntry and appends
// it to the underlying Store. Store.Append itself carries no
// identity.PermAuditRead gate — only the read paths (Query, Export,
// Purge, VerifyTenantChain) do — since the sign-off Service has
// already authorized the underlying decision via identity.PermSignOff
// before AppendAudit was ever called; requiring audit-read permission
// just to record an event would be backwards.
func (s *SignoffAuditSink) RecordSignoffEntry(ctx context.Context, entry *signoff.AuditEntry) (Event, error) {
	if entry == nil {
		return Event{}, wrapf("RecordSignoffEntry", ErrNilEvent)
	}
	return s.store.Append(ctx, FromSignoffEntry(entry))
}
