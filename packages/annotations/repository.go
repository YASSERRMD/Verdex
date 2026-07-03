package annotations

import (
	"context"

	"github.com/google/uuid"
)

// AnchorFilter narrows ListByCase to annotations attached to a specific
// anchor. The zero value (Type == "") means "no anchor filter — return
// every annotation for the case regardless of anchor".
type AnchorFilter struct {
	// Type, if non-empty, restricts results to this AnchorType.
	Type AnchorType

	// ID, if non-empty, further restricts results to this AnchorID
	// within Type. Ignored if Type is empty. Leaving ID empty while
	// Type is set returns every annotation of that anchor type,
	// regardless of which specific node/segment it is attached to.
	ID string
}

// Repository persists Annotation records and their derived Mention and
// AuditRecord logs, scoped to a tenant on every call, mirroring
// packages/casesearch.SavedSearchRepository's and
// packages/caselifecycle.Repository's convention exactly.
// Implementations must refuse (via ErrCrossTenantAccess) to operate on
// an Annotation whose TenantID does not match the tenantID argument.
//
// Two implementations are provided: InMemoryRepository (tests and
// other packages' fixtures) and PostgresRepository/
// TenantScopedRepository (backed by the `annotations`,
// `annotation_mentions`, and `annotation_audit_events` tables — see
// packages/persistence/migrations/000012_create_annotations.up.sql).
type Repository interface {
	// Create inserts a. a.ID is generated if zero. Returns validation
	// errors from a.Validate(), ErrCrossTenantAccess if a.TenantID does
	// not match tenantID, and ErrParentNotFound/ErrParentIsReply if
	// a.ParentID is set but does not reference a valid thread root
	// visible to tenantID within a.CaseID.
	Create(ctx context.Context, tenantID uuid.UUID, a *Annotation) error

	// Get returns the annotation with the given id, scoped to tenantID.
	// Returns ErrNotFound if no such annotation is visible to
	// tenantID.
	Get(ctx context.Context, tenantID, id uuid.UUID) (*Annotation, error)

	// ListByCase returns every annotation for caseID visible to
	// tenantID, optionally narrowed by filter, ordered by CreatedAt
	// ascending (oldest first). Both thread roots and replies are
	// included; callers that want threaded grouping should call
	// Thread per root, or group client-side via Annotation.RootID.
	ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, filter AnchorFilter) ([]*Annotation, error)

	// Thread returns rootID's annotation followed by every reply to it
	// (ParentID == rootID), ordered by CreatedAt ascending. Returns
	// ErrNotFound if rootID does not identify a visible annotation.
	Thread(ctx context.Context, tenantID, rootID uuid.UUID) ([]*Annotation, error)

	// UpdateBody overwrites the Body of the annotation identified by
	// id, scoped to tenantID, re-deriving and persisting any Mentions
	// implied by the new Body. Returns ErrNotFound if no such
	// annotation is visible to tenantID.
	UpdateBody(ctx context.Context, tenantID, id uuid.UUID, body string) (*Annotation, error)

	// Delete removes the annotation identified by id, scoped to
	// tenantID. Deleting a thread root also deletes its replies
	// (cascade), mirroring the FK ON DELETE CASCADE in the migration.
	// Returns ErrNotFound if no such annotation is visible to
	// tenantID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error

	// Resolve marks the annotation identified by id as resolved by
	// resolvedBy. Returns ErrNotFound if no such annotation is visible
	// to tenantID, and ErrAlreadyResolved if it is already resolved.
	Resolve(ctx context.Context, tenantID, id, resolvedBy uuid.UUID) (*Annotation, error)

	// Reopen clears the resolved state of the annotation identified by
	// id. Returns ErrNotFound if no such annotation is visible to
	// tenantID, and ErrNotResolved if it is not currently resolved.
	Reopen(ctx context.Context, tenantID, id uuid.UUID) (*Annotation, error)

	// MentionsFor returns every Mention recorded for userID within
	// tenantID, ordered by CreatedAt descending (most recent first) —
	// the real, queryable hook Phase 072's notification system will
	// read from, per this package's Mention/MentionSink split.
	MentionsFor(ctx context.Context, tenantID, userID uuid.UUID) ([]Mention, error)

	// AppendAudit persists rec as part of the tenant's annotation audit
	// log. Implementations should treat this as append-only.
	AppendAudit(ctx context.Context, tenantID uuid.UUID, rec *AuditRecord) error

	// ListAudit returns every AuditRecord for annotationID, scoped to
	// tenantID, ordered by OccurredAt ascending (oldest first).
	ListAudit(ctx context.Context, tenantID, annotationID uuid.UUID) ([]*AuditRecord, error)
}
