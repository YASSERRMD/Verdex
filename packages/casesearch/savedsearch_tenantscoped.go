package casesearch

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedRepository composes packages/tenancy.WithTenantScope with
// PostgresRepository, exactly as packages/signoff.TenantScopedRepository
// does: every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to PostgresRepository's own application-level requireMatchingTenant
// guard.
//
// This is the type production code should use against a live
// *pgxpool.Pool. PostgresRepository remains available directly for
// callers that already hold an open persistence.Executor.
type TenantScopedRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedRepository builds a TenantScopedRepository backed by
// pool.
func NewTenantScopedRepository(pool *pgxpool.Pool) *TenantScopedRepository {
	return &TenantScopedRepository{pool: pool}
}

// Create implements SavedSearchRepository.
func (r *TenantScopedRepository) Create(ctx context.Context, tenantID uuid.UUID, s *SavedSearch) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Create(ctx, tenantID, s)
	})
}

// Get implements SavedSearchRepository.
func (r *TenantScopedRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*SavedSearch, error) {
	var out *SavedSearch
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		s, err := NewPostgresRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = s
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListByOwner implements SavedSearchRepository.
func (r *TenantScopedRepository) ListByOwner(ctx context.Context, tenantID, ownerID uuid.UUID) ([]*SavedSearch, error) {
	var out []*SavedSearch
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRepository(exec).ListByOwner(ctx, tenantID, ownerID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Delete implements SavedSearchRepository.
func (r *TenantScopedRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Delete(ctx, tenantID, id)
	})
}

var _ SavedSearchRepository = (*TenantScopedRepository)(nil)
