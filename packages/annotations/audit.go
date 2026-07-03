package annotations

import (
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/observability"
)

// AuditVerb is the closed set of actions recorded against an
// annotation, mirroring the "<noun>.<verb>" convention
// observability.AuditEvent.Action documents.
type AuditVerb string

const (
	// AuditCreated is recorded when an annotation is created.
	AuditCreated AuditVerb = "annotation.created"

	// AuditEdited is recorded when an annotation's Body is changed via
	// UpdateBody.
	AuditEdited AuditVerb = "annotation.edited"

	// AuditDeleted is recorded when an annotation is removed via
	// Delete.
	AuditDeleted AuditVerb = "annotation.deleted"

	// AuditResolved is recorded when an annotation is marked resolved
	// via Resolve.
	AuditResolved AuditVerb = "annotation.resolved"

	// AuditReopened is recorded when a resolved annotation is reopened
	// via Reopen.
	AuditReopened AuditVerb = "annotation.reopened"
)

// AuditRecord is one immutable entry in an Annotation's audit log: who
// did what, to which annotation, when. Every Create/UpdateBody/
// Delete/Resolve/Reopen call on a Repository implementation appends
// exactly one AuditRecord, satisfying task 7 ("every create/edit/
// delete/resolve recorded, queryable").
type AuditRecord struct {
	// ID uniquely identifies this audit record.
	ID uuid.UUID `json:"id"`

	// AnnotationID identifies the annotation this record describes.
	// Retained even after the annotation itself is deleted, so the
	// audit trail survives Delete.
	AnnotationID uuid.UUID `json:"annotation_id"`

	// CaseID identifies the case the annotation belonged to.
	CaseID uuid.UUID `json:"case_id"`

	// TenantID is copied from the annotation at the time of the
	// action, so audit queries can be tenant-scoped without a join.
	TenantID uuid.UUID `json:"tenant_id"`

	// Verb identifies which action occurred.
	Verb AuditVerb `json:"verb"`

	// Actor is the ID of the identity.User who performed the action.
	Actor uuid.UUID `json:"actor"`

	// OccurredAt is when the action was recorded.
	OccurredAt time.Time `json:"occurred_at"`
}

// ToAuditEvent projects r into an observability.AuditEvent so
// annotation history flows through the same audit channel as the rest
// of the system, rather than a second, parallel logging path,
// mirroring packages/caselifecycle.TransitionRecord.ToAuditEvent
// exactly.
func (r *AuditRecord) ToAuditEvent() observability.AuditEvent {
	return observability.AuditEvent{
		Time:    r.OccurredAt,
		Actor:   r.Actor.String(),
		Action:  string(r.Verb),
		Target:  r.AnnotationID.String(),
		Outcome: "success",
	}
}

// newAuditRecord builds an AuditRecord for annotation a, verb v, and
// actor, stamped with the current time.
func newAuditRecord(a *Annotation, v AuditVerb, actor uuid.UUID) *AuditRecord {
	return &AuditRecord{
		ID:           uuid.New(),
		AnnotationID: a.ID,
		CaseID:       a.CaseID,
		TenantID:     a.TenantID,
		Verb:         v,
		Actor:        actor,
		OccurredAt:   time.Now().UTC(),
	}
}
