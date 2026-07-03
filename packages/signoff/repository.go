package signoff

import (
	"context"

	"github.com/google/uuid"
)

// Repository persists SignoffRecord state and its AuditEntry trail,
// scoped to a tenant on every call, mirroring
// packages/caselifecycle.Repository's shape exactly. Implementations
// must refuse (via ErrCrossTenantAccess) to operate on a record whose
// TenantID does not match the tenantID argument, before touching
// storage.
//
// Two implementations are provided: PostgresRepository (backed by the
// `signoff_records` and `signoff_audit_entries` tables — see
// packages/persistence/migrations/000008_create_signoff.up.sql) and
// InMemoryRepository (for tests and other packages' fixtures).
type Repository interface {
	// Get returns the current SignoffRecord for caseID, scoped to
	// tenantID. Returns ErrNotFound if no record exists yet (a case
	// that has never entered the sign-off workflow) or is not visible
	// to tenantID.
	Get(ctx context.Context, tenantID, caseID uuid.UUID) (*SignoffRecord, error)

	// Upsert creates or overwrites the current SignoffRecord for
	// r.CaseID, scoped to tenantID. Exactly one current record exists
	// per case; callers that need history append an AuditEntry
	// alongside calling this (see AppendAudit).
	Upsert(ctx context.Context, tenantID uuid.UUID, r *SignoffRecord) error

	// AppendAudit persists e as part of the case's immutable sign-off
	// audit trail. Implementations must treat this as append-only.
	AppendAudit(ctx context.Context, tenantID uuid.UUID, e *AuditEntry) error

	// ListAudit returns every AuditEntry for caseID, scoped to
	// tenantID, ordered by OccurredAt ascending (oldest first).
	ListAudit(ctx context.Context, tenantID, caseID uuid.UUID) ([]*AuditEntry, error)
}

// CaseVersionReader is the minimal read-only view this package needs
// of packages/caselifecycle.Case in order to detect "the case's
// content changed after approval": the case's current
// MetadataVersion. This package deliberately does not import
// packages/caselifecycle.Repository directly (a narrower interface
// keeps this package's dependency surface minimal and makes it easy
// to fake in tests), but packages/caselifecycle.Repository.Get
// trivially satisfies the shape a caller needs to adapt into this
// interface — see doc/signoff-workflow.md for the adapter example.
type CaseVersionReader interface {
	// CaseVersion returns the current MetadataVersion for caseID,
	// scoped to tenantID. Implementations should return
	// caselifecycle.ErrNotFound (or an equivalent not-found error) if
	// the case does not exist.
	CaseVersion(ctx context.Context, tenantID, caseID uuid.UUID) (int, error)
}
