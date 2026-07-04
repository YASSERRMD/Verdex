package localization

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedPreferenceRepository composes
// packages/tenancy.WithTenantScope with PostgresPreferenceRepository,
// exactly as packages/privacy.TenantScopedInventoryRepository does:
// every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to PostgresPreferenceRepository's own application-level
// requireMatchingTenant guard.
type TenantScopedPreferenceRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedPreferenceRepository builds a
// TenantScopedPreferenceRepository backed by pool.
func NewTenantScopedPreferenceRepository(pool *pgxpool.Pool) *TenantScopedPreferenceRepository {
	return &TenantScopedPreferenceRepository{pool: pool}
}

// Upsert implements PreferenceRepository.
func (r *TenantScopedPreferenceRepository) Upsert(ctx context.Context, tenantID uuid.UUID, p *Preference) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresPreferenceRepository(exec).Upsert(ctx, tenantID, p)
	})
}

// Get implements PreferenceRepository.
func (r *TenantScopedPreferenceRepository) Get(ctx context.Context, tenantID, userID uuid.UUID) (*Preference, error) {
	var out *Preference
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		p, err := NewPostgresPreferenceRepository(exec).Get(ctx, tenantID, userID)
		if err != nil {
			return err
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Delete implements PreferenceRepository.
func (r *TenantScopedPreferenceRepository) Delete(ctx context.Context, tenantID, userID uuid.UUID) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresPreferenceRepository(exec).Delete(ctx, tenantID, userID)
	})
}

var _ PreferenceRepository = (*TenantScopedPreferenceRepository)(nil)
