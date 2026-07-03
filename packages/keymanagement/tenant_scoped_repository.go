package keymanagement

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedRepository composes packages/tenancy.WithTenantScope
// with PostgresRepository, exactly as
// packages/notifications.TenantScopedRepository does: every method
// opens its own RLS-scoped transaction, so Row-Level Security
// enforces tenant isolation at the database layer in addition to
// PostgresRepository's own application-level requireMatchingTenant
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

// Create implements Repository.
func (r *TenantScopedRepository) Create(ctx context.Context, tenantID uuid.UUID, m *KeyMetadata) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Create(ctx, tenantID, m)
	})
}

// Get implements Repository.
func (r *TenantScopedRepository) Get(ctx context.Context, tenantID uuid.UUID, id string) (*KeyMetadata, error) {
	var out *KeyMetadata
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		m, err := NewPostgresRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetActive implements Repository.
func (r *TenantScopedRepository) GetActive(ctx context.Context, tenantID uuid.UUID) (*KeyMetadata, error) {
	var out *KeyMetadata
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		m, err := NewPostgresRepository(exec).GetActive(ctx, tenantID)
		if err != nil {
			return err
		}
		out = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListForTenant implements Repository.
func (r *TenantScopedRepository) ListForTenant(ctx context.Context, tenantID uuid.UUID, filter Filter) ([]*KeyMetadata, error) {
	var out []*KeyMetadata
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRepository(exec).ListForTenant(ctx, tenantID, filter)
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

// UpdateState implements Repository.
func (r *TenantScopedRepository) UpdateState(ctx context.Context, tenantID uuid.UUID, id string, newState KeyState) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).UpdateState(ctx, tenantID, id, newState)
	})
}

// MaxVersion implements Repository.
func (r *TenantScopedRepository) MaxVersion(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var out int
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		v, err := NewPostgresRepository(exec).MaxVersion(ctx, tenantID)
		if err != nil {
			return err
		}
		out = v
		return nil
	})
	if err != nil {
		return 0, err
	}
	return out, nil
}

var _ Repository = (*TenantScopedRepository)(nil)

// TenantScopedAuditRepository composes packages/tenancy.WithTenantScope
// with PostgresAuditRepository, mirroring TenantScopedRepository
// exactly.
type TenantScopedAuditRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedAuditRepository builds a TenantScopedAuditRepository
// backed by pool.
func NewTenantScopedAuditRepository(pool *pgxpool.Pool) *TenantScopedAuditRepository {
	return &TenantScopedAuditRepository{pool: pool}
}

// Record implements AuditRepository.
func (r *TenantScopedAuditRepository) Record(ctx context.Context, tenantID uuid.UUID, entry *AuditEntry) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresAuditRepository(exec).Record(ctx, tenantID, entry)
	})
}

// ListForTenant implements AuditRepository.
func (r *TenantScopedAuditRepository) ListForTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]*AuditEntry, error) {
	var out []*AuditEntry
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresAuditRepository(exec).ListForTenant(ctx, tenantID, limit)
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

var _ AuditRepository = (*TenantScopedAuditRepository)(nil)
