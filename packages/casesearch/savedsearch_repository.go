package casesearch

import (
	"context"

	"github.com/google/uuid"
)

// SavedSearchRepository persists SavedSearch records, scoped to a tenant
// on every call, mirroring packages/signoff.Repository's and
// packages/caselifecycle.Repository's convention exactly.
// Implementations must refuse (via ErrCrossTenantAccess) to operate on a
// SavedSearch whose TenantID does not match the tenantID argument.
//
// Two implementations are provided: InMemoryRepository (tests and other
// packages' fixtures) and PostgresRepository/TenantScopedRepository
// (backed by the `saved_searches` table — see
// packages/persistence/migrations/000010_create_saved_searches.up.sql).
type SavedSearchRepository interface {
	// Create inserts s. Returns ErrInvalidCase-equivalent validation
	// errors from s.Validate(), and ErrCrossTenantAccess if s.TenantID
	// does not match tenantID.
	Create(ctx context.Context, tenantID uuid.UUID, s *SavedSearch) error

	// Get returns the saved search with the given id, scoped to
	// tenantID. Returns ErrNotFound if no such saved search is visible
	// to tenantID.
	Get(ctx context.Context, tenantID, id uuid.UUID) (*SavedSearch, error)

	// ListByOwner returns every saved search owned by ownerID within
	// tenantID, ordered by CreatedAt descending (most recent first).
	ListByOwner(ctx context.Context, tenantID, ownerID uuid.UUID) ([]*SavedSearch, error)

	// Delete removes the saved search identified by id, scoped to
	// tenantID. Returns ErrNotFound if no such saved search is visible
	// to tenantID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}
