package caselifecycle

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/observability"
)

// auditAction is the stable verb-phrase recorded on every
// observability.AuditEvent produced from a TransitionRecord, mirroring
// the "<noun>.<verb>" convention observability.AuditEvent.Action
// documents (e.g. "case.viewed", "ruling.exported").
const auditAction = "case.transitioned"

// ToAuditEvent projects r into an observability.AuditEvent so
// transition history can flow through the same audit channel as the
// rest of the system, rather than a second, parallel logging path.
// Actor and Target are stringified UUIDs; Outcome is always "success"
// since only successful transitions ever produce a TransitionRecord.
func (r *TransitionRecord) ToAuditEvent() observability.AuditEvent {
	return observability.AuditEvent{
		Time:    r.OccurredAt,
		Actor:   r.Actor.String(),
		Action:  auditAction,
		Target:  r.CaseID.String(),
		Outcome: "success",
	}
}

// History returns the full transition audit log for caseID, scoped to
// tenantID, ordered oldest-first, exactly as
// Repository.ListTransitions does. This wrapper exists so callers who
// already have a Repository in hand do not need to know the
// underlying method name is ListTransitions specifically — it is kept
// as a thin, stable read entrypoint for the audit log described in
// doc.go.
func History(ctx context.Context, repo Repository, tenantID, caseID uuid.UUID) ([]*TransitionRecord, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	return repo.ListTransitions(ctx, tenantID, caseID)
}
