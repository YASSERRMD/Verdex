package caseversioning

import (
	"context"

	"github.com/google/uuid"
)

// SnapshotFilter narrows ListByCase to snapshots of a specific
// ArtifactKind. The zero value (Kind == "") means "no filter — return
// every snapshot for the case regardless of kind".
type SnapshotFilter struct {
	// Kind, if non-empty, restricts results to this ArtifactKind.
	Kind ArtifactKind
}

// Repository persists Snapshot records, scoped to a tenant on every
// call, mirroring packages/annotations.Repository's and
// packages/caselifecycle.Repository's convention exactly.
// Implementations must refuse (via ErrCrossTenantAccess) to operate on a
// Snapshot whose TenantID does not match the tenantID argument.
//
// Two implementations are provided: InMemoryRepository (tests and other
// packages' fixtures) and PostgresRepository/TenantScopedRepository
// (backed by the `case_version_snapshots` table — see
// packages/persistence/migrations/000014_create_case_version_snapshots.up.sql).
type Repository interface {
	// Create inserts s. s.ID is generated if zero, and s.CreatedAt is
	// set if zero. Returns validation errors from s.Validate() and
	// ErrCrossTenantAccess if s.TenantID does not match tenantID.
	Create(ctx context.Context, tenantID uuid.UUID, s *Snapshot) error

	// Get returns the snapshot with the given id, scoped to tenantID.
	// Returns ErrNotFound if no such snapshot is visible to tenantID.
	Get(ctx context.Context, tenantID, id uuid.UUID) (*Snapshot, error)

	// ListByCase returns every snapshot for caseID visible to tenantID,
	// optionally narrowed by filter, ordered by CreatedAt ascending
	// (oldest first) — the chronological version-history timeline.
	ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, filter SnapshotFilter) ([]*Snapshot, error)

	// Latest returns the most recently created snapshot for caseID and
	// kind, visible to tenantID. Returns ErrNotFound if no snapshot of
	// that kind exists yet for the case.
	Latest(ctx context.Context, tenantID, caseID uuid.UUID, kind ArtifactKind) (*Snapshot, error)
}
