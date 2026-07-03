package auditlog

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedRepository composes packages/tenancy.WithTenantScope
// with PostgresRepository, exactly as
// packages/keymanagement.TenantScopedRepository does: every method
// opens its own RLS-scoped transaction, so Row-Level Security enforces
// tenant isolation at the database layer in addition to
// PostgresRepository's own application-level cross-tenant guard.
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

// Append implements Repository.
func (r *TenantScopedRepository) Append(ctx context.Context, tenantID uuid.UUID, event *Event) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Append(ctx, tenantID, event)
	})
}

// Last implements Repository.
func (r *TenantScopedRepository) Last(ctx context.Context, tenantID uuid.UUID) (*Event, error) {
	var out *Event
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		e, err := NewPostgresRepository(exec).Last(ctx, tenantID)
		if err != nil {
			return err
		}
		out = e
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListAll implements Repository.
func (r *TenantScopedRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Event, error) {
	var out []Event
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRepository(exec).ListAll(ctx, tenantID)
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

// Query implements Repository.
func (r *TenantScopedRepository) Query(ctx context.Context, tenantID uuid.UUID, filter Filter) ([]Event, error) {
	var out []Event
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRepository(exec).Query(ctx, tenantID, filter)
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

// PurgeBefore implements Repository.
func (r *TenantScopedRepository) PurgeBefore(ctx context.Context, tenantID uuid.UUID, cutoff time.Time) (int, error) {
	var out int
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		n, err := NewPostgresRepository(exec).PurgeBefore(ctx, tenantID, cutoff)
		if err != nil {
			return err
		}
		out = n
		return nil
	})
	if err != nil {
		return 0, err
	}
	return out, nil
}

var _ Repository = (*TenantScopedRepository)(nil)
