package auditlog

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// defaultQueryLimit caps Query results when Filter.Limit is zero, so a
// forgotten limit cannot accidentally return an unbounded result set.
const defaultQueryLimit = 1000

// Repository persists Event rows, scoped to a tenant on every call,
// mirroring packages/keymanagement.AuditRepository's and
// packages/signoff.Repository's conventions. Implementations must
// never expose an UPDATE or DELETE path for individual events —
// Purge is the only bulk-deletion operation, and it is bounded by a
// RetentionPolicy cutoff (task 6), never arbitrary.
//
// Three implementations are provided: InMemoryRepository (tests and
// other packages' fixtures), PostgresRepository (backed by the
// `audit_events` table — see
// packages/persistence/migrations/000020_create_auditlog.up.sql),
// and TenantScopedRepository, which wraps PostgresRepository with
// packages/tenancy.WithTenantScope for RLS-enforced isolation.
type Repository interface {
	// Append inserts event. Callers are expected to have already
	// populated PrevHash/ChainHash (see chain.go / Store.Append in
	// store.go, which does this automatically). Returns
	// ErrCrossTenantAccess if event.TenantID does not match tenantID.
	Append(ctx context.Context, tenantID uuid.UUID, event *Event) error

	// Query returns events for tenantID matching filter, ordered by
	// Time ascending (chain order) so VerifyChain can be run directly
	// on the result.
	Query(ctx context.Context, tenantID uuid.UUID, filter Filter) ([]Event, error)

	// ListAll returns every event for tenantID in chain order (Time
	// ascending, ID as a tiebreaker), with no filter applied. Used by
	// VerifyChain (full-chain integrity checks) and Purge (to find the
	// tail eligible for deletion).
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]Event, error)

	// Last returns the most recently appended event for tenantID (by
	// chain order, i.e. the current chain tail), or (nil, ErrNotFound-
	// equivalent handled by the caller as a nil PrevHash) if the tenant
	// has no events yet. Store.Append uses this to link a new event to
	// the existing chain without holding ListAll's full result in
	// memory on every append.
	Last(ctx context.Context, tenantID uuid.UUID) (*Event, error)

	// PurgeBefore deletes every event for tenantID with Time strictly
	// before cutoff, returning the number of rows removed. This is the
	// only deletion path Repository exposes (task 6); callers must go
	// through RetentionPolicy-driven Purge (see retention.go), never
	// call PurgeBefore with an arbitrary cutoff computed elsewhere.
	PurgeBefore(ctx context.Context, tenantID uuid.UUID, cutoff time.Time) (int, error)
}
