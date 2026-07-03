package keymanagement

import (
	"context"

	"github.com/google/uuid"
)

// Filter narrows ListForTenant to a subset of a tenant's key
// versions.
type Filter struct {
	// State, if non-empty, restricts results to this KeyState.
	State KeyState

	// Limit caps the number of results returned. Zero means "no
	// limit".
	Limit int
}

// Repository persists KeyMetadata rows, scoped to a tenant on every
// call, mirroring packages/notifications.Repository's and
// packages/caseversioning's convention exactly. Implementations must
// refuse (via ErrCrossTenantAccess) to operate on a KeyMetadata whose
// TenantID does not match the tenantID argument.
//
// Repository never persists key material — see types.go, "Metadata
// vs. material". Three implementations are provided: InMemoryRepository
// (tests and other packages' fixtures), PostgresRepository (backed by
// the `key_metadata` table — see
// packages/persistence/migrations/000018_create_keymanagement.up.sql),
// and TenantScopedRepository, which wraps PostgresRepository with
// packages/tenancy.WithTenantScope for RLS-enforced isolation.
type Repository interface {
	// Create inserts m. Returns validation errors from m.Validate()
	// and ErrCrossTenantAccess if m.TenantID does not match tenantID.
	Create(ctx context.Context, tenantID uuid.UUID, m *KeyMetadata) error

	// Get returns the key version identified by id, scoped to
	// tenantID. Returns ErrNotFound if no such key is visible to
	// tenantID.
	Get(ctx context.Context, tenantID uuid.UUID, id string) (*KeyMetadata, error)

	// GetActive returns the tenant's current Active key version.
	// Returns ErrNoActiveKey if no key is in the Active state for
	// tenantID — a fail-closed condition, never an implicit fallback.
	GetActive(ctx context.Context, tenantID uuid.UUID) (*KeyMetadata, error)

	// ListForTenant returns every key version belonging to tenantID,
	// optionally narrowed by filter, ordered by Version descending
	// (newest first).
	ListForTenant(ctx context.Context, tenantID uuid.UUID, filter Filter) ([]*KeyMetadata, error)

	// UpdateState transitions the key version identified by id to
	// newState, scoped to tenantID. Returns ErrNotFound if no such key
	// is visible to tenantID.
	UpdateState(ctx context.Context, tenantID uuid.UUID, id string, newState KeyState) error

	// MaxVersion returns the highest Version recorded for tenantID
	// across all key states, or 0 if the tenant has no key versions
	// yet. Rotate uses this to assign the next sequential Version.
	MaxVersion(ctx context.Context, tenantID uuid.UUID) (int, error)
}

// AuditRepository persists AuditEntry rows, scoped to a tenant on
// every call, mirroring Repository's convention. See audit.go for
// AuditEntry itself.
type AuditRepository interface {
	// Record inserts entry.
	Record(ctx context.Context, tenantID uuid.UUID, entry *AuditEntry) error

	// ListForTenant returns every AuditEntry recorded for tenantID,
	// ordered by OccurredAt descending (newest first), optionally
	// capped at limit (0 means "no limit").
	ListForTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]*AuditEntry, error)
}
